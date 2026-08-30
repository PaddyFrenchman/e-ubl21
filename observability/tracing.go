// Package observability owns the process-wide distributed-tracing contract.
// It propagates only W3C trace context; baggage is deliberately excluded so
// caller-controlled business data cannot cross durable queue boundaries.
package observability

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/PaddyFrenchman/e-ubl21"

var traceContext = propagation.TraceContext{}

// ConfigureTracing installs a process tracer provider. Standard
// OTEL_EXPORTER_OTLP_* variables enable OTLP/HTTP export; without an endpoint,
// spans are still assigned usable trace IDs for logs and queue correlation but
// are intentionally not exported.
func ConfigureTracing(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		serviceName = "e-ubl21"
	}
	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.name", serviceName)),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, err
	}
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	}
	if otlpEndpointConfigured() {
		exporter, exporterErr := otlptracehttp.New(ctx)
		if exporterErr != nil {
			return nil, exporterErr
		}
		options = append(options, sdktrace.WithBatcher(exporter))
	}
	provider := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(traceContext)
	return provider.Shutdown, nil
}

func otlpEndpointConfigured() bool {
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")) != ""
}

// Tracer returns the shared instrumentation tracer.
func Tracer() trace.Tracer {
	return otel.Tracer(instrumentationName)
}

// ExtractHTTP accepts a valid remote W3C parent from HTTP headers. Invalid
// values are ignored by the standard propagator and become a new local trace.
func ExtractHTTP(ctx context.Context, header http.Header) context.Context {
	return traceContext.Extract(ctx, propagation.HeaderCarrier(header))
}

// InjectHTTP exposes the current trace context to an HTTP peer or response.
func InjectHTTP(ctx context.Context, header http.Header) {
	traceContext.Inject(ctx, propagation.HeaderCarrier(header))
}

// ExtractNATS restores W3C context from a durable NATS message.
func ExtractNATS(ctx context.Context, header nats.Header) context.Context {
	return traceContext.Extract(ctx, propagation.HeaderCarrier(header))
}

// InjectNATS writes only traceparent/tracestate into a NATS message header.
func InjectNATS(ctx context.Context, header nats.Header) {
	traceContext.Inject(ctx, propagation.HeaderCarrier(header))
}

// CorrelationFields returns stable identifiers suitable for structured logs.
func CorrelationFields(ctx context.Context) (traceID, spanID string) {
	spanContext := trace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return "", ""
	}
	return spanContext.TraceID().String(), spanContext.SpanID().String()
}

// RecordError marks an operation failed without recording err.Error().
// Validator and broker diagnostics may contain business content, so arbitrary
// error text must not become an exception event or trace attribute.
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.SetStatus(codes.Error, "operation failed")
}

// End records err, when present, and ends span.
func End(span trace.Span, err error) {
	RecordError(span, err)
	span.End()
}

// ShutdownAll combines tracing shutdown with another cleanup without losing
// either failure.
func ShutdownAll(ctx context.Context, shutdowns ...func(context.Context) error) error {
	var result error
	for _, shutdown := range shutdowns {
		if shutdown != nil {
			result = errors.Join(result, shutdown(ctx))
		}
	}
	return result
}
