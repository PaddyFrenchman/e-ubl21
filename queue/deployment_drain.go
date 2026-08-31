package queue

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
)

// LegacyDrainStatus is the activation barrier for the combined worker queue.
// Ready is true only when no retained, delivered, or waiting pull work exists.
type LegacyDrainStatus struct {
	StreamMessages     uint64 `json:"streamMessages"`
	ConsumerExists     bool   `json:"consumerExists"`
	ConsumerPending    uint64 `json:"consumerPending"`
	ConsumerAckPending int    `json:"consumerAckPending"`
	ConsumerWaiting    int    `json:"consumerWaiting"`
	Ready              bool   `json:"ready"`
}

// InspectLegacyDrain reads the legacy stream and shared durable consumer
// without creating or mutating either resource.
func (c *Client) InspectLegacyDrain(ctx context.Context, consumerName string) (LegacyDrainStatus, error) {
	if consumerName == "" {
		return LegacyDrainStatus{}, errors.New("legacy consumer name is required")
	}
	streamName := c.Namespace.DocumentsStreamName()
	stream, err := c.JS.Stream(ctx, streamName)
	if err != nil {
		return LegacyDrainStatus{}, fmt.Errorf("open legacy documents stream %s: %w", streamName, err)
	}
	streamInfo, err := stream.Info(ctx)
	if err != nil {
		return LegacyDrainStatus{}, fmt.Errorf("inspect legacy documents stream %s: %w", streamName, err)
	}
	status := LegacyDrainStatus{StreamMessages: streamInfo.State.Msgs}
	consumer, err := c.JS.Consumer(ctx, streamName, consumerName)
	if err != nil {
		if errors.Is(err, jetstream.ErrConsumerNotFound) {
			status.Ready = status.StreamMessages == 0
			return status, nil
		}
		return LegacyDrainStatus{}, fmt.Errorf("open legacy consumer %s: %w", consumerName, err)
	}
	info, err := consumer.Info(ctx)
	if err != nil {
		return LegacyDrainStatus{}, fmt.Errorf("inspect legacy consumer %s: %w", consumerName, err)
	}
	status.ConsumerExists = true
	status.ConsumerPending = info.NumPending
	status.ConsumerAckPending = info.NumAckPending
	status.ConsumerWaiting = info.NumWaiting
	status.Ready = status.StreamMessages == 0 && status.ConsumerPending == 0 && status.ConsumerAckPending == 0 && status.ConsumerWaiting == 0
	return status, nil
}
