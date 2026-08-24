package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// UploadRecord is the durable, canonical record of a single accepted
// upload, stored in the queue Namespace's uploads KV bucket (see
// Namespace.UploadsBucketName) keyed by ID. It is the sole source of truth
// for GET /api/v1/ubl/{id}: created (Status Pending) by the HTTP API when a
// document is queued, then updated by the worker once validation
// completes.
type UploadRecord struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"documentType"`
	Status       ValidationStatus `json:"status"`
	Error        string           `json:"error,omitempty"`
	Content      string           `json:"content"`
	ReceivedAt   string           `json:"receivedAt"`
	ValidatedAt  string           `json:"validatedAt,omitempty"`
}

// ValidID reports whether id has the shape produced by NewID: a 32-character
// lowercase hex string. Callers must check this before using an
// externally-supplied id (e.g. a GET /api/v1/ubl/{id} path parameter) as a
// KV key, both to reject garbage early and to keep the constraint on what a
// valid upload id looks like in one place.
var ValidID = regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString

// NewID generates a new random upload id (128 bits of randomness, hex
// encoded), matching ValidID.
func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// ErrRecordNotFound is returned by GetUploadRecord when id does not exist
// (or is not well-formed).
var ErrRecordNotFound = errors.New("upload record not found")

// PutUploadRecord creates or overwrites the record for record.ID.
func (c *Client) PutUploadRecord(ctx context.Context, record UploadRecord) error {
	if !ValidID(record.ID) {
		return fmt.Errorf("invalid upload id %q", record.ID)
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal upload record %s: %w", record.ID, err)
	}
	if _, err := kv.Put(ctx, record.ID, encoded); err != nil {
		return fmt.Errorf("put upload record %s: %w", record.ID, err)
	}
	return nil
}

// GetUploadRecord retrieves the record for id, or ErrRecordNotFound if none
// exists.
func (c *Client) GetUploadRecord(ctx context.Context, id string) (UploadRecord, error) {
	if !ValidID(id) {
		return UploadRecord{}, ErrRecordNotFound
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return UploadRecord{}, err
	}
	entry, err := kv.Get(ctx, id)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return UploadRecord{}, ErrRecordNotFound
		}
		return UploadRecord{}, fmt.Errorf("get upload record %s: %w", id, err)
	}
	var record UploadRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return UploadRecord{}, fmt.Errorf("decode upload record %s: %w", id, err)
	}
	return record, nil
}

// UpdateUploadStatus loads the record for id, applies the given validation
// outcome, and writes it back. Used by the worker once it has validated (or
// failed to validate) a document.
func (c *Client) UpdateUploadStatus(ctx context.Context, id string, status ValidationStatus, validationErr string, validatedAt time.Time) error {
	record, err := c.GetUploadRecord(ctx, id)
	if err != nil {
		return err
	}
	record.Status = status
	record.Error = validationErr
	record.ValidatedAt = validatedAt.UTC().Format(time.RFC3339)
	return c.PutUploadRecord(ctx, record)
}

func (c *Client) uploadsBucket(ctx context.Context) (jetstream.KeyValue, error) {
	uploadsBucketName := c.Namespace.UploadsBucketName()
	kv, err := c.JS.KeyValue(ctx, uploadsBucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", uploadsBucketName, err)
	}
	return kv, nil
}
