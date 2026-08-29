// Package orchestrator owns validation-plan sequencing and terminal execution
// decisions. It never parses invoice XML or manufactures validation evidence.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
)

// Machine advances exact validation execution generations under JetStream KV
// compare-and-swap. Crash-window outbox reconciliation is deliberately added
// by TODO-VALIDATION-SPLIT-006; deterministic publication already makes an
// explicit EnsureDispatch retry harmless.
type Machine struct {
	Queue *queue.Client
	Now   func() time.Time
}

func New(client *queue.Client) *Machine {
	return &Machine{Queue: client, Now: time.Now}
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
	if _, err := machine.Queue.CreateValidationExecution(ctx, execution); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current, err := machine.Queue.GetValidationExecutionVersion(ctx, execution.UploadID, execution.ValidationID, execution.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	return machine.EnsureDispatch(ctx, current)
}

// EnsureDispatch authorizes the first stage from pending state or republishes
// the deterministic request for an already queued state. It never skips or
// selects a stage outside the frozen plan.
func (machine *Machine) EnsureDispatch(ctx context.Context, current queue.VersionedValidationExecution) (queue.VersionedValidationExecution, error) {
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	execution := cloneExecution(current.Execution)
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
			}},
			UpdatedAt: machine.now().UTC().Format(time.RFC3339Nano),
		}
		revision, err := machine.Queue.UpdateValidationExecutionState(ctx, current, execution)
		if err != nil {
			return queue.VersionedValidationExecution{}, err
		}
		current = queue.VersionedValidationExecution{Execution: execution, Revision: revision}
		if err := machine.Queue.PublishStageRequest(ctx, request); err != nil {
			return current, err
		}
		return current, nil
	case queue.ExecutionStatusStageQueued:
		request, err := machine.currentRequest(execution)
		if err != nil {
			return queue.VersionedValidationExecution{}, err
		}
		if err := machine.Queue.PublishStageRequest(ctx, request); err != nil {
			return current, err
		}
		return current, nil
	case queue.ExecutionStatusValid, queue.ExecutionStatusInvalid, queue.ExecutionStatusProcessingFailed:
		return current, nil
	default:
		return queue.VersionedValidationExecution{}, fmt.Errorf("unsupported orchestration status %q", execution.Orchestration.Status)
	}
}

// HandleStageResult accepts one worker-produced result, persists its complete
// immutable evidence, CAS-updates stage state, and publishes only the next
// request selected by the frozen plan. Terminal outcomes publish no later work.
func (machine *Machine) HandleStageResult(ctx context.Context, result queue.StageResult) (queue.VersionedValidationExecution, error) {
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
			if !reflect.DeepEqual(evidence, result) {
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
	nextStage.Attempts = append(nextStage.Attempts, queue.ValidationStageAttempt{
		Attempt: result.Attempt, RequestID: result.RequestID, ResultID: result.ResultID,
		Outcome: result.Outcome, Reason: result.Reason,
	})
	now := machine.now()
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
			})
			request = &created
		}
	case queue.StageOutcomeStandardsInvalid:
		nextStage.TerminalOutcome = result.Outcome
		next.Orchestration.Status = queue.ExecutionStatusInvalid
		next.Orchestration.FinalizedAt = now.UTC().Format(time.RFC3339Nano)
	case queue.StageOutcomeOperationalFailure:
		if result.Attempt >= current.Execution.RetryPolicy.MaxAttempts {
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
			request = &created
		}
	default:
		return queue.VersionedValidationExecution{}, fmt.Errorf("unsupported stage outcome %q", result.Outcome)
	}
	revision, err := machine.Queue.UpdateValidationExecutionState(ctx, current, next)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	updated := queue.VersionedValidationExecution{Execution: next, Revision: revision}
	if request != nil {
		if err := machine.Queue.PublishStageRequest(ctx, *request); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

// HandleStageResultMessage is the durable-consumer boundary: malformed or
// failed transitions remain unacknowledged; success ACKs only after evidence,
// state, and any next request publication complete.
func (machine *Machine) HandleStageResultMessage(ctx context.Context, message jetstream.Msg) error {
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
