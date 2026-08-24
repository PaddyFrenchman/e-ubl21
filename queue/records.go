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
// that is stored separately in JetStream Object Store (see
// PutUploadContent/GetUploadContent), so status updates never have to read
// or rewrite the potentially large document body.
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
// externally-supplied id as a JetStream key/object name.
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

// ErrRecordNotFound is returned by GetUploadRecord or GetUploadContent when
// id does not exist (or is not well-formed).
var ErrRecordNotFound = errors.New("upload record not found")

// PutUploadRecord creates or overwrites the metadata record for record.ID.
// It never reads or writes raw XML content.
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

// GetUploadRecord retrieves the metadata record for id, or ErrRecordNotFound
// if none exists. It does not fetch the upload's raw XML content.
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
// validation outcome, and writes it back. The raw XML object is never read
// or rewritten.
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

// PutUploadContent stores content as the immutable raw XML object for upload
// id. JetStream Object Store chunks large objects internally rather than
// requiring the complete XML to fit in a single KV value.
func (c *Client) PutUploadContent(ctx context.Context, id string, content []byte) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid upload id %q", id)
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return err
	}
	if _, err := store.PutBytes(ctx, id, content); err != nil {
		return fmt.Errorf("put upload content %s: %w", id, err)
	}
	return nil
}

// GetUploadContent retrieves the raw XML object previously stored for id,
// or ErrRecordNotFound if none exists.
func (c *Client) GetUploadContent(ctx context.Context, id string) ([]byte, error) {
	if !ValidID(id) {
		return nil, ErrRecordNotFound
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return nil, err
	}
	content, err := store.GetBytes(ctx, id)
	if err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("get upload content %s: %w", id, err)
	}
	return content, nil
}

func (c *Client) uploadsBucket(ctx context.Context) (jetstream.KeyValue, error) {
	c.handlesMu.RLock()
	cached := c.cachedUploadsKV
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	uploadsBucketName := c.Namespace.UploadsBucketName()
	kv, err := c.JS.KeyValue(ctx, uploadsBucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", uploadsBucketName, err)
	}
	c.handlesMu.Lock()
	if c.cachedUploadsKV == nil {
		c.cachedUploadsKV = kv
	}
	cached = c.cachedUploadsKV
	c.handlesMu.Unlock()
	return cached, nil
}

func (c *Client) uploadContentStore(ctx context.Context) (jetstream.ObjectStore, error) {
	c.handlesMu.RLock()
	cached := c.cachedUploadContentStore
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	bucketName := c.Namespace.UploadContentBucketName()
	store, err := c.JS.ObjectStore(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s Object Store: %w", bucketName, err)
	}
	c.handlesMu.Lock()
	if c.cachedUploadContentStore == nil {
		c.cachedUploadContentStore = store
	}
	cached = c.cachedUploadContentStore
	c.handlesMu.Unlock()
	return cached, nil
}
