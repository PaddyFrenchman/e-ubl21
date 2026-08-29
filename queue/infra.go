package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// documentsStreamMaxAge is a safety-net retention cap on the documents
// work-queue stream: messages are normally removed as soon as the worker
// acknowledges them, but this bounds storage growth if no worker is
// running to consume a document type's queue for an extended period.
const documentsStreamMaxAge = 7 * 24 * time.Hour

// uploadMetadataMaxValueSize caps a single upload metadata KV entry's
// size. The raw XML content is stored separately in an Object Store, so a
// metadata record only ever holds a handful of small fields (id, document
// type, status, error message and timestamps).
const uploadMetadataMaxValueSize = 64 << 10

// validationExecutionMaxValueSize bounds frozen plan and execution metadata.
// Stage evidence is introduced separately and raw XML never belongs here.
const validationExecutionMaxValueSize = 64 << 10

// A complete valid StageResult can approach the normative 2 MiB result-stream
// ceiling. Attempt evidence is stored separately from compact execution state.
const validationEvidenceMaxValueSize = 2 << 20

// InfraConfig controls deployer-owned JetStream storage limits. A zero
// UploadContentMaxBytes leaves the Object Store unlimited; raw-content expiry
// is coordinated by the application after validation rather than by the
// Object Store TTL, so queued documents cannot disappear before processing.
type InfraConfig struct {
	UploadContentMaxBytes int64
}

// EnsureInfra idempotently creates the streams, metadata KV bucket and raw
// XML Object Store the queue package depends on. Both the HTTP API and the
// validation worker call this on startup, since either may start first.
func (c *Client) EnsureInfra(ctx context.Context) error {
	return c.EnsureInfraWithConfig(ctx, InfraConfig{})
}

// EnsureInfraWithConfig is EnsureInfra with explicit storage limits.
func (c *Client) EnsureInfraWithConfig(ctx context.Context, cfg InfraConfig) error {
	if cfg.UploadContentMaxBytes < 0 {
		return fmt.Errorf("upload content max bytes must be non-negative")
	}
	documentsStreamName := c.Namespace.DocumentsStreamName()
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        documentsStreamName,
		Description: "Incoming UBL 2.1 validation work descriptors, one subject per document type.",
		Subjects:    []string{c.Namespace.documentSubjectWildcard()},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      documentsStreamMaxAge,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", documentsStreamName, err)
	}

	deadLettersStreamName := c.Namespace.DeadLettersStreamName()
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:                 deadLettersStreamName,
		Description:          "Terminal UBL processing failures retained for operator inspection and replay.",
		Subjects:             []string{c.Namespace.DeadLettersFilterSubject()},
		Retention:            jetstream.LimitsPolicy,
		Storage:              jetstream.FileStorage,
		Discard:              jetstream.DiscardNew,
		DiscardNewPerSubject: true,
		MaxMsgsPerSubject:    1,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", deadLettersStreamName, err)
	}

	resultsStreamName := c.Namespace.ResultsStreamName()
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:                 resultsStreamName,
		Description:          "UBL 2.1 validation results, durably unique by result identity.",
		Subjects:             []string{c.Namespace.ResultsFilterSubject()},
		Retention:            jetstream.LimitsPolicy,
		Storage:              jetstream.FileStorage,
		Discard:              jetstream.DiscardNew,
		DiscardNewPerSubject: true,
		MaxMsgsPerSubject:    1,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", resultsStreamName, err)
	}

	uploadsBucketName := c.Namespace.UploadsBucketName()
	uploadsKV, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       uploadsBucketName,
		Description:  "Durable metadata record (status, error, timestamps) for every UBL upload, keyed by upload id. Raw XML is stored separately in the upload-content Object Store.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: uploadMetadataMaxValueSize,
	})
	if err != nil {
		return fmt.Errorf("ensure %s KV bucket: %w", uploadsBucketName, err)
	}

	uploadContentBucketName := c.Namespace.UploadContentBucketName()
	uploadContentStore, err := c.JS.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Bucket:      uploadContentBucketName,
		Description: "Immutable raw XML content of every UBL upload, keyed by upload id and chunked by JetStream Object Store.",
		Storage:     jetstream.FileStorage,
		MaxBytes:    cfg.UploadContentMaxBytes,
	})
	if err != nil {
		return fmt.Errorf("ensure %s Object Store: %w", uploadContentBucketName, err)
	}

	c.handlesMu.Lock()
	c.cachedUploadsKV = uploadsKV
	c.cachedUploadContentStore = uploadContentStore
	c.handlesMu.Unlock()
	return nil
}

// ProvisionValidationExecutionInfra is the deployer-owned mutation that
// creates or updates the execution bucket. Runtime services must use
// VerifyValidationExecutionInfra (or a data operation that performs the same
// cold-handle verification) and never call this method during startup.
func (c *Client) ProvisionValidationExecutionInfra(ctx context.Context) error {
	bucketName := c.Namespace.ValidationExecutionsBucketName()
	kv, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       bucketName,
		Description:  "Durable validation execution generations, immutable ordered plans and compact CAS orchestration state.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: validationExecutionMaxValueSize,
	})
	if err != nil {
		return fmt.Errorf("provision %s KV bucket: %w", bucketName, err)
	}
	evidenceBucketName := c.Namespace.ValidationStageEvidenceBucketName()
	evidenceKV, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       evidenceBucketName,
		Description:  "Immutable complete stage-attempt evidence keyed by execution identity, stage ordinal and attempt.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: validationEvidenceMaxValueSize,
	})
	if err != nil {
		return fmt.Errorf("provision %s KV bucket: %w", evidenceBucketName, err)
	}
	c.handlesMu.Lock()
	c.cachedValidationExecutionsKV = kv
	c.cachedValidationEvidenceKV = evidenceKV
	c.handlesMu.Unlock()
	return nil
}

// VerifyValidationExecutionInfra performs a read-only compatibility check and
// caches the verified runtime handle.
func (c *Client) VerifyValidationExecutionInfra(ctx context.Context) error {
	if _, err := c.validationExecutionsBucket(ctx); err != nil {
		return err
	}
	_, err := c.validationEvidenceBucket(ctx)
	return err
}
