package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestXSDWorkerPublishesPassedResultBeforeAcknowledgingRequest(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	request := publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice>streamed</Invoice>"))
	message := fetchXSDWorkerRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(_ context.Context, schemaPath string, body io.Reader, timeout time.Duration) error {
		if schemaPath != "/schemas/maindoc/UBL-Invoice-2.1.xsd" || timeout != 3*time.Second {
			t.Fatalf("validator path/timeout = %q/%s", schemaPath, timeout)
		}
		content, err := io.ReadAll(body)
		if err != nil || string(content) != "<Invoice>streamed</Invoice>" {
			t.Fatalf("streamed content = %q, %v", content, err)
		}
		return nil
	})
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchXSDWorkerResult(t, ctx, client)
	if result.RequestID != request.RequestID || result.Outcome != queue.StageOutcomePassed || result.Reason != queue.StageReasonNone {
		t.Fatalf("XSD result = %+v", result)
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
}

func TestXSDWorkerClassifiesStandardsAndOperationalFailures(t *testing.T) {
	t.Run("standards invalid", func(t *testing.T) {
		client, ctx := newXSDWorkerTestClient(t)
		publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))
		message := fetchXSDWorkerRequest(t, ctx, client)
		worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error {
			return fmt.Errorf("%w: missing required ID", errSchemaStandardsInvalid)
		})
		if err := worker.handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		result := fetchXSDWorkerResult(t, ctx, client)
		if result.Outcome != queue.StageOutcomeStandardsInvalid || result.Reason != queue.StageReasonNone || len(result.Issues) != 1 || result.Issues[0].Kind != queue.ValidationIssueXSDReport {
			t.Fatalf("standards-invalid result = %+v", result)
		}
	})

	t.Run("content unavailable", func(t *testing.T) {
		client, ctx := newXSDWorkerTestClient(t)
		publishXSDWorkerTestRequestWithoutContent(t, ctx, client)
		message := fetchXSDWorkerRequest(t, ctx, client)
		worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error {
			t.Fatal("validator ran without upload content")
			return nil
		})
		if err := worker.handle(ctx, message); err != nil {
			t.Fatal(err)
		}
		result := fetchXSDWorkerResult(t, ctx, client)
		if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != queue.StageReasonContentUnavailable || !strings.Contains(result.Diagnostic, "open upload content") {
			t.Fatalf("content-unavailable result = %+v", result)
		}
	})
}

func TestXSDWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	completedBefore := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), string(queue.StageOutcomePassed)))
	publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))
	message := fetchXSDWorkerRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	publicationFailure := errors.New("result broker unavailable")
	worker.publishResult = func(context.Context, queue.StageResult) error { return publicationFailure }
	worker.nakDelay = 10 * time.Millisecond
	if err := worker.handle(ctx, message); !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v, want %v", err, publicationFailure)
	}
	if got := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), string(queue.StageOutcomePassed))) - completedBefore; got != 0 {
		t.Fatalf("completed XSD requests after failed publication = %v, want 0", got)
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 1)

	redelivered := fetchXSDWorkerRequest(t, ctx, client)
	metadata, err := redelivered.Metadata()
	if err != nil || metadata.NumDelivered < 2 {
		t.Fatalf("redelivery metadata = %+v, %v", metadata, err)
	}
	worker.publishResult = client.PublishStageResult
	if err := worker.handle(ctx, redelivered); err != nil {
		t.Fatal(err)
	}
	_ = fetchXSDWorkerResult(t, ctx, client)
	if got := prometheusCounterValue(t, stageRequestsTotal.WithLabelValues(string(queue.ValidationLayerUBL21XSD), string(queue.StageOutcomePassed))) - completedBefore; got != 1 {
		t.Fatalf("completed XSD requests after redelivery = %v, want 1", got)
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
}

