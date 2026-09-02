package queue

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
)

var orchestrationTestTime = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

func cloneOrchestrationExecution(t *testing.T, execution ValidationExecution) ValidationExecution {
	t.Helper()
	payload, err := json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	var clone ValidationExecution
	if err := json.Unmarshal(payload, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func orchestrationTestExecution(t *testing.T, stages int) ValidationExecution {
	t.Helper()
	uploadID, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	intentID, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	planStages := []FrozenValidationStage{{
		ID: ValidationLayerUBL21XSD, Ordinal: 1, Requirement: ValidationStageRequired, RulesetVersion: "2.1",
	}}
	profile := ValidationProfileUBL21
	if stages == 2 {
		planStages = append(planStages, FrozenValidationStage{
			ID: ValidationLayerEN16931, Ordinal: 2, Requirement: ValidationStageRequired, RulesetVersion: "1.3.16",
		})
		profile = ValidationProfileUBL21EN16931
	}
	execution, err := NewValidationExecution(ValidationExecutionIntent{
		IntentID: intentID, UploadID: uploadID, DocumentType: "Invoice", Trigger: ValidationTriggerInitial,
		RequestedProfile: profile,
		Plan:             FrozenValidationPlan{ID: "coverage-plan", Version: "1", Stages: planStages},
		CreatedAt:        orchestrationTestTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	return execution
}

func orchestrationRequest(t *testing.T, execution ValidationExecution, ordinal, attempt uint32, at time.Time) StageRequest {
	t.Helper()
	request, err := NewStageRequest(execution, ordinal, attempt, at)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func queuedOrchestration(t *testing.T, execution ValidationExecution, published bool) ValidationExecution {
	t.Helper()
	request := orchestrationRequest(t, execution, 1, 1, orchestrationTestTime.Add(time.Second))
	dispatch := StageDispatchPending
	dispatchedAt := ""
	if published {
		dispatch = StageDispatchPublished
		dispatchedAt = canonicalStageTime(orchestrationTestTime.Add(2 * time.Second))
	}
	execution.Orchestration = ValidationOrchestrationState{
		Status: ExecutionStatusStageQueued, CurrentStageOrdinal: 1,
		Stages: []ValidationStageState{{
			Stage: request.Stage, Ordinal: 1, AuthorizedAttempt: 1, RequestID: request.RequestID,
			RequestCreatedAt: request.CreatedAt, DispatchStatus: dispatch, DispatchedAt: dispatchedAt,
		}},
		UpdatedAt: canonicalStageTime(orchestrationTestTime.Add(2 * time.Second)),
	}
	return execution
}

func terminalOrchestration(t *testing.T, outcome StageOutcome) ValidationExecution {
	t.Helper()
	execution := queuedOrchestration(t, orchestrationTestExecution(t, 1), true)
	stage := &execution.Orchestration.Stages[0]
	reason := StageReasonNone
	status := ExecutionStatusValid
	if outcome == StageOutcomeStandardsInvalid {
		status = ExecutionStatusInvalid
	} else if outcome == StageOutcomeOperationalFailure {
		status = ExecutionStatusProcessingFailed
		reason = StageReasonRuntimeFailure
		execution.RetryPolicy.MaxAttempts = 1
	}
	stage.Attempts = []ValidationStageAttempt{{
		Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
		DispatchedAt: stage.DispatchedAt, Outcome: outcome, Reason: reason,
	}}
	stage.TerminalOutcome = outcome
	execution.Orchestration.Status = status
	execution.Orchestration.FinalizedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
	execution.Orchestration.UpdatedAt = execution.Orchestration.FinalizedAt
	return execution
}

func TestValidationOrchestrationValidLifecycleStates(t *testing.T) {
	pending := orchestrationTestExecution(t, 1)
	queuedPending := queuedOrchestration(t, pending, false)
	queuedPublished := queuedOrchestration(t, pending, true)
	valid := terminalOrchestration(t, StageOutcomePassed)
	invalid := terminalOrchestration(t, StageOutcomeStandardsInvalid)
	failed := terminalOrchestration(t, StageOutcomeOperationalFailure)

	finalPending := valid
	finalPending.Orchestration.FinalResultStatus = FinalResultPending
	finalPending.Orchestration.FinalResultID, _ = ValidationExecutionResultID(finalPending.ValidationID, finalPending.Generation)
	finalPublished := finalPending
	finalPublished.Orchestration.FinalResultStatus = FinalResultPublished
	finalPublished.Orchestration.FinalResultPublishedAt = canonicalStageTime(orchestrationTestTime.Add(4 * time.Second))

	for name, execution := range map[string]ValidationExecution{
		"pending": pending, "queued pending": queuedPending, "queued published": queuedPublished,
		"valid": valid, "invalid": invalid, "processing failed": failed,
		"final pending": finalPending, "final published": finalPublished,
	} {
		t.Run(name, func(t *testing.T) {
			if err := execution.Validate(); err != nil {
				t.Fatalf("valid state rejected: %v\n%+v", err, execution.Orchestration)
			}
		})
	}
}

func TestValidationOrchestrationRejectsCorruptState(t *testing.T) {
	validTrace := &observability.TraceCarrier{TraceParent: "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01"}
	badTrace := &observability.TraceCarrier{TraceParent: "invalid"}

	tests := []struct {
		name   string
		base   func(*testing.T) ValidationExecution
		mutate func(*ValidationExecution)
	}{
		{"updated time", func(t *testing.T) ValidationExecution { return orchestrationTestExecution(t, 1) }, func(e *ValidationExecution) { e.Orchestration.UpdatedAt = "bad" }},
		{"finalized time", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.FinalizedAt = "bad" }},
		{"initial trace", func(t *testing.T) ValidationExecution { return orchestrationTestExecution(t, 1) }, func(e *ValidationExecution) { e.Orchestration.InitialDispatchTrace = badTrace }},
		{"too many stages", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) {
			e.Orchestration.Stages = append(e.Orchestration.Stages, e.Orchestration.Stages[0])
		}},
		{"dispatch trace", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].DispatchTrace = badTrace }},
		{"plan mismatch", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].Ordinal = 2 }},
		{"zero attempt", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].AuthorizedAttempt = 0 }},
		{"attempt over policy", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].AuthorizedAttempt = 4 }},
		{"request identity", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].RequestID = "wrong" }},
		{"request time", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].RequestCreatedAt = "bad" }},
		{"pending publication time", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) {
			e.Orchestration.Stages[0].DispatchedAt = canonicalStageTime(orchestrationTestTime)
		}},
		{"published time", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), true)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].DispatchedAt = "bad" }},
		{"dispatch status", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].DispatchStatus = "other" }},
		{"attempt sequence", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].Attempt = 2 }},
		{"attempt identity", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].ResultID = "wrong" }},
		{"attempt time", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].DispatchedAt = "bad" }},
		{"passing reason", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].Reason = StageReasonTimeout }},
		{"operational reason", func(t *testing.T) ValidationExecution {
			return terminalOrchestration(t, StageOutcomeOperationalFailure)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].Reason = "other" }},
		{"attempt outcome", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].Outcome = "other" }},
		{"terminal unpublished", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.Stages[0].DispatchStatus = StageDispatchPending
			e.Orchestration.Stages[0].DispatchedAt = ""
		}},
		{"terminal missing attempt", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts = nil }},
		{"terminal dispatch mismatch", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.Stages[0].Attempts[0].DispatchedAt = canonicalStageTime(orchestrationTestTime)
		}},
		{"terminal outcome mismatch", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].TerminalOutcome = StageOutcomeStandardsInvalid }},
		{"pending contains state", func(t *testing.T) ValidationExecution { return orchestrationTestExecution(t, 1) }, func(e *ValidationExecution) { e.Orchestration.CurrentStageOrdinal = 1 }},
		{"queued initial trace", func(t *testing.T) ValidationExecution {
			return queuedOrchestration(t, orchestrationTestExecution(t, 1), false)
		}, func(e *ValidationExecution) { e.Orchestration.InitialDispatchTrace = validTrace }},
		{"valid incomplete", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) { e.Orchestration.FinalizedAt = "" }},
		{"invalid wrong outcome", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomeStandardsInvalid) }, func(e *ValidationExecution) { e.Orchestration.Stages[0].TerminalOutcome = StageOutcomePassed }},
		{"failed wrong outcome", func(t *testing.T) ValidationExecution {
			return terminalOrchestration(t, StageOutcomeOperationalFailure)
		}, func(e *ValidationExecution) { e.Orchestration.Stages[0].TerminalOutcome = StageOutcomePassed }},
		{"failed not exhausted", func(t *testing.T) ValidationExecution {
			return terminalOrchestration(t, StageOutcomeOperationalFailure)
		}, func(e *ValidationExecution) { e.RetryPolicy.MaxAttempts = 2 }},
		{"unknown status", func(t *testing.T) ValidationExecution { return orchestrationTestExecution(t, 1) }, func(e *ValidationExecution) { e.Orchestration.Status = "other" }},
		{"final trace", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.FinalResultStatus = FinalResultPending
			e.Orchestration.FinalResultID, _ = ValidationExecutionResultID(e.ValidationID, e.Generation)
			e.Orchestration.FinalResultTrace = badTrace
		}},
		{"final id", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.FinalResultStatus = FinalResultPending
			e.Orchestration.FinalResultID = "wrong"
		}},
		{"pending final published time", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.FinalResultStatus = FinalResultPending
			e.Orchestration.FinalResultID, _ = ValidationExecutionResultID(e.ValidationID, e.Generation)
			e.Orchestration.FinalResultPublishedAt = canonicalStageTime(orchestrationTestTime)
		}},
		{"published final time", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.FinalResultStatus = FinalResultPublished
			e.Orchestration.FinalResultID, _ = ValidationExecutionResultID(e.ValidationID, e.Generation)
			e.Orchestration.FinalResultPublishedAt = "bad"
		}},
		{"final status", func(t *testing.T) ValidationExecution { return terminalOrchestration(t, StageOutcomePassed) }, func(e *ValidationExecution) {
			e.Orchestration.FinalResultStatus = "other"
			e.Orchestration.FinalResultID, _ = ValidationExecutionResultID(e.ValidationID, e.Generation)
		}},
		{"oversized trace", func(t *testing.T) ValidationExecution { return orchestrationTestExecution(t, 1) }, func(e *ValidationExecution) {
			e.Orchestration.InitialDispatchTrace = &observability.TraceCarrier{TraceParent: strings.Repeat("a", observability.MaxTraceParentBytes+1)}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			execution := test.base(t)
			test.mutate(&execution)
			if err := execution.Validate(); err == nil {
				t.Fatalf("corrupt orchestration state was accepted: %+v", execution.Orchestration)
			}
		})
	}
}

