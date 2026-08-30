package observability_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestDurableTraceCarrierIsBoundedValidatedAndRestorable(t *testing.T) {
	traceID := trace.TraceID{9, 8, 7, 6, 5, 4, 3, 2, 1, 2, 3, 4, 5, 6, 7, 8}
	spanID := trace.SpanID{8, 6, 4, 2, 1, 3, 5, 7}
	state, err := trace.ParseTraceState("vendor=value")
	if err != nil {
		t.Fatal(err)
	}
	parent := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, TraceState: state})
	carrier := observability.CaptureTrace(trace.ContextWithSpanContext(context.Background(), parent))
	if carrier == nil || carrier.Validate() != nil {
		t.Fatalf("captured carrier = %+v", carrier)
	}

	type contextKey string
	base, cancel := context.WithTimeout(context.WithValue(context.Background(), contextKey("retained"), "yes"), time.Minute)
	defer cancel()
	restored := observability.RestoreTrace(base, carrier)
	got := trace.SpanContextFromContext(restored)
	if got.TraceID() != traceID || got.SpanID() != spanID || got.TraceState().String() != state.String() || !got.IsRemote() {
		t.Fatalf("restored context = %+v", got)
	}
	if restored.Value(contextKey("retained")) != "yes" {
		t.Fatal("restoring trace discarded context values")
	}
	if _, ok := restored.Deadline(); !ok {
		t.Fatal("restoring trace discarded context deadline")
	}

	invalid := []observability.TraceCarrier{
		{},
		{TraceState: "vendor=value"},
		{TraceParent: "invalid"},
		{TraceParent: carrier.TraceParent, TraceState: "invalid tracestate"},
		{TraceParent: strings.Repeat("a", observability.MaxTraceParentBytes+1)},
		{TraceParent: carrier.TraceParent, TraceState: strings.Repeat("a", observability.MaxTraceStateBytes+1)},
	}
	for _, candidate := range invalid {
		if err := candidate.Validate(); err == nil {
			t.Errorf("invalid carrier was accepted: %+v", candidate)
		}
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
