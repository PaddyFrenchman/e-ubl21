package queue

import (
	"errors"
	"fmt"
	"time"
)

// ValidationExecutionStatus is the orchestrator-owned lifecycle of one exact
// validation execution generation. Workers never mutate this state.
type ValidationExecutionStatus string

const (
	ExecutionStatusPending          ValidationExecutionStatus = "pending"
	ExecutionStatusStageQueued      ValidationExecutionStatus = "stage-queued"
	ExecutionStatusValid            ValidationExecutionStatus = "valid"
	ExecutionStatusInvalid          ValidationExecutionStatus = "invalid"
	ExecutionStatusProcessingFailed ValidationExecutionStatus = "processing-failed"
)

// ValidationStageAttempt records the immutable identity and classification of
// accepted attempt evidence. The complete StageResult is stored separately so
// bounded diagnostics cannot overflow the compact execution-state record.
type ValidationStageAttempt struct {
	Attempt   uint32                 `json:"attempt"`
	RequestID string                 `json:"requestId"`
	ResultID  string                 `json:"resultId"`
	Outcome   StageOutcome           `json:"outcome"`
	Reason    StageOperationalReason `json:"reason,omitempty"`
}

// ValidationStageState is the orchestrator projection for one frozen plan
// ordinal. AuthorizedAttempt is the only attempt a worker may currently run.
type ValidationStageState struct {
	Stage             ValidationLayer          `json:"stage"`
	Ordinal           uint32                   `json:"ordinal"`
	AuthorizedAttempt uint32                   `json:"authorizedAttempt"`
	RequestID         string                   `json:"requestId"`
	RequestCreatedAt  string                   `json:"requestCreatedAt"`
	Attempts          []ValidationStageAttempt `json:"attempts,omitempty"`
	TerminalOutcome   StageOutcome             `json:"terminalOutcome,omitempty"`
}

// ValidationOrchestrationState is the mutable, CAS-protected projection kept
// beside an execution's immutable identity and frozen plan.
type ValidationOrchestrationState struct {
	Status              ValidationExecutionStatus `json:"status"`
	CurrentStageOrdinal uint32                    `json:"currentStageOrdinal,omitempty"`
	Stages              []ValidationStageState    `json:"stages,omitempty"`
	UpdatedAt           string                    `json:"updatedAt"`
	FinalizedAt         string                    `json:"finalizedAt,omitempty"`
}

func newPendingOrchestrationState(at time.Time) ValidationOrchestrationState {
	return ValidationOrchestrationState{
		Status:    ExecutionStatusPending,
		UpdatedAt: canonicalStageTime(at),
	}
}