func TestOrchestrationTransitionRules(t *testing.T) {
	pending := orchestrationTestExecution(t, 1)
	queuedPending := queuedOrchestration(t, pending, false)
	queuedPublished := queuedOrchestration(t, pending, true)
	valid := cloneOrchestrationExecution(t, queuedPublished)
	stage := &valid.Orchestration.Stages[0]
	stage.Attempts = []ValidationStageAttempt{{
		Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
		DispatchedAt: stage.DispatchedAt, Outcome: StageOutcomePassed,
	}}
	stage.TerminalOutcome = StageOutcomePassed
	valid.Orchestration.Status = ExecutionStatusValid
	valid.Orchestration.FinalizedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
	valid.Orchestration.UpdatedAt = valid.Orchestration.FinalizedAt

	finalPending := cloneOrchestrationExecution(t, valid)
	finalPending.Orchestration.FinalResultStatus = FinalResultPending
	finalPending.Orchestration.FinalResultID, _ = ValidationExecutionResultID(valid.ValidationID, valid.Generation)
	finalPending.Orchestration.UpdatedAt = canonicalStageTime(orchestrationTestTime.Add(4 * time.Second))
	finalPublished := cloneOrchestrationExecution(t, finalPending)
	finalPublished.Orchestration.FinalResultStatus = FinalResultPublished
	finalPublished.Orchestration.FinalResultPublishedAt = canonicalStageTime(orchestrationTestTime.Add(5 * time.Second))
	finalPublished.Orchestration.UpdatedAt = finalPublished.Orchestration.FinalResultPublishedAt

	for name, pair := range map[string][2]ValidationExecution{
		"dispatch first stage": {pending, queuedPending},
		"record publication":   {queuedPending, queuedPublished},
		"accept final pass":    {queuedPublished, valid},
		"claim final result":   {valid, finalPending},
		"publish final result": {finalPending, finalPublished},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateOrchestrationTransition(pair[0], pair[1]); err != nil {
				t.Fatalf("valid transition rejected: %v", err)
			}
		})
	}

	bad := cloneOrchestrationExecution(t, valid)
	bad.Orchestration.UpdatedAt = canonicalStageTime(orchestrationTestTime)
	if err := validateOrchestrationTransition(valid, bad); err == nil {
		t.Fatal("backwards updatedAt was accepted")
	}
	bad = cloneOrchestrationExecution(t, valid)
	bad.Orchestration.Status = ExecutionStatusPending
	if err := validateOrchestrationTransition(valid, bad); err == nil {
		t.Fatal("terminal successor rewrite was accepted")
	}
	bad = cloneOrchestrationExecution(t, queuedPending)
	bad.Orchestration.Stages[0].AuthorizedAttempt = 2
	if err := validateOrchestrationTransition(pending, bad); err == nil {
		t.Fatal("invalid first dispatch was accepted")
	}
	bad = cloneOrchestrationExecution(t, queuedPublished)
	bad.Orchestration.Stages[0].RequestID = "different"
	if err := validateOrchestrationTransition(queuedPending, bad); err == nil {
		t.Fatal("invalid publication transition was accepted")
	}
	unsupported := cloneOrchestrationExecution(t, pending)
	unsupported.Orchestration.Status = "other"
	if err := validateOrchestrationTransition(unsupported, pending); err == nil {
		t.Fatal("unsupported current status was accepted")
	}
}

