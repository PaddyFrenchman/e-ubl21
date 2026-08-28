package queue_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
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
func newTestClientWithNamespace(t *testing.T, server queuetest.Server, namespace queue.Namespace) *queue.Client {
	t.Helper()
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
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
		if got := client.Namespace.DocumentTypeFromSubject(msg.Subject()); got != "Invoice" {
			t.Fatalf("document type = %q, want %q", got, "Invoice")
		}
		if got := msg.Headers().Get(queue.HeaderUploadID); got != id {
			t.Fatalf("upload id header = %q, want %q", got, id)
		}
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

func TestDocumentsConsumerAppliesExplicitBackpressureConfig(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	consumer, err := client.DocumentsConsumerWithConfig(ctx, "bounded-worker", "", 60*time.Second, 7, 4)
	if err != nil {
		t.Fatal(err)
	}
	info := consumer.CachedInfo()
	if info.Config.AckWait != 60*time.Second || info.Config.MaxDeliver != 7 || info.Config.MaxAckPending != 4 {
		t.Fatalf("consumer config = AckWait %s, MaxDeliver %d, MaxAckPending %d", info.Config.AckWait, info.Config.MaxDeliver, info.Config.MaxAckPending)
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
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatalf("put record: %v", err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
		t.Fatalf("put content: %v", err)
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

	content, err := client.GetUploadContent(ctx, id)
	if err != nil {
		t.Fatalf("get content after status update: %v", err)
	}
	if string(content) != "<Invoice/>" {
		t.Fatalf("content lost after status update: %q", content)
	}
}

// TestPutUploadContentAndGet proves the acceptance criterion of separating
// raw content from mutable status/metadata: content is stored and
// retrieved independently of any UploadRecord, and GetUploadContent
// reports ErrRecordNotFound for an id that was never given content, even
// if a status/metadata record already exists for that id.
func TestPutUploadContentAndGet(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	body := []byte("<Invoice><cbc:ID>INV-1</cbc:ID></Invoice>")
	if err := client.PutUploadContent(ctx, id, body); err != nil {
		t.Fatalf("put content: %v", err)
	}

	got, err := client.GetUploadContent(ctx, id)
	if err != nil {
		t.Fatalf("get content: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("content = %q, want %q", got, body)
	}

	otherID, _ := queue.NewID()
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{
		ID:           otherID,
		DocumentType: "Invoice",
		Status:       queue.StatusPending,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("put record for otherID: %v", err)
	}
	if _, err := client.GetUploadContent(ctx, otherID); err != queue.ErrRecordNotFound {
		t.Fatalf("GetUploadContent for a record with no stored content: err = %v, want %v", err, queue.ErrRecordNotFound)
	}
}

// TestUpdateUploadStatusDoesNotTouchContent proves that repeated status
// updates never read or rewrite the upload's raw content.
func TestUpdateUploadStatusDoesNotTouchContent(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	body := []byte("<Invoice/>")
	if err := client.PutUploadContent(ctx, id, body); err != nil {
		t.Fatalf("put content: %v", err)
	}
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusPending,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("put record: %v", err)
	}

	for _, status := range []queue.ValidationStatus{queue.StatusValid, queue.StatusInvalid, queue.StatusValid} {
		if err := client.UpdateUploadStatus(ctx, id, status, "", time.Now()); err != nil {
			t.Fatalf("update status to %q: %v", status, err)
		}
		content, err := client.GetUploadContent(ctx, id)
		if err != nil {
			t.Fatalf("get content after updating status to %q: %v", status, err)
		}
		if !bytes.Equal(content, body) {
			t.Fatalf("content changed after updating status to %q: got %q, want %q", status, content, body)
		}
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

func TestOpenUploadContentStreamsLargeXML(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	body := bytes.Repeat([]byte("streamed-content-"), 1<<17)
	if err := client.PutUploadContent(ctx, id, body); err != nil {
		t.Fatalf("put content: %v", err)
	}
	content, err := client.OpenUploadContent(ctx, id)
	if err != nil {
		t.Fatalf("open content: %v", err)
	}
	defer content.Close()
	var got bytes.Buffer
	if _, err := io.CopyBuffer(&got, content, make([]byte, 4096)); err != nil {
		t.Fatalf("stream content: %v", err)
	}
	if err := content.Error(); err != nil {
		t.Fatalf("stream result: %v", err)
	}
	if !bytes.Equal(got.Bytes(), body) {
		t.Fatalf("streamed content mismatch: got %d bytes, want %d", got.Len(), len(body))
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
	if _, err := client.GetUploadContent(ctx, "00000000000000000000000000000000"); err != queue.ErrRecordNotFound {
		t.Fatalf("content err = %v, want %v", err, queue.ErrRecordNotFound)
	}
}

func TestPublishResult(t *testing.T) {
	t.Parallel()
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, _ := queue.NewID()
	result := queue.ValidationResult{
		ID:           id,
		DocumentType: "Invoice",
		Status:       queue.StatusValid,
		ValidatedAt:  time.Now().UTC().Format(time.RFC3339),
		ValidationStages: queue.ValidationStages{{
			Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true,
			ValidatedAt: time.Now().UTC().Format(time.RFC3339), DurationMS: 4,
		}},
	}
	if exists, err := client.HasValidationResult(ctx, id); err != nil || exists {
		t.Fatalf("result exists before publication = %t, %v", exists, err)
	}
	if err := client.PublishResult(ctx, result); err != nil {
		t.Fatalf("publish result: %v", err)
	}
	if exists, err := client.HasValidationResult(ctx, id); err != nil || !exists {
		t.Fatalf("result exists after publication = %t, %v", exists, err)
	}

	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatalf("get results stream: %v", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("get results stream info: %v", err)
	}
	if info.Config.Discard != jetstream.DiscardNew || !info.Config.DiscardNewPerSubject || info.Config.MaxMsgsPerSubject != 1 || info.Config.MaxAge != 0 {
		t.Fatalf("results stream is not permanently idempotent: %+v", info.Config)
	}
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != client.Namespace.ResultsFilterSubject() {
		t.Fatalf("results stream subjects = %v, want [%s]", info.Config.Subjects, client.Namespace.ResultsFilterSubject())
	}

	// Shrink the ordinary Nats-Msg-Id window, wait until it has elapsed, then
	// retry through PublishResult. Per-subject insert semantics must still make
	// the retry successful without storing a second event.
	cfg := info.Config
	cfg.Duplicates = 100 * time.Millisecond
	if _, err := client.JS.UpdateStream(ctx, cfg); err != nil {
		t.Fatalf("shrink result deduplication window: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := client.PublishResult(ctx, result); err != nil {
		t.Fatalf("publish result after message-ID window: %v", err)
	}
	info, err = stream.Info(ctx)
	if err != nil {
		t.Fatalf("get results stream info after retry: %v", err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("result retry stored %d messages, want 1", info.State.Msgs)
	}

	consumer, err := client.JS.CreateOrUpdateConsumer(ctx, client.Namespace.ResultsStreamName(), jetstream.ConsumerConfig{
		Durable:       "results-test",
		FilterSubject: client.Namespace.ResultsFilterSubject(),
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
		if got := msg.Subject(); got != client.Namespace.ResultSubject(id) {
			t.Fatalf("result subject = %q, want %q", got, client.Namespace.ResultSubject(id))
		}
		if got := msg.Headers().Get(queue.HeaderUploadID); got != id {
			t.Fatalf("upload id header = %q, want %q", got, id)
		}
		var published queue.ValidationResult
		if err := json.Unmarshal(msg.Data(), &published); err != nil {
			t.Fatalf("decode published result: %v", err)
		}
		stage, ok := published.ValidationStages.Find(queue.ValidationLayerUBL21XSD)
		if !ok || !stage.Valid || stage.Version != "2.1" {
			t.Fatalf("published XSD stage = %+v, present %t", stage, ok)
		}
		msg.Ack()
	}
	if count != 1 {
		t.Fatalf("received %d messages, want 1", count)
	}
}

func TestEnsureInfraPreservesLegacyResultsWhileCreatingIdempotentStream(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	const legacyStreamName = "UBL_VALIDATION_RESULTS"
	if _, err := client.JS.CreateStream(ctx, jetstream.StreamConfig{
		Name:      legacyStreamName,
		Subjects:  []string{client.Namespace.ResultsSubject()},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    7 * 24 * time.Hour,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.Publish(ctx, client.Namespace.ResultsSubject(), []byte(`{"id":"legacy-one"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.Publish(ctx, client.Namespace.ResultsSubject(), []byte(`{"id":"legacy-two"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.JS.CreateConsumer(ctx, legacyStreamName, jetstream.ConsumerConfig{
		Durable:       "legacy-results-consumer",
		FilterSubject: client.Namespace.ResultsSubject(),
		AckPolicy:     jetstream.AckExplicitPolicy,
	}); err != nil {
		t.Fatal(err)
	}

	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatalf("migrate legacy result stream: %v", err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != client.Namespace.ResultsFilterSubject() || !info.Config.DiscardNewPerSubject {
		t.Fatalf("legacy result stream was not migrated: %+v", info.Config)
	}
	legacy, err := client.JS.Stream(ctx, legacyStreamName)
	if err != nil {
		t.Fatal(err)
	}
	legacyInfo, err := legacy.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if legacyInfo.State.Msgs != 2 || len(legacyInfo.Config.Subjects) != 1 || legacyInfo.Config.Subjects[0] != client.Namespace.ResultsSubject() {
		t.Fatalf("migration changed the legacy stream: %+v", legacyInfo)
	}
}

// TestNamespacesDoNotCrossConsume proves different queue namespaces can
// share the same JetStream server without cross-observing work, metadata or
// raw content.
func TestNamespacesDoNotCrossConsume(t *testing.T) {
	t.Parallel()
	server := queuetest.StartServer(t)
	dev := newTestClientWithNamespace(t, server, queue.MustNamespace("eubl21.dev"))
	prod := newTestClientWithNamespace(t, server, queue.MustNamespace("eubl21.prod"))

	if dev.Namespace.DocumentsStreamName() == prod.Namespace.DocumentsStreamName() {
		t.Fatal("dev and prod must derive distinct documents stream names")
	}
	if dev.Namespace.ResultsStreamName() == prod.Namespace.ResultsStreamName() {
		t.Fatal("dev and prod must derive distinct results stream names")
	}
	if dev.Namespace.UploadsBucketName() == prod.Namespace.UploadsBucketName() {
		t.Fatal("dev and prod must derive distinct uploads bucket names")
	}
	if dev.Namespace.ValidationExecutionsBucketName() == prod.Namespace.ValidationExecutionsBucketName() {
		t.Fatal("dev and prod must derive distinct validation execution bucket names")
	}
	if dev.Namespace.UploadContentBucketName() == prod.Namespace.UploadContentBucketName() {
		t.Fatal("dev and prod must derive distinct upload content bucket names")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	if err := dev.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
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
	}

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
		t.Fatalf("dev record status = %q, want %q", devRecord.Status, queue.StatusValid)
	}

	if err := dev.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatalf("provision dev validation execution infrastructure: %v", err)
	}
	if err := prod.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatalf("provision prod validation execution infrastructure: %v", err)
	}
	execution, err := queue.NewValidationExecution(testInitialIntent(t, id, testFrozenPlan(), time.Now()))
	if err != nil {
		t.Fatalf("new dev validation execution: %v", err)
	}
	if _, err := dev.CreateValidationExecution(ctx, execution); err != nil {
		t.Fatalf("create dev validation execution: %v", err)
	}
	if _, err := prod.GetValidationExecution(ctx, id, execution.ValidationID, execution.Generation); !errors.Is(err, queue.ErrValidationExecutionNotFound) {
		t.Fatalf("prod observed dev validation execution: %v", err)
	}

	if _, err := prod.DocumentsConsumer(ctx, "prod-cross-namespace-worker", dev.Namespace.DocumentSubject("Invoice")); err == nil {
		t.Fatal("expected an error creating a prod consumer filtered to a dev namespace subject")
	}

	if err := dev.PutUploadContent(ctx, id, []byte("<Invoice/>dev")); err != nil {
		t.Fatalf("put dev content: %v", err)
	}
	if _, err := prod.GetUploadContent(ctx, id); err != queue.ErrRecordNotFound {
		t.Fatalf("prod.GetUploadContent for a dev-only id: err = %v, want %v", err, queue.ErrRecordNotFound)
	}
	if err := prod.PutUploadContent(ctx, id, []byte("<Invoice/>prod")); err != nil {
		t.Fatalf("put prod content: %v", err)
	}
	devContent, err := dev.GetUploadContent(ctx, id)
	if err != nil {
		t.Fatalf("get dev content: %v", err)
	}
	if string(devContent) != "<Invoice/>dev" {
		t.Fatalf("dev content = %q, want %q", devContent, "<Invoice/>dev")
	}
}
