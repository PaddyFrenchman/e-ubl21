package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func newTestClient(t *testing.T) *queue.Client {
	t.Helper()
	url := queuetest.StartServer(t)
	client, err := queue.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(client.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatalf("ensure infra: %v", err)
	}
	return client
}

func TestPublishDocumentAndConsume(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := queue.NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}
	receivedAt := time.Now()
	if err := client.PublishDocument(ctx, id, "Invoice", receivedAt, []byte("<Invoice/>")); err != nil {
		t.Fatalf("publish document: %v", err)
	}

	consumer, err := client.DocumentsConsumer(ctx, "test-worker", "")
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}

	received := make(chan jetstream.Msg, 1)
	cc, err := consumer.Consume(func(msg jetstream.Msg) { received <- msg })
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	defer cc.Stop()

	select {
	case msg := <-received:
		if got := queue.DocumentTypeFromSubject(msg.Subject()); got != "Invoice" {
			t.Fatalf("document type = %q, want %q", got, "Invoice")
		}
		if got := msg.Headers().Get(queue.HeaderUploadID); got != id {
			t.Fatalf("upload id header = %q, want %q", got, id)
		}
		if string(msg.Data()) != "<Invoice/>" {
			t.Fatalf("data = %q", msg.Data())
		}
		if err := msg.Ack(); err != nil {
			t.Fatalf("ack: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}

func TestDocumentsConsumerFiltersByType(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invoiceID, _ := queue.NewID()
	orderID, _ := queue.NewID()
	if err := client.PublishDocument(ctx, invoiceID, "Invoice", time.Now(), []byte("<Invoice/>")); err != nil {
		t.Fatalf("publish invoice: %v", err)
	}
	if err := client.PublishDocument(ctx, orderID, "Order", time.Now(), []byte("<Order/>")); err != nil {
		t.Fatalf("publish order: %v", err)
	}

	consumer, err := client.DocumentsConsumer(ctx, "invoice-only-worker", queue.DocumentSubject("Invoice"))
	if err != nil {
		t.Fatalf("consumer: %v", err)
	}

	received := make(chan jetstream.Msg, 2)
	cc, err := consumer.Consume(func(msg jetstream.Msg) { received <- msg })
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	defer cc.Stop()

	select {
	case msg := <-received:
		if got := queue.DocumentTypeFromSubject(msg.Subject()); got != "Invoice" {
			t.Fatalf("document type = %q, want %q (filter should exclude Order)", got, "Invoice")
		}
		msg.Ack()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for filtered message")
	}

	select {
	case msg := <-received:
		t.Fatalf("unexpected second message delivered: subject=%s", msg.Subject())
	case <-time.After(300 * time.Millisecond):
		// expected: the Order message must not reach this filtered consumer
	}
}

func TestUploadRecordLifecycle(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	record := queue.UploadRecord{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusPending,
		Content:      "<Invoice/>",
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatalf("put record: %v", err)
	}

	got, err := client.GetUploadRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if got.Status != queue.StatusPending {
		t.Fatalf("status = %q, want %q", got.Status, queue.StatusPending)
	}

	validatedAt := time.Now()
	if err := client.UpdateUploadStatus(ctx, id, queue.StatusInvalid, "missing required element", validatedAt); err != nil {
		t.Fatalf("update status: %v", err)
	}

	updated, err := client.GetUploadRecord(ctx, id)
	if err != nil {
		t.Fatalf("get updated record: %v", err)
	}
	if updated.Status != queue.StatusInvalid {
		t.Fatalf("status = %q, want %q", updated.Status, queue.StatusInvalid)
	}
	if updated.Error != "missing required element" {
		t.Fatalf("error = %q", updated.Error)
	}
	if updated.Content != "<Invoice/>" {
		t.Fatalf("content lost after update: %q", updated.Content)
	}
}

func TestGetUploadRecordNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.GetUploadRecord(ctx, "00000000000000000000000000000000"); err != queue.ErrRecordNotFound {
		t.Fatalf("err = %v, want %v", err, queue.ErrRecordNotFound)
	}
	if _, err := client.GetUploadRecord(ctx, "not-a-valid-id"); err != queue.ErrRecordNotFound {
		t.Fatalf("err = %v, want %v", err, queue.ErrRecordNotFound)
	}
}

func TestPublishResult(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	if err := client.PublishResult(ctx, queue.ValidationResult{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusValid,
		ValidatedAt:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("publish result: %v", err)
	}

	consumer, err := client.JS.CreateOrUpdateConsumer(ctx, queue.ResultsStreamName, jetstream.ConsumerConfig{
		Durable:       "results-test",
		FilterSubject: queue.ResultsSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("create results consumer: %v", err)
	}

	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	var count int
	for msg := range batch.Messages() {
		count++
		if got := msg.Headers().Get(queue.HeaderUploadID); got != id {
			t.Fatalf("upload id header = %q, want %q", got, id)
		}
		msg.Ack()
	}
	if count != 1 {
		t.Fatalf("received %d messages, want 1", count)
	}
}
