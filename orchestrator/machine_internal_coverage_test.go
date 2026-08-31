package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func internalMachineExecution(t *testing.T) queue.ValidationExecution {
	t.Helper()
	uploadID, _ := queue.NewID()
	intentID, _ := queue.NewID()
	execution, err := queue.NewValidationExecution(queue.ValidationExecutionIntent{
		IntentID: intentID, UploadID: uploadID, DocumentType: "Invoice", Trigger: queue.ValidationTriggerInitial,
		RequestedProfile: queue.ValidationProfileUBL21, CreatedAt: time.Now().UTC(),
		Plan: queue.FrozenValidationPlan{ID: "internal-test", Version: "1", Stages: []queue.FrozenValidationStage{{
			ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1",
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return execution
}

func TestMachineValidationReportsEveryMissingDependency(t *testing.T) {
	if err := (*Machine)(nil).validate(); err == nil {
		t.Fatal("nil machine was accepted")
	}
	base := Machine{
		Queue: &queue.Client{}, Now: time.Now,
		PublishStageRequest:     func(context.Context, queue.StageRequest) error { return nil },
		PublishValidationResult: func(context.Context, queue.ValidationResult) error { return nil },
	}
	for name, mutate := range map[string]func(*Machine){
		"queue":             func(m *Machine) { m.Queue = nil },
		"clock":             func(m *Machine) { m.Now = nil },
		"request publisher": func(m *Machine) { m.PublishStageRequest = nil },
		"result publisher":  func(m *Machine) { m.PublishValidationResult = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if err := candidate.validate(); err == nil {
				t.Fatal("missing dependency was accepted")
			}
		})
	}
	if err := base.validate(); err != nil {
		t.Fatalf("complete machine rejected: %v", err)
	}
}

func TestMachineEntryPointsRejectMissingDependenciesAndMalformedInputs(t *testing.T) {
	var machine *Machine
	if _, err := machine.Start(context.Background(), queue.ValidationExecutionIntent{}); err == nil {
		t.Fatal("nil machine started an execution")
	}
	if _, err := machine.ReplayStage(context.Background(), queue.StageDeadLetter{}); err == nil {
		t.Fatal("nil machine replayed a stage")
	}
	if _, err := machine.EnsureDispatch(context.Background(), queue.VersionedValidationExecution{}); err == nil {
		t.Fatal("nil machine dispatched work")
	}
	if _, err := machine.HandleStageResult(context.Background(), queue.StageResult{}); err == nil {
		t.Fatal("nil machine handled a result")
	}
	if _, err := machine.EnsureFinalResult(context.Background(), queue.VersionedValidationExecution{}); err == nil {
		t.Fatal("nil machine finalized an execution")
	}

	complete := &Machine{
		Queue: &queue.Client{}, Now: time.Now,
		PublishStageRequest:     func(context.Context, queue.StageRequest) error { return nil },
		PublishValidationResult: func(context.Context, queue.ValidationResult) error { return nil },
	}
	if _, err := complete.Start(context.Background(), queue.ValidationExecutionIntent{}); err == nil {
		t.Fatal("malformed execution intent was accepted")
	}
	unsupported := internalMachineExecution(t)
	unsupported.Plan.Stages[0].ID = "unsupported"
	if err := complete.validatePlanTopology(unsupported); err == nil {
		t.Fatal("unsupported plan topology was accepted")
	}
	if _, err := complete.HandleStageResult(context.Background(), queue.StageResult{}); err == nil {
		t.Fatal("malformed stage result was accepted")
	}
	if _, err := complete.currentRequest(internalMachineExecution(t)); err == nil {
		t.Fatal("pending execution produced a current request")
	}
	malformedCurrent := internalMachineExecution(t)
	malformedCurrent.Orchestration.CurrentStageOrdinal = 1
	malformedCurrent.Orchestration.Stages = []queue.ValidationStageState{{RequestCreatedAt: "bad"}}
	if _, err := complete.currentRequest(malformedCurrent); err == nil {
		t.Fatal("malformed durable request time was accepted")
	}
	malformedCurrent.Orchestration.Stages[0].RequestCreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := complete.currentRequest(malformedCurrent); err == nil {
		t.Fatal("invalid durable execution produced a request")
	}
}

func TestResultMatchingAndExecutionCloneIsolation(t *testing.T) {
	execution := internalMachineExecution(t)
	result := queue.StageResult{
		UploadID: execution.UploadID, ValidationID: execution.ValidationID, Generation: execution.Generation,
		DocumentType: execution.DocumentType, Plan: execution.Plan.ID, PlanVersion: execution.Plan.Version,
		Stage: execution.Plan.Stages[0].ID, StageOrdinal: 1, RulesetVersion: execution.Plan.Stages[0].RulesetVersion,
	}
	if err := resultMatchesExecution(result, execution); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*queue.StageResult){
		"identity":      func(r *queue.StageResult) { r.UploadID = "different" },
		"zero ordinal":  func(r *queue.StageResult) { r.StageOrdinal = 0 },
		"large ordinal": func(r *queue.StageResult) { r.StageOrdinal = 2 },
		"stage":         func(r *queue.StageResult) { r.Stage = queue.ValidationLayerEN16931 },
		"ruleset":       func(r *queue.StageResult) { r.RulesetVersion = "future" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := result
			mutate(&candidate)
			if err := resultMatchesExecution(candidate, execution); err == nil {
				t.Fatal("mismatched result was accepted")
			}
		})
	}

	trace := &observability.TraceCarrier{TraceParent: "00-0102030405060708090a0b0c0d0e0f10-0102030405060708-01"}
	execution.Orchestration.InitialDispatchTrace = trace
	execution.Orchestration.FinalResultTrace = trace
	execution.Orchestration.Stages = []queue.ValidationStageState{{
		Attempts:          []queue.ValidationStageAttempt{{Attempt: 1}},
		InheritedEvidence: &queue.ValidationStageEvidenceReference{Attempt: 1}, DispatchTrace: trace,
	}}
	cloned := cloneExecution(execution)
	cloned.Plan.Stages[0].RulesetVersion = "changed"
	cloned.Orchestration.InitialDispatchTrace.TraceParent = "changed"
	cloned.Orchestration.FinalResultTrace.TraceParent = "changed"
	cloned.Orchestration.Stages[0].Attempts[0].Attempt = 2
	cloned.Orchestration.Stages[0].InheritedEvidence.Attempt = 2
	cloned.Orchestration.Stages[0].DispatchTrace.TraceParent = "changed"
	if execution.Plan.Stages[0].RulesetVersion == "changed" || execution.Orchestration.InitialDispatchTrace.TraceParent == "changed" ||
		execution.Orchestration.FinalResultTrace.TraceParent == "changed" || execution.Orchestration.Stages[0].Attempts[0].Attempt == 2 ||
		execution.Orchestration.Stages[0].InheritedEvidence.Attempt == 2 || execution.Orchestration.Stages[0].DispatchTrace.TraceParent == "changed" {
		t.Fatal("clone shared mutable execution state")
	}
}
