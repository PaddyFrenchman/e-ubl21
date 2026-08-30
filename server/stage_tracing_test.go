package server

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestXSDWorkerEmitsCorrelatedStageSpansWithoutInvoiceContent(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	client, baseCtx := newXSDWorkerTestClient(t)
	rootCtx, rootSpan := observability.Tracer().Start(baseCtx, "test.upload")
	const sensitiveXML = "TOP-SECRET-INVOICE-CONTENT"
	request := publishXSDWorkerTestRequest(t, rootCtx, client, []byte(sensitiveXML))
	message := fetchXSDWorkerRequest(t, rootCtx, client)
	worker := newTestXSDStageWorker(client, func(_ context.Context, _ string, body io.Reader, _ time.Duration) error {
		_, err := io.ReadAll(body)
		return err
	})
	createdAt, err := time.Parse(time.RFC3339Nano, request.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	clock := createdAt.Add(time.Second)
	worker.now = func() time.Time {
		clock = clock.Add(time.Millisecond)
		return clock
	}
	if err := worker.handle(rootCtx, message); err != nil {
		t.Fatal(err)
	}
	rootSpan.End()

	wanted := map[string]bool{
		"queue.stage_request.publish": false,
		"queue.stage_request.wait":    false,
		"stage.request.process":       false,
		"stage.xml.fetch":             false,
		"stage.validator.execute":     false,
		"queue.stage_result.publish":  false,
	}
	rootTraceID := rootSpan.SpanContext().TraceID()
	for _, span := range recorder.Ended() {
		if _, tracked := wanted[span.Name()]; tracked {
			wanted[span.Name()] = true
			if span.SpanContext().TraceID() != rootTraceID {
				t.Fatalf("span %q trace = %s, want %s", span.Name(), span.SpanContext().TraceID(), rootTraceID)
			}
		}
		if span.Name() == "queue.stage_request.wait" {
			if duration := span.EndTime().Sub(span.StartTime()); duration < time.Second || duration > 1100*time.Millisecond {
				t.Fatalf("queue-wait span duration = %s, want handler-entry delay near one second", duration)
			}
		}
		for _, attr := range span.Attributes() {
			if attr.Value.AsString() == sensitiveXML {
				t.Fatalf("span %q leaked invoice content in attribute %q", span.Name(), attr.Key)
			}
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("required span %q was not emitted", name)
		}
	}
}
