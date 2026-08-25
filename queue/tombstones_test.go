package queue_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestPurgeUploadContentTombstonesIsBoundedAndProtectsLivePendingContent(t *testing.T) {
	server := queuetest.StartServer(t)
	prod := newTestClientWithNamespace(t, server, queue.MustNamespace("eubl21.prod"))
	dev := newTestClientWithNamespace(t, server, queue.MustNamespace("eubl21.dev"))
	ctx := context.Background()
	now := time.Now().UTC()

	makeObject := func(client *queue.Client, status queue.ValidationStatus, expired bool, deleted bool) string {
		t.Helper()
		id, _ := queue.NewID()
		record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: status, ReceivedAt: now.Format(time.RFC3339)}
		if expired {
			record.ValidatedAt = now.Add(-time.Hour).Format(time.RFC3339)
			record.ContentExpiresAt = now.Add(-30 * time.Minute).Format(time.RFC3339)
			record.ContentExpiredAt = now.Add(-20 * time.Minute).Format(time.RFC3339)
		}
		if err := client.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		if err := client.PutUploadContent(ctx, id, []byte("content-"+id)); err != nil {
			t.Fatal(err)
		}
		if deleted {
			if err := client.DeleteUploadContent(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}

	oldEligible := []string{
		makeObject(prod, queue.StatusValid, true, true),
		makeObject(prod, queue.StatusInvalid, true, true),
	}
	pendingDeleted := makeObject(prod, queue.StatusPending, false, true)
	foreignDeleted := makeObject(dev, queue.StatusValid, true, true)
	time.Sleep(75 * time.Millisecond)
	recentDeleted := makeObject(prod, queue.StatusValid, true, true)
	livePending := makeObject(prod, queue.StatusPending, false, false)
	scanNow := time.Now()

	dryRun, err := prod.PurgeUploadContentTombstones(ctx, 50*time.Millisecond, 1, true, scanNow)
	if err != nil || dryRun.Eligible != 1 || dryRun.Purged != 0 || dryRun.Failed != 0 {
		t.Fatalf("dry-run report = %+v, err=%v", dryRun, err)
	}

	first, err := prod.PurgeUploadContentTombstones(ctx, 50*time.Millisecond, 1, false, scanNow)
	if err != nil || first.Eligible != 1 || first.Purged != 1 || first.Failed != 0 {
		t.Fatalf("first purge report = %+v, err=%v", first, err)
	}
	prodStore, err := prod.JS.ObjectStore(ctx, prod.Namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	purged := 0
	for _, id := range oldEligible {
		if _, err := prodStore.GetInfo(ctx, id, jetstream.GetObjectInfoShowDeleted()); errors.Is(err, jetstream.ErrObjectNotFound) {
			purged++
		}
	}
	if purged != 1 {
		t.Fatalf("bounded purge removed %d old eligible markers, want 1", purged)
	}

	second, err := prod.PurgeUploadContentTombstones(ctx, 50*time.Millisecond, 10, false, scanNow)
	if err != nil || second.Eligible != 1 || second.Purged != 1 || second.Failed != 0 {
		t.Fatalf("second purge report = %+v, err=%v", second, err)
	}
	if _, err := prodStore.GetInfo(ctx, pendingDeleted, jetstream.GetObjectInfoShowDeleted()); err != nil {
		t.Fatalf("pending upload tombstone was purged: %v", err)
	}
	if _, err := prodStore.GetInfo(ctx, recentDeleted, jetstream.GetObjectInfoShowDeleted()); err != nil {
		t.Fatalf("recent tombstone was purged: %v", err)
	}
	if content, err := prod.GetUploadContent(ctx, livePending); err != nil || string(content) != "content-"+livePending {
		t.Fatalf("live pending content changed: %q, %v", content, err)
	}
	devStore, err := dev.JS.ObjectStore(ctx, dev.Namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := devStore.GetInfo(ctx, foreignDeleted, jetstream.GetObjectInfoShowDeleted()); err != nil {
		t.Fatalf("other namespace tombstone was purged: %v", err)
	}

	repeated, err := prod.PurgeUploadContentTombstones(ctx, 50*time.Millisecond, 10, false, scanNow)
	if err != nil || repeated.Eligible != 0 || repeated.Purged != 0 || repeated.Failed != 0 {
		t.Fatalf("repeated purge report = %+v, err=%v", repeated, err)
	}
}

func TestPurgeUploadContentTombstonesIsSafeAcrossOverlappingRuns(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	id, _ := queue.NewID()
	now := time.Now().UTC()
	record := queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt:       now.Add(-time.Hour).Format(time.RFC3339),
		ValidatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		ContentExpiredAt: now.Add(-time.Minute).Format(time.RFC3339),
	}
	if err := client.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("expired")); err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteUploadContent(ctx, id); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			report, err := client.PurgeUploadContentTombstones(ctx, 0, 10, false, time.Now())
			if err != nil {
				errCh <- err
				return
			}
			if report.Failed != 0 {
				errCh <- errors.New("overlapping purge reported a failed item")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	store, err := client.JS.ObjectStore(ctx, client.Namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetInfo(ctx, id, jetstream.GetObjectInfoShowDeleted()); !errors.Is(err, jetstream.ErrObjectNotFound) {
		t.Fatalf("tombstone remains after overlapping purge: %v", err)
	}
}

func TestPurgeUploadContentTombstonesRejectsUnboundedArguments(t *testing.T) {
	client := newTestClient(t)
	if _, err := client.PurgeUploadContentTombstones(context.Background(), -time.Second, 1, false, time.Time{}); err == nil {
		t.Fatal("negative grace period was accepted")
	}
	if _, err := client.PurgeUploadContentTombstones(context.Background(), time.Hour, 0, false, time.Time{}); err == nil {
		t.Fatal("non-positive max-items was accepted")
	}
}
