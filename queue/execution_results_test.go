package queue_test

import (
	"context"
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

func TestExecutionValidationResultsArePermanentPerGenerationAndRejectConflicts(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := validExecutionResult(t, 1)
	if err := client.PublishResult(ctx, first); err != nil {
		t.Fatal(err)
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
