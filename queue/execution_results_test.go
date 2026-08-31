package queue_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestExecutionValidationResultRequiresCompleteFrozenPlanForValid(t *testing.T) {
	result := validExecutionResult(t, 1)
	result.PlanStages = append(result.PlanStages, queue.FrozenValidationStage{
		ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16",
	})
	if err := result.ValidateExecutionResult(); err == nil || !strings.Contains(err.Error(), "every frozen-plan stage") {
		t.Fatalf("partial valid result error = %v", err)
	}
}

func TestExecutionValidationResultLookupsFailClosed(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if found, err := client.HasValidationExecutionResult(ctx, "bad", 1); err == nil || found {
		t.Fatalf("invalid result identity = %t, %v", found, err)
	}
	validationID, _ := queue.NewID()
	if _, err := client.GetValidationExecutionResult(ctx, validationID, 1); err == nil {
		t.Fatal("missing execution result was returned")
	}

	for _, mode := range []string{"malformed", "invalid", "identity"} {
		t.Run(mode, func(t *testing.T) {
			client := newTestClient(t)
			ctx := context.Background()
			validationID, _ := queue.NewID()
			resultID, err := queue.ValidationExecutionResultID(validationID, 1)
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("{")
			if mode == "invalid" {
				payload = []byte(`{}`)
			} else if mode == "identity" {
				other := validExecutionResult(t, 1)
				payload, err = json.Marshal(other)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := client.JS.Publish(ctx, client.Namespace.ResultSubject(resultID), payload); err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetValidationExecutionResult(ctx, validationID, 1); err == nil {
				t.Fatal("corrupt execution result was returned")
			}
		})
	}
}

func TestExecutionValidationResultsArePermanentPerGenerationAndRejectConflicts(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := validExecutionResult(t, 1)
	if found, err := client.HasValidationExecutionResult(ctx, first.ValidationID, first.Generation); err != nil || found {
		t.Fatalf("result existed before publication: %v, %v", found, err)
	}
	if err := client.PublishResult(ctx, first); err != nil {
		t.Fatal(err)
	}
	if found, err := client.HasValidationExecutionResult(ctx, first.ValidationID, first.Generation); err != nil || !found {
		t.Fatalf("published result not found: %v, %v", found, err)
	}
	conflict := first
	conflict.ValidatedAt = "2026-08-29T11:00:01Z"
	if err := client.PublishResult(ctx, conflict); err == nil || !strings.Contains(err.Error(), "conflicting permanent validation result") {
		t.Fatalf("conflicting permanent result error = %v", err)
	}
	stored, err := client.GetValidationExecutionResult(ctx, first.ValidationID, first.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if !queue.SemanticallyEqualValidationResults(stored, first) {
		t.Fatalf("conflict replaced first result: got %+v, want %+v", stored, first)
	}

	second := validExecutionResult(t, 2)
	second.ID = first.ID
	second.ValidationID = first.ValidationID
	second.ResultID, err = queue.ValidationExecutionResultID(second.ValidationID, second.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PublishResult(ctx, second); err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 2 {
		t.Fatalf("two execution generations stored %d results, want 2", info.State.Msgs)
	}
}

func TestExecutionValidationResultRejectsMalformedTerminalEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*queue.ValidationResult)
	}{
		{"upload id", func(r *queue.ValidationResult) { r.ID = "bad" }},
		{"validation id", func(r *queue.ValidationResult) { r.ValidationID = "bad" }},
		{"result id", func(r *queue.ValidationResult) { r.ResultID = "bad" }},
		{"document type", func(r *queue.ValidationResult) { r.DocumentType = "bad type" }},
		{"profile", func(r *queue.ValidationResult) { r.Profile = "bad" }},
		{"plan", func(r *queue.ValidationResult) { r.Plan = "Bad Plan" }},
		{"validated at", func(r *queue.ValidationResult) { r.ValidatedAt = "bad" }},
		{"error bound", func(r *queue.ValidationResult) { r.Error = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1) }},
		{"no stages", func(r *queue.ValidationResult) { r.ValidationStages = nil }},
		{"stage plan mismatch", func(r *queue.ValidationResult) { r.ValidationStages[0].Layer = queue.ValidationLayerEN16931 }},
		{"stage version", func(r *queue.ValidationResult) { r.ValidationStages[0].Version = "" }},
		{"stage time", func(r *queue.ValidationResult) { r.ValidationStages[0].ValidatedAt = "bad" }},
		{"stage duration", func(r *queue.ValidationResult) { r.ValidationStages[0].DurationMS = -1 }},
		{"stage diagnostic", func(r *queue.ValidationResult) {
			r.ValidationStages[0].Diagnostic = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1)
		}},
		{"passed invalid", func(r *queue.ValidationResult) { r.ValidationStages[0].Valid = false }},
		{"passed reason", func(r *queue.ValidationResult) { r.ValidationStages[0].Reason = queue.StageReasonTimeout }},
		{"outcome", func(r *queue.ValidationResult) { r.ValidationStages[0].Outcome = "other" }},
		{"valid error", func(r *queue.ValidationResult) { r.Error = "failure" }},
		{"status", func(r *queue.ValidationResult) { r.Status = queue.StatusPending }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validExecutionResult(t, 1)
			test.mutate(&result)
			if err := result.ValidateExecutionResult(); err == nil {
				t.Fatalf("malformed result was accepted: %+v", result)
			}
		})
	}
}

func TestExecutionValidationResultAcceptsInvalidAndProcessingFailedOutcomes(t *testing.T) {
	invalid := validExecutionResult(t, 1)
	invalid.Status = queue.StatusInvalid
	invalid.ValidationStages[0].Valid = false
	invalid.ValidationStages[0].Outcome = queue.StageOutcomeStandardsInvalid
	invalid.ValidationStages[0].Diagnostic = "schema-invalid"
	if err := invalid.ValidateExecutionResult(); err != nil {
		t.Fatalf("invalid result rejected: %v", err)
	}

	failed := validExecutionResult(t, 1)
	failed.Status = queue.StatusProcessingFailed
	failed.Error = "runtime unavailable"
	failed.ValidationStages[0].Valid = false
	failed.ValidationStages[0].Outcome = queue.StageOutcomeOperationalFailure
	failed.ValidationStages[0].Reason = queue.StageReasonRuntimeUnavailable
	if err := failed.ValidateExecutionResult(); err != nil {
		t.Fatalf("processing-failed result rejected: %v", err)
	}
	failed.ValidationStages[0].Issues = []queue.ValidationIssue{{Layer: queue.ValidationLayerUBL21XSD, Severity: "error", Message: "bad"}}
	if err := failed.ValidateExecutionResult(); err == nil {
		t.Fatal("operational result with standards issues was accepted")
	}
}

func validExecutionResult(t *testing.T, generation uint64) queue.ValidationResult {
	t.Helper()
	uploadID, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	validationID, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	resultID, err := queue.ValidationExecutionResultID(validationID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return queue.ValidationResult{
		ID: uploadID, ResultID: resultID, ValidationID: validationID, Generation: generation,
		Profile: queue.ValidationProfileUBL21, Plan: "ubl21", PlanVersion: "1",
		PlanStages: []queue.FrozenValidationStage{{
			ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1",
		}},
		DocumentType: "Order", Status: queue.StatusValid, ValidatedAt: "2026-08-29T11:00:00Z",
		ValidationStages: queue.ValidationStages{{
			Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, Outcome: queue.StageOutcomePassed,
			ValidatedAt: "2026-08-29T10:59:59Z", DurationMS: 4,
		}},
	}
}
