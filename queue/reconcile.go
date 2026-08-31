package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/nats-io/nats.go/jetstream"
)

// ReconcileReport summarizes one acceptance reconciliation scan.
type ReconcileReport struct {
	Scanned, Requeued, Deferred, Failed, Errors int
}

// UploadWorkPublisher is the acceptance-reconciliation handoff. Legacy mode
// publishes a document descriptor; split mode creates a frozen execution.
type UploadWorkPublisher func(context.Context, UploadRecord, time.Time) error

// ReconcileUploads repairs stale accepting/enqueue_pending records. minAge
// prevents a background scan from racing an active HTTP upload; tests and
// explicit operator runs may pass zero.
func (c *Client) ReconcileUploads(ctx context.Context, minAge time.Duration) (ReconcileReport, error) {
	return c.ReconcileUploadsWithPublisher(ctx, minAge, func(ctx context.Context, record UploadRecord, receivedAt time.Time) error {
		return c.PublishDocument(ctx, record.ID, record.DocumentType, receivedAt)
	})
}

// ReconcileUploadsWithPublisher repairs partial acceptance using the active
// deployment's handoff without assuming the legacy documents queue.
func (c *Client) ReconcileUploadsWithPublisher(ctx context.Context, minAge time.Duration, publish UploadWorkPublisher) (ReconcileReport, error) {
	var report ReconcileReport
	if publish == nil {
		return report, errors.New("upload work publisher is required")
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return report, err
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return report, nil
		}
		return report, fmt.Errorf("list upload records for reconciliation: %w", err)
	}
	now := time.Now()
	for _, id := range keys {
		versioned, err := c.GetUploadRecordVersion(ctx, id)
		if err != nil {
			report.Errors++
			continue
		}
		record := versioned.Record
		revision := versioned.Revision
		// Operator replay owns records carrying a stable replay key. Publishing
		// them with the acceptance path's original message ID would defeat replay
		// idempotence and could race DLQ cleanup.
		if record.ReplayMessageID != "" {
			continue
		}
		if record.Status != StatusAccepting && record.Status != StatusEnqueuePending && record.Status != StatusCleanupPending {
			continue
		}
		receivedAt, err := time.Parse(time.RFC3339, record.ReceivedAt)
		if err != nil || now.Sub(receivedAt) < minAge {
			continue
		}
		report.Scanned++
		if record.Status == StatusCleanupPending {
			if err := c.DeleteUploadContent(ctx, id); err != nil {
				report.Errors++
				continue
			}
			record.Status = StatusAcceptanceFailed
			record.Error = "upload content persistence failed; partial content reclaimed"
			if _, err := c.UpdateUploadRecord(ctx, record, revision); err != nil {
				report.Errors++
			} else {
				report.Failed++
			}
			continue
		}
		hasContent, err := c.HasUploadContent(ctx, id)
		if err != nil || !hasContent {
			if err == nil {
				record.Status = StatusAcceptanceFailed
				record.Error = "upload content was not durably persisted"
				if _, putErr := c.UpdateUploadRecord(ctx, record, revision); putErr != nil {
					report.Errors++
				} else {
					report.Failed++
				}
			} else {
				report.Errors++
			}
			continue
		}
		record.Status = StatusEnqueuePending
		if versioned.Record.Status == StatusAccepting {
			record.Error = ""
			revision, err = c.UpdateUploadRecord(ctx, record, revision)
			if err != nil {
				report.Errors++
				continue
			}
		}
		publishCtx := observability.RestoreTrace(ctx, record.EnqueueTrace)
		if err := publish(publishCtx, record, receivedAt); err != nil {
			if errors.Is(err, ErrDeploymentDraining) {
				report.Deferred++
				continue
			}
			record.Error = "awaiting queue publication: " + err.Error()
			_, _ = c.UpdateUploadRecord(ctx, record, revision)
			report.Errors++
			continue
		}
		record.Status = StatusPending
		record.Error = ""
		if _, err := c.UpdateUploadRecord(ctx, record, revision); err != nil {
			// A worker may have already advanced this revision to a terminal
			// validation status. CAS prevents reconciliation from regressing it.
			latest, getErr := c.GetUploadRecord(ctx, id)
			if getErr != nil || (latest.Status != StatusPending && latest.Status != StatusValid && latest.Status != StatusInvalid) {
				report.Errors++
				continue
			}
		}
		report.Requeued++
	}
	return report, nil
}