func TestQueuedOrchestrationTransitionOutcomes(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		current := queuedOrchestration(t, orchestrationTestExecution(t, 1), true)
		request := orchestrationRequest(t, current, 1, 2, orchestrationTestTime.Add(3*time.Second))
		next := cloneOrchestrationExecution(t, current)
		stage := &next.Orchestration.Stages[0]
		stage.Attempts = append(stage.Attempts, ValidationStageAttempt{
			Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
			DispatchedAt: stage.DispatchedAt, Outcome: StageOutcomeOperationalFailure, Reason: StageReasonTimeout,
		})
		stage.AuthorizedAttempt = 2
		stage.RequestID = request.RequestID
		stage.RequestCreatedAt = request.CreatedAt
		stage.DispatchStatus = StageDispatchPending
		stage.DispatchedAt = ""
		next.Orchestration.UpdatedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
		if err := next.Validate(); err != nil {
			t.Fatalf("retry state invalid: %v", err)
		}
		if err := validateOrchestrationTransition(current, next); err != nil {
			t.Fatalf("retry transition rejected: %v", err)
		}
	})

	for name, outcome := range map[string]StageOutcome{
		"standards invalid": StageOutcomeStandardsInvalid,
		"processing failed": StageOutcomeOperationalFailure,
	} {
		t.Run(name, func(t *testing.T) {
			current := queuedOrchestration(t, orchestrationTestExecution(t, 1), true)
			if outcome == StageOutcomeOperationalFailure {
				current.RetryPolicy.MaxAttempts = 1
			}
			next := cloneOrchestrationExecution(t, current)
			stage := &next.Orchestration.Stages[0]
			reason := StageReasonNone
			status := ExecutionStatusInvalid
			if outcome == StageOutcomeOperationalFailure {
				reason = StageReasonRuntimeUnavailable
				status = ExecutionStatusProcessingFailed
			}
			stage.Attempts = []ValidationStageAttempt{{
				Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
				DispatchedAt: stage.DispatchedAt, Outcome: outcome, Reason: reason,
			}}
			stage.TerminalOutcome = outcome
			next.Orchestration.Status = status
			next.Orchestration.FinalizedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
			next.Orchestration.UpdatedAt = next.Orchestration.FinalizedAt
			if err := next.Validate(); err != nil {
				t.Fatalf("terminal state invalid: %v", err)
			}
			if err := validateOrchestrationTransition(current, next); err != nil {
				t.Fatalf("terminal transition rejected: %v", err)
			}
		})
	}

	t.Run("next plan stage", func(t *testing.T) {
		current := queuedOrchestration(t, orchestrationTestExecution(t, 2), true)
		request := orchestrationRequest(t, current, 2, 1, orchestrationTestTime.Add(3*time.Second))
		next := cloneOrchestrationExecution(t, current)
		first := &next.Orchestration.Stages[0]
		first.Attempts = []ValidationStageAttempt{{
			Attempt: 1, RequestID: first.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", first.RequestID),
			DispatchedAt: first.DispatchedAt, Outcome: StageOutcomePassed,
		}}
		first.TerminalOutcome = StageOutcomePassed
		next.Orchestration.Stages = append(next.Orchestration.Stages, ValidationStageState{
			Stage: request.Stage, Ordinal: 2, AuthorizedAttempt: 1, RequestID: request.RequestID,
			RequestCreatedAt: request.CreatedAt, DispatchStatus: StageDispatchPending,
		})
		next.Orchestration.CurrentStageOrdinal = 2
		next.Orchestration.UpdatedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
		if err := next.Validate(); err != nil {
			t.Fatalf("advanced state invalid: %v", err)
		}
		if err := validateOrchestrationTransition(current, next); err != nil {
			t.Fatalf("advance transition rejected: %v", err)
		}
	})
}