func TestXSDWorkerPermanentDeadLetterFreezesExhaustedRedeliveryOutcome(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	uploadID := testXSDWorkerID(t)
	if err := client.PutUploadContent(ctx, uploadID, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	machine := orchestrator.New(client)
	if _, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21,
		Plan: xsdWorkerTestPlan(false), RetryPolicy: queue.FrozenValidationRetryPolicy{
			ID: queue.DefaultValidationRetryPolicyID, Version: queue.DefaultValidationRetryPolicyVersion, MaxAttempts: 1,
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	validatorCalls := 0
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error {
		validatorCalls++
		return errors.New("xmllint temporarily unavailable")
	})
	publicationFailure := errors.New("result broker unavailable")
	worker.publishResult = func(context.Context, queue.StageResult) error { return publicationFailure }
	message := fetchXSDWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, message); !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v, want %v", err, publicationFailure)
	}
	if depth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerUBL21XSD); err != nil || depth != 1 {
		t.Fatalf("XSD dead-letter depth = %d, %v; want 1", depth, err)
	}

	worker.validate = func(context.Context, string, io.Reader, time.Duration) error {
		validatorCalls++
		return nil
	}
	worker.publishResult = client.PublishStageResult
	redelivered := fetchXSDWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, redelivered); err != nil {
		t.Fatal(err)
	}
	if validatorCalls != 1 {
		t.Fatalf("validator calls = %d, want 1; permanent DLQ must prevent rerun", validatorCalls)
	}
	result := fetchXSDWorkerResult(t, ctx, client)
	if result.Outcome != queue.StageOutcomeOperationalFailure || result.Reason != queue.StageReasonRuntimeFailure {
		t.Fatalf("recovered XSD result = %+v", result)
	}
	terminal, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.Status != queue.ExecutionStatusProcessingFailed {
		t.Fatalf("terminal execution = %+v", terminal.Execution.Orchestration)
	}
	if depth, err := client.StageDeadLetterDepth(ctx, queue.ValidationLayerUBL21XSD); err != nil || depth != 1 {
		t.Fatalf("final XSD dead-letter depth = %d, %v; want 1", depth, err)
	}
}

func TestXSDWorkerShutdownCancelsValidationAndRedeliversRequest(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))
	started := make(chan struct{})
	workerCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() {
		done <- RunXSDWorker(workerCtx, XSDWorkerConfig{
			queueClient: client, catalog: testXSDCatalog(), MetricsAddr: "127.0.0.1:0",
			ValidationTimeout: time.Minute, Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			xsdValidate: func(ctx context.Context, _ string, _ io.Reader, _ time.Duration) error {
				close(started)
				<-ctx.Done()
				return ctx.Err()
			},
		})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("worker-xsd did not start validation")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-xsd did not stop after cancellation")
	}
	redelivered := fetchXSDWorkerRequest(t, ctx, client)
	if err := redelivered.Ack(); err != nil {
		t.Fatal(err)
	}
}

func TestXSDWorkerStartsWithActualNonDefaultRequestConsumerSettings(t *testing.T) {
	config := queue.DefaultStageQueueInfraConfig()
	for index := range config.Stages {
		if config.Stages[index].Stage != queue.ValidationLayerUBL21XSD {
			continue
		}
		config.Stages[index].RequestDelivery = queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
		config.Stages[index].ResultDelivery = queue.StageDeliveryConfig{AckWait: 120 * time.Second, MaxAckPending: 40}
	}
	client, ctx := newXSDWorkerTestClientWithConfig(t, config)
	publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))

	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunXSDWorker(workerCtx, XSDWorkerConfig{
			queueClient: client, catalog: testXSDCatalog(), MetricsAddr: "",
			ValidationTimeout: 3 * time.Second, Concurrency: 10, Prefetch: 10,
			ProgressInterval: 80 * time.Second,
			xsdValidate:      func(context.Context, string, io.Reader, time.Duration) error { return nil },
			Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	result := fetchXSDWorkerResult(t, ctx, client)
	if result.Outcome != queue.StageOutcomePassed {
		t.Fatalf("XSD result = %+v", result)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-xsd did not stop")
	}
}

func TestXSDWorkerFeedsOrchestratorWhileENWorkerIsStopped(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	uploadID := testXSDWorkerID(t)
	if err := client.PutUploadContent(ctx, uploadID, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	plan := xsdWorkerTestPlan(true)
	machine := orchestrator.New(client)
	started, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan: plan, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	message := fetchXSDWorkerRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchXSDWorkerResult(t, ctx, client)
	advanced, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Execution.ValidationID != started.Execution.ValidationID || advanced.Execution.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("advanced execution = %+v", advanced.Execution.Orchestration)
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerEN16931, 1)
}

func TestXSDWorkerUsesHardenedXMLLintAdapter(t *testing.T) {
	catalog, err := loadValidationCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		body        string
		wantOutcome queue.StageOutcome
	}{
		{name: "valid", body: validInvoiceXML, wantOutcome: queue.StageOutcomePassed},
		{name: "standards invalid", body: invalidInvoiceXML, wantOutcome: queue.StageOutcomeStandardsInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newXSDWorkerTestClient(t)
			publishXSDWorkerTestRequest(t, ctx, client, []byte(test.body))
			message := fetchXSDWorkerRequest(t, ctx, client)
			worker := newTestXSDStageWorker(client, validateReaderAgainstSchema)
			worker.catalog = catalog
			worker.timeout = 5 * time.Second
			if err := worker.handle(ctx, message); err != nil {
				t.Fatal(err)
			}
			result := fetchXSDWorkerResult(t, ctx, client)
			if result.Outcome != test.wantOutcome {
				t.Fatalf("real xmllint outcome = %q, want %q: %+v", result.Outcome, test.wantOutcome, result)
			}
		})
	}
}

