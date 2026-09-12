package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"go.opentelemetry.io/otel/trace"
)

// EnsureFinalResult claims any pre-#56 terminal state, derives a canonical
// result only from permanent accepted evidence, publishes it, and CAS-marks
// the durable outbox complete. Concurrent replicas converge on the same
// deterministic result and publication slot.
func (machine *Machine) EnsureFinalResult(ctx context.Context, current queue.VersionedValidationExecution) (queue.VersionedValidationExecution, error) {
	if err := machine.validate(); err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	latest, err := machine.Queue.GetValidationExecutionVersion(ctx, current.Execution.UploadID, current.Execution.ValidationID, current.Execution.Generation)
	if err != nil {
		return queue.VersionedValidationExecution{}, err
	}
	current = latest
	if !terminalExecutionStatus(current.Execution.Orchestration.Status) {
		return current, fmt.Errorf("validation execution status %q is not terminal", current.Execution.Orchestration.Status)
	}
	if current.Execution.Orchestration.FinalResultStatus == queue.FinalResultPublished {
		return current, nil
	}
	if current.Execution.Orchestration.FinalResultStatus == "" {
		next := cloneExecution(current.Execution)
		resultID, idErr := queue.ValidationExecutionResultID(next.ValidationID, next.Generation)
		if idErr != nil {
			return current, idErr
		}
		now := machine.now().Format(time.RFC3339Nano)
		next.Orchestration.FinalResultStatus = queue.FinalResultPending
		next.Orchestration.FinalResultID = resultID
		next.Orchestration.FinalResultTrace = observability.CaptureTrace(ctx)
		next.Orchestration.UpdatedAt = now
		revision, updateErr := machine.Queue.UpdateValidationExecutionState(ctx, current, next)
		if updateErr != nil {
			if errors.Is(updateErr, queue.ErrValidationExecutionConflict) {
				return machine.EnsureFinalResult(ctx, current)
			}
			return current, updateErr
		}
		current = queue.VersionedValidationExecution{Execution: next, Revision: revision}
	}
	if current.Execution.Orchestration.FinalResultStatus != queue.FinalResultPending {
		return current, fmt.Errorf("validation execution has invalid final result status %q", current.Execution.Orchestration.FinalResultStatus)
	}
	ctx = observability.RestoreTrace(ctx, current.Execution.Orchestration.FinalResultTrace)
	result, err := machine.buildValidationResult(ctx, current.Execution)
	if err != nil {
		return current, err
	}
	publishStarted := time.Now()
	publishCtx, publishSpan := startExecutionSpan(ctx, "orchestrator.final_result.publish", current.Execution, trace.WithSpanKind(trace.SpanKindProducer))
	if err := machine.PublishValidationResult(publishCtx, result); err != nil {
		observability.End(publishSpan, err)
		orchestratorResultPublicationLatency.Observe(time.Since(publishStarted).Seconds())
		return current, fmt.Errorf("publish validation result %s: %w", result.ResultID, err)
	}
	publishSpan.End()
	orchestratorResultPublicationLatency.Observe(time.Since(publishStarted).Seconds())

	for {
		latest, getErr := machine.Queue.GetValidationExecutionVersion(ctx, current.Execution.UploadID, current.Execution.ValidationID, current.Execution.Generation)
		if getErr != nil {
			return current, getErr
		}
		if latest.Execution.Orchestration.FinalResultStatus == queue.FinalResultPublished {
			return latest, nil
		}
		if latest.Execution.Orchestration.FinalResultStatus != queue.FinalResultPending || latest.Execution.Orchestration.FinalResultID != result.ResultID {
			return latest, errors.New("validation final-result outbox changed after publication")
		}
		next := cloneExecution(latest.Execution)
		publishedAt := machine.now().Format(time.RFC3339Nano)
		next.Orchestration.FinalResultStatus = queue.FinalResultPublished
		next.Orchestration.FinalResultPublishedAt = publishedAt
		next.Orchestration.UpdatedAt = publishedAt
		revision, updateErr := machine.Queue.UpdateValidationExecutionState(ctx, latest, next)
		if updateErr == nil {
			return queue.VersionedValidationExecution{Execution: next, Revision: revision}, nil
		}
		if !errors.Is(updateErr, queue.ErrValidationExecutionConflict) {
			return latest, updateErr
		}
		current = latest
	}
}

