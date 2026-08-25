package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

// ValidationStatus describes the lifecycle state of an uploaded document.
type ValidationStatus string

const (
	// StatusAccepting records durable upload intent while immutable content is
	// being persisted. A stale accepting record is reconciled or failed.
	StatusAccepting ValidationStatus = "accepting"
	// StatusEnqueuePending means metadata and content are durable but work
	// publication has not yet been confirmed. Reconciliation retries it.
	StatusEnqueuePending ValidationStatus = "enqueue_pending"
	// StatusCleanupPending means a failed content write may have left partial
	// Object Store state which reconciliation must reclaim.
	StatusCleanupPending ValidationStatus = "cleanup_pending"
	// StatusPending is the status a document is given the moment it is
	// queued for processing, before the worker has picked it up.
	StatusPending ValidationStatus = "pending"
	// StatusValid means the worker validated the document against its UBL
	// 2.1 XSD successfully.
	StatusValid ValidationStatus = "valid"
	// StatusInvalid means the worker validated the document against its
	// UBL 2.1 XSD and it failed (see ValidationResult.Error /
	// UploadRecord.Error for details).
	StatusInvalid ValidationStatus = "invalid"
	// StatusAcceptanceFailed is a terminal ingress failure for which no work
	// will be queued. Any partially written content is reclaimed.
	StatusAcceptanceFailed ValidationStatus = "acceptance_failed"
)

// ValidationResult is the payload published to Namespace.ResultsSubject
// once a document has been processed by the worker.
type ValidationResult struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"documentType"`
	Status       ValidationStatus `json:"status"`
	Error        string           `json:"error,omitempty"`
	ValidatedAt  string           `json:"validatedAt"`
}

// PublishResult publishes result to this client's namespace's single
// output results stream.
func (c *Client) PublishResult(ctx context.Context, result ValidationResult) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal validation result for %s: %w", result.ID, err)
	}
	msg := nats.NewMsg(c.Namespace.ResultsSubject())
	msg.Data = payload
	msg.Header.Set(HeaderUploadID, result.ID)
	msg.Header.Set(HeaderDocumentType, result.DocumentType)
	msg.Header.Set(nats.MsgIdHdr, result.ID+":validation-result")

	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish validation result for %s: %w", result.ID, err)
	}
	return nil
}
