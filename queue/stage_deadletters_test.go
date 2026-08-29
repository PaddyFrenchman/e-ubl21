package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestStageDeadLetterIsPermanentPerExhaustedAttemptAndStageIsolated(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("stagedlq"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), testFrozenPlan(), time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 2, 3, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	result, err := queue.NewStageResult(request, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeUnavailable, nil, "Saxon unavailable", time.Now().UTC(), time.Now().UTC(), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := queue.NewStageDeadLetter(result, 4, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PublishStageDeadLetter(ctx, dead); err != nil {
		t.Fatal(err)
	}
	redelivered := dead
	redelivered.Deliveries = 5
	if err := client.PublishStageDeadLetter(ctx, redelivered); err != nil {
		t.Fatalf("redelivered identical dead letter was not idempotent: %v", err)
	}
	stored, err := client.GetStageDeadLetter(ctx, queue.ValidationLayerEN16931, dead.DeadLetterID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Deliveries != 4 || stored.Stage != queue.ValidationLayerEN16931 || stored.RulesetVersion != request.RulesetVersion || stored.Attempt != 3 || stored.Reason != queue.StageReasonRuntimeUnavailable {
		t.Fatalf("stored stage dead letter = %+v", stored)
	}
	conflict := dead
	conflict.Diagnostic = "different failure"
	if err := client.PublishStageDeadLetter(ctx, conflict); err == nil {
		t.Fatal("conflicting stage dead letter replaced permanent evidence")
	}
	if enDepth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerEN16931); err != nil || enDepth != 1 {
		t.Fatalf("EN dead-letter depth = %d, %v", enDepth, err)
	}
	if xsdDepth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerUBL21XSD); err != nil || xsdDepth != 0 {
		t.Fatalf("XSD dead-letter depth = %d, %v", xsdDepth, err)
	}
}

func TestStageDeadLetterRejectsStandardsInvalidEvidence(t *testing.T) {
	execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), testFrozenPlan(), time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 1, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	result, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone,
		[]queue.ValidationIssue{{Kind: queue.ValidationIssueXSDReport, Layer: queue.ValidationLayerUBL21XSD, Severity: "error", Message: "invalid"}}, "", time.Now().UTC(), time.Now().UTC(), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.NewStageDeadLetter(result, 1, time.Now().UTC()); err == nil {
		t.Fatal("standards-invalid evidence entered the operational stage DLQ")
	}
}