func TestXSDWorkerOperationalResultsDriveIndependentRetryAndExhaustion(t *testing.T) {
	for _, test := range []struct {
		name       string
		secondPass bool
		wantStatus queue.ValidationExecutionStatus
	}{
		{name: "retry passes", secondPass: true, wantStatus: queue.ExecutionStatusValid},
		{name: "retry exhausted", secondPass: false, wantStatus: queue.ExecutionStatusProcessingFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, ctx := newXSDWorkerTestClient(t)
			if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
				t.Fatal(err)
			}
			uploadID := testXSDWorkerID(t)
			if err := client.PutUploadContent(ctx, uploadID, []byte("<Invoice/>")); err != nil {
				t.Fatal(err)
			}
			machine := orchestrator.New(client)
			started, err := machine.Start(ctx, queue.ValidationExecutionIntent{
				IntentID: testXSDWorkerID(t), UploadID: uploadID, DocumentType: "Invoice",
				Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21,
				Plan: xsdWorkerTestPlan(false),
				RetryPolicy: queue.FrozenValidationRetryPolicy{
					ID: queue.DefaultValidationRetryPolicyID, Version: queue.DefaultValidationRetryPolicyVersion, MaxAttempts: 2,
				},
				CreatedAt: time.Now().UTC(),
			})
			if err != nil {
				t.Fatal(err)
			}
			worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error {
				return errors.New("xmllint runtime crashed")
			})
			firstRequest := fetchXSDWorkerRequest(t, ctx, client)
			if err := worker.handle(ctx, firstRequest); err != nil {
				t.Fatal(err)
			}
			firstResult := fetchXSDWorkerResult(t, ctx, client)
			if firstResult.Outcome != queue.StageOutcomeOperationalFailure || firstResult.Reason != queue.StageReasonRuntimeFailure {
				t.Fatalf("first operational result = %+v", firstResult)
			}
			retrying, err := machine.HandleStageResult(ctx, firstResult)
			if err != nil {
				t.Fatal(err)
			}
			if retrying.Execution.Orchestration.Stages[0].AuthorizedAttempt != 2 {
				t.Fatalf("retry state = %+v", retrying.Execution.Orchestration)
			}

			if test.secondPass {
				worker.validate = func(context.Context, string, io.Reader, time.Duration) error { return nil }
			}
			secondRequest := fetchXSDWorkerRequest(t, ctx, client)
			if err := worker.handle(ctx, secondRequest); err != nil {
				t.Fatal(err)
			}
			secondResult := fetchXSDWorkerResult(t, ctx, client)
			deadDepth, depthErr := client.StageDeadLetterDepth(ctx, queue.ValidationLayerUBL21XSD)
			if depthErr != nil {
				t.Fatal(depthErr)
			}
			if test.secondPass && deadDepth != 0 {
				t.Fatalf("passing retry created %d XSD dead letters", deadDepth)
			}
			if !test.secondPass {
				if deadDepth != 1 {
					t.Fatalf("exhausted retry created %d XSD dead letters, want 1", deadDepth)
				}
				dead, deadErr := queue.NewStageDeadLetter(secondResult, 1, time.Now().UTC())
				if deadErr != nil {
					t.Fatal(deadErr)
				}
				stored, getErr := client.GetStageDeadLetter(ctx, queue.ValidationLayerUBL21XSD, dead.DeadLetterID)
				if getErr != nil || stored.Attempt != 2 || stored.Reason != queue.StageReasonRuntimeFailure || stored.RulesetVersion != secondResult.RulesetVersion {
					t.Fatalf("stored XSD dead letter = %+v, %v", stored, getErr)
				}
			}
			terminal, err := machine.HandleStageResult(ctx, secondResult)
			if err != nil {
				t.Fatal(err)
			}
			if terminal.Execution.ValidationID != started.Execution.ValidationID || terminal.Execution.Orchestration.Status != test.wantStatus {
				t.Fatalf("terminal execution = %+v, want %q", terminal.Execution.Orchestration, test.wantStatus)
			}
			assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
		})
	}
}

