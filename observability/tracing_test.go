package observability_test

import (
	"context"
	"errors"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestNATSPropagationCarriesOnlyW3CTraceContext(t *testing.T) {
	traceID := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	traceState, err := trace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatal(err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, TraceState: traceState})
	member, err := baggage.NewMember("invoice.customer", "must-not-propagate")
	if err != nil {
		t.Fatal(err)
	}
	bag, err := baggage.New(member)
	if err != nil {
		t.Fatal(err)
	}
	ctx := baggage.ContextWithBaggage(trace.ContextWithSpanContext(context.Background(), spanContext), bag)
	header := nats.Header{}

	observability.InjectNATS(ctx, header)
	if header.Get("Traceparent") == "" {
		t.Fatal("traceparent was not injected")
	}
	if got := header.Get("Baggage"); got != "" {
		t.Fatalf("baggage header = %q, want omitted", got)
	}
	extracted := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), header))
	if extracted.TraceID() != traceID || extracted.SpanID() != spanID || extracted.TraceState().String() != traceState.String() || !extracted.IsRemote() {
		t.Fatalf("extracted span context = %+v, want remote %s/%s", extracted, traceID, spanID)
	}
}

func TestInvalidRemoteTraceContextIsIgnored(t *testing.T) {
	header := nats.Header{}
	header.Set("Traceparent", "caller-controlled-garbage")
	if got := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), header)); got.IsValid() {
		t.Fatalf("invalid traceparent produced valid context %+v", got)
	}
}

func TestOperationErrorsDoNotCopyDiagnosticsIntoTrace(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})

	_, span := observability.Tracer().Start(context.Background(), "sensitive.failure")
	observability.End(span, errors.New("customer VAT ID TOP-SECRET"))
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	if ended[0].Status().Code != codes.Error || ended[0].Status().Description != "operation failed" {
		t.Fatalf("span status = %+v", ended[0].Status())
	}
	if len(ended[0].Events()) != 0 {
		t.Fatalf("error produced trace events %+v", ended[0].Events())
	}
}
