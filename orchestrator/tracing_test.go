package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestOrchestratorTransitionsAndFinalPublicationShareTrace(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	machine, client, baseCtx := newMachine(t)
	ctx, rootSpan := observability.Tracer().Start(baseCtx, "test.api.accept")
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	if _, err := machine.HandleStageResult(ctx, resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
		t.Fatal(err)
	}
	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	if _, err := machine.HandleStageResult(ctx, resultFor(t, enRequest, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
		t.Fatal(err)
	}
	rootSpan.End()

	wanted := map[string]bool{
		"orchestrator.start":                   false,
		"orchestrator.stage_request.queue":     false,
		"queue.stage_request.publish":          false,
		"orchestrator.stage_result.transition": false,
		"orchestrator.final_result.publish":    false,
	}
	rootTraceID := rootSpan.SpanContext().TraceID()
	for _, span := range recorder.Ended() {
		if _, tracked := wanted[span.Name()]; tracked {
			wanted[span.Name()] = true
			if span.SpanContext().TraceID() != rootTraceID {
				t.Fatalf("span %q trace = %s, want %s", span.Name(), span.SpanContext().TraceID(), rootTraceID)
			}
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("required span %q was not emitted", name)
		}
	}
}

func TestStageResultTraceIsPreservedWhenOrchestratorPublishesNextRequest(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	xsdResult := resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	traceID := trace.TraceID{2, 4, 6, 8, 10, 12, 14, 16, 1, 3, 5, 7, 9, 11, 13, 15}
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: trace.SpanID{1, 3, 5, 7, 2, 4, 6, 8}, TraceFlags: trace.FlagsSampled,
	})
	if err := client.PublishStageResult(trace.ContextWithSpanContext(ctx, parent), xsdResult); err != nil {
		t.Fatal(err)
	}
	resultConsumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	resultMessage := fetchTraceMessage(t, ctx, resultConsumer)
	if err := machine.HandleStageResultMessage(context.Background(), resultMessage); err != nil {
		t.Fatal(err)
	}
	nextConsumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatal(err)
	}
	nextRequest := fetchTraceMessage(t, ctx, nextConsumer)
	extracted := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), nextRequest.Headers()))
	if extracted.TraceID() != traceID {
		t.Fatalf("next request trace = %s, want %s", extracted.TraceID(), traceID)
	}
	if err := nextRequest.Ack(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingStageDispatchReconciliationRestoresDurableTrace(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	traceID := trace.TraceID{6, 0, 6, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	traceCtx := contextWithTraceID(ctx, traceID)
	machine.PublishStageRequest = func(context.Context, queue.StageRequest) error { return context.Canceled }
	pending, err := machine.HandleStageResult(traceCtx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if err == nil {
		t.Fatal("next-stage publication unexpectedly succeeded")
	}
	carrier := pending.Execution.Orchestration.Stages[1].DispatchTrace
	if carrier == nil || carrier.TraceParent == "" {
		t.Fatalf("pending dispatch has no durable trace: %+v", pending.Execution.Orchestration.Stages[1])
	}

	restarted := orchestrator.New(client)
	restarted.Now = machine.Now
	if report, err := restarted.ReconcilePendingDispatches(context.Background()); err != nil || report.Reconciled != 1 {
		t.Fatalf("dispatch reconciliation = %+v, %v", report, err)
	}
	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatal(err)
	}
	message := fetchTraceMessage(t, ctx, consumer)
	assertRecoveredTraceID(t, message, traceID)
	if err := message.Ack(); err != nil {
		t.Fatal(err)
	}
}

func TestPendingFinalResultReconciliationRestoresDurableTrace(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	if _, err := machine.Start(ctx, testIntent(t, plan)); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	traceID := trace.TraceID{6, 0, 6, 2, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	machine.PublishValidationResult = func(context.Context, queue.ValidationResult) error { return context.Canceled }
	pending, err := machine.HandleStageResult(contextWithTraceID(ctx, traceID), resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if err == nil {
		t.Fatal("final-result publication unexpectedly succeeded")
	}
	if pending.Execution.Orchestration.FinalResultTrace == nil {
		t.Fatal("pending final result has no durable trace")
	}

	restarted := orchestrator.New(client)
	restarted.Now = machine.Now
	var publishedTraceID trace.TraceID
	restarted.PublishValidationResult = func(publishCtx context.Context, result queue.ValidationResult) error {
		publishedTraceID = trace.SpanContextFromContext(publishCtx).TraceID()
		return client.PublishResult(publishCtx, result)
	}
	if report, err := restarted.ReconcilePendingFinalResults(context.Background()); err != nil || report.Reconciled != 1 {
		t.Fatalf("final-result reconciliation = %+v, %v", report, err)
	}
	if publishedTraceID != traceID {
		t.Fatalf("recovered final-result trace = %s, want %s", publishedTraceID, traceID)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	published, err := stream.GetLastMsgForSubject(ctx, client.Namespace.ResultSubject(pending.Execution.Orchestration.FinalResultID))
	if err != nil {
		t.Fatal(err)
	}
	got := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), published.Header))
	if got.TraceID() != traceID || !got.IsRemote() {
		t.Fatalf("recovered final-result message trace = %+v, want remote trace %s", got, traceID)
	}
}

func contextWithTraceID(ctx context.Context, traceID trace.TraceID) context.Context {
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: trace.SpanID{8, 7, 6, 5, 4, 3, 2, 1}, TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(ctx, parent)
}

func assertRecoveredTraceID(t *testing.T, message jetstream.Msg, want trace.TraceID) {
	t.Helper()
	got := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), message.Headers()))
	if got.TraceID() != want || !got.IsRemote() {
		t.Fatalf("recovered message trace = %+v, want remote trace %s", got, want)
	}
}

func fetchTraceMessage(t *testing.T, ctx context.Context, consumer jetstream.Consumer) jetstream.Msg {
	t.Helper()
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
	t.Fatal("no trace message received")
	return nil
}
