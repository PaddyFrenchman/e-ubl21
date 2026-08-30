package orchestrator_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
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
