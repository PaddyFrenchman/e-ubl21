package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel/trace"
)

func TestStageTraceContextSurvivesRequestRedeliveryAndResultPublication(t *testing.T) {
	server := queuetest.StartServer(t)
	client := connectStageInfraClient(t, server, queue.MustNamespace("trace"))
	ctx := context.Background()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	request := validStageRequestAtOrdinal(t, 1)
	traceID := trace.TraceID{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: trace.SpanID{8, 7, 6, 5, 4, 3, 2, 1}, TraceFlags: trace.FlagsSampled,
	})
	if err := client.PublishStageRequest(trace.ContextWithSpanContext(ctx, parent), request); err != nil {
		t.Fatal(err)
	}

	consumer, err := client.StageRequestConsumer(ctx, request.Stage)
	if err != nil {
		t.Fatal(err)
	}
	first := fetchOneTraceMessage(t, ctx, consumer)
	assertMessageTraceID(t, first, traceID)
	if err := first.Nak(); err != nil {
		t.Fatal(err)
	}
	redelivered := fetchOneTraceMessage(t, ctx, consumer)
	assertMessageTraceID(t, redelivered, traceID)

	result, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", time.Now().UTC(), time.Now().UTC(), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	resultCtx := observability.ExtractNATS(ctx, redelivered.Headers())
	if err := client.PublishStageResult(resultCtx, result); err != nil {
		t.Fatal(err)
	}
	if err := redelivered.Ack(); err != nil {
		t.Fatal(err)
	}
	resultConsumer, err := client.StageResultConsumer(ctx, result.Stage)
	if err != nil {
		t.Fatal(err)
	}
	resultMessage := fetchOneTraceMessage(t, ctx, resultConsumer)
	assertMessageTraceID(t, resultMessage, traceID)
	if err := resultMessage.Ack(); err != nil {
		t.Fatal(err)
	}
}

func fetchOneTraceMessage(t *testing.T, ctx context.Context, consumer jetstream.Consumer) jetstream.Msg {
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
	t.Fatal("no trace test message received")
	return nil
}

func assertMessageTraceID(t *testing.T, message jetstream.Msg, want trace.TraceID) {
	t.Helper()
	got := trace.SpanContextFromContext(observability.ExtractNATS(context.Background(), message.Headers()))
	if got.TraceID() != want || !got.IsRemote() {
		t.Fatalf("message trace context = %+v, want remote trace %s", got, want)
	}
}
