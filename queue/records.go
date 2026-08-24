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

// UploadRecord is the durable, canonical metadata record of a single
// accepted upload, stored in the queue Namespace's uploads KV bucket (see
// Namespace.UploadsBucketName) keyed by ID. It is the sole source of truth
// for GET /api/v1/ubl/{id}'s status/metadata fields: created (Status
// Pending) by the HTTP API when a document is queued, then updated by the
// worker once validation completes.
//
// UploadRecord deliberately does not carry the upload's raw XML content:
// that is stored separately (see PutUploadContent/GetUploadContent), so
// that UpdateUploadStatus - called once per document by the worker, and
// far more often than any human ever calls PutUploadRecord directly -
// never has to read or rewrite the (potentially large) document body just
// to flip a status field.
type UploadRecord struct {
	ID           string           `json:"id"`
	DocumentType string           `json:"documentType"`
	Status       ValidationStatus `json:"status"`
	Error        string           `json:"error,omitempty"`
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

// ErrRecordNotFound is returned by GetUploadRecord (or GetUploadContent)
// when id does not exist (or is not well-formed).
var ErrRecordNotFound = errors.New("upload record not found")

// PutUploadRecord creates or overwrites the metadata record for record.ID.
// It never reads or writes the upload's raw XML content - see
// PutUploadContent - so callers that only need to change status/metadata
// (e.g. UpdateUploadStatus) pay a cost proportional to the small metadata
// record, not to the document size.
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

// GetUploadRecord retrieves the metadata record for id, or
// ErrRecordNotFound if none exists. It does not fetch the upload's raw XML
// content - see GetUploadContent.
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

// UpdateUploadStatus loads the metadata record for id, applies the given
// validation outcome, and writes it back. Used by the worker once it has
// validated (or failed to validate) a document. Because UploadRecord no
// longer carries the raw XML content, this reads and rewrites only the
// small metadata record - the (potentially large) document content stored
// via PutUploadContent is never touched.
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

// PutUploadContent stores content as the immutable raw XML body for
// upload id, in the separate upload-content KV bucket (see
// Namespace.UploadContentBucketName). content is written to the KV entry
// directly, with no JSON wrapping and no []byte<->string conversion, so
// storing an upload's content costs exactly one copy (the unavoidable
// network/storage write), not an extra copy on top of it.
func (c *Client) PutUploadContent(ctx context.Context, id string, content []byte) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid upload id %q", id)
	}
	kv, err := c.uploadContentBucket(ctx)
	if err != nil {
		return err
	}
	if _, err := kv.Put(ctx, id, content); err != nil {
		return fmt.Errorf("put upload content %s: %w", id, err)
	}
	return nil
}

// GetUploadContent retrieves the raw XML content previously stored for id
// via PutUploadContent, or ErrRecordNotFound if none exists.
func (c *Client) GetUploadContent(ctx context.Context, id string) ([]byte, error) {
	if !ValidID(id) {
		return nil, ErrRecordNotFound
	}
	kv, err := c.uploadContentBucket(ctx)
	if err != nil {
		return nil, err
	}
	entry, err := kv.Get(ctx, id)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("get upload content %s: %w", id, err)
	}
	return entry.Value(), nil
}

func (c *Client) uploadsBucket(ctx context.Context) (jetstream.KeyValue, error) {
	uploadsBucketName := c.Namespace.UploadsBucketName()
	kv, err := c.JS.KeyValue(ctx, uploadsBucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", uploadsBucketName, err)
	}
	return kv, nil
}

func (c *Client) uploadContentBucket(ctx context.Context) (jetstream.KeyValue, error) {
	uploadContentBucketName := c.Namespace.UploadContentBucketName()
	kv, err := c.JS.KeyValue(ctx, uploadContentBucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", uploadContentBucketName, err)
	}
	return kv, nil
}