func TestQueuedTransitionRejectsHistoryAndOutcomeRewrites(t *testing.T) {
	current := queuedOrchestration(t, orchestrationTestExecution(t, 1), true)
	terminal := cloneOrchestrationExecution(t, current)
	stage := &terminal.Orchestration.Stages[0]
	stage.Attempts = []ValidationStageAttempt{{
		Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
		DispatchedAt: stage.DispatchedAt, Outcome: StageOutcomePassed,
	}}
	stage.TerminalOutcome = StageOutcomePassed
	terminal.Orchestration.Status = ExecutionStatusValid
	terminal.Orchestration.FinalizedAt = canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
	terminal.Orchestration.UpdatedAt = terminal.Orchestration.FinalizedAt

	tests := []struct {
		name   string
		mutate func(*ValidationExecution)
	}{
		{"remove stage", func(e *ValidationExecution) { e.Orchestration.Stages = nil }},
		{"add two stages", func(e *ValidationExecution) {
			e.Orchestration.Stages = append(e.Orchestration.Stages, ValidationStageState{}, ValidationStageState{})
		}},
		{"stage identity", func(e *ValidationExecution) { e.Orchestration.Stages[0].Stage = ValidationLayerEN16931 }},
		{"no appended attempt", func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts = nil }},
		{"accepted attempt", func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].Attempt = 2 }},
		{"accepted request", func(e *ValidationExecution) { e.Orchestration.Stages[0].Attempts[0].RequestID = "other" }},
		{"dispatch evidence", func(e *ValidationExecution) {
			e.Orchestration.Stages[0].Attempts[0].DispatchedAt = canonicalStageTime(orchestrationTestTime)
		}},
		{"retry after pass", func(e *ValidationExecution) {
			e.Orchestration.Status = ExecutionStatusStageQueued
			e.Orchestration.Stages[0].TerminalOutcome = ""
			e.Orchestration.FinalizedAt = ""
		}},
		{"invalid after pass", func(e *ValidationExecution) {
			e.Orchestration.Status = ExecutionStatusInvalid
			e.Orchestration.Stages[0].TerminalOutcome = StageOutcomeStandardsInvalid
		}},
		{"failed after pass", func(e *ValidationExecution) {
			e.Orchestration.Status = ExecutionStatusProcessingFailed
			e.Orchestration.Stages[0].TerminalOutcome = StageOutcomeOperationalFailure
		}},
		{"unsupported status", func(e *ValidationExecution) { e.Orchestration.Status = "other" }},
		{"authorized request", func(e *ValidationExecution) { e.Orchestration.Stages[0].AuthorizedAttempt = 2 }},
		{"request created time", func(e *ValidationExecution) {
			e.Orchestration.Stages[0].RequestCreatedAt = canonicalStageTime(orchestrationTestTime)
		}},
		{"dispatch trace", func(e *ValidationExecution) {
			e.Orchestration.Stages[0].DispatchTrace = &observability.TraceCarrier{TraceParent: "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01"}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := cloneOrchestrationExecution(t, terminal)
			test.mutate(&candidate)
			if err := validateQueuedOrchestrationTransition(current, candidate); err == nil {
				t.Fatalf("invalid transition was accepted: %+v", candidate.Orchestration)
			}
		})
	}
}

