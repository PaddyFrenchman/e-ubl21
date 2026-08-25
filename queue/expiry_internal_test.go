package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

type errorObjectDeleter struct {
	err error
}

func (d errorObjectDeleter) Delete(context.Context, string) error { return d.err }

func TestReclaimUploadContentReportsDeleteFailures(t *testing.T) {
	var missing ContentExpiryReport
	reclaimUploadContent(errorObjectDeleter{err: jetstream.ErrObjectNotFound}, context.Background(), "id", 100, &missing)
	if missing.AlreadyMissing != 1 || missing.Errors != 0 || missing.ExpiredBytes != 0 {
		t.Fatalf("already-missing report = %+v", missing)
	}

	var failed ContentExpiryReport
	reclaimUploadContent(errorObjectDeleter{err: errors.New("delete failed")}, context.Background(), "id", 100, &failed)
	if failed.Errors != 1 || failed.AlreadyMissing != 0 || failed.ExpiredBytes != 0 {
		t.Fatalf("failed deletion report = %+v", failed)
	}
}

func TestMarkUploadContentExpiredPreservesConcurrentMetadata(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := ConnectWithOptions(server.URL, DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	id, _ := NewID()
	if err := client.PutUploadRecord(ctx, UploadRecord{
		ID: id, DocumentType: "Invoice", Status: StatusValid,
		ReceivedAt:    now.Add(-time.Hour).Format(time.RFC3339),
		ValidatedAt:   now.Add(-time.Hour).Format(time.RFC3339),
		ResultPending: true,
	}); err != nil {
		t.Fatal(err)
	}
	versioned, err := client.GetUploadRecordVersion(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// A stale revision exercises the retry that reloads and preserves fields
	// changed by a concurrent terminal-record update.
	versioned.Revision++
	marked, err := client.markUploadContentExpired(ctx, versioned, now, now)
	if err != nil || !marked {
		t.Fatalf("mark after stale revision = %t, %v", marked, err)
	}
	stored, err := client.GetUploadRecordVersion(ctx, id)
	if err != nil || !stored.Record.ResultPending || stored.Record.ContentExpiredAt == "" {
		t.Fatalf("marked metadata=%+v err=%v", stored.Record, err)
	}
	if marked, err := client.markUploadContentExpired(ctx, stored, now, now); err != nil || marked {
		t.Fatalf("already marked result = %t, %v", marked, err)
	}

	pendingID, _ := NewID()
	if err := client.PutUploadRecord(ctx, UploadRecord{ID: pendingID, Status: StatusPending, ReceivedAt: now.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	pending, err := client.GetUploadRecordVersion(ctx, pendingID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.markUploadContentExpired(ctx, pending, now, now); err == nil {
		t.Fatal("non-terminal record was marked expired")
	}

	missingID, _ := NewID()
	missing := VersionedUploadRecord{Record: UploadRecord{ID: missingID, Status: StatusValid}, Revision: 1}
	if _, err := client.markUploadContentExpired(ctx, missing, now, now); err == nil {
		t.Fatal("missing record was marked expired")
	}
}
