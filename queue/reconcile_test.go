package queue_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestReconcileUploadsRepairsAndTerminatesPartialAcceptance(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	receivedAt := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	ids := make([]string, 4)
	for i, status := range []queue.ValidationStatus{queue.StatusAccepting, queue.StatusEnqueuePending, queue.StatusAccepting, queue.StatusCleanupPending} {
		ids[i], _ = queue.NewID()
		if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: ids[i], DocumentType: "Invoice", Status: status, ReceivedAt: receivedAt}); err != nil {
			t.Fatal(err)
		}
		if i < 2 || i == 3 {
			if err := client.PutUploadContent(ctx, ids[i], []byte("<Invoice/>")); err != nil {
				t.Fatal(err)
			}
		}
	}

	report, err := client.ReconcileUploads(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 4 || report.Requeued != 2 || report.Failed != 2 || report.Errors != 0 {
		t.Fatalf("report = %+v", report)
	}
	for i, want := range []queue.ValidationStatus{queue.StatusPending, queue.StatusPending, queue.StatusAcceptanceFailed, queue.StatusAcceptanceFailed} {
		record, err := client.GetUploadRecord(ctx, ids[i])
		if err != nil {
			t.Fatal(err)
		}
		if record.Status != want {
			t.Fatalf("record %d status = %s, want %s", i, record.Status, want)
		}
	}
	if exists, err := client.HasUploadContent(ctx, ids[3]); err != nil || exists {
		t.Fatalf("cleanup-pending content exists=%v err=%v", exists, err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 2 {
		t.Fatalf("queued messages = %d, want 2", info.State.Msgs)
	}

	second, err := client.ReconcileUploads(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if second.Scanned != 0 || second.Requeued != 0 {
		t.Fatalf("second reconciliation was not idempotent: %+v", second)
	}
}

func TestUploadRecordCASPreventsStatusRegression(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	revision, err := client.CreateUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusEnqueuePending, ReceivedAt: time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		t.Fatal(err)
	}
	terminal := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusValid, ReceivedAt: time.Now().UTC().Format(time.RFC3339), ValidatedAt: time.Now().UTC().Format(time.RFC3339)}
	if _, err := client.UpdateUploadRecord(ctx, terminal, revision); err != nil {
		t.Fatal(err)
	}
	stale := terminal
	stale.Status = queue.StatusPending
	if _, err := client.UpdateUploadRecord(ctx, stale, revision); !errors.Is(err, jetstream.ErrKeyRevisionMismatch) {
		t.Fatalf("stale CAS error = %v, want revision mismatch", err)
	}
	record, err := client.GetUploadRecord(ctx, id)
	if err != nil || record.Status != queue.StatusValid {
		t.Fatalf("record=%+v err=%v", record, err)
	}
}

func TestReconcileUploadsPreservesPublishFailureAndHandlesAmbiguousAck(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	received := time.Now().Add(-time.Hour).UTC()
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusEnqueuePending, ReceivedAt: received.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	if err := client.JS.DeleteStream(ctx, client.Namespace.DocumentsStreamName()); err != nil {
		t.Fatal(err)
	}
	report, err := client.ReconcileUploads(ctx, 0)
	if err != nil || report.Errors != 1 {
		t.Fatalf("failed-publish report=%+v err=%v", report, err)
	}
	record, err := client.GetUploadRecord(ctx, id)
	if err != nil || !strings.Contains(record.Error, "awaiting queue publication") {
		t.Fatalf("publish error not retained: record=%+v err=%v", record, err)
	}
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	// Model an ambiguous acknowledgement: the broker stored this publish,
	// but the durable record still says enqueue_pending.
	if err := client.PublishDocument(ctx, id, "Invoice", received); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReconcileUploads(ctx, 0); err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("ambiguous publish produced %d messages, want 1", info.State.Msgs)
	}
}