func TestValidationEvidenceIdentityHelpers(t *testing.T) {
	execution := orchestrationTestExecution(t, 1)
	request := orchestrationRequest(t, execution, 1, 1, orchestrationTestTime)
	result, err := NewStageResult(request, StageOutcomePassed, StageReasonNone, nil, "", orchestrationTestTime, orchestrationTestTime.Add(time.Second), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	want := execution.UploadID + "." + execution.ValidationID + ".1.1.1"
	if got := validationStageEvidenceKey(result); got != want {
		t.Fatalf("evidence key = %q, want %q", got, want)
	}
	copy := execution
	copy.Orchestration.UpdatedAt = canonicalStageTime(orchestrationTestTime.Add(time.Minute))
	if !sameValidationExecutionIdentity(execution, copy) {
		t.Fatal("mutable orchestration changed immutable identity comparison")
	}
	copy.Plan.Version = "2"
	if sameValidationExecutionIdentity(execution, copy) {
		t.Fatal("different frozen plan compared as the same identity")
	}
}

func TestFrozenRetryPolicyStageOverrides(t *testing.T) {
	policy := FrozenValidationRetryPolicy{
		MaxAttempts: 9, XSDMaxAttempts: 1, EN16931MaxAttempts: 2, PeppolBISMaxAttempts: 3,
	}
	for stage, want := range map[ValidationLayer]uint32{
		ValidationLayerUBL21XSD: 1, ValidationLayerEN16931: 2, ValidationLayerPeppolBISBilling: 3,
		ValidationLayer("future"): 9,
	} {
		if got := policy.MaxAttemptsFor(stage); got != want {
			t.Errorf("MaxAttemptsFor(%q) = %d, want %d", stage, got, want)
		}
	}
	policy.XSDMaxAttempts = 0
	if got := policy.MaxAttemptsFor(ValidationLayerUBL21XSD); got != policy.MaxAttempts {
		t.Fatalf("zero override = %d, want inherited %d", got, policy.MaxAttempts)
	}
}

func TestNextFailedStageReplayGenerationStartsAtExhaustedStage(t *testing.T) {
	failed := terminalOrchestration(t, StageOutcomeOperationalFailure)
	failed.Orchestration.FinalResultStatus = FinalResultPublished
	failed.Orchestration.FinalResultID, _ = ValidationExecutionResultID(failed.ValidationID, failed.Generation)
	failed.Orchestration.FinalResultPublishedAt = canonicalStageTime(orchestrationTestTime.Add(4 * time.Second))
	request := orchestrationRequest(t, failed, 1, 1, orchestrationTestTime.Add(time.Second))
	result, err := NewStageResult(request, StageOutcomeOperationalFailure, StageReasonRuntimeFailure, nil, "runtime unavailable",
		orchestrationTestTime.Add(2*time.Second), orchestrationTestTime.Add(2*time.Second), 0)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := NewStageDeadLetter(result, 1, orchestrationTestTime.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	replayAt := orchestrationTestTime.Add(time.Hour)
	replay, err := NextFailedStageReplayGeneration(failed, dead, replayAt)
	if err != nil {
		t.Fatal(err)
	}
	if replay.Generation != 2 || replay.Trigger != ValidationTriggerReplay || replay.CreatedAt != canonicalStageTime(replayAt) {
		t.Fatalf("replay identity = %+v", replay)
	}
	if replay.Orchestration.Status != ExecutionStatusStageQueued || len(replay.Orchestration.Stages) != 1 ||
		replay.Orchestration.Stages[0].AuthorizedAttempt != 1 || replay.Orchestration.Stages[0].RequestCreatedAt != canonicalStageTime(replayAt) {
		t.Fatalf("replay orchestration = %+v", replay.Orchestration)
	}

	for name, mutate := range map[string]func(*ValidationExecution, *StageDeadLetter){
		"not failed":             func(e *ValidationExecution, _ *StageDeadLetter) { e.Orchestration.Status = ExecutionStatusValid },
		"no stage":               func(e *ValidationExecution, _ *StageDeadLetter) { e.Orchestration.CurrentStageOrdinal = 0 },
		"no local attempt":       func(e *ValidationExecution, _ *StageDeadLetter) { e.Orchestration.Stages[0].Attempts = nil },
		"mismatched dead letter": func(_ *ValidationExecution, d *StageDeadLetter) { d.Reason = StageReasonTimeout },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cloneOrchestrationExecution(t, failed)
			candidateDead := dead
			mutate(&candidate, &candidateDead)
			if _, err := NextFailedStageReplayGeneration(candidate, candidateDead, replayAt); err == nil {
				t.Fatal("invalid failed-stage replay was accepted")
			}
		})
	}
}

