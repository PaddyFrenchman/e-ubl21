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
	if record.Status == StatusProcessingFailed {
		if ok, contentErr := c.HasUploadContent(ctx, id); contentErr != nil {
			return contentErr
		} else if !ok {
			return fmt.Errorf("upload %s has no raw content to replay", id)
		}
		record.Status = StatusEnqueuePending
		record.Error = ""
		record.ResultPending = false
		record.ReplayGeneration++
		record.ReplayMessageID = fmt.Sprintf("%s:replay:%d", id, record.ReplayGeneration)
		record.ReplayReady = false
		record.ReplayClaimedSeq = 0
		revision, updateErr := c.UpdateUploadRecord(ctx, record, versioned.Revision)
		if updateErr != nil {
			return updateErr
		}
		versioned.Revision = revision
	} else if record.ReplayMessageID == "" || (record.Status != StatusEnqueuePending && record.Status != StatusPending && record.Status != StatusValid && record.Status != StatusInvalid) {
		if record.Status == StatusPending && record.ReplayClaimedSeq != 0 {
			return nil
		}
		return fmt.Errorf("upload %s is %s and has no replay in progress", id, record.Status)
	}
	receivedAt, err := time.Parse(time.RFC3339, record.ReceivedAt)
	if err != nil {
		return fmt.Errorf("parse receivedAt for %s: %w", id, err)
	}
	if record.Status == StatusEnqueuePending {
		if err := c.PublishDocumentWithID(ctx, id, record.DocumentType, receivedAt, record.ReplayMessageID); err != nil {
			return err
		}
		record.Status = StatusPending
		revision, updateErr := c.UpdateUploadRecord(ctx, record, versioned.Revision)
		if updateErr != nil {
			latest, getErr := c.GetUploadRecordVersion(ctx, id)
			if getErr != nil || latest.Record.ReplayMessageID != record.ReplayMessageID || (latest.Record.Status != StatusPending && latest.Record.Status != StatusValid && latest.Record.Status != StatusInvalid) {
				return updateErr
			}
			versioned, record = latest, latest.Record
		} else {
			versioned.Revision = revision
		}
	}
	stream, err := c.JS.Stream(ctx, c.Namespace.DeadLettersStreamName())
	if err != nil {
		return err
	}
	if err := stream.Purge(ctx, jetstream.WithPurgeSubject(c.Namespace.DeadLetterSubject(id))); err != nil {
		return err
	}
	// Mark the replay ready last. Workers defer descriptors until this handoff
	// completes, then one descriptor permanently claims its stream sequence by
	// CAS. Keeping ReplayMessageID until that claim closes the duplicate-window
	// gap without making operator publication and KV mutation atomic.
	for i := 0; i < 3; i++ {
		latest, getErr := c.GetUploadRecordVersion(ctx, id)
		if getErr != nil {
			return getErr
		}
		if latest.Record.ReplayMessageID == "" && latest.Record.ReplayClaimedSeq != 0 {
			return nil
		}
		if latest.Record.ReplayMessageID != record.ReplayMessageID {
			return fmt.Errorf("upload %s replay generation changed concurrently", id)
		}
		latest.Record.ReplayReady = true
		if _, updateErr := c.UpdateUploadRecord(ctx, latest.Record, latest.Revision); updateErr == nil {
			return nil
		} else if i == 2 {
			return updateErr
		}
	}
	return nil
}
