package queue_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestReplayProcessingFailureRejectsUnsafeStates(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	put := func(record queue.UploadRecord) string {
		t.Helper()
		record.ID, _ = queue.NewID()
		if record.DocumentType == "" {
			record.DocumentType = "Invoice"
		}
		if err := client.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		return record.ID
	}

	missingContent := put(queue.UploadRecord{Status: queue.StatusProcessingFailed, ReceivedAt: now.Format(time.RFC3339)})
	if err := client.ReplayProcessingFailure(ctx, missingContent); err == nil || !strings.Contains(err.Error(), "no raw content") {
		t.Fatalf("missing-content replay error = %v", err)
	}

	pending := put(queue.UploadRecord{Status: queue.StatusPending, ReceivedAt: now.Format(time.RFC3339)})
	if err := client.ReplayProcessingFailure(ctx, pending); err == nil || !strings.Contains(err.Error(), "no replay in progress") {
		t.Fatalf("ordinary pending replay error = %v", err)
	}

	claimed := put(queue.UploadRecord{Status: queue.StatusPending, ReceivedAt: now.Format(time.RFC3339), ReplayClaimedSeq: 9})
	if err := client.ReplayProcessingFailure(ctx, claimed); err != nil {
		t.Fatalf("claimed replay should be idempotent: %v", err)
	}

	malformedTime := put(queue.UploadRecord{Status: queue.StatusEnqueuePending, ReceivedAt: "bad-time", ReplayMessageID: "replay"})
	if err := client.ReplayProcessingFailure(ctx, malformedTime); err == nil || !strings.Contains(err.Error(), "parse receivedAt") {
		t.Fatalf("malformed-time replay error = %v", err)
	}
}

func TestReplayRecoversAfterDLQPurgeFailureWithoutDuplicateWork(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	id, _ := queue.NewID()
	receivedAt := time.Now().UTC().Truncate(time.Second)
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusProcessingFailed, Error: "storage unavailable", LastError: "storage unavailable", ReceivedAt: receivedAt.Format(time.RFC3339), ProcessingAttempts: 5, ProcessingFailedAt: receivedAt.Add(time.Minute).Format(time.RFC3339)}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	if err := client.PublishDeadLetter(ctx, queue.DeadLetter{ID: id, DocumentType: "Invoice", ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: 5, FailedAt: record.ProcessingFailedAt}); err != nil {
		t.Fatal(err)
	}
	if err := client.JS.DeleteStream(ctx, client.Namespace.DeadLettersStreamName()); err != nil {
		t.Fatal(err)
	}

	if err := client.ReplayProcessingFailure(ctx, id); err == nil {
		t.Fatal("replay succeeded despite missing DLQ stream")
	}
	interrupted, err := client.GetUploadRecord(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Status != queue.StatusPending || interrupted.ReplayMessageID == "" {
		t.Fatalf("interrupted replay = %+v", interrupted)
	}
	documents, err := client.JS.Stream(ctx, client.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	before, err := documents.Info(ctx)
	if err != nil || before.State.Msgs != 1 {
		t.Fatalf("work after interrupted replay=%d err=%v", before.State.Msgs, err)
	}

	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplayProcessingFailure(ctx, id); err != nil {
		t.Fatal(err)
	}
	after, err := documents.Info(ctx)
	if err != nil || after.State.Msgs != 1 {
		t.Fatalf("work after retry=%d err=%v, want one stable publication", after.State.Msgs, err)
	}
	completed, err := client.GetUploadRecord(ctx, id)
	if err != nil || completed.Status != queue.StatusPending || completed.ReplayMessageID == "" || !completed.ReplayReady || completed.ReplayClaimedSeq != 0 {
		t.Fatalf("completed replay=%+v err=%v", completed, err)
	}
}

func TestReplayUsesStableIDAfterAmbiguousPublication(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	id, _ := queue.NewID()
	receivedAt := time.Now().UTC().Truncate(time.Second)
	messageID := id + ":replay:7"
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusEnqueuePending, ReceivedAt: receivedAt.Format(time.RFC3339), LastError: "timeout", ProcessingAttempts: 5, ProcessingFailedAt: receivedAt.Add(time.Minute).Format(time.RFC3339), ReplayGeneration: 7, ReplayMessageID: messageID}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	if err := client.PublishDeadLetter(ctx, queue.DeadLetter{ID: id, DocumentType: "Invoice", ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: 5, FailedAt: record.ProcessingFailedAt}); err != nil {
		t.Fatal(err)
	}
	// Model an acknowledgement timeout: JetStream stored the first publish,
	// but the operator retries while canonical state is still enqueue_pending.
	if err := client.PublishDocumentWithID(ctx, id, "Invoice", receivedAt, messageID); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplayProcessingFailure(ctx, id); err != nil {
		t.Fatal(err)
	}
	documents, err := client.JS.Stream(ctx, client.Namespace.DocumentsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := documents.Info(ctx)
	if err != nil || info.State.Msgs != 1 {
		t.Fatalf("ambiguous replay queued %d descriptors, err=%v", info.State.Msgs, err)
	}
}
