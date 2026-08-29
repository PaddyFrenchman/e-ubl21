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
		request := newXSDWorkerTestRequest(t)
		if err := client.PublishStageRequest(ctx, request); err != nil {
			t.Fatal(err)
		}
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
	publishXSDWorkerTestRequest(t, ctx, client, []byte("<Invoice/>"))
	message := fetchXSDWorkerRequest(t, ctx, client)
	worker := newTestXSDStageWorker(client, func(context.Context, string, io.Reader, time.Duration) error { return nil })
	publicationFailure := errors.New("result broker unavailable")
	worker.publishResult = func(context.Context, queue.StageResult) error { return publicationFailure }
	worker.nakDelay = 10 * time.Millisecond
	if err := worker.handle(ctx, message); !errors.Is(err, publicationFailure) {
		t.Fatalf("publication error = %v, want %v", err, publicationFailure)
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
	assertStageRequestStreamMessages(t, ctx, client, queue.ValidationLayerUBL21XSD, 0)
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
			queueClient: client, catalog: testXSDCatalog(), MetricsAddr: "",
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
	request := newXSDWorkerTestRequest(t)
	if err := client.PutUploadContent(ctx, request.UploadID, content); err != nil {
		t.Fatal(err)
	}
	if err := client.PublishStageRequest(ctx, request); err != nil {
		t.Fatal(err)
	}
	return request
}

func newXSDWorkerTestRequest(t *testing.T) queue.StageRequest {
	t.Helper()
	execution, err := queue.NewValidationExecution(queue.ValidationExecutionIntent{
		IntentID: testXSDWorkerID(t), UploadID: testXSDWorkerID(t), DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21,
		Plan: xsdWorkerTestPlan(false), CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 1, 1, time.Now().UTC())
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