func TestXSDWorkerRecoversAfterTemporaryObjectStoreOutage(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	request := publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))
	if content, err := client.GetUploadContent(ctx, request.UploadID); err != nil || string(content) != "<Invoice/>" {
		t.Fatalf("content before Object Store outage = %q, %v", content, err)
	}
	machine := orchestrator.New(client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	backendFailure := errors.New("Object Store backend temporarily unavailable")
	backendAvailable := false
	worker.openContent = func(ctx context.Context, uploadID string) (queue.UploadContentReader, error) {
		if !backendAvailable {
			return nil, backendFailure
		}
		return client.OpenUploadContent(ctx, uploadID)
	}

	firstMessage := fetchXSDWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, firstMessage); err != nil {
		t.Fatal(err)
	}
	firstResult := fetchXSDWorkerResult(t, ctx, client)
	if firstResult.Outcome != queue.StageOutcomeOperationalFailure || firstResult.Reason != queue.StageReasonContentUnavailable {
		t.Fatalf("Object Store outage result = %+v", firstResult)
	}
	if !strings.Contains(firstResult.Diagnostic, backendFailure.Error()) {
		t.Fatalf("Object Store outage diagnostic = %q", firstResult.Diagnostic)
	}
	retrying, err := machine.HandleStageResult(ctx, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	if retrying.Execution.Orchestration.Stages[0].AuthorizedAttempt != 2 {
		t.Fatalf("Object Store recovery retry state = %+v", retrying.Execution.Orchestration)
	}

	backendAvailable = true
	if content, err := client.GetUploadContent(ctx, request.UploadID); err != nil || string(content) != "<Invoice/>" {
		t.Fatalf("content after Object Store recovery = %q, %v", content, err)
	}
	secondMessage := fetchXSDWorkerRequest(t, ctx, client)
	if err := worker.handle(ctx, secondMessage); err != nil {
		t.Fatal(err)
	}
	secondResult := fetchXSDWorkerResult(t, ctx, client)
	if secondResult.Outcome != queue.StageOutcomePassed || secondResult.Attempt != 2 {
		t.Fatalf("recovered Object Store result = %+v", secondResult)
	}
	terminal, err := machine.HandleStageResult(ctx, secondResult)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.Status != queue.ExecutionStatusValid || terminal.Execution.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("Object Store recovery terminal state = %+v", terminal.Execution.Orchestration)
	}
	if _, err := client.GetValidationExecutionResult(ctx, terminal.Execution.ValidationID, terminal.Execution.Generation); err != nil {
		t.Fatalf("Object Store recovery permanent result: %v", err)
	}
	assertPermanentResultStreamMessages(t, ctx, client, 1)
}

