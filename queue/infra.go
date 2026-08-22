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

// uploadRecordMaxValueSize caps a single KV entry's size: the 10 MiB upload
// body cap enforced by the HTTP API, plus room for JSON metadata overhead.
const uploadRecordMaxValueSize = 12 << 20

// EnsureInfra idempotently creates the streams and KV bucket the queue
// package depends on. Both the HTTP API and the validation worker call
// this on startup, since either may start first.
func (c *Client) EnsureInfra(ctx context.Context) error {
	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        DocumentsStreamName,
		Description: "Incoming UBL 2.1 documents awaiting XSD validation, one subject per document type.",
		Subjects:    []string{documentSubjectWildcard},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      documentsStreamMaxAge,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", DocumentsStreamName, err)
	}

	if _, err := c.JS.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        ResultsStreamName,
		Description: "UBL 2.1 document validation results, published once per processed document.",
		Subjects:    []string{ResultsSubject},
		Retention:   jetstream.LimitsPolicy,
		Storage:     jetstream.FileStorage,
		MaxAge:      resultsStreamMaxAge,
	}); err != nil {
		return fmt.Errorf("ensure %s stream: %w", ResultsStreamName, err)
	}

	if _, err := c.JS.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:       UploadsBucketName,
		Description:  "Durable record (status, content, validation outcome) for every UBL upload, keyed by upload id.",
		Storage:      jetstream.FileStorage,
		MaxValueSize: uploadRecordMaxValueSize,
	}); err != nil {
		return fmt.Errorf("ensure %s KV bucket: %w", UploadsBucketName, err)
	}

	return nil
}
