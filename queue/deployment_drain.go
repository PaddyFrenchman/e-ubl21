package queue

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

var ErrLegacyConsumerNotFound = errors.New("expected legacy consumer not found")

// LegacyConsumerDrainStatus records the drain counters for one consumer on
// the legacy documents stream. Activation evaluates every consumer, not only
// the configured worker durable.
type LegacyConsumerDrainStatus struct {
	Name          string `json:"name"`
	FilterSubject string `json:"filterSubject,omitempty"`
	Pending       uint64 `json:"pending"`
	AckPending    int    `json:"ackPending"`
	Waiting       int    `json:"waiting"`
}

// LegacyDrainStatus is the activation barrier for the combined worker queue.
// Ready is true only when no retained, delivered, or waiting pull work exists.
type LegacyDrainStatus struct {
	StreamMessages     uint64                      `json:"streamMessages"`
	ConsumerExists     bool                        `json:"consumerExists"`
	ConsumerPending    uint64                      `json:"consumerPending"`
	ConsumerAckPending int                         `json:"consumerAckPending"`
	ConsumerWaiting    int                         `json:"consumerWaiting"`
	ConsumerCount      int                         `json:"consumerCount"`
	Consumers          []LegacyConsumerDrainStatus `json:"consumers,omitempty"`
	Ready              bool                        `json:"ready"`
}

// InspectLegacyDrain reads the legacy stream and shared durable consumer
// without creating or mutating either resource.
func (c *Client) InspectLegacyDrain(ctx context.Context, consumerName string) (LegacyDrainStatus, error) {
	return c.InspectLegacyDrainWithPolicy(ctx, consumerName, false)
}

// InspectLegacyDrainWithPolicy can explicitly allow an absent expected
// consumer for installations where the combined worker was never started.
// Other consumers are always enumerated and included in the barrier.
func (c *Client) InspectLegacyDrainWithPolicy(ctx context.Context, consumerName string, allowMissingExpected bool) (LegacyDrainStatus, error) {
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
	lister := stream.ListConsumers(ctx)
	for info := range lister.Info() {
		consumer := LegacyConsumerDrainStatus{
			Name: info.Name, FilterSubject: info.Config.FilterSubject,
			Pending: info.NumPending, AckPending: info.NumAckPending, Waiting: info.NumWaiting,
		}
		status.Consumers = append(status.Consumers, consumer)
		status.ConsumerPending += consumer.Pending
		status.ConsumerAckPending += consumer.AckPending
		status.ConsumerWaiting += consumer.Waiting
		if info.Name == consumerName {
			status.ConsumerExists = true
		}
	}
	if err := lister.Err(); err != nil {
		return LegacyDrainStatus{}, fmt.Errorf("list legacy consumers on %s: %w", streamName, err)
	}
	sort.Slice(status.Consumers, func(i, j int) bool { return status.Consumers[i].Name < status.Consumers[j].Name })
	status.ConsumerCount = len(status.Consumers)
	status.Ready = status.StreamMessages == 0 && status.ConsumerPending == 0 && status.ConsumerAckPending == 0 && status.ConsumerWaiting == 0
	if !status.ConsumerExists && !allowMissingExpected {
		status.Ready = false
		return status, fmt.Errorf("%w: %s", ErrLegacyConsumerNotFound, consumerName)
	}
	return status, nil
}
