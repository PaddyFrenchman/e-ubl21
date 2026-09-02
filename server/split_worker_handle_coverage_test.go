package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	xsdschemas "github.com/PaddyFrenchman/e-ubl21/XSD"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go/jetstream"
)

func splitWorkerRequest(t *testing.T, stage queue.ValidationLayer, documentType, ruleset string) (queue.ValidationExecution, queue.StageRequest) {
	t.Helper()
	uploadID, _ := queue.NewID()
	intentID, _ := queue.NewID()
	profile := queue.ValidationProfileUBL21
	stages := []queue.FrozenValidationStage{{
		ID: queue.ValidationLayerUBL21XSD, Ordinal: 1,
		Requirement: queue.ValidationStageRequired, RulesetVersion: ruleset,
	}}
	ordinal := uint32(1)
	if stage == queue.ValidationLayerEN16931 {
		profile = queue.ValidationProfileUBL21EN16931
		stages[0].RulesetVersion = xsdschemas.Version
		stages = append(stages, queue.FrozenValidationStage{
			ID: queue.ValidationLayerEN16931, Ordinal: 2,
			Requirement: queue.ValidationStageRequired, RulesetVersion: ruleset,
		})
		ordinal = 2
	}
	execution, err := queue.NewValidationExecution(queue.ValidationExecutionIntent{
		IntentID: intentID, UploadID: uploadID, DocumentType: documentType, Trigger: queue.ValidationTriggerInitial,
		RequestedProfile: profile, CreatedAt: time.Now().UTC(),
		Plan: queue.FrozenValidationPlan{ID: "worker-coverage", Version: "1", Stages: stages},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, ordinal, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	return execution, request
}

func splitWorkerMessage(t *testing.T, request queue.StageRequest) *workerStateMsg {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return &workerStateMsg{data: payload, metadata: &jetstream.MsgMetadata{NumDelivered: 1}}
}

func TestXSDWorkerHandleControlAndPublicationFailures(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	execution, request := splitWorkerRequest(t, queue.ValidationLayerUBL21XSD, "Invoice", "future")
	newWorker := func() (*xsdStageWorker, *[]queue.StageResult) {
		published := []queue.StageResult{}
		clock := time.Now().UTC()
		worker := &xsdStageWorker{
			catalog: &validationCatalog{documentCatalog: &documentCatalog{entryByElement: map[string]schemaEntry{}}}, timeout: time.Second,
			logger: logger, nakDelay: time.Millisecond,
			now:              func() time.Time { clock = clock.Add(time.Millisecond); return clock },
			authorizeRequest: func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) { return execution, nil },
			publishResult: func(_ context.Context, result queue.StageResult) error {
				published = append(published, result)
				return nil
			},
		}
		return worker, &published
	}

	worker, published := newWorker()
	message := splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if message.acked != 1 || len(*published) != 1 || (*published)[0].Reason != queue.StageReasonRuntimeUnavailable {
		t.Fatalf("unsupported ruleset handling: message=%+v results=%+v", message, *published)
	}

	worker, _ = newWorker()
	worker.authorizeRequest = func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) {
		return queue.ValidationExecution{}, errors.New("store offline")
	}
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err == nil || message.delayed != 1 {
		t.Fatalf("authorization failure = %v, delayed=%d", err, message.delayed)
	}

	worker, _ = newWorker()
	worker.publishResult = func(context.Context, queue.StageResult) error { return errors.New("result offline") }
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err == nil || message.delayed != 1 {
		t.Fatalf("publication failure = %v, delayed=%d", err, message.delayed)
	}

	worker, _ = newWorker()
	message = splitWorkerMessage(t, request)
	message.ackErr = errors.New("ack failed")
	if err := worker.handle(context.Background(), message); err == nil || message.acked != 1 {
		t.Fatalf("ack failure = %v, acked=%d", err, message.acked)
	}

	worker, _ = newWorker()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	message = splitWorkerMessage(t, request)
	if err := worker.handle(canceled, message); err != nil || message.delayed != 1 {
		t.Fatalf("canceled handling = %v, delayed=%d", err, message.delayed)
	}

	_, enRequest := splitWorkerRequest(t, queue.ValidationLayerEN16931, "Invoice", validationassets.EN16931().Version)
	message = splitWorkerMessage(t, enRequest)
	if err := worker.handle(context.Background(), message); err == nil || message.terminated != 1 {
		t.Fatalf("misrouted handling = %v, terminated=%d", err, message.terminated)
	}

	execution, request = splitWorkerRequest(t, queue.ValidationLayerUBL21XSD, "Invoice", xsdschemas.Version)
	worker, published = newWorker()
	worker.authorizeRequest = func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) { return execution, nil }
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err != nil || len(*published) != 1 || (*published)[0].Reason != queue.StageReasonRuntimeUnavailable {
		t.Fatalf("missing schema handling = %v, results=%+v", err, *published)
	}
}

