package queue_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestExpireUploadContentRetainsMetadataAndSkipsPendingWork(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	retention := time.Hour

	create := func(status queue.ValidationStatus, validatedAt time.Time, withContent bool) string {
		t.Helper()
		id, _ := queue.NewID()
		record := queue.UploadRecord{
			ID: id, DocumentType: "Invoice", Status: status,
			ReceivedAt: now.Add(-3 * time.Hour).Format(time.RFC3339),
		}
		if !validatedAt.IsZero() {
			record.ValidatedAt = validatedAt.Format(time.RFC3339)
		}
		if err := client.PutUploadRecord(ctx, record); err != nil {
			t.Fatal(err)
		}
		if withContent {
			if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}

	expiredID := create(queue.StatusValid, now.Add(-2*time.Hour), true)
	scheduledID := create(queue.StatusInvalid, now.Add(-30*time.Minute), true)
	pendingID := create(queue.StatusPending, time.Time{}, true)
	missingID := create(queue.StatusValid, now.Add(-2*time.Hour), false)
	cleanupRetryID := create(queue.StatusValid, now.Add(-2*time.Hour), true)
	cleanupRetry, err := client.GetUploadRecord(ctx, cleanupRetryID)
	if err != nil {
		t.Fatal(err)
	}
	cleanupRetry.ContentExpiresAt = now.Add(-time.Hour).Format(time.RFC3339)
	cleanupRetry.ContentExpiredAt = now.Add(-time.Minute).Format(time.RFC3339)
	if err := client.PutUploadRecord(ctx, cleanupRetry); err != nil {
		t.Fatal(err)
	}

	report, err := client.ExpireUploadContent(ctx, retention, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 3 || report.Scheduled != 1 || report.Expired != 1 || report.Reclaimed != 2 || report.AlreadyMissing != 0 || report.Errors != 0 || report.ExpiredBytes != 2*uint64(len("<Invoice/>")) {
		t.Fatalf("expiry report = %+v", report)
	}

	expired, err := client.GetUploadRecord(ctx, expiredID)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Status != queue.StatusValid || expired.ContentExpiresAt != now.Add(-time.Hour).Format(time.RFC3339) || expired.ContentExpiredAt != now.Format(time.RFC3339) {
		t.Fatalf("expired metadata = %+v", expired)
	}
	if _, err := client.GetUploadContent(ctx, expiredID); !errors.Is(err, queue.ErrRecordNotFound) {
		t.Fatalf("expired content error = %v, want record not found", err)
	}
	if _, err := client.GetUploadContent(ctx, cleanupRetryID); !errors.Is(err, queue.ErrRecordNotFound) {
		t.Fatalf("pre-marked content was not reclaimed on retry: %v", err)
	}

	scheduled, err := client.GetUploadRecord(ctx, scheduledID)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.ContentExpiresAt != now.Add(30*time.Minute).Format(time.RFC3339) || scheduled.ContentExpiredAt != "" {
		t.Fatalf("scheduled metadata = %+v", scheduled)
	}
	if _, err := client.GetUploadContent(ctx, scheduledID); err != nil {
		t.Fatalf("scheduled content was removed early: %v", err)
	}

	pending, err := client.GetUploadRecord(ctx, pendingID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.ContentExpiresAt != "" || pending.ContentExpiredAt != "" {
		t.Fatalf("pending work was enrolled in expiry: %+v", pending)
	}
	if _, err := client.GetUploadContent(ctx, pendingID); err != nil {
		t.Fatalf("pending content was removed: %v", err)
	}

	missing, err := client.GetUploadRecord(ctx, missingID)
	if err != nil || missing.ContentExpiredAt != "" {
		t.Fatalf("record without active raw content should not enter the bounded object scan: metadata=%+v err=%v", missing, err)
	}
	second, err := client.ExpireUploadContent(ctx, retention, now)
	if err != nil {
		t.Fatal(err)
	}
	if second.Expired != 0 || second.Scheduled != 0 || second.Errors != 0 {
		t.Fatalf("second expiry scan was not idempotent: %+v", second)
	}

	rescheduled, err := client.ExpireUploadContent(ctx, 2*time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if rescheduled.Scheduled != 1 || rescheduled.Errors != 0 {
		t.Fatalf("changed retention did not reschedule live content: %+v", rescheduled)
	}
	scheduled, err = client.GetUploadRecord(ctx, scheduledID)
	if err != nil || scheduled.ContentExpiresAt != now.Add(90*time.Minute).Format(time.RFC3339) {
		t.Fatalf("rescheduled metadata=%+v err=%v", scheduled, err)
	}

	disabled, err := client.ExpireUploadContent(ctx, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Unscheduled != 1 || disabled.Expired != 0 || disabled.Errors != 0 {
		t.Fatalf("disabled retention did not clear live schedules: %+v", disabled)
	}
	scheduled, err = client.GetUploadRecord(ctx, scheduledID)
	if err != nil || scheduled.ContentExpiresAt != "" || scheduled.ContentExpiredAt != "" {
		t.Fatalf("unscheduled metadata=%+v err=%v", scheduled, err)
	}
	if _, err := client.GetUploadContent(ctx, scheduledID); err != nil {
		t.Fatalf("disabling retention removed content: %v", err)
	}
}

func TestExpireUploadContentIsSafeAcrossConcurrentSweepers(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	id, _ := queue.NewID()
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt:  now.Add(-2 * time.Hour).Format(time.RFC3339),
		ValidatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, id, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}

	reports := make(chan queue.ContentExpiryReport, 2)
	errs := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			report, err := client.ExpireUploadContent(ctx, time.Hour, now)
			reports <- report
			errs <- err
		}()
	}
	start.Done()
	var expired, reportErrors int
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
		report := <-reports
		expired += report.Expired
		reportErrors += report.Errors
	}
	if expired != 1 || reportErrors != 0 {
		t.Fatalf("concurrent expiry totals = expired %d errors %d", expired, reportErrors)
	}
	record, err := client.GetUploadRecord(ctx, id)
	if err != nil || record.ContentExpiredAt == "" || record.Status != queue.StatusValid {
		t.Fatalf("final metadata=%+v err=%v", record, err)
	}
}

func TestExpireUploadContentHandlesEmptyAndMalformedInventories(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if _, err := client.ExpireUploadContent(ctx, -time.Second, time.Time{}); err == nil {
		t.Fatal("negative retention was accepted")
	}
	if report, err := client.ExpireUploadContent(ctx, time.Hour, time.Time{}); err != nil || report != (queue.ContentExpiryReport{}) {
		t.Fatalf("empty Object Store report=%+v err=%v", report, err)
	}

	orphanID, _ := queue.NewID()
	if err := client.PutUploadContent(ctx, orphanID, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	malformedID, _ := queue.NewID()
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{
		ID: malformedID, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.PutUploadContent(ctx, malformedID, []byte("<Invoice/>")); err != nil {
		t.Fatal(err)
	}
	report, err := client.ExpireUploadContent(ctx, time.Hour, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Errors != 2 || report.Scanned != 1 {
		t.Fatalf("malformed inventory report = %+v", report)
	}

	client.Close()
	if _, err := client.UploadContentStorageSize(ctx); err == nil {
		t.Fatal("storage status succeeded after connection close")
	}
}

func TestEnsureInfraAppliesUploadContentByteLimit(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	const maxBytes = int64(1 << 20)
	if err := client.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: maxBytes}); err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, "OBJ_"+client.Namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Config.MaxBytes != maxBytes || info.Config.MaxAge != 0 {
		t.Fatalf("upload content stream limits = maxBytes %d maxAge %s", info.Config.MaxBytes, info.Config.MaxAge)
	}
	if size, err := client.UploadContentStorageSize(ctx); err != nil || size != 0 {
		t.Fatalf("initial upload content storage size = %d, %v", size, err)
	}
	id, _ := queue.NewID()
	if err := client.PutUploadContent(ctx, id, make([]byte, 2<<20)); err == nil {
		t.Fatal("Object Store accepted content larger than its hard byte limit")
	}
}
