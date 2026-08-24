package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// PublishDocument publishes a raw UBL document body to its per-document-type
// subject (see Namespace.DocumentSubject) on this client's namespace's
// documents stream, tagging it with the upload id, document type and
// received-at time as message headers so a consumer never needs to parse
// the payload just to route or correlate it.
// The upload id also doubles as the JetStream de-duplication key (Nats-Msg-Id),
// so an accidental double-publish of the same accepted upload is not queued
// twice for processing.
func (c *Client) PublishDocument(ctx context.Context, id, documentType string, receivedAt time.Time, body []byte) error {
	msg := nats.NewMsg(c.Namespace.DocumentSubject(documentType))
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
// durableName on this client's namespace's documents stream, filtered to
// filterSubject. Passing an empty filterSubject (or
// c.Namespace.DocumentsFilterSubject()) processes every document type from
// a single consumer; a narrower filter (e.g.
// c.Namespace.DocumentSubject("Invoice")) dedicates the consumer to just
// one document type's queue. filterSubject must belong to this client's
// namespace (see Namespace.HasDocumentSubject); this stops a
// misconfigured filter from silently subscribing to another namespace's
// documents (or, being a subject with no matching stream, from failing
// only once messages should have started arriving).
func (c *Client) DocumentsConsumer(ctx context.Context, durableName, filterSubject string) (jetstream.Consumer, error) {
	if filterSubject == "" {
		filterSubject = c.Namespace.DocumentsFilterSubject()
	} else if !c.Namespace.HasDocumentSubject(filterSubject) {
		return nil, fmt.Errorf("subject filter %q does not belong to queue namespace %q", filterSubject, c.Namespace)
	}
	documentsStreamName := c.Namespace.DocumentsStreamName()
	consumer, err := c.JS.CreateOrUpdateConsumer(ctx, documentsStreamName, jetstream.ConsumerConfig{
		Durable:       durableName,
		Description:   "Validates queued UBL 2.1 documents against their XSD and publishes the outcome.",
		FilterSubject: filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    5,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s consumer on %s: %w", durableName, documentsStreamName, err)
	}
	return consumer, nil
}