func (state ValidationOrchestrationState) validate(execution ValidationExecution) error {
	if _, err := parseCanonicalStageTime("orchestration updatedAt", state.UpdatedAt); err != nil {
		return err
	}
	if state.FinalizedAt != "" {
		if _, err := parseCanonicalStageTime("orchestration finalizedAt", state.FinalizedAt); err != nil {
			return err
		}
	}
	if len(state.Stages) > len(execution.Plan.Stages) {
		return errors.New("orchestration state contains more stages than the frozen plan")
	}
	for index, stageState := range state.Stages {
		planStage := execution.Plan.Stages[index]
		if stageState.Ordinal != planStage.Ordinal || stageState.Stage != planStage.ID {
			return fmt.Errorf("orchestration stage %d does not match frozen plan ordinal", index+1)
		}
		if stageState.AuthorizedAttempt == 0 {
			return fmt.Errorf("orchestration stage %d has no authorized attempt", index+1)
		}
		if stageState.AuthorizedAttempt > execution.RetryPolicy.MaxAttempts {
			return fmt.Errorf("orchestration stage %d authorized attempt exceeds the frozen retry policy", index+1)
		}
		wantRequestID := stageRequestID(StageContractVersion, execution.ValidationID, execution.UploadID, execution.DocumentType,
			execution.Plan.ID, execution.Plan.Version, execution.Generation, planStage.ID, planStage.Ordinal,
			planStage.RulesetVersion, stageState.AuthorizedAttempt)
		if stageState.RequestID != wantRequestID {
			return fmt.Errorf("orchestration stage %d request id does not match authorized attempt", index+1)
		}
		if _, err := parseCanonicalStageTime("orchestration requestCreatedAt", stageState.RequestCreatedAt); err != nil {
			return fmt.Errorf("orchestration stage %d: %w", index+1, err)
		}
		for attemptIndex, attempt := range stageState.Attempts {
			wantAttempt := uint32(attemptIndex + 1)
			if attempt.Attempt != wantAttempt {
				return fmt.Errorf("orchestration stage %d attempt %d is not contiguous", index+1, attempt.Attempt)
			}
			wantAttemptRequestID := stageRequestID(StageContractVersion, execution.ValidationID, execution.UploadID, execution.DocumentType,
				execution.Plan.ID, execution.Plan.Version, execution.Generation, planStage.ID, planStage.Ordinal,
				planStage.RulesetVersion, attempt.Attempt)
			wantResultID := stageMessageID("e-ubl21/stage-result", wantAttemptRequestID)
			if attempt.RequestID != wantAttemptRequestID || attempt.ResultID != wantResultID {
				return fmt.Errorf("orchestration stage %d attempt %d has invalid evidence identity", index+1, attempt.Attempt)
			}
			switch attempt.Outcome {
			case StageOutcomePassed, StageOutcomeStandardsInvalid:
				if attempt.Reason != StageReasonNone {
					return fmt.Errorf("orchestration stage %d attempt %d has an invalid operational reason", index+1, attempt.Attempt)
				}
			case StageOutcomeOperationalFailure:
				if !validStageOperationalReason(attempt.Reason) {
					return fmt.Errorf("orchestration stage %d attempt %d has invalid operational reason", index+1, attempt.Attempt)
				}
			default:
				return fmt.Errorf("orchestration stage %d attempt %d has invalid outcome %q", index+1, attempt.Attempt, attempt.Outcome)
			}
			if attemptIndex < len(stageState.Attempts)-1 && attempt.Outcome != StageOutcomeOperationalFailure {
				return fmt.Errorf("orchestration stage %d advanced to another attempt after terminal standards evidence", index+1)
			}
		}
		if stageState.TerminalOutcome == "" {
			if index != len(state.Stages)-1 || state.Status != ExecutionStatusStageQueued {
				return fmt.Errorf("only the current queued stage may lack a terminal outcome")
			}
			if len(stageState.Attempts)+1 != int(stageState.AuthorizedAttempt) {
				return fmt.Errorf("queued stage %d authorized attempt does not follow accepted evidence", index+1)
			}
			continue
		}
		if len(stageState.Attempts) == 0 || stageState.AuthorizedAttempt != uint32(len(stageState.Attempts)) {
			return fmt.Errorf("terminal stage %d must end on its last authorized attempt", index+1)
		}
		if got := stageState.Attempts[len(stageState.Attempts)-1].Outcome; got != stageState.TerminalOutcome {
			return fmt.Errorf("terminal stage %d outcome %q does not match final attempt %q", index+1, stageState.TerminalOutcome, got)
		}
		if index < len(state.Stages)-1 && stageState.TerminalOutcome != StageOutcomePassed {
			return fmt.Errorf("orchestration advanced beyond non-passing stage %d", index+1)
		}
	}

	switch state.Status {
	case ExecutionStatusPending:
		if state.CurrentStageOrdinal != 0 || len(state.Stages) != 0 || state.FinalizedAt != "" {
			return errors.New("pending orchestration state cannot contain stage or final state")
		}
	case ExecutionStatusStageQueued:
		if len(state.Stages) == 0 || state.CurrentStageOrdinal != uint32(len(state.Stages)) || state.FinalizedAt != "" || state.Stages[len(state.Stages)-1].TerminalOutcome != "" {
			return errors.New("queued orchestration state does not identify exactly one current stage")
		}
	case ExecutionStatusValid:
		if len(state.Stages) != len(execution.Plan.Stages) || state.CurrentStageOrdinal != uint32(len(state.Stages)) || state.FinalizedAt == "" {
			return errors.New("valid orchestration state requires every frozen-plan stage and a finalization time")
		}
		for _, stage := range state.Stages {
			if stage.TerminalOutcome != StageOutcomePassed {
				return errors.New("valid orchestration state contains a non-passing stage")
			}
		}
	case ExecutionStatusInvalid:
		if len(state.Stages) == 0 || state.CurrentStageOrdinal != uint32(len(state.Stages)) || state.FinalizedAt == "" || state.Stages[len(state.Stages)-1].TerminalOutcome != StageOutcomeStandardsInvalid {
			return errors.New("invalid orchestration state requires terminal standards-invalid evidence")
		}
	case ExecutionStatusProcessingFailed:
		if len(state.Stages) == 0 || state.CurrentStageOrdinal != uint32(len(state.Stages)) || state.FinalizedAt == "" || state.Stages[len(state.Stages)-1].TerminalOutcome != StageOutcomeOperationalFailure {
			return errors.New("processing-failed orchestration state requires exhausted operational evidence")
		}
		if state.Stages[len(state.Stages)-1].AuthorizedAttempt != execution.RetryPolicy.MaxAttempts {
			return errors.New("processing-failed orchestration state has not exhausted the frozen retry policy")
		}
	default:
		return fmt.Errorf("invalid validation orchestration status %q", state.Status)
	}
	return nil
}
