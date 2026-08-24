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

	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish validation result for %s: %w", result.ID, err)
	}
	return nil
}
