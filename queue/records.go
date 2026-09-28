package queue

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/observability"
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
// that is stored separately in the configured document store (see
// PutUploadContent/GetUploadContent), so status updates never have to read
// or rewrite the potentially large document body.
type UploadRecord struct {
	ID               string            `json:"id"`
	DocumentType     string            `json:"documentType"`
	RequestedProfile ValidationProfile `json:"requestedProfile,omitempty"`
	ResolvedProfile  ValidationProfile `json:"resolvedProfile,omitempty"`
	// Profile is the legacy alias for ResolvedProfile and remains populated for
	// stored-record and HTTP response compatibility.
	Profile            ValidationProfile           `json:"profile,omitempty"`
	Status             ValidationStatus            `json:"status"`
	Error              string                      `json:"error,omitempty"`
	ReceivedAt         string                      `json:"receivedAt"`
	ValidatedAt        string                      `json:"validatedAt,omitempty"`
	ValidationStages   ValidationStages            `json:"validationStages,omitempty"`
	ContentExpiresAt   string                      `json:"contentExpiresAt,omitempty"`
	ContentExpiredAt   string                      `json:"contentExpiredAt,omitempty"`
	ContentSHA256      string                      `json:"contentSha256,omitempty"`
	ResultPending      bool                        `json:"resultPending,omitempty"`
	LastError          string                      `json:"lastError,omitempty"`
	ProcessingAttempts uint64                      `json:"processingAttempts,omitempty"`
	ProcessingFailedAt string                      `json:"processingFailedAt,omitempty"`
	ReplayGeneration   uint64                      `json:"replayGeneration,omitempty"`
	ReplayMessageID    string                      `json:"replayMessageId,omitempty"`
	ReplayReady        bool                        `json:"replayReady,omitempty"`
	ReplayClaimedSeq   uint64                      `json:"replayClaimedSequence,omitempty"`
	EnqueueTrace       *observability.TraceCarrier `json:"enqueueTrace,omitempty"`
}

// VersionedUploadRecord pairs a record with its JetStream KV revision for
// compare-and-swap state transitions.
type VersionedUploadRecord struct {
	Record   UploadRecord
	Revision uint64
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

// UploadContentReader streams an Object Store object and reports errors that
// can arrive after OpenUploadContent has returned (for example, a missing or
// corrupt chunk). Callers must close it and check Error after consuming it.
type UploadContentReader interface {
	io.ReadCloser
	Error() error
}

// PutUploadRecord creates or overwrites the metadata record for record.ID.
// It never reads or writes raw XML content.
func (c *Client) PutUploadRecord(ctx context.Context, record UploadRecord) error {
	if store := c.UploadMetadataStore(); store != nil {
		return store.Put(ctx, record)
	}
	if !ValidID(record.ID) {
		return fmt.Errorf("invalid upload id %q", record.ID)
	}
	if err := validateUploadTrace(record); err != nil {
		return err
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

// CreateUploadRecord creates durable intent only if the key does not exist.
func (c *Client) CreateUploadRecord(ctx context.Context, record UploadRecord) (uint64, error) {
	if store := c.UploadMetadataStore(); store != nil {
		return store.Create(ctx, record)
	}
	if !ValidID(record.ID) {
		return 0, fmt.Errorf("invalid upload id %q", record.ID)
	}
	if err := validateUploadTrace(record); err != nil {
		return 0, err
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("marshal upload record %s: %w", record.ID, err)
	}
	revision, err := kv.Create(ctx, record.ID, encoded)
	if err != nil {
		return 0, fmt.Errorf("create upload record %s: %w", record.ID, err)
	}
	return revision, nil
}

// UpdateUploadRecord performs a CAS update at expectedRevision.
func (c *Client) UpdateUploadRecord(ctx context.Context, record UploadRecord, expectedRevision uint64) (uint64, error) {
	if store := c.UploadMetadataStore(); store != nil {
		return store.Update(ctx, record, expectedRevision)
	}
	if !ValidID(record.ID) {
		return 0, fmt.Errorf("invalid upload id %q", record.ID)
	}
	if err := validateUploadTrace(record); err != nil {
		return 0, err
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return 0, fmt.Errorf("marshal upload record %s: %w", record.ID, err)
	}
	revision, err := kv.Update(ctx, record.ID, encoded, expectedRevision)
	if err != nil {
		return 0, fmt.Errorf("update upload record %s at revision %d: %w", record.ID, expectedRevision, err)
	}
	return revision, nil
}

func validateUploadTrace(record UploadRecord) error {
	if record.EnqueueTrace == nil {
		return nil
	}
	if err := record.EnqueueTrace.Validate(); err != nil {
		return fmt.Errorf("upload enqueue trace: %w", err)
	}
	return nil
}

// GetUploadRecord retrieves the metadata record for id, or ErrRecordNotFound
// if none exists. It does not fetch the upload's raw XML content.
func (c *Client) GetUploadRecord(ctx context.Context, id string) (UploadRecord, error) {
	versioned, err := c.GetUploadRecordVersion(ctx, id)
	return versioned.Record, err
}

// GetUploadRecordVersion retrieves a record and the revision required for a
// subsequent monotonic compare-and-swap transition.
func (c *Client) GetUploadRecordVersion(ctx context.Context, id string) (VersionedUploadRecord, error) {
	if !ValidID(id) {
		return VersionedUploadRecord{}, ErrRecordNotFound
	}
	if store := c.UploadMetadataStore(); store != nil {
		return store.Get(ctx, id)
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return VersionedUploadRecord{}, err
	}
	entry, err := kv.Get(ctx, id)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return VersionedUploadRecord{}, ErrRecordNotFound
		}
		return VersionedUploadRecord{}, fmt.Errorf("get upload record %s: %w", id, err)
	}
	var record UploadRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return VersionedUploadRecord{}, fmt.Errorf("decode upload record %s: %w", id, err)
	}
	if err := validateUploadTrace(record); err != nil {
		return VersionedUploadRecord{}, fmt.Errorf("decode upload record %s: %w", id, err)
	}
	return VersionedUploadRecord{Record: record, Revision: entry.Revision()}, nil
}