func (machine *Machine) buildValidationResult(ctx context.Context, execution queue.ValidationExecution) (queue.ValidationResult, error) {
	state := execution.Orchestration
	if !terminalExecutionStatus(state.Status) || state.FinalResultStatus != queue.FinalResultPending {
		return queue.ValidationResult{}, errors.New("validation execution has no pending terminal result")
	}
	stages := make(queue.ValidationStages, 0, len(state.Stages))
	for index, stage := range state.Stages {
		if stage.TerminalOutcome == "" || (len(stage.Attempts) == 0 && stage.InheritedEvidence == nil) {
			return queue.ValidationResult{}, fmt.Errorf("terminal validation stage %d has no accepted terminal evidence", index+1)
		}
		var evidence queue.StageResult
		var err error
		if stage.InheritedEvidence != nil {
			reference := stage.InheritedEvidence
			evidence, err = machine.Queue.GetValidationStageEvidence(ctx, execution.UploadID, reference.ValidationID, reference.Generation, reference.StageOrdinal, reference.Attempt)
		} else {
			accepted := stage.Attempts[len(stage.Attempts)-1]
			evidence, err = machine.Queue.GetValidationStageEvidence(ctx, execution.UploadID, execution.ValidationID, execution.Generation, stage.Ordinal, accepted.Attempt)
		}
		if err != nil {
			return queue.ValidationResult{}, fmt.Errorf("load terminal validation stage %d evidence: %w", index+1, err)
		}
		if err := evidence.Validate(); err != nil {
			return queue.ValidationResult{}, fmt.Errorf("validate terminal validation stage %d evidence: %w", index+1, err)
		}
		planStage := execution.Plan.Stages[index]
		if evidence.UploadID != execution.UploadID || evidence.Stage != planStage.ID || evidence.StageOrdinal != planStage.Ordinal || evidence.RulesetVersion != planStage.RulesetVersion || evidence.Outcome != stage.TerminalOutcome {
			return queue.ValidationResult{}, fmt.Errorf("terminal validation stage %d evidence does not match frozen plan", index+1)
		}
		if stage.InheritedEvidence != nil {
			reference := stage.InheritedEvidence
			if evidence.ValidationID != reference.ValidationID || evidence.Generation != reference.Generation || evidence.RequestID != reference.RequestID || evidence.ResultID != reference.ResultID || evidence.Attempt != reference.Attempt || evidence.Outcome != queue.StageOutcomePassed {
				return queue.ValidationResult{}, fmt.Errorf("terminal validation stage %d inherited evidence does not match orchestration state", index+1)
			}
		} else {
			accepted := stage.Attempts[len(stage.Attempts)-1]
			if err := resultMatchesExecution(evidence, execution); err != nil {
				return queue.ValidationResult{}, fmt.Errorf("terminal validation stage %d: %w", index+1, err)
			}
			if evidence.ResultID != accepted.ResultID || evidence.RequestID != accepted.RequestID || evidence.Attempt != accepted.Attempt ||
				evidence.Outcome != accepted.Outcome || evidence.Reason != accepted.Reason {
				return queue.ValidationResult{}, fmt.Errorf("terminal validation stage %d evidence does not match orchestration state", index+1)
			}
		}
		stageResult := queue.ValidationStageResult{
			Layer: evidence.Stage, Version: evidence.RulesetVersion, Valid: evidence.Outcome == queue.StageOutcomePassed,
			Outcome: evidence.Outcome, Reason: evidence.Reason, Issues: append([]queue.ValidationIssue(nil), evidence.Issues...),
			Diagnostic: evidence.Diagnostic, ValidatedAt: evidence.CompletedAt, DurationMS: evidence.DurationMS,
		}
		if evidence.Stage == queue.ValidationLayerPeppolBISBilling {
			if err := machine.populatePeppolIdentity(ctx, execution, &stageResult); err != nil {
				return queue.ValidationResult{}, fmt.Errorf("build Peppol terminal identity: %w", err)
			}
		}
		stages = append(stages, stageResult)
	}
	status := queue.StatusProcessingFailed
	switch state.Status {
	case queue.ExecutionStatusValid:
		status = queue.StatusValid
	case queue.ExecutionStatusInvalid:
		status = queue.StatusInvalid
	case queue.ExecutionStatusProcessingFailed:
		status = queue.StatusProcessingFailed
	}
	errorText := ""
	if status == queue.StatusProcessingFailed {
		terminal := stages[len(stages)-1]
		errorText = strings.TrimSpace(terminal.Diagnostic)
		if errorText == "" {
			errorText = fmt.Sprintf("validation stage %s failed operationally: %s", terminal.Layer, terminal.Reason)
		}
	}
	result := queue.ValidationResult{
		ID: execution.UploadID, ResultID: state.FinalResultID, ValidationID: execution.ValidationID, Generation: execution.Generation,
		Profile: execution.RequestedProfile, Plan: execution.Plan.ID, PlanVersion: execution.Plan.Version, PlanStages: append([]queue.FrozenValidationStage(nil), execution.Plan.Stages...),
		DocumentType: execution.DocumentType, Status: status, Error: errorText, ValidatedAt: state.FinalizedAt, ValidationStages: stages,
	}
	if err := result.ValidateExecutionResult(); err != nil {
		return queue.ValidationResult{}, fmt.Errorf("build canonical validation result: %w", err)
	}
	return result, nil
}