func TestXSDWorkerRejectsStalePreviousGenerationAfterRevalidation(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	machine := orchestrator.New(client)
	started, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: testXSDWorkerID(t), DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21,
		Plan: xsdWorkerTestPlan(false), RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, started.Execution.UploadID, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	requestCreatedAt, err := time.Parse(time.RFC3339Nano, started.Execution.Orchestration.Stages[0].RequestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	oldRequest, err := queue.NewStageRequest(started.Execution, 1, 1, requestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	oldMessage := fetchXSDWorkerRequest(t, ctx, client)
	if err := oldMessage.Ack(); err != nil {
		t.Fatal(err)
	}
	completedAt := time.Now().UTC()
	passed, err := queue.NewStageResult(oldRequest, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", completedAt, completedAt, 0)
	if err != nil {
		t.Fatal(err)
	}
	initialTerminal, err := machine.HandleStageResult(ctx, passed)
	if err != nil {
		t.Fatal(err)
	}
	if initialTerminal.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("initial terminal generation = %+v", initialTerminal.Execution.Orchestration)
	}
	evidenceBefore := validationEvidenceCount(t, ctx, client)
	revalidated, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: started.Execution.UploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerRevalidation, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		RevalidationOf: &queue.ValidationExecutionReference{
			ValidationID: started.Execution.ValidationID, Generation: started.Execution.Generation,
		},
		Plan: xsdWorkerTestPlan(true), RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(),
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	currentDelivery := fetchXSDWorkerRequest(t, ctx, client)
	var currentRequest queue.StageRequest
	if err := currentRequest.UnmarshalJSON(currentDelivery.Data()); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Generation != revalidated.Execution.Generation {
		t.Fatalf("queued current generation = %d, want %d", currentRequest.Generation, revalidated.Execution.Generation)
	}
	payload, err := oldRequest.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	names, err := client.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.Publish(ctx, names.RequestSubject, payload); err != nil {
		t.Fatal(err)
	}
	staleDelivery := fetchXSDWorkerRequest(t, ctx, client)
	validatorCalled := false
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error {
		validatorCalled = true
		return nil
	})
	if err := worker.handle(ctx, staleDelivery); !errors.Is(err, errXSDStageRequestStale) {
		t.Fatalf("previous-generation broker delivery error = %v, want stale", err)
	}
	if validatorCalled {
		t.Fatal("XSD validator ran for a stale previous-generation delivery")
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 1)
	assertStageResultStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
	if evidenceAfter := validationEvidenceCount(t, ctx, client); evidenceAfter != evidenceBefore {
		t.Fatalf("validation evidence count after stale delivery = %d, want %d", evidenceAfter, evidenceBefore)
	}
	storedOld, err := client.GetValidationExecution(ctx, oldRequest.UploadID, oldRequest.ValidationID, oldRequest.Generation)
	if err != nil || storedOld.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("stale request changed old generation = %+v, %v", storedOld.Orchestration, err)
	}

	if err := worker.handle(ctx, currentDelivery); err != nil {
		t.Fatalf("handle current revalidation delivery: %v", err)
	}
	if !validatorCalled {
		t.Fatal("XSD validator did not run for the current revalidation delivery")
	}
	xsdResult := fetchXSDWorkerResult(t, ctx, client)
	advanced, err := machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatal(err)
	}
	if advanced.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued || advanced.Execution.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("current revalidation after XSD = %+v", advanced.Execution.Orchestration)
	}
	enRequest := fetchAndAckStageRequest(t, ctx, client, queue.ValidationLayerEN16931)
	enCompletedAt := time.Now().UTC()
	enResult, err := queue.NewStageResult(enRequest, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", enCompletedAt, enCompletedAt, 0)
	if err != nil {
		t.Fatal(err)
	}
	currentTerminal, err := machine.HandleStageResult(ctx, enResult)
	if err != nil {
		t.Fatal(err)
	}
	if currentTerminal.Execution.Orchestration.Status != queue.ExecutionStatusValid || currentTerminal.Execution.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("current revalidation terminal state = %+v", currentTerminal.Execution.Orchestration)
	}
	if _, err := client.GetValidationExecutionResult(ctx, currentTerminal.Execution.ValidationID, currentTerminal.Execution.Generation); err != nil {
		t.Fatalf("current revalidation permanent result: %v", err)
	}
	storedOld, err = client.GetValidationExecution(ctx, oldRequest.UploadID, oldRequest.ValidationID, oldRequest.Generation)
	if err != nil || storedOld.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("current completion changed old generation = %+v, %v", storedOld.Orchestration, err)
	}
	assertPermanentResultStreamMessages(t, ctx, client, 2)
}