// UploadRecordKeys returns the current metadata keys for maintenance scans.
// Callers must load each record by version so concurrent state transitions
// remain protected by compare-and-swap.
func (c *Client) UploadRecordKeys(ctx context.Context) ([]string, error) {
	if store := c.UploadMetadataStore(); store != nil {
		return store.Keys(ctx)
	}
	kv, err := c.uploadsBucket(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("list upload records: %w", err)
	}
	return keys, nil
}

// UpdateUploadStatus loads the metadata record for id, applies the given
// validation outcome, and writes it back. The raw XML object is never read
// or rewritten.
func (c *Client) UpdateUploadStatus(ctx context.Context, id string, status ValidationStatus, validationErr string, validatedAt time.Time) error {
	versioned, err := c.GetUploadRecordVersion(ctx, id)
	if err != nil {
		return err
	}
	record := versioned.Record
	record.Status = status
	record.Error = validationErr
	record.ValidatedAt = validatedAt.UTC().Format(time.RFC3339)
	_, err = c.UpdateUploadRecord(ctx, record, versioned.Revision)
	return err
}

// PutUploadContent stores content as the immutable raw XML object for upload
// id. JetStream Object Store chunks large objects internally rather than
// requiring the complete XML to fit in a single KV value.
func (c *Client) PutUploadContent(ctx context.Context, id string, content []byte) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid upload id %q", id)
	}
	if store := c.DocumentStore(); store != nil {
		if claims := c.ClaimStore(); claims != nil {
			if err := claims.Put(ctx, store, documentstore.ObjectKey(id), content); err != nil {
				return fmt.Errorf("guarded put upload content %s: %w", id, err)
			}
			return nil
		}
		if err := store.Put(ctx, documentstore.ObjectKey(id), bytes.NewReader(content)); err != nil {
			return fmt.Errorf("put upload content %s: %w", id, err)
		}
		return nil
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
	content, err := c.OpenUploadContent(ctx, id)
	if err != nil {
		return nil, err
	}
	defer content.Close()
	body, readErr := io.ReadAll(content)
	if readErr == nil {
		readErr = content.Error()
	}
	if readErr != nil {
		return nil, fmt.Errorf("get upload content %s: %w", id, readErr)
	}
	return body, nil
}

