package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
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
