package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// DocumentWorkItem is the deliberately small payload carried by the
// documents work stream. The raw XML is stored once in the upload-content
// Object Store and fetched by the worker using ID, avoiding a second
// multi-megabyte durable copy in the work stream.
type DocumentWorkItem struct {
	ID           string `json:"id"`
	DocumentType string `json:"documentType"`
	ReceivedAt   string `json:"receivedAt"`
}

// DecodeDocumentWorkItem decodes and minimally validates a queued work
// descriptor. It is exported so workers and integration tests share the
// same wire-format validation.
func DecodeDocumentWorkItem(data []byte) (DocumentWorkItem, error) {
	var item DocumentWorkItem
	if err := json.Unmarshal(data, &item); err != nil {
		return DocumentWorkItem{}, fmt.Errorf("decode document work item: %w", err)
	}
	if !ValidID(item.ID) {
		return DocumentWorkItem{}, fmt.Errorf("invalid upload id %q in document work item", item.ID)
	}
	if item.DocumentType == "" {
		return DocumentWorkItem{}, fmt.Errorf("document type is required in document work item")
	}
	if item.ReceivedAt == "" {
		return DocumentWorkItem{}, fmt.Errorf("receivedAt is required in document work item")
	}
	return item, nil
}

// PublishDocument publishes a compact work descriptor to the document
// type's subject. The raw XML is intentionally not part of the message; it
// must already have been stored through PutUploadContent. Headers duplicate
// the routing/correlation fields so consumers can reject inconsistent
// messages before dereferencing the stored content.
//
// The upload id also doubles as the JetStream de-duplication key
// (Nats-Msg-Id), so an accidental double-publish of the same accepted upload
// is not queued twice for processing.
func (c *Client) PublishDocument(ctx context.Context, id, documentType string, receivedAt time.Time) error {
	return c.publishDocument(ctx, id, documentType, receivedAt, id, false)
}

// PublishDocumentWithID publishes a descriptor with an explicit stable
// idempotency key. It is used by recoverable operator replay.
func (c *Client) PublishDocumentWithID(ctx context.Context, id, documentType string, receivedAt time.Time, messageID string) error {
	if messageID == "" {
		return fmt.Errorf("document message id is required")
	}
	return c.publishDocument(ctx, id, documentType, receivedAt, messageID, true)
}

func (c *Client) publishDocument(ctx context.Context, id, documentType string, receivedAt time.Time, messageID string, replay bool) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid upload id %q", id)
	}
	if documentType == "" {
		return fmt.Errorf("document type is required")
	}

	item := DocumentWorkItem{
		ID:           id,
		DocumentType: documentType,
		ReceivedAt:   receivedAt.UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal document work item %s: %w", id, err)
	}

	msg := nats.NewMsg(c.Namespace.DocumentSubject(documentType))
	msg.Data = payload
	msg.Header.Set(HeaderUploadID, id)
	msg.Header.Set(HeaderDocumentType, documentType)
	msg.Header.Set(HeaderReceivedAt, item.ReceivedAt)
	msg.Header.Set(nats.MsgIdHdr, messageID)
	if replay {
		msg.Header.Set(HeaderReplayMessageID, messageID)
	}

	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		return fmt.Errorf("publish document work item %s to %s: %w", id, msg.Subject, err)
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
	return c.DocumentsConsumerWithConfig(ctx, durableName, filterSubject, 30*time.Second, 5, 0)
}

// DocumentsConsumerWithConfig creates the documents consumer with explicit
// delivery/backpressure settings. maxAckPending bounds outstanding work
// across the worker's active validations and local prefetch buffer.
func (c *Client) DocumentsConsumerWithConfig(ctx context.Context, durableName, filterSubject string, ackWait time.Duration, maxDeliver, maxAckPending int) (jetstream.Consumer, error) {
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
		AckWait:       ackWait,
		MaxDeliver:    maxDeliver,
		MaxAckPending: maxAckPending,
	})
	if err != nil {
		return nil, fmt.Errorf("create %s consumer on %s: %w", durableName, documentsStreamName, err)
	}
	return consumer, nil
}