func TestFailedStageReplayCarriesLocalThenInheritedPrerequisiteEvidence(t *testing.T) {
	failed := orchestrationTestExecution(t, 2)
	failed.RetryPolicy.MaxAttempts = 1
	firstRequest := orchestrationRequest(t, failed, 1, 1, orchestrationTestTime.Add(time.Second))
	secondRequest := orchestrationRequest(t, failed, 2, 1, orchestrationTestTime.Add(2*time.Second))
	firstDispatched := canonicalStageTime(orchestrationTestTime.Add(3 * time.Second))
	secondDispatched := canonicalStageTime(orchestrationTestTime.Add(4 * time.Second))
	failed.Orchestration = ValidationOrchestrationState{
		Status: ExecutionStatusProcessingFailed, CurrentStageOrdinal: 2,
		Stages: []ValidationStageState{
			{
				Stage: firstRequest.Stage, Ordinal: 1, AuthorizedAttempt: 1, RequestID: firstRequest.RequestID,
				RequestCreatedAt: firstRequest.CreatedAt, DispatchStatus: StageDispatchPublished, DispatchedAt: firstDispatched,
				Attempts:        []ValidationStageAttempt{{Attempt: 1, RequestID: firstRequest.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", firstRequest.RequestID), DispatchedAt: firstDispatched, Outcome: StageOutcomePassed}},
				TerminalOutcome: StageOutcomePassed,
			},
			{
				Stage: secondRequest.Stage, Ordinal: 2, AuthorizedAttempt: 1, RequestID: secondRequest.RequestID,
				RequestCreatedAt: secondRequest.CreatedAt, DispatchStatus: StageDispatchPublished, DispatchedAt: secondDispatched,
				Attempts:        []ValidationStageAttempt{{Attempt: 1, RequestID: secondRequest.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", secondRequest.RequestID), DispatchedAt: secondDispatched, Outcome: StageOutcomeOperationalFailure, Reason: StageReasonRuntimeFailure}},
				TerminalOutcome: StageOutcomeOperationalFailure,
			},
		},
		UpdatedAt: canonicalStageTime(orchestrationTestTime.Add(5 * time.Second)), FinalizedAt: canonicalStageTime(orchestrationTestTime.Add(5 * time.Second)),
		FinalResultStatus: FinalResultPublished, FinalResultPublishedAt: canonicalStageTime(orchestrationTestTime.Add(6 * time.Second)),
	}
	failed.Orchestration.FinalResultID, _ = ValidationExecutionResultID(failed.ValidationID, failed.Generation)
	if err := failed.Validate(); err != nil {
		t.Fatalf("failed two-stage execution: %v", err)
	}
	secondResult, err := NewStageResult(secondRequest, StageOutcomeOperationalFailure, StageReasonRuntimeFailure, nil, "runtime failed",
		orchestrationTestTime.Add(4*time.Second), orchestrationTestTime.Add(4*time.Second), 0)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := NewStageDeadLetter(secondResult, 1, orchestrationTestTime.Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := NextFailedStageReplayGeneration(failed, dead, orchestrationTestTime.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(replay.Orchestration.Stages) != 2 || replay.Orchestration.Stages[0].InheritedEvidence == nil ||
		replay.Orchestration.Stages[0].InheritedEvidence.Generation != 1 || replay.Orchestration.Stages[1].Ordinal != 2 {
		t.Fatalf("first replay evidence = %+v", replay.Orchestration)
	}

	replayRequestAt, _ := time.Parse(time.RFC3339Nano, replay.Orchestration.Stages[1].RequestCreatedAt)
	replayRequest := orchestrationRequest(t, replay, 2, 1, replayRequestAt)
	dispatchedAt := canonicalStageTime(orchestrationTestTime.Add(time.Hour + time.Second))
	stage := &replay.Orchestration.Stages[1]
	stage.DispatchStatus = StageDispatchPublished
	stage.DispatchedAt = dispatchedAt
	stage.Attempts = []ValidationStageAttempt{{
		Attempt: 1, RequestID: stage.RequestID, ResultID: stageMessageID("e-ubl21/stage-result", stage.RequestID),
		DispatchedAt: dispatchedAt, Outcome: StageOutcomeOperationalFailure, Reason: StageReasonRuntimeFailure,
	}}
	stage.TerminalOutcome = StageOutcomeOperationalFailure
	replay.Orchestration.Status = ExecutionStatusProcessingFailed
	replay.Orchestration.FinalizedAt = canonicalStageTime(orchestrationTestTime.Add(time.Hour + 2*time.Second))
	replay.Orchestration.UpdatedAt = replay.Orchestration.FinalizedAt
	replay.Orchestration.FinalResultStatus = FinalResultPublished
	replay.Orchestration.FinalResultID, _ = ValidationExecutionResultID(replay.ValidationID, replay.Generation)
	replay.Orchestration.FinalResultPublishedAt = canonicalStageTime(orchestrationTestTime.Add(time.Hour + 3*time.Second))
	if err := replay.Validate(); err != nil {
		t.Fatalf("failed replay generation: %v", err)
	}
	replayResult, err := NewStageResult(replayRequest, StageOutcomeOperationalFailure, StageReasonRuntimeFailure, nil, "runtime failed",
		replayRequestAt, replayRequestAt, 0)
	if err != nil {
		t.Fatal(err)
	}
	replayDead, err := NewStageDeadLetter(replayResult, 1, orchestrationTestTime.Add(time.Hour+2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	third, err := NextFailedStageReplayGeneration(replay, replayDead, orchestrationTestTime.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if third.Generation != 3 || third.Orchestration.Stages[0].InheritedEvidence == nil || third.Orchestration.Stages[0].InheritedEvidence.Generation != 1 {
		t.Fatalf("inherited replay evidence = %+v", third.Orchestration)
	}
}