// populatePeppolIdentity derives client-visible identity from the same
// immutable upload bytes selected by the frozen plan. StageResult remains the
// stable generic worker contract; the orchestrator owns final-result shaping.
func (machine *Machine) populatePeppolIdentity(ctx context.Context, execution queue.ValidationExecution, stage *queue.ValidationStageResult) error {
	profile := validationassets.Peppol()
	stage.BISVersion = profile.Version
	stage.BISReleaseDate = profile.PublicationDate
	stage.BISMandatoryFrom = profile.MandatoryFrom

	content, err := machine.Queue.OpenUploadContent(ctx, execution.UploadID)
	if err != nil {
		if stage.Outcome == queue.StageOutcomeOperationalFailure {
			return nil
		}
		return fmt.Errorf("open immutable upload content: %w", err)
	}
	defer content.Close()
	limited := io.LimitReader(content, peppol.MaxResolutionDocumentBytes+1)
	document, err := io.ReadAll(limited)
	if err != nil {
		if stage.Outcome == queue.StageOutcomeOperationalFailure {
			return nil
		}
		return fmt.Errorf("read immutable upload content: %w", err)
	}
	if streamErr := content.Error(); streamErr != nil {
		if stage.Outcome == queue.StageOutcomeOperationalFailure {
			return nil
		}
		return fmt.Errorf("stream immutable upload content: %w", streamErr)
	}
	if execution.RequestedProfile == queue.ValidationProfilePeppolEnvelope {
		document, err = peppol.ExtractEnvelopePayload(document)
		if err != nil {
			if stage.Outcome == queue.StageOutcomeOperationalFailure {
				return nil
			}
			return fmt.Errorf("extract immutable Peppol envelope payload: %w", err)
		}
	}
	resolution, err := peppol.Resolve(execution.DocumentType, queue.ValidationProfilePeppolBISBilling, document)
	if err != nil {
		if stage.Outcome == queue.StageOutcomeOperationalFailure {
			return nil
		}
		return err
	}
	stage.CustomizationID = resolution.Identifiers.CustomizationID
	stage.ProfileID = resolution.Identifiers.ProfileID
	stage.ProcessProfile = resolution.Identifiers.ProcessProfile
	return nil
}

func terminalExecutionStatus(status queue.ValidationExecutionStatus) bool {
	return status == queue.ExecutionStatusValid || status == queue.ExecutionStatusInvalid || status == queue.ExecutionStatusProcessingFailed
}
