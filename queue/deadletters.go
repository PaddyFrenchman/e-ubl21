package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// DeadLetter is the durable operator-facing record of exhausted processing.
type DeadLetter struct {
	ID           string `json:"id"`
	DocumentType string `json:"documentType"`
	ReceivedAt   string `json:"receivedAt"`
	LastError    string `json:"lastError"`
	Attempts     uint64 `json:"attempts"`
	FailedAt     string `json:"failedAt"`
}

func (c *Client) PublishDeadLetter(ctx context.Context, dead DeadLetter) error {
	if !ValidID(dead.ID) {
		return fmt.Errorf("invalid dead-letter upload id %q", dead.ID)
	}
	payload, err := json.Marshal(dead)
	if err != nil {
		return fmt.Errorf("marshal dead letter %s: %w", dead.ID, err)
	}
	msg := nats.NewMsg(c.Namespace.DeadLetterSubject(dead.ID))
	msg.Data = payload
	msg.Header.Set(HeaderUploadID, dead.ID)
	msg.Header.Set(HeaderDocumentType, dead.DocumentType)
	// MaxMsgsPerSubject provides durable one-entry idempotence. A fresh broker
	// message ID also lets replay restore the entry after purging it within the
	// stream duplicate window if requeue publication fails.
	msg.Header.Set(nats.MsgIdHdr, fmt.Sprintf("%s:dead-letter:%d", dead.ID, time.Now().UnixNano()))
	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		var apiErr *jetstream.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode == jsErrCodeStreamMaxMsgsPerSubject {
			return nil
		}
		return fmt.Errorf("publish dead letter %s: %w", dead.ID, err)
	}
	return nil
}

func (c *Client) DeadLetterDepth(ctx context.Context) (uint64, error) {
	stream, err := c.JS.Stream(ctx, c.Namespace.DeadLettersStreamName())
	if err != nil {
		return 0, fmt.Errorf("open dead-letter stream: %w", err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return 0, fmt.Errorf("read dead-letter stream: %w", err)
	}
	return info.State.Msgs, nil
}

// ReplayProcessingFailure safely re-enqueues a terminal failure. The
// enqueue_pending transition makes a failed publication visible/retryable.
func (c *Client) ReplayProcessingFailure(ctx context.Context, id string) error {
	versioned, err := c.GetUploadRecordVersion(ctx, id)
	if err != nil {
		return err
	}
	record := versioned.Record
	if record.Status != StatusProcessingFailed && record.Status != StatusEnqueuePending {
		return fmt.Errorf("upload %s is %s, not processing_failed", id, record.Status)
	}
	if ok, err := c.HasUploadContent(ctx, id); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("upload %s has no raw content to replay", id)
	}
	if record.Status == StatusProcessingFailed {
		record.Status = StatusEnqueuePending
		record.Error = ""
		record.ResultPending = false
		if revision, err := c.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
			return err
		} else {
			versioned.Revision = revision
		}
	}
	receivedAt, err := time.Parse(time.RFC3339, record.ReceivedAt)
	if err != nil {
		return fmt.Errorf("parse receivedAt for %s: %w", id, err)
	}
	stream, err := c.JS.Stream(ctx, c.Namespace.DeadLettersStreamName())
	if err != nil {
		return err
	}
	if err := stream.Purge(ctx, jetstream.WithPurgeSubject(c.Namespace.DeadLetterSubject(id))); err != nil {
		return err
	}
	if err := c.publishDocument(ctx, id, record.DocumentType, receivedAt, fmt.Sprintf("%s:replay:%d", id, time.Now().UnixNano())); err != nil {
		// Restore operator visibility when requeue fails after the pending
		// transition. Replaying enqueue_pending is safe and idempotent.
		_ = c.PublishDeadLetter(ctx, DeadLetter{ID: id, DocumentType: record.DocumentType, ReceivedAt: record.ReceivedAt, LastError: record.LastError, Attempts: record.ProcessingAttempts, FailedAt: record.ProcessingFailedAt})
		return err
	}
	record.Status = StatusPending
	if _, err := c.UpdateUploadRecord(ctx, record, versioned.Revision); err != nil {
		latest, getErr := c.GetUploadRecord(ctx, id)
		if getErr != nil || (latest.Status != StatusValid && latest.Status != StatusInvalid) {
			return err
		}
	}
	return nil
}