// OpenUploadContent opens the immutable raw XML object for streaming without
// materializing it in memory. Errors encountered while receiving chunks are
// exposed by the returned reader's Error method.
func (c *Client) OpenUploadContent(ctx context.Context, id string) (UploadContentReader, error) {
	if !ValidID(id) {
		return nil, ErrRecordNotFound
	}
	if store := c.DocumentStore(); store != nil {
		content, err := store.Get(ctx, documentstore.ObjectKey(id))
		if err != nil {
			if isNotFound(store, err) {
				return nil, ErrRecordNotFound
			}
			return nil, fmt.Errorf("get upload content %s: %w", id, err)
		}
		return contentReader{ReadCloser: content}, nil
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return nil, err
	}
	content, err := store.Get(ctx, id)
	if err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("get upload content %s: %w", id, err)
	}
	return content, nil
}

// VerifyUploadContentInfra performs a read-only runtime check that the shared
// immutable XML Object Store exists and can be opened. Stage workers use this
// at startup instead of provisioning or mutating infrastructure.
func (c *Client) VerifyUploadContentInfra(ctx context.Context) error {
	if c.DocumentStore() != nil {
		// Exists is a read-only request that verifies the configured bucket,
		// credentials, and network path without creating a probe object.
		_, err := c.DocumentStore().Exists(ctx, ".eubl21-readiness-probe")
		if err != nil {
			return fmt.Errorf("verify configured document store: %w", err)
		}
		return nil
	}
	_, err := c.uploadContentStore(ctx)
	return err
}

// DeleteUploadContent removes a partially persisted upload object. Missing
// objects are treated as already cleaned up.
func (c *Client) DeleteUploadContent(ctx context.Context, id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid upload id %q", id)
	}
	if store := c.DocumentStore(); store != nil {
		err := store.Delete(ctx, documentstore.ObjectKey(id))
		if err != nil && !isNotFound(store, err) {
			return fmt.Errorf("delete upload content %s: %w", id, err)
		}
		return nil
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, id); err != nil && !errors.Is(err, jetstream.ErrObjectNotFound) {
		return fmt.Errorf("delete upload content %s: %w", id, err)
	}
	return nil
}

// HasUploadContent checks object metadata without materializing the XML.
func (c *Client) HasUploadContent(ctx context.Context, id string) (bool, error) {
	if !ValidID(id) {
		return false, nil
	}
	if store := c.DocumentStore(); store != nil {
		return store.Exists(ctx, documentstore.ObjectKey(id))
	}
	store, err := c.uploadContentStore(ctx)
	if err != nil {
		return false, err
	}
	if _, err := store.GetInfo(ctx, id); err != nil {
		if errors.Is(err, jetstream.ErrObjectNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("inspect upload content %s: %w", id, err)
	}
	return true, nil
}

type contentReader struct{ io.ReadCloser }

func (contentReader) Error() error { return nil }
func isNotFound(store documentstore.Store, err error) bool {
	return store.IsNotFound(err)
}

func (c *Client) uploadsBucket(ctx context.Context) (jetstream.KeyValue, error) {
	c.handlesMu.RLock()
	cached := c.cachedUploadsKV
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	c.handlesMu.Lock()
	defer c.handlesMu.Unlock()
	if c.cachedUploadsKV != nil {
		return c.cachedUploadsKV, nil
	}
	uploadsBucketName := c.Namespace.UploadsBucketName()
	kv, err := c.JS.KeyValue(ctx, uploadsBucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", uploadsBucketName, err)
	}
	c.cachedUploadsKV = kv
	return kv, nil
}

func (c *Client) uploadContentStore(ctx context.Context) (jetstream.ObjectStore, error) {
	c.handlesMu.RLock()
	cached := c.cachedUploadContentStore
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	c.handlesMu.Lock()
	defer c.handlesMu.Unlock()
	if c.cachedUploadContentStore != nil {
		return c.cachedUploadContentStore, nil
	}
	bucketName := c.Namespace.UploadContentBucketName()
	store, err := c.JS.ObjectStore(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s Object Store: %w", bucketName, err)
	}
	c.cachedUploadContentStore = store
	return store, nil
}
