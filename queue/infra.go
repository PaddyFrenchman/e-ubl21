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

// resultsStreamMaxAge bounds how long published validation results are
// retained for downstream consumers to pick up.
const resultsStreamMaxAge = 7 * 24 * time.Hour

// uploadMetadataMaxValueSize caps a single upload metadata KV entry's
// size. Since the raw XML content is stored separately (see
// uploadContentMaxValueSize), a metadata record only ever holds a handful
// of small fields (id, document type, status, error message, timestamps),
// so this is generous headroom rather than a tight fit.
const uploadMetadataMaxValueSize = 64 << 10

// uploadContentMaxValueSize caps a single upload content KV entry's size:
// the 10 MiB upload body cap enforced by the HTTP API, plus a safety
// margin.
const uploadContentMaxValueSize = 12 << 20

// EnsureInfra idempotently creates the streams and KV bucket the queue
// package depends on. Both the HTTP API and the validation worker call
// this on startup, since either may start first.
func (c *Client) EnsureInfra(ctx context.Context) error {
	documentsStreamName := c.Namespace.DocumentsStreamName()
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        documentsStreamName,
		Description: "Incoming UBL 2.1 documents awaiting XSD validation, one subject per document type.",
		Subjects:    []string{c.Namespace.documentSubjectWildcard()},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      documentsStreamMaxAge,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", documentsStreamName, err)
	}

	resultsStreamName := c.Namespace.ResultsStreamName()
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        resultsStreamName,
		Description: "UBL 2.1 document validation results, published once per processed document.",
		Subjects:    []string{c.Namespace.ResultsSubject()},
		Retention:   jetstream.LimitsPolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      resultsStreamMaxAge,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", resultsStreamName, err)
	}

	uploadsBucketName := c.Namespace.UploadsBucketName()
	if _, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       uploadsBucketName,
		Description:  "Durable metadata record (status, error, timestamps) for every UBL upload, keyed by upload id. The raw XML content itself lives in the separate upload-content bucket.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: uploadMetadataMaxValueSize,
	}); err != nil {
		return fmt.Errorf("ensure %s KV bucket: %w", uploadsBucketName, err)
	}

	uploadContentBucketName := c.Namespace.UploadContentBucketName()
	if _, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       uploadContentBucketName,
		Description:  "Immutable raw XML content of every UBL upload, keyed by the same upload id as the metadata bucket.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: uploadContentMaxValueSize,
	}); err != nil {
		return fmt.Errorf("ensure %s KV bucket: %w", uploadContentBucketName, err)
	}

	return nil
}
