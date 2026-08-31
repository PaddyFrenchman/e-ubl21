package queue_test

import (
	"context"
	"strings"
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
	if found, ok, err := client.StageDeadLetterForRequest(ctx, request); err != nil || ok || found.DeadLetterID != "" {
		t.Fatalf("missing dead letter = %+v, %t, %v", found, ok, err)
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
	derivedID, err := queue.StageDeadLetterIDForRequest(request)
	if err != nil || derivedID != dead.DeadLetterID {
		t.Fatalf("derived dead-letter ID = %q, %v; want %q", derivedID, err, dead.DeadLetterID)
	}
	found, ok, err := client.StageDeadLetterForRequest(ctx, request)
	if err != nil || !ok || found != stored {
		t.Fatalf("dead letter for request = %+v, %t, %v; want %+v", found, ok, err, stored)
	}
	recovered, err := found.StageResultForRequest(request)
	if err != nil || !queue.SemanticallyEqualStageResults(recovered, result) {
		t.Fatalf("recovered stage result = %+v, %v; want semantic match for %+v", recovered, err, result)
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
	if err := client.DeleteStageDeadLetter(ctx, queue.ValidationLayerEN16931, dead.DeadLetterID); err != nil {
		t.Fatal(err)
	}
	if enDepth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerEN16931); err != nil || enDepth != 0 {
		t.Fatalf("deleted EN dead-letter depth = %d, %v", enDepth, err)
	}
	if err := client.DeleteStageDeadLetter(ctx, queue.ValidationLayerEN16931, "bad"); err == nil {
		t.Fatal("invalid dead-letter deletion identity was accepted")
	}
	if _, err := client.GetStageDeadLetter(ctx, queue.ValidationLayerEN16931, "bad"); err == nil {
		t.Fatal("invalid dead-letter lookup identity was accepted")
	}
	if _, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayer("unknown")); err == nil {
		t.Fatal("unknown stage dead-letter depth was accepted")
	}
	invalidDead := dead
	invalidDead.ContractVersion++
	if err := client.PublishStageDeadLetter(ctx, invalidDead); err == nil {
		t.Fatal("invalid dead letter was published")
	}
}

func TestStageDeadLetterValidationRejectsIdentityAndEvidenceDrift(t *testing.T) {
	execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), testFrozenPlan(), time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 1, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	result, err := queue.NewStageResult(request, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure, nil, "failure", time.Now().UTC(), time.Now().UTC(), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	dead, err := queue.NewStageDeadLetter(result, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*queue.StageDeadLetter){
		"contract":   func(d *queue.StageDeadLetter) { d.ContractVersion++ },
		"identity":   func(d *queue.StageDeadLetter) { d.DeadLetterID = "bad" },
		"generation": func(d *queue.StageDeadLetter) { d.Generation = 0 },
		"reason":     func(d *queue.StageDeadLetter) { d.Reason = "other" },
		"diagnostic": func(d *queue.StageDeadLetter) { d.Diagnostic = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1) },
		"failed at":  func(d *queue.StageDeadLetter) { d.FailedAt = "bad" },
		"request":    func(d *queue.StageDeadLetter) { d.RequestID = "bad" },
		"result":     func(d *queue.StageDeadLetter) { d.ResultID = "bad" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := dead
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("malformed dead letter was accepted: %+v", candidate)
			}
		})
	}

	wrongRequest := request
	wrongRequest.Attempt = 2
	if _, err := dead.StageResultForRequest(wrongRequest); err == nil {
		t.Fatal("dead letter reconstructed a different request")
	}
	invalidDead := dead
	invalidDead.ContractVersion++
	if _, err := invalidDead.StageResultForRequest(request); err == nil {
		t.Fatal("invalid dead letter reconstructed evidence")
	}
	if _, err := queue.StageDeadLetterIDForRequest(queue.StageRequest{}); err == nil {
		t.Fatal("invalid request produced a dead-letter ID")
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
