package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ContentExpiryReport summarizes one raw-content retention scan. Metadata is
// retained; only Object Store content is reclaimed.
type ContentExpiryReport struct {
	Scanned        int
	Scheduled      int
	Unscheduled    int
	Expired        int
	Reclaimed      int
	AlreadyMissing int
	Errors         int
	ExpiredBytes   uint64
}

// ExpireUploadContent schedules and performs raw-content expiry for terminal
// validation records. Pending work is deliberately never expired, even when
// it is older than retention, so a deep backlog cannot lose input before the
// worker validates it. Existing terminal records with active content and no
// ContentExpiresAt are enrolled using ValidatedAt plus retention. Scanning the
// bounded object inventory avoids work proportional to permanent metadata.
func (c *Client) ExpireUploadContent(ctx context.Context, retention time.Duration, now time.Time) (ContentExpiryReport, error) {
	var report ContentExpiryReport
	if retention < 0 {
		return report, fmt.Errorf("upload content retention must be non-negative")
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return report, err
	}
	objects, err := store.List(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoObjectsFound) {
			return report, nil
		}
		return report, fmt.Errorf("list raw upload content for expiry: %w", err)
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	for _, object := range objects {
		id := object.Name
		versioned, err := c.GetUploadRecordVersion(ctx, id)
		if err != nil {
			report.Errors++
			continue
		}
		record := versioned.Record
		projected, err := c.ProjectUploadRecord(ctx, record)
		if err != nil {
			report.Errors++
			continue
		}
		if projected.Status != StatusValid && projected.Status != StatusInvalid {
			continue
		}
		report.Scanned++
		if record.ContentExpiredAt != "" {
			reclaimUploadContent(store, ctx, id, object.Size, &report)
			continue
		}

		if retention == 0 {
			if record.ContentExpiresAt != "" {
				record.ContentExpiresAt = ""
				if _, err := c.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
					latest, getErr := c.GetUploadRecord(ctx, id)
					if getErr != nil || latest.ContentExpiresAt != "" {
						report.Errors++
					}
				} else {
					report.Unscheduled++
				}
			}
			continue
		}

		deadline, err := contentExpiryDeadline(projected, retention)
		if err != nil {
			report.Errors++
			continue
		}
		record.ContentExpiresAt = deadline.Format(time.RFC3339)
		if now.Before(deadline) {
			if versioned.Record.ContentExpiresAt != record.ContentExpiresAt {
				if _, err := c.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
					latest, getErr := c.GetUploadRecord(ctx, id)
					if getErr != nil || latest.ContentExpiresAt != record.ContentExpiresAt {
						report.Errors++
					}
				} else {
					report.Scheduled++
				}
			}
			continue
		}

		marked, err := c.markUploadContentExpired(ctx, versioned, deadline, now)
		if err != nil {
			report.Errors++
			continue
		}
		if marked {
			report.Expired++
		}
		// Metadata is committed before deletion. If deletion fails or the
		// process exits here, the still-visible object is retried next scan.
		reclaimUploadContent(store, ctx, id, object.Size, &report)
	}
	return report, nil
}

type objectDeleter interface {
	Delete(context.Context, string) error
}

func reclaimUploadContent(store objectDeleter, ctx context.Context, id string, size uint64, report *ContentExpiryReport) {
	if err := store.Delete(ctx, id); err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			report.AlreadyMissing++
			return
		}
		report.Errors++
		return
	}
	report.Reclaimed++
	report.ExpiredBytes += size
}

// markUploadContentExpired preserves any metadata update that races logical
// expiry, such as clearing ResultPending after result-event publication. The
// marker is committed before Object Store deletion so a failed metadata write
// can never leave missing content without an explicit expired state.
func (c *Client) markUploadContentExpired(ctx context.Context, versioned VersionedUploadRecord, deadline, now time.Time) (bool, error) {
	var lastErr error
	for range 4 {
		record := versioned.Record
		if record.ContentExpiredAt != "" {
			// Another sweeper committed the same expiry.
			return false, nil
		}
		projected, err := c.ProjectUploadRecord(ctx, record)
		if err != nil {
			return false, err
		}
		if projected.Status != StatusValid && projected.Status != StatusInvalid {
			return false, fmt.Errorf("upload %s left terminal state during content expiry", record.ID)
		}
		record.ContentExpiresAt = deadline.UTC().Format(time.RFC3339)
		record.ContentExpiredAt = now.UTC().Format(time.RFC3339)
		if _, err := c.UpdateUploadRecord(ctx, record, versioned.Revision); err == nil {
			return true, nil
		} else {
			lastErr = err
		}

		latest, err := c.GetUploadRecordVersion(ctx, record.ID)
		if err != nil {
			return false, err
		}
		versioned = latest
	}
	return false, fmt.Errorf("mark upload content expired after concurrent metadata updates: %w", lastErr)
}

func contentExpiryDeadline(record UploadRecord, retention time.Duration) (time.Time, error) {
	validatedAt, err := time.Parse(time.RFC3339, record.ValidatedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse validation time for content expiry %s: %w", record.ID, err)
	}
	return validatedAt.Add(retention).UTC(), nil
}

// UploadContentStorageSize reports the Object Store's current physical size,
// including JetStream object metadata.
func (c *Client) UploadContentStorageSize(ctx context.Context) (uint64, error) {
	// nats.go's ObjectStore Status refreshes mutable stream information on its
	// handle. Use a maintenance-only handle so the cached hot-path handle can
	// serve concurrent Get/Put/List calls without racing that refresh.
	store, err := c.JS.ObjectStore(ctx, c.Namespace.UploadContentBucketName())
	if err != nil {
		return 0, fmt.Errorf("open upload content Object Store for status: %w", err)
	}
	status, err := store.Status(ctx)
	if err != nil {
		return 0, fmt.Errorf("read upload content Object Store status: %w", err)
	}
	return status.Size(), nil
}
