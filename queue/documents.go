package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// PublishDocument publishes a raw UBL document body to its per-document-type
// subject (see DocumentSubject) on DocumentsStreamName, tagging it with the
// upload id, document type and received-at time as message headers so a
// consumer never needs to parse the payload just to route or correlate it.
// The upload id also doubles as the JetStream de-duplication key (Nats-Msg-Id),
// so an accidental double-publish of the same accepted upload is not queued
// twice for processing.
func (c *Client) PublishDocument(ctx context.Context, id, documentType string, receivedAt time.Time, body []byte) error {
	msg := nats.NewMsg(DocumentSubject(documentType))
	msg.Data = body
	msg.Header.Set(HeaderUploadID, id)
	msg.Header.Set(HeaderDocumentType, documentType)
	msg.Header.Set(HeaderReceivedAt, receivedAt.UTC().Format(time.RFC3339))
	msg.Header.Set(nats.MsgIdHdr, id)

	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish document %s to %s: %w", id, msg.Subject, err)
	}
	return nil
}

// DocumentsConsumer creates (or reuses) a durable pull consumer named
// durableName on DocumentsStreamName, filtered to filterSubject. Passing
// DocumentsFilterSubject processes every document type from a single
// consumer; a narrower filter (e.g. DocumentSubject("Invoice")) dedicates
// the consumer to just one document type's queue.
func (c *Client) DocumentsConsumer(ctx context.Context, durableName, filterSubject string) (jetstream.Consumer, error) {
	if filterSubject == "" {
		filterSubject = DocumentsFilterSubject
	}
	consumer, err := c.JS.CreateOrUpdateConsumer(ctx, DocumentsStreamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		Description:   "Validates queued UBL 2.1 documents against their XSD and publishes the outcome.",
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    5,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s consumer on %s: %w", durableName, DocumentsStreamName, err)
	}
	return consumer, nil
}
