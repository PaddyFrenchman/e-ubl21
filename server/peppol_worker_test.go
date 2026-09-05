package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go/jetstream"
)

type peppolValidatorFunc func(context.Context, io.Reader) (queue.ValidationStageResult, error)

func (fn peppolValidatorFunc) Validate(ctx context.Context, input io.Reader) (queue.ValidationStageResult, error) {
	return fn(ctx, input)
}

func TestPeppolWorkerConsumesOnlyAuthorizedThirdStage(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, request, message := authorizePeppolWorkerRequest(t, ctx, client, []byte(peppolWorkerIdentityDocument()))
	worker := newTestPeppolStageWorker(client, peppolValidatorFunc(func(_ context.Context, input io.Reader) (queue.ValidationStageResult, error) {
		if body, err := io.ReadAll(input); err != nil || string(body) != peppolWorkerIdentityDocument() {
			t.Fatalf("streamed content = %q, %v", body, err)
		}
		return queue.ValidationStageResult{
			Layer:   queue.ValidationLayerPeppolBISBilling,
			Version: validationassets.Peppol().ValidationArtefactsVersion,
			Valid:   true,
		}, nil
	}))
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	result := fetchPeppolWorkerResult(t, ctx, client)
	if result.RequestID != request.RequestID || result.StageOrdinal != 3 || result.Outcome != queue.StageOutcomePassed {
		t.Fatalf("Peppol result = %+v", result)
	}
	completed, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("final status = %s", completed.Execution.Orchestration.Status)
	}
	permanent, err := client.GetValidationExecutionResult(ctx, completed.Execution.ValidationID, completed.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if len(permanent.ValidationStages) != 3 || permanent.ValidationStages[2].ProcessProfile != queue.PeppolProcessProfile01 {
		t.Fatalf("permanent stages = %+v", permanent.ValidationStages)
	}
}

func TestPeppolOperationalFailureUsesIndependentRetry(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, request, message := authorizePeppolWorkerRequest(t, ctx, client, []byte(peppolWorkerIdentityDocument()))
	worker := newTestPeppolStageWorker(client, peppolValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, errors.New("Saxon crashed")
	}))
	if err := worker.handle(ctx, message); err != nil {
		t.Fatal(err)
	}
	failed := fetchPeppolWorkerResult(t, ctx, client)
	if failed.Outcome != queue.StageOutcomeOperationalFailure || failed.Reason != queue.StageReasonRuntimeFailure {
		t.Fatalf("operational result = %+v", failed)
	}
	pending, err := machine.HandleStageResult(ctx, failed)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued || pending.Execution.Orchestration.CurrentStageOrdinal != 3 {
		t.Fatalf("retry state = %+v", pending.Execution.Orchestration)
	}
	retry := fetchStageRequest(t, ctx, client, queue.ValidationLayerPeppolBISBilling)
	var retryRequest queue.StageRequest
	if err := retryRequest.UnmarshalJSON(retry.Data()); err != nil {
		t.Fatal(err)
	}
	if retryRequest.Attempt != request.Attempt+1 {
		t.Fatalf("retry attempt = %d", retryRequest.Attempt)
	}
	if err := retry.Ack(); err != nil {
		t.Fatal(err)
	}
}

func TestPeppolProfileAwareValidatorRejectsUnsupportedProfileID(t *testing.T) {
	validator := peppolProfileAwareValidator{delegate: peppolValidatorFunc(func(_ context.Context, input io.Reader) (queue.ValidationStageResult, error) {
		if _, err := io.Copy(io.Discard, input); err != nil {
			return queue.ValidationStageResult{}, err
		}
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerPeppolBISBilling, Version: validationassets.Peppol().ValidationArtefactsVersion, Valid: true,
		}, nil
	})}
	document := strings.Replace(peppolWorkerIdentityDocument(),
		"urn:fdc:peppol.eu:2017:poacc:billing:01:1.0", "urn:unsupported:profile", 1)
	result, err := validator.Validate(context.Background(), strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if result.Valid || len(result.Issues) != 1 || result.Issues[0].Rule != "EUBL21-PEPPOL-PROFILE-ID-UNSUPPORTED" ||
		result.Issues[0].Layer != queue.ValidationLayerPeppolBISBilling {
		t.Fatalf("unsupported ProfileID result = %+v", result)
	}
}

func TestPeppolWorkerScalesFromZeroAndStopsCleanly(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, _, pendingMessage := authorizePeppolWorkerRequest(t, ctx, client, []byte(peppolWorkerIdentityDocument()))
	if err := pendingMessage.Nak(); err != nil {
		t.Fatal(err)
	}
	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunPeppolWorker(workerCtx, PeppolWorkerConfig{
			queueClient: client, MetricsAddr: "", ValidationTimeout: 3 * time.Second,
			Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			validator: peppolValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{Layer: queue.ValidationLayerPeppolBISBilling, Version: validationassets.Peppol().ValidationArtefactsVersion, Valid: true}, nil
			}), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	result := fetchPeppolWorkerResult(t, ctx, client)
	completed, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("status = %s", completed.Execution.Orchestration.Status)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-peppol did not stop")
	}
}

