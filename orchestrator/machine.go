// Package orchestrator owns validation-plan sequencing and terminal execution
// decisions. It never parses invoice XML or manufactures validation evidence.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Machine advances exact validation execution generations under JetStream KV
// compare-and-swap and drains their durable StageRequest outbox intents.
type Machine struct {
	Queue                   *queue.Client
	Now                     func() time.Time
	PublishStageRequest     func(context.Context, queue.StageRequest) error
	PublishValidationResult func(context.Context, queue.ValidationResult) error
}

func New(client *queue.Client) *Machine {
	machine := &Machine{Queue: client, Now: time.Now}
	if client != nil {
		machine.PublishStageRequest = client.PublishStageRequest
		machine.PublishValidationResult = client.PublishResult
	}
	return machine
}

// Start creates (or loads after command redelivery) a generation and ensures
// its first authorized stage request is published.
func (machine *Machine) Start(ctx context.Context, intent queue.ValidationExecutionIntent) (queue.VersionedValidationExecution, error) {
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	execution, err := queue.NewValidationExecution(intent)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if err := machine.validatePlanTopology(execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	ctx, span := startExecutionSpan(ctx, "orchestrator.start", execution)
	defer span.End()
	if _, err := machine.Queue.CreateValidationExecution(ctx, execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current, err := machine.Queue.GetValidationExecutionVersion(ctx, execution.UploadID, execution.ValidationID, execution.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	return machine.EnsureDispatch(ctx, current)
}

// ReplayStage authorizes an exhausted operational stage in a new generation,
// carrying only verified passing prerequisite evidence. The old generation
// and its permanent processing-failed result remain immutable.
func (machine *Machine) ReplayStage(ctx context.Context, dead queue.StageDeadLetter) (queue.VersionedValidationExecution, error) {
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	stored, err := machine.Queue.GetStageDeadLetter(ctx, dead.Stage, dead.DeadLetterID)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if stored != dead {
		return queue.VersionedValidationExecution{}, errors.New("stage replay dead letter does not match the permanent broker record")
	}
	previous, err := machine.Queue.GetValidationExecution(ctx, dead.UploadID, dead.ValidationID, dead.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	next, err := queue.NextFailedStageReplayGeneration(previous, dead, machine.now())
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if err := machine.validatePlanTopology(next); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if _, err := machine.Queue.CreateValidationExecution(ctx, next); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current, err := machine.Queue.GetValidationExecutionVersion(ctx, next.UploadID, next.ValidationID, next.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current, err = machine.EnsureDispatch(ctx, current)
	if err != nil {
		return current, err
	}
	if err := machine.Queue.DeleteStageDeadLetter(ctx, dead.Stage, dead.DeadLetterID); err != nil {
		return current, fmt.Errorf("delete replayed stage dead letter %s: %w", dead.DeadLetterID, err)
	}
	return current, nil
}

// EnsureDispatch authorizes the first stage from pending state or republishes
// the deterministic request for an already queued state. It never skips or
// selects a stage outside the frozen plan.
func (machine *Machine) EnsureDispatch(ctx context.Context, current queue.VersionedValidationExecution) (queue.VersionedValidationExecution, error) {
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	latest, err := machine.Queue.GetValidationExecutionVersion(ctx, current.Execution.UploadID, current.Execution.ValidationID, current.Execution.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current = latest
	execution := cloneExecution(latest.Execution)
	if err := machine.validatePlanTopology(execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	switch execution.Orchestration.Status {
	case queue.ExecutionStatusPending:
		request, err := queue.NewStageRequest(execution, 1, 1, machine.now())
		if err != nil {
			return queue.VersionedValidationExecution{}, err
		}
		execution.Orchestration = queue.ValidationOrchestrationState{
			Status: queue.ExecutionStatusStageQueued, CurrentStageOrdinal: 1,
			Stages: []queue.ValidationStageState{{
				Stage: execution.Plan.Stages[0].ID, Ordinal: 1,
				AuthorizedAttempt: 1, RequestID: request.RequestID, RequestCreatedAt: request.CreatedAt,
				DispatchStatus: queue.StageDispatchPending,
			}},
			UpdatedAt: machine.now().UTC().Format(time.RFC3339Nano),
		}
		revision, err := machine.Queue.UpdateValidationExecutionState(ctx, current, execution)
		if err != nil {
			if errors.Is(err, queue.ErrValidationExecutionConflict) {
				return machine.EnsureDispatch(ctx, current)
			}
			return queue.VersionedValidationExecution{}, err
		}
		current = queue.VersionedValidationExecution{Execution: execution, Revision: revision}
		return machine.dispatchPending(ctx, current, request)
	case queue.ExecutionStatusStageQueued:
		stage := execution.Orchestration.Stages[execution.Orchestration.CurrentStageOrdinal-1]
		if stage.DispatchStatus == queue.StageDispatchPublished {
			return current, nil
		}
		request, err := machine.currentRequest(execution)
		if err != nil {
			return queue.VersionedValidationExecution{}, err
		}
		return machine.dispatchPending(ctx, current, request)
	case queue.ExecutionStatusValid, queue.ExecutionStatusInvalid, queue.ExecutionStatusProcessingFailed:
		return machine.EnsureFinalResult(ctx, current)
	default:
		return queue.VersionedValidationExecution{}, fmt.Errorf("unsupported orchestration status %q", execution.Orchestration.Status)
	}
}

// HandleStageResult accepts one worker-produced result, persists its complete
// immutable evidence, CAS-updates stage state, and publishes only the next
// request selected by the frozen plan. Terminal outcomes publish no later work.
func (machine *Machine) HandleStageResult(ctx context.Context, result queue.StageResult) (queue.VersionedValidationExecution, error) {
	transitionStarted := time.Now()
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if err := result.Validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current, err := machine.Queue.GetValidationExecutionVersion(ctx, result.UploadID, result.ValidationID, result.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if err := machine.validatePlanTopology(current.Execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	if err := resultMatchesExecution(result, current.Execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	ctx, transitionSpan := startResultSpan(ctx, "orchestrator.stage_result.transition", result, trace.WithTimestamp(transitionStarted))
	defer transitionSpan.End()
	defer observeOrchestratorTransition("handle_stage_result", result.Stage, transitionStarted)
	stageIndex := int(result.StageOrdinal - 1)
	if stageIndex < len(current.Execution.Orchestration.Stages) {
		accepted := current.Execution.Orchestration.Stages[stageIndex].Attempts
		if int(result.Attempt) <= len(accepted) {
			if accepted[result.Attempt-1].ResultID != result.ResultID {
				return queue.VersionedValidationExecution{}, errors.New("conflicting result for an already accepted stage attempt")
			}
			evidence, err := machine.Queue.GetValidationStageEvidence(ctx, result.UploadID, result.ValidationID, result.Generation, result.StageOrdinal, result.Attempt)
			if err != nil {
				return queue.VersionedValidationExecution{}, fmt.Errorf("load evidence for an already accepted stage attempt: %w", err)
			}
			if !queue.SemanticallyEqualStageResults(evidence, result) {
				return queue.VersionedValidationExecution{}, errors.New("conflicting result for an already accepted stage attempt")
			}
			return machine.EnsureDispatch(ctx, current)
		}
	}
	if current.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued ||
		current.Execution.Orchestration.CurrentStageOrdinal != result.StageOrdinal || stageIndex != len(current.Execution.Orchestration.Stages)-1 {
		return queue.VersionedValidationExecution{}, errors.New("stage result is not for the current queued plan ordinal")
	}
	stageState := current.Execution.Orchestration.Stages[stageIndex]
	if result.Attempt != stageState.AuthorizedAttempt || result.RequestID != stageState.RequestID {
		return queue.VersionedValidationExecution{}, errors.New("stage result is not for the currently authorized attempt")
	}
	if _, err := machine.Queue.CreateValidationStageEvidence(ctx, result); err != nil {
		return queue.VersionedValidationExecution{}, err
	}

	next := cloneExecution(current.Execution)
	nextStage := &next.Orchestration.Stages[stageIndex]
	now := machine.now()
	dispatchedAt := stageState.DispatchedAt
	if stageState.DispatchStatus == queue.StageDispatchPending {
		dispatchedAt = now.UTC().Format(time.RFC3339Nano)
		nextStage.DispatchStatus = queue.StageDispatchPublished
		nextStage.DispatchedAt = dispatchedAt
	}
	nextStage.Attempts = append(nextStage.Attempts, queue.ValidationStageAttempt{
		Attempt: result.Attempt, RequestID: result.RequestID, ResultID: result.ResultID,
		DispatchedAt: dispatchedAt, Outcome: result.Outcome, Reason: result.Reason,
	})
	next.Orchestration.UpdatedAt = now.UTC().Format(time.RFC3339Nano)
	var request *queue.StageRequest
	switch result.Outcome {
	case queue.StageOutcomePassed:
		nextStage.TerminalOutcome = result.Outcome
		if int(result.StageOrdinal) == len(next.Plan.Stages) {
			next.Orchestration.Status = queue.ExecutionStatusValid
			next.Orchestration.FinalizedAt = now.UTC().Format(time.RFC3339Nano)
		} else {
			nextOrdinal := result.StageOrdinal + 1
			created, createErr := queue.NewStageRequest(current.Execution, nextOrdinal, 1, now)
			if createErr != nil {
				return queue.VersionedValidationExecution{}, createErr
			}
			next.Orchestration.Status = queue.ExecutionStatusStageQueued
			next.Orchestration.CurrentStageOrdinal = nextOrdinal
			next.Orchestration.Stages = append(next.Orchestration.Stages, queue.ValidationStageState{
				Stage: created.Stage, Ordinal: created.StageOrdinal, AuthorizedAttempt: 1, RequestID: created.RequestID, RequestCreatedAt: created.CreatedAt,
				DispatchStatus: queue.StageDispatchPending,
			})
			request = &created
		}
	case queue.StageOutcomeStandardsInvalid:
		nextStage.TerminalOutcome = result.Outcome
		next.Orchestration.Status = queue.ExecutionStatusInvalid
		next.Orchestration.FinalizedAt = now.UTC().Format(time.RFC3339Nano)
	case queue.StageOutcomeOperationalFailure:
		if result.Attempt >= current.Execution.RetryPolicy.MaxAttemptsFor(result.Stage) {
			nextStage.TerminalOutcome = result.Outcome
			next.Orchestration.Status = queue.ExecutionStatusProcessingFailed
			next.Orchestration.FinalizedAt = now.UTC().Format(time.RFC3339Nano)
		} else {
			nextAttempt := result.Attempt + 1
			created, createErr := queue.NewStageRequest(current.Execution, result.StageOrdinal, nextAttempt, now)
			if createErr != nil {
				return queue.VersionedValidationExecution{}, createErr
			}
			nextStage.AuthorizedAttempt = nextAttempt
			nextStage.RequestID = created.RequestID
			nextStage.RequestCreatedAt = created.CreatedAt
			nextStage.DispatchStatus = queue.StageDispatchPending
			nextStage.DispatchedAt = ""
			request = &created
		}
	default:
		return queue.VersionedValidationExecution{}, fmt.Errorf("unsupported stage outcome %q", result.Outcome)
	}
	if next.Orchestration.Status == queue.ExecutionStatusValid || next.Orchestration.Status == queue.ExecutionStatusInvalid || next.Orchestration.Status == queue.ExecutionStatusProcessingFailed {
		resultID, resultIDErr := queue.ValidationExecutionResultID(next.ValidationID, next.Generation)
		if resultIDErr != nil {
			return queue.VersionedValidationExecution{}, resultIDErr
		}
		next.Orchestration.FinalResultStatus = queue.FinalResultPending
		next.Orchestration.FinalResultID = resultID
	}
	revision, err := machine.Queue.UpdateValidationExecutionState(ctx, current, next)
	if err != nil {
		if errors.Is(err, queue.ErrValidationExecutionConflict) {
			return machine.HandleStageResult(ctx, result)
		}
		return queue.VersionedValidationExecution{}, err
	}
	updated := queue.VersionedValidationExecution{Execution: next, Revision: revision}
	if request != nil {
		return machine.EnsureDispatch(ctx, updated)
	}
	return machine.EnsureFinalResult(ctx, updated)
}

// dispatchPending publishes one exact durable outbox intent, then records the
// broker acknowledgement under CAS. A conflict means another replica either
// marked the same request or advanced the execution; reloading converges on
// that winner without rewriting history.
func (machine *Machine) dispatchPending(ctx context.Context, current queue.VersionedValidationExecution, request queue.StageRequest) (queue.VersionedValidationExecution, error) {
	transitionStarted := time.Now()
	ctx, span := observability.Tracer().Start(ctx, "orchestrator.stage_request.queue",
		trace.WithAttributes(
			attribute.String("upload.id", request.UploadID),
			attribute.String("validation.id", request.ValidationID),
			attribute.Int64("validation.generation", int64(request.Generation)),
			attribute.String("stage.name", string(request.Stage)),
			attribute.Int64("stage.ordinal", int64(request.StageOrdinal)),
			attribute.Int64("stage.attempt", int64(request.Attempt)),
			attribute.String("stage.request_id", request.RequestID),
		),
	)
	defer span.End()
	defer observeOrchestratorTransition("dispatch_stage_request", request.Stage, transitionStarted)
	if err := machine.PublishStageRequest(ctx, request); err != nil {
		return current, err
	}
	for {
		next := cloneExecution(current.Execution)
		stageIndex := int(next.Orchestration.CurrentStageOrdinal - 1)
		stage := &next.Orchestration.Stages[stageIndex]
		if stage.RequestID != request.RequestID || stage.DispatchStatus != queue.StageDispatchPending {
			return machine.EnsureDispatch(ctx, current)
		}
		now := machine.now()
		stage.DispatchStatus = queue.StageDispatchPublished
		stage.DispatchedAt = now.UTC().Format(time.RFC3339Nano)
		next.Orchestration.UpdatedAt = stage.DispatchedAt
		revision, err := machine.Queue.UpdateValidationExecutionState(ctx, current, next)
		if err == nil {
			return queue.VersionedValidationExecution{Execution: next, Revision: revision}, nil
		}
		if !errors.Is(err, queue.ErrValidationExecutionConflict) {
			return current, err
		}
		latest, getErr := machine.Queue.GetValidationExecutionVersion(ctx, current.Execution.UploadID, current.Execution.ValidationID, current.Execution.Generation)
		if getErr != nil {
			return current, getErr
		}
		if latest.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued {
			return latest, nil
		}
		latestStage := latest.Execution.Orchestration.Stages[latest.Execution.Orchestration.CurrentStageOrdinal-1]
		if latestStage.RequestID != request.RequestID || latestStage.DispatchStatus == queue.StageDispatchPublished {
			return machine.EnsureDispatch(ctx, latest)
		}
		current = latest
	}
}

// HandleStageResultMessage is the durable-consumer boundary: malformed or
// failed transitions remain unacknowledged; success ACKs only after evidence,
// state, and any next request publication complete.
func (machine *Machine) HandleStageResultMessage(ctx context.Context, message jetstream.Msg) error {
	ctx = observability.ExtractNATS(ctx, message.Headers())
	ctx, consumeSpan := observability.Tracer().Start(ctx, "queue.stage_result.consume", trace.WithSpanKind(trace.SpanKindConsumer))
	defer consumeSpan.End()
	var result queue.StageResult
	if err := json.Unmarshal(message.Data(), &result); err != nil {
		return fmt.Errorf("decode stage result: %w", err)
	}
	if _, err := machine.HandleStageResult(ctx, result); err != nil {
		return err
	}
	if err := message.Ack(); err != nil {
		return fmt.Errorf("ack stage result %s: %w", result.ResultID, err)
	}
	return nil
}

func (machine *Machine) currentRequest(execution queue.ValidationExecution) (queue.StageRequest, error) {
	ordinal := execution.Orchestration.CurrentStageOrdinal
	stage := execution.Orchestration.Stages[ordinal-1]
	createdAt, err := time.Parse(time.RFC3339Nano, stage.RequestCreatedAt)
	if err != nil {
		return queue.StageRequest{}, fmt.Errorf("parse durable request creation time: %w", err)
	}
	request, err := queue.NewStageRequest(execution, ordinal, stage.AuthorizedAttempt, createdAt)
	if err != nil {
		return queue.StageRequest{}, err
	}
	if request.RequestID != stage.RequestID {
		return queue.StageRequest{}, errors.New("durable queued request identity is inconsistent")
	}
	return request, nil
}

func (machine *Machine) validate() error {
	if machine == nil || machine.Queue == nil {
		return errors.New("orchestrator queue client is required")
	}
	if machine.Now == nil {
		return errors.New("orchestrator clock is required")
	}
	if machine.PublishStageRequest == nil {
		return errors.New("orchestrator stage request publisher is required")
	}
	if machine.PublishValidationResult == nil {
		return errors.New("orchestrator validation result publisher is required")
	}
	return nil
}

func (machine *Machine) validatePlanTopology(execution queue.ValidationExecution) error {
	for _, stage := range execution.Plan.Stages {
		if _, err := machine.Queue.Namespace.StageQueue(stage.ID); err != nil {
			return fmt.Errorf("frozen plan stage %q has no provisionable queue topology: %w", stage.ID, err)
		}
	}
	return nil
}

func (machine *Machine) now() time.Time {
	return machine.Now().UTC()
}

func resultMatchesExecution(result queue.StageResult, execution queue.ValidationExecution) error {
	if result.UploadID != execution.UploadID || result.ValidationID != execution.ValidationID || result.Generation != execution.Generation ||
		result.DocumentType != execution.DocumentType || result.Plan != execution.Plan.ID || result.PlanVersion != execution.Plan.Version {
		return errors.New("stage result identity does not match durable execution")
	}
	if result.StageOrdinal == 0 || int(result.StageOrdinal) > len(execution.Plan.Stages) {
		return errors.New("stage result ordinal is outside the frozen plan")
	}
	stage := execution.Plan.Stages[result.StageOrdinal-1]
	if result.Stage != stage.ID || result.RulesetVersion != stage.RulesetVersion {
		return errors.New("stage result does not match the frozen plan stage")
	}
	return nil
}

func cloneExecution(execution queue.ValidationExecution) queue.ValidationExecution {
	cloned := execution
	cloned.Plan.Stages = append([]queue.FrozenValidationStage(nil), execution.Plan.Stages...)
	cloned.Orchestration.Stages = append([]queue.ValidationStageState(nil), execution.Orchestration.Stages...)
	for index := range cloned.Orchestration.Stages {
		cloned.Orchestration.Stages[index].Attempts = append([]queue.ValidationStageAttempt(nil), execution.Orchestration.Stages[index].Attempts...)
	}
	return cloned
}
