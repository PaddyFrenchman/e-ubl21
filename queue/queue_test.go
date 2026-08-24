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
	return newTestClientWithNamespace(t, queuetest.StartServer(t), queue.DefaultNamespace)
}

// newTestClientWithNamespace connects to the given NATS URL scoped to
// namespace, letting a test start multiple namespace-scoped clients against
// the same embedded server (see TestNamespacesDoNotCrossConsume).
func newTestClientWithNamespace(t *testing.T, url string, namespace queue.Namespace) *queue.Client {
	t.Helper()
	client, err := queue.Connect(url, namespace)
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
		if got := client.Namespace.DocumentTypeFromSubject(msg.Subject()); got != "Invoice" {
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

	consumer, err := client.DocumentsConsumer(ctx, "invoice-only-worker", client.Namespace.DocumentSubject("Invoice"))
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
		if got := client.Namespace.DocumentTypeFromSubject(msg.Subject()); got != "Invoice" {
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

	consumer, err := client.JS.CreateOrUpdateConsumer(ctx, client.Namespace.ResultsStreamName(), jetstream.ConsumerConfig{
		Durable:       "results-test",
		FilterSubject: client.Namespace.ResultsSubject(),
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

// TestNamespacesDoNotCrossConsume proves the acceptance criteria for
// TODO-001: two e-ubl21 deployments configured with different queue
// namespaces can share the same JetStream server without a worker or
// upload-status read/write in one namespace ever observing the other
// namespace's documents, results or upload records.
func TestNamespacesDoNotCrossConsume(t *testing.T) {
	t.Parallel()
	url := queuetest.StartServer(t)
	dev := newTestClientWithNamespace(t, url, queue.MustNamespace("eubl21.dev"))
	prod := newTestClientWithNamespace(t, url, queue.MustNamespace("eubl21.prod"))

	if dev.Namespace.DocumentsStreamName() == prod.Namespace.DocumentsStreamName() {
		t.Fatal("dev and prod must derive distinct documents stream names")
	}
	if dev.Namespace.ResultsStreamName() == prod.Namespace.ResultsStreamName() {
		t.Fatal("dev and prod must derive distinct results stream names")
	}
	if dev.Namespace.UploadsBucketName() == prod.Namespace.UploadsBucketName() {
		t.Fatal("dev and prod must derive distinct uploads bucket names")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// A worker consuming prod's namespace must never see a document
	// published to dev's namespace.
	devConsumer, err := dev.DocumentsConsumer(ctx, "dev-worker", "")
	if err != nil {
		t.Fatalf("dev consumer: %v", err)
	}
	prodConsumer, err := prod.DocumentsConsumer(ctx, "prod-worker", "")
	if err != nil {
		t.Fatalf("prod consumer: %v", err)
	}

	devReceived := make(chan jetstream.Msg, 1)
	devCC, err := devConsumer.Consume(func(msg jetstream.Msg) { devReceived <- msg })
	if err != nil {
		t.Fatalf("consume dev: %v", err)
	}
	defer devCC.Stop()

	prodReceived := make(chan jetstream.Msg, 1)
	prodCC, err := prodConsumer.Consume(func(msg jetstream.Msg) { prodReceived <- msg })
	if err != nil {
		t.Fatalf("consume prod: %v", err)
	}
	defer prodCC.Stop()

	id, _ := queue.NewID()
	if err := dev.PublishDocument(ctx, id, "Invoice", time.Now(), []byte("<Invoice/>")); err != nil {
		t.Fatalf("publish to dev: %v", err)
	}

	select {
	case msg := <-devReceived:
		msg.Ack()
	case <-time.After(3 * time.Second):
		t.Fatal("dev consumer never received the document published to its own namespace")
	}

	select {
	case msg := <-prodReceived:
		t.Fatalf("prod consumer unexpectedly received a document published to dev's namespace: subject=%s", msg.Subject())
	case <-time.After(300 * time.Millisecond):
		// expected: prod must not observe dev's document
	}

	// An upload record written under one namespace must not be readable
	// (or overwritable) through the other namespace's client, even when
	// both use the exact same upload id.
	if err := dev.PutUploadRecord(ctx, queue.UploadRecord{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusValid,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("put dev record: %v", err)
	}
	if _, err := prod.GetUploadRecord(ctx, id); err != queue.ErrRecordNotFound {
		t.Fatalf("prod.GetUploadRecord for a dev-only id: err = %v, want %v", err, queue.ErrRecordNotFound)
	}

	if err := prod.PutUploadRecord(ctx, queue.UploadRecord{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusInvalid,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("put prod record: %v", err)
	}
	devRecord, err := dev.GetUploadRecord(ctx, id)
	if err != nil {
		t.Fatalf("get dev record: %v", err)
	}
	if devRecord.Status != queue.StatusValid {
		t.Fatalf("dev record status = %q, want %q (prod's write must not have leaked into dev's namespace)", devRecord.Status, queue.StatusValid)
	}

	// A worker's subject filter from another namespace must be rejected
	// rather than silently creating a cross-namespace consumer.
	if _, err := prod.DocumentsConsumer(ctx, "prod-cross-namespace-worker", dev.Namespace.DocumentSubject("Invoice")); err == nil {
		t.Fatal("expected an error creating a prod consumer filtered to a dev namespace subject")
	}
}