func TestENWorkerHandleControlAndPublicationFailures(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	execution, request := splitWorkerRequest(t, queue.ValidationLayerEN16931, "Invoice", "future")
	newWorker := func() (*enStageWorker, *[]queue.StageResult) {
		published := []queue.StageResult{}
		clock := time.Now().UTC()
		worker := &enStageWorker{
			timeout: time.Second, logger: logger, nakDelay: time.Millisecond,
			now:              func() time.Time { clock = clock.Add(time.Millisecond); return clock },
			authorizeRequest: func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) { return execution, nil },
			publishResult: func(_ context.Context, result queue.StageResult) error {
				published = append(published, result)
				return nil
			},
		}
		return worker, &published
	}

	worker, published := newWorker()
	message := splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err != nil || message.acked != 1 || len(*published) != 1 || (*published)[0].Reason != queue.StageReasonRuntimeUnavailable {
		t.Fatalf("unsupported ruleset handling = %v, message=%+v results=%+v", err, message, *published)
	}
	worker, _ = newWorker()
	worker.authorizeRequest = func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) {
		return queue.ValidationExecution{}, errors.New("store offline")
	}
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err == nil || message.delayed != 1 {
		t.Fatalf("authorization failure = %v, delayed=%d", err, message.delayed)
	}
	worker, _ = newWorker()
	worker.publishResult = func(context.Context, queue.StageResult) error { return errors.New("result offline") }
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err == nil || message.delayed != 1 {
		t.Fatalf("publication failure = %v, delayed=%d", err, message.delayed)
	}
	worker, _ = newWorker()
	message = splitWorkerMessage(t, request)
	message.ackErr = errors.New("ack failed")
	if err := worker.handle(context.Background(), message); err == nil {
		t.Fatal("ack failure was ignored")
	}
	worker, _ = newWorker()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	message = splitWorkerMessage(t, request)
	if err := worker.handle(canceled, message); err != nil || message.delayed != 1 {
		t.Fatalf("canceled handling = %v, delayed=%d", err, message.delayed)
	}
	_, xsdRequest := splitWorkerRequest(t, queue.ValidationLayerUBL21XSD, "Invoice", xsdschemas.Version)
	message = splitWorkerMessage(t, xsdRequest)
	if err := worker.handle(context.Background(), message); err == nil || message.terminated != 1 {
		t.Fatalf("misrouted handling = %v, terminated=%d", err, message.terminated)
	}

	execution, request = splitWorkerRequest(t, queue.ValidationLayerEN16931, "Order", validationassets.EN16931().Version)
	worker, published = newWorker()
	worker.authorizeRequest = func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) { return execution, nil }
	message = splitWorkerMessage(t, request)
	if err := worker.handle(context.Background(), message); err != nil || len(*published) != 1 || (*published)[0].Reason != queue.StageReasonRuntimeUnavailable {
		t.Fatalf("unsupported document handling = %v, results=%+v", err, *published)
	}
}