func TestXSDWorkerTerminatesMalformedOrMisroutedRequests(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	names, err := client.Namespace.StageQueue(queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.Publish(ctx, names.RequestSubject, []byte(`{"contractVersion":99}`)); err != nil {
		t.Fatal(err)
	}
	message := fetchXSDWorkerRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	if err := worker.handle(ctx, message); err == nil || !strings.Contains(err.Error(), "terminate malformed") {
		t.Fatalf("malformed request error = %v", err)
	}
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
}

func TestXSDWorkerSettingsRespectProvisionedConsumerBounds(t *testing.T) {
	settings, err := resolveXSDWorkerSettings(XSDWorkerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if settings.validationTimeout != defaultXSDWorkerValidationTimeout ||
		settings.concurrency != defaultXSDWorkerConcurrency ||
		settings.prefetch != defaultXSDWorkerPrefetch ||
		settings.progressInterval != defaultXSDWorkerProgressInterval {
		t.Fatalf("default XSD worker settings = %+v", settings)
	}

	delivery := queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
	settings, err = resolveXSDWorkerSettings(XSDWorkerConfig{Concurrency: 10, Prefetch: 10, ProgressInterval: 80 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateXSDWorkerCapacity(settings, delivery); err != nil {
		t.Fatalf("actual provisioned settings rejected: %v", err)
	}
	settings.concurrency = 20
	settings.prefetch = 1
	if err := validateXSDWorkerCapacity(settings, delivery); err == nil || !strings.Contains(err.Error(), "MaxAckPending") {
		t.Fatalf("capacity validation error = %v", err)
	}
	settings.concurrency = 1
	settings.prefetch = 1
	settings.progressInterval = 90 * time.Second
	if err := validateXSDWorkerCapacity(settings, delivery); err == nil || !strings.Contains(err.Error(), "AckWait") {
		t.Fatalf("progress validation error = %v", err)
	}
}

func TestXSDWorkerTreatsContentReadFailureAsOperationalEvenWithSchemaDiagnostics(t *testing.T) {
	err := errors.Join(errSchemaStandardsInvalid, errValidationContentStream)
	if !xsdOperationalFailure(context.Background(), err) {
		t.Fatal("schema diagnostics joined with a content read failure were classified as standards-invalid")
	}
	if reason := xsdStageOperationalReason(err); reason != queue.StageReasonContentUnavailable {
		t.Fatalf("operational reason = %q, want %q", reason, queue.StageReasonContentUnavailable)
	}
}

func TestXSDWorkerAuthorizationRejectsDurableIdentityDrift(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	request := startXSDWorkerTestRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	if _, err := worker.authorize(ctx, request); err != nil {
		t.Fatalf("current request rejected: %v", err)
	}
	for name, mutate := range map[string]func(*queue.StageRequest){
		"document":   func(r *queue.StageRequest) { r.DocumentType = "Order" },
		"plan":       func(r *queue.StageRequest) { r.Plan = "different" },
		"ordinal":    func(r *queue.StageRequest) { r.StageOrdinal = 2 },
		"stage":      func(r *queue.StageRequest) { r.Stage = queue.ValidationLayerEN16931 },
		"ruleset":    func(r *queue.StageRequest) { r.RulesetVersion = "future" },
		"attempt":    func(r *queue.StageRequest) { r.Attempt = 2 },
		"request id": func(r *queue.StageRequest) { r.RequestID = "different" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := request
			mutate(&candidate)
			if _, err := worker.authorize(ctx, candidate); !errors.Is(err, errXSDStageRequestStale) {
				t.Fatalf("identity drift error = %v", err)
			}
		})
	}
	missing := request
	missing.UploadID = testXSDWorkerID(t)
	if _, err := worker.authorize(ctx, missing); !errors.Is(err, errXSDStageRequestStale) {
		t.Fatalf("missing execution error = %v", err)
	}
}

func TestBoundedStageTextSanitizesInvalidUTF8(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		maximum int
	}{
		{name: "shorter than maximum", value: "xmllint \xff diagnostic", maximum: 64},
		{name: "requires truncation", value: "prefix \xff" + strings.Repeat("x", 64), maximum: 26},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := boundedStageText(test.value, test.maximum)
			if !utf8.ValidString(result) {
				t.Fatalf("bounded diagnostic is not valid UTF-8: %q", result)
			}
			if len(result) > test.maximum {
				t.Fatalf("bounded diagnostic has %d bytes, maximum %d: %q", len(result), test.maximum, result)
			}
			if !strings.Contains(result, "\uFFFD") {
				t.Fatalf("bounded diagnostic did not replace invalid UTF-8: %q", result)
			}
		})
	}
}

func newXSDWorkerTestClient(t *testing.T) (*queue.Client, context.Context) {
	return newXSDWorkerTestClientWithConfig(t, queue.DefaultStageQueueInfraConfig())
}

