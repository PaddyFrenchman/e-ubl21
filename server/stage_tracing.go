package server

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func beginStageTrace(ctx context.Context, request queue.StageRequest, message jetstream.Msg, handledAt time.Time) (context.Context, trace.Span) {
	ctx = observability.ExtractNATS(ctx, message.Headers())
	attributes := stageTraceAttributes(request)
	if metadata, err := message.Metadata(); err == nil {
		attributes = append(attributes, attribute.Int64("messaging.message.receive_count", int64(metadata.NumDelivered)))
	}
	if createdAt, err := time.Parse(time.RFC3339Nano, request.CreatedAt); err == nil && !handledAt.Before(createdAt) {
		_, waitSpan := observability.Tracer().Start(ctx, "queue.stage_request.wait",
			trace.WithTimestamp(createdAt), trace.WithAttributes(attributes...),
		)
		waitSpan.End(trace.WithTimestamp(handledAt))
	}
	return observability.Tracer().Start(ctx, "stage.request.process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithTimestamp(handledAt),
		trace.WithAttributes(attributes...),
	)
}

func stageTraceAttributes(request queue.StageRequest) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("upload.id", request.UploadID),
		attribute.String("validation.id", request.ValidationID),
		attribute.Int64("validation.generation", int64(request.Generation)),
		attribute.String("stage.name", string(request.Stage)),
		attribute.Int64("stage.ordinal", int64(request.StageOrdinal)),
		attribute.Int64("stage.attempt", int64(request.Attempt)),
		attribute.String("stage.request_id", request.RequestID),
	}
}

func beginStageOperation(ctx context.Context, name string, request queue.StageRequest) (context.Context, trace.Span) {
	return observability.Tracer().Start(ctx, name, trace.WithAttributes(stageTraceAttributes(request)...))
}

type tracedUploadContent struct {
	queue.UploadContentReader
	span      trace.Span
	once      sync.Once
	mu        sync.Mutex
	streamErr error
}

func traceUploadContent(content queue.UploadContentReader, span trace.Span) queue.UploadContentReader {
	return &tracedUploadContent{UploadContentReader: content, span: span}
}

func (content *tracedUploadContent) Read(buffer []byte) (int, error) {
	read, err := content.UploadContentReader.Read(buffer)
	if err != nil && err != io.EOF {
		content.remember(err)
		observability.RecordError(content.span, err)
	}
	return read, err
}

func (content *tracedUploadContent) Error() error {
	err := content.UploadContentReader.Error()
	if err != nil {
		content.remember(err)
		observability.RecordError(content.span, err)
	}
	return err
}

func (content *tracedUploadContent) Close() error {
	err := content.UploadContentReader.Close()
	if err != nil {
		observability.RecordError(content.span, err)
	}
	endErr := content.remembered()
	if endErr == nil {
		endErr = err
	}
	content.once.Do(func() { observability.End(content.span, endErr) })
	return err
}

func (content *tracedUploadContent) remember(err error) {
	content.mu.Lock()
	defer content.mu.Unlock()
	if content.streamErr == nil {
		content.streamErr = err
	}
}

func (content *tracedUploadContent) remembered() error {
	content.mu.Lock()
	defer content.mu.Unlock()
	return content.streamErr
}
