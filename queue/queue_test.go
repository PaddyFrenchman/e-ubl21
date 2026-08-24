package queue_test

import (
	"bytes"
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

func TestPublishDocumentAndConsumeCompactWorkItem(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	receivedAt := time.Now().UTC().Truncate(time.Second)
	if err := client.PublishDocument(ctx, id, "Invoice", receivedAt); err != nil {
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
		item, err := queue.DecodeDocumentWorkItem(msg.Data())
		if err != nil {
			t.Fatalf("decode work item: %v", err)
		}
		if item.ID != id || item.DocumentType != "Invoice" {
			t.Fatalf("work item = %+v", item)
		}
		if len(msg.Data()) >= 256 {
			t.Fatalf("work descriptor unexpectedly large: %d bytes", len(msg.Data()))
		}
		if bytes.Contains(msg.Data(), []byte("<Invoice")) {
			t.Fatalf("work descriptor contains raw XML: %q", msg.Data())
		}
		if got := msg.Headers().Get(queue.HeaderUploadID); got != id {
			t.Fatalf("upload id header = %q, want %q", got, id)
		}
		_ = msg.Ack()
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
	if err := client.PublishDocument(ctx, invoiceID, "Invoice", time.Now()); err != nil {
		t.Fatalf("publish invoice: %v", err)
	}
	if err := client.PublishDocument(ctx, orderID, "Order", time.Now()); err != nil {
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
			t.Fatalf("document type = %q", got)
		}
		_ = msg.Ack()
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for filtered message")
	}
	select {
	case msg := <-received:
		t.Fatalf("unexpected second message: %s", msg.Subject())
	case <-time.After(300 * time.Millisecond):
	}
}

func TestUploadRecordLifecycle(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, _ := queue.NewID()
	body := []byte("<Invoice/>")
	if err := client.PutUploadContent(ctx, id, body); err != nil {
		t.Fatalf("put content: %v", err)
	}
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusPending, ReceivedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("put record: %v", err)
	}
	if err := client.UpdateUploadStatus(ctx, id, queue.StatusInvalid, "missing required element", time.Now()); err != nil {
		t.Fatalf("update status: %v", err)
	}
	record, err := client.GetUploadRecord(ctx, id)
	if err != nil || record.Status != queue.StatusInvalid {
		t.Fatalf("record = %+v, err=%v", record, err)
	}
	content, err := client.GetUploadContent(ctx, id)
	if err != nil || !bytes.Equal(content, body) {
		t.Fatalf("content changed: %q, err=%v", content, err)
	}
}

func TestUploadContentObjectStoreHandlesLargeXML(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id, _ := queue.NewID()
	body := bytes.Repeat([]byte("x"), 2<<20)
	if err := client.PutUploadContent(ctx, id, body); err != nil {
		t.Fatalf("put large content: %v", err)
	}
	got, err := client.GetUploadContent(ctx, id)
	if err != nil {
		t.Fatalf("get large content: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("large content mismatch: got %d bytes, want %d", len(got), len(body))
	}
}

func TestGetUploadRecordNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.GetUploadRecord(ctx, "00000000000000000000000000000000"); err != queue.ErrRecordNotFound {
		t.Fatalf("err = %v", err)
	}
	if _, err := client.GetUploadContent(ctx, "00000000000000000000000000000000"); err != queue.ErrRecordNotFound {
		t.Fatalf("content err = %v", err)
	}
}

func TestPublishResult(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	id, _ := queue.NewID()
	if err := client.PublishResult(ctx, queue.ValidationResult{ID: id, DocumentType: "Invoice", Status: queue.StatusValid, ValidatedAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatalf("publish result: %v", err)
	}
	consumer, err := client.JS.CreateOrUpdateConsumer(ctx, client.Namespace.ResultsStreamName(), jetstream.ConsumerConfig{Durable: "results-test", FilterSubject: client.Namespace.ResultsSubject(), AckPolicy: jetstream.AckExplicitPolicy})
	if err != nil {
		t.Fatalf("create results consumer: %v", err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(3*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	count := 0
	for msg := range batch.Messages() {
		count++
		_ = msg.Ack()
	}
	if count != 1 {
		t.Fatalf("received %d messages, want 1", count)
	}
}

func TestNamespacesDoNotCrossConsume(t *testing.T) {
	t.Parallel()
	url := queuetest.StartServer(t)
	dev := newTestClientWithNamespace(t, url, queue.MustNamespace("eubl21.dev"))
	prod := newTestClientWithNamespace(t, url, queue.MustNamespace("eubl21.prod"))
	if dev.Namespace.DocumentsStreamName() == prod.Namespace.DocumentsStreamName() || dev.Namespace.UploadContentBucketName() == prod.Namespace.UploadContentBucketName() {
		t.Fatal("namespaces derived colliding resources")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	devConsumer, err := dev.DocumentsConsumer(ctx, "dev-worker", "")
	if err != nil {
		t.Fatal(err)
	}
	prodConsumer, err := prod.DocumentsConsumer(ctx, "prod-worker", "")
	if err != nil {
		t.Fatal(err)
	}
	devReceived := make(chan jetstream.Msg, 1)
	prodReceived := make(chan jetstream.Msg, 1)
	devCC, _ := devConsumer.Consume(func(msg jetstream.Msg) { devReceived <- msg })
	prodCC, _ := prodConsumer.Consume(func(msg jetstream.Msg) { prodReceived <- msg })
	defer devCC.Stop()
	defer prodCC.Stop()
	id, _ := queue.NewID()
	if err := dev.PutUploadContent(ctx, id, []byte("dev")); err != nil {
		t.Fatal(err)
	}
	if err := dev.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-devReceived:
		_ = msg.Ack()
	case <-time.After(3 * time.Second):
		t.Fatal("dev consumer did not receive dev work")
	}
	select {
	case msg := <-prodReceived:
		t.Fatalf("prod received dev work: %s", msg.Subject())
	case <-time.After(300 * time.Millisecond):
	}
	if _, err := prod.GetUploadContent(ctx, id); err != queue.ErrRecordNotFound {
		t.Fatalf("prod saw dev content: %v", err)
	}
}