func newXSDWorkerTestClientWithConfig(t *testing.T, config queue.StageQueueInfraConfig) (*queue.Client, context.Context) {
	t.Helper()
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("workerxsd"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, config); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	return client, ctx
}

func newTestXSDStageWorker(client *queue.Client, validate xsdStreamValidator) *xsdStageWorker {
	clock := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	return &xsdStageWorker{
		queue: client, catalog: testXSDCatalog(), validate: validate, timeout: 3 * time.Second,
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)), publishResult: client.PublishStageResult,
		nakDelay: 10 * time.Millisecond,
		now: func() time.Time {
			clock = clock.Add(time.Millisecond)
			return clock
		},
	}
}

func testXSDCatalog() *validationCatalog {
	return &validationCatalog{
		documentCatalog: &documentCatalog{entryByElement: map[string]schemaEntry{
			"Invoice": {file: "UBL-Invoice-2.1.xsd", namespace: "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"},
		}},
		dir: "/schemas",
	}
}

func publishXSDWorkerTestRequest(t *testing.T, ctx context.Context, client *queue.Client, content []byte) queue.StageRequest {
	t.Helper()
	request := startXSDWorkerTestRequest(t, ctx, client)
	if err := client.PutUploadContent(ctx, request.UploadID, content); err != nil {
		t.Fatal(err)
	}
	return request
}

func publishXSDWorkerTestRequestWithoutContent(t *testing.T, ctx context.Context, client *queue.Client) queue.StageRequest {
	t.Helper()
	return startXSDWorkerTestRequest(t, ctx, client)
}

func startXSDWorkerTestRequest(t *testing.T, ctx context.Context, client *queue.Client) queue.StageRequest {
	t.Helper()
	started, err := orchestrator.New(client).Start(ctx, queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: testXSDWorkerID(t), DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21,
		Plan: xsdWorkerTestPlan(false), CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, started.Execution.Orchestration.Stages[0].RequestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(started.Execution, 1, 1, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func xsdWorkerTestPlan(withEN bool) queue.FrozenValidationPlan {
	stages := []queue.FrozenValidationStage{{
		ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: xsdschemas.Version,
	}}
	if withEN {
		stages = append(stages, queue.FrozenValidationStage{
			ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16",
		})
	}
	return queue.FrozenValidationPlan{ID: "worker-xsd-test", Version: "1", Stages: stages}
}

func testXSDWorkerID(t *testing.T) string {
	t.Helper()
	id, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func fetchXSDWorkerRequest(t *testing.T, ctx context.Context, client *queue.Client) jetstream.Msg {
	t.Helper()
	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for message := range batch.Messages() {
		return message
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("no XSD stage request received")
	return nil
}

func fetchXSDWorkerResult(t *testing.T, ctx context.Context, client *queue.Client) queue.StageResult {
	t.Helper()
	consumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	for message := range batch.Messages() {
		var result queue.StageResult
		if err := result.UnmarshalJSON(message.Data()); err != nil {
			t.Fatal(err)
		}
		if err := message.Ack(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("no XSD stage result received")
	return queue.StageResult{}
}

func assertStageRequestStreamMessages(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, want uint64) {
	t.Helper()
	names, err := client.Namespace.StageQueue(stage)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, names.RequestStream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != want {
		t.Fatalf("%s request stream messages = %d, want %d", stage, info.State.Msgs, want)
	}
}

func assertStageResultStreamMessages(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer, want uint64) {
	t.Helper()
	names, err := client.Namespace.StageQueue(stage)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, names.ResultStream)
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != want {
		t.Fatalf("%s result stream messages = %d, want %d", stage, info.State.Msgs, want)
	}
}

func validationEvidenceCount(t *testing.T, ctx context.Context, client *queue.Client) int {
	t.Helper()
	kv, err := client.JS.KeyValue(ctx, client.Namespace.ValidationStageEvidenceBucketName())
	if err != nil {
		t.Fatal(err)
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return 0
		}
		t.Fatal(err)
	}
	return len(keys)
}

func assertPermanentResultStreamMessages(t *testing.T, ctx context.Context, client *queue.Client, want uint64) {
	t.Helper()
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != want {
		t.Fatalf("permanent result stream messages = %d, want %d", info.State.Msgs, want)
	}
}