func TestPeppolWorkerShutdownRequeuesActiveValidationForReplacement(t *testing.T) {
	client, ctx := newENWorkerTestClient(t)
	machine, _, pendingMessage := authorizePeppolWorkerRequest(t, ctx, client, []byte(peppolWorkerIdentityDocument()))
	if err := pendingMessage.Nak(); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	workerCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunPeppolWorker(workerCtx, PeppolWorkerConfig{
			queueClient: client, MetricsAddr: "", ValidationTimeout: 45 * time.Second,
			Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			validator: peppolValidatorFunc(func(ctx context.Context, _ io.Reader) (queue.ValidationStageResult, error) {
				close(entered)
				<-ctx.Done()
				return queue.ValidationStageResult{}, ctx.Err()
			}), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Peppol validation did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker-peppol did not stop during validation")
	}
	assertStageResultStreamMessages(t, ctx, client, queue.ValidationLayerPeppolBISBilling, 0)

	replacementCtx, stopReplacement := context.WithCancel(context.Background())
	replacementDone := make(chan error, 1)
	go func() {
		replacementDone <- RunPeppolWorker(replacementCtx, PeppolWorkerConfig{
			queueClient: client, MetricsAddr: "", ValidationTimeout: 3 * time.Second,
			Concurrency: 1, Prefetch: 1, ProgressInterval: 10 * time.Millisecond,
			validator: peppolValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				return queue.ValidationStageResult{Layer: queue.ValidationLayerPeppolBISBilling, Version: validationassets.Peppol().ValidationArtefactsVersion, Valid: true}, nil
			}), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()
	result := fetchPeppolWorkerResult(t, ctx, client)
	completed, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("replacement final status = %s", completed.Execution.Orchestration.Status)
	}
	stopReplacement()
	select {
	case err := <-replacementDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("replacement worker-peppol did not stop")
	}
}

func TestPeppolWorkerSettingsRespectProvisionedBounds(t *testing.T) {
	settings := resolvePeppolWorkerSettings(PeppolWorkerConfig{})
	if settings.validationTimeout != defaultPeppolValidationTimeout || settings.concurrency != 1 || settings.prefetch != 1 || settings.progressInterval != 20*time.Second {
		t.Fatalf("defaults = %+v", settings)
	}
	delivery := queue.StageDeliveryConfig{AckWait: 90 * time.Second, MaxAckPending: 20}
	settings = resolvePeppolWorkerSettings(PeppolWorkerConfig{Concurrency: 10, Prefetch: 10, ProgressInterval: 80 * time.Second})
	if err := validatePeppolWorkerCapacity(settings, delivery); err != nil {
		t.Fatal(err)
	}
	settings.concurrency, settings.prefetch = 20, 1
	if err := validatePeppolWorkerCapacity(settings, delivery); err == nil {
		t.Fatal("accepted capacity above MaxAckPending")
	}
	settings.concurrency, settings.prefetch, settings.progressInterval = 1, 1, 90*time.Second
	if err := validatePeppolWorkerCapacity(settings, delivery); err == nil {
		t.Fatal("accepted progress interval at AckWait")
	}
	settings.progressInterval, settings.validationTimeout = 20*time.Second, 90*time.Second
	if err := validatePeppolWorkerCapacity(settings, delivery); err == nil {
		t.Fatal("accepted validation timeout at AckWait")
	}
}

func authorizePeppolWorkerRequest(t *testing.T, ctx context.Context, client *queue.Client, content []byte) (*orchestrator.Machine, queue.StageRequest, jetstream.Msg) {
	t.Helper()
	uploadID := enWorkerTestID(t)
	if err := client.PutUploadContent(ctx, uploadID, content); err != nil {
		t.Fatal(err)
	}
	plan := queue.FrozenValidationPlan{ID: "peppol-bis-billing", Version: "1", Stages: []queue.FrozenValidationStage{
		{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
		{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.EN16931().Version},
		{ID: queue.ValidationLayerPeppolBISBilling, Ordinal: 3, Requirement: queue.ValidationStageRequired, RulesetVersion: validationassets.Peppol().ValidationArtefactsVersion},
	}}
	machine := orchestrator.New(client)
	if _, err := machine.Start(ctx, queue.ValidationExecutionIntent{
		IntentID: enWorkerTestID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfilePeppolBISBilling,
		Plan: plan, RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(), CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []queue.ValidationLayer{queue.ValidationLayerUBL21XSD, queue.ValidationLayerEN16931} {
		request := fetchAndAckStageRequest(t, ctx, client, stage)
		started := time.Now().UTC()
		result, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", started, started.Add(time.Millisecond), time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := machine.HandleStageResult(ctx, result); err != nil {
			t.Fatal(err)
		}
	}
	message := fetchStageRequest(t, ctx, client, queue.ValidationLayerPeppolBISBilling)
	var request queue.StageRequest
	if err := request.UnmarshalJSON(message.Data()); err != nil {
		t.Fatal(err)
	}
	return machine, request, message
}

func newTestPeppolStageWorker(client *queue.Client, validator PeppolValidator) *enStageWorker {
	clock := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	return &enStageWorker{
		queue: client, validator: validator, layer: queue.ValidationLayerPeppolBISBilling,
		rulesetVersion: validationassets.Peppol().ValidationArtefactsVersion,
		workerName:     "worker-peppol", supportsDocument: requiresEN16931,
		operationalReason: peppolStageOperationalReason, staleError: errPeppolStageRequestStale,
		timeout: 3 * time.Second, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		publishResult: client.PublishStageResult, publishDeadLetter: client.PublishStageDeadLetter,
		nakDelay: 10 * time.Millisecond,
		now:      func() time.Time { clock = clock.Add(time.Millisecond); return clock },
	}
}

func fetchPeppolWorkerResult(t *testing.T, ctx context.Context, client *queue.Client) queue.StageResult {
	t.Helper()
	consumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerPeppolBISBilling)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
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
	t.Fatal("no Peppol result received")
	return queue.StageResult{}
}

func peppolWorkerIdentityDocument() string {
	return `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"><cbc:CustomizationID>urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0</cbc:CustomizationID><cbc:ProfileID>urn:fdc:peppol.eu:2017:poacc:billing:01:1.0</cbc:ProfileID></Invoice>`
}
