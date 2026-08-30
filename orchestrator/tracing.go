package orchestrator

import (
	"context"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func startExecutionSpan(ctx context.Context, name string, execution queue.ValidationExecution, options ...trace.SpanStartOption) (context.Context, trace.Span) {
	attributes := []attribute.KeyValue{
		attribute.String("upload.id", execution.UploadID),
		attribute.String("validation.id", execution.ValidationID),
		attribute.Int64("validation.generation", int64(execution.Generation)),
	}
	options = append(options, trace.WithAttributes(attributes...))
	return observability.Tracer().Start(ctx, name, options...)
}

func startResultSpan(ctx context.Context, name string, result queue.StageResult, options ...trace.SpanStartOption) (context.Context, trace.Span) {
	attributes := []attribute.KeyValue{
		attribute.String("upload.id", result.UploadID),
		attribute.String("validation.id", result.ValidationID),
		attribute.Int64("validation.generation", int64(result.Generation)),
		attribute.String("stage.name", string(result.Stage)),
		attribute.Int64("stage.ordinal", int64(result.StageOrdinal)),
		attribute.Int64("stage.attempt", int64(result.Attempt)),
		attribute.String("stage.request_id", result.RequestID),
		attribute.String("stage.result_id", result.ResultID),
		attribute.String("stage.outcome", string(result.Outcome)),
	}
	options = append(options, trace.WithAttributes(attributes...))
	return observability.Tracer().Start(ctx, name, options...)
}
