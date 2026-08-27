package regression

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
)

type natsState struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Namespace     string               `json:"namespace"`
	Streams       []natsStreamSnapshot `json:"streams"`
}

type natsStreamSnapshot struct {
	Name      string                 `json:"name"`
	Subjects  []string               `json:"subjects"`
	Messages  uint64                 `json:"messages"`
	Bytes     uint64                 `json:"bytes"`
	FirstSeq  uint64                 `json:"firstSequence"`
	LastSeq   uint64                 `json:"lastSequence"`
	Consumers []natsConsumerSnapshot `json:"consumers"`
}

type natsConsumerSnapshot struct {
	Name           string   `json:"name"`
	FilterSubjects []string `json:"filterSubjects,omitempty"`
	Delivered      uint64   `json:"deliveredStreamSequence"`
	AckFloor       uint64   `json:"ackFloorStreamSequence"`
	AckPending     int      `json:"ackPending"`
	Redelivered    int      `json:"redelivered"`
	Pending        uint64   `json:"pending"`
	Waiting        int      `json:"waiting"`
	Paused         bool     `json:"paused"`
}

func collectNATSSnapshot(ctx context.Context, cfg EvidenceConfig) ([]byte, string, error) {
	if cfg.NATSURL == "" {
		return nil, "", errors.New("NATS URL is required when NATS evidence is enabled")
	}
	connectTimeout := 5 * time.Second
	if cfg.SourceTimeout > 0 && cfg.SourceTimeout < connectTimeout {
		connectTimeout = cfg.SourceTimeout
	}
	options := append([]nats.Option(nil), cfg.NATSOptions...)
	options = append(options, nats.Timeout(connectTimeout), nats.RetryOnFailedConnect(false), nats.MaxReconnects(0))
	client, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, options...)
	if err != nil {
		return nil, "", err
	}
	defer client.Close()
	state := natsState{SchemaVersion: 1, Namespace: cfg.QueueNamespace.String()}
	streamNames := []string{
		cfg.QueueNamespace.DocumentsStreamName(),
		cfg.QueueNamespace.ResultsStreamName(),
		cfg.QueueNamespace.DeadLettersStreamName(),
	}
	sort.Strings(streamNames)
	for _, name := range streamNames {
		stream, err := client.JS.Stream(ctx, name)
		if err != nil {
			return nil, client.NC.ConnectedServerVersion(), fmt.Errorf("read NATS stream %s: %w", name, err)
		}
		info, err := stream.Info(ctx)
		if err != nil {
			return nil, client.NC.ConnectedServerVersion(), fmt.Errorf("read NATS stream info %s: %w", name, err)
		}
		snapshot := natsStreamSnapshot{
			Name: name, Subjects: append([]string(nil), info.Config.Subjects...),
			Messages: info.State.Msgs, Bytes: info.State.Bytes,
			FirstSeq: info.State.FirstSeq, LastSeq: info.State.LastSeq,
		}
		sort.Strings(snapshot.Subjects)
		lister := stream.ListConsumers(ctx)
		for consumer := range lister.Info() {
			filters := append([]string(nil), consumer.Config.FilterSubjects...)
			if consumer.Config.FilterSubject != "" {
				filters = append(filters, consumer.Config.FilterSubject)
			}
			sort.Strings(filters)
			snapshot.Consumers = append(snapshot.Consumers, natsConsumerSnapshot{
				Name: consumer.Name, FilterSubjects: filters,
				Delivered: consumer.Delivered.Stream, AckFloor: consumer.AckFloor.Stream,
				AckPending: consumer.NumAckPending, Redelivered: consumer.NumRedelivered,
				Pending: consumer.NumPending, Waiting: consumer.NumWaiting, Paused: consumer.Paused,
			})
		}
		if err := lister.Err(); err != nil {
			return nil, client.NC.ConnectedServerVersion(), fmt.Errorf("list NATS consumers for %s: %w", name, err)
		}
		sort.Slice(snapshot.Consumers, func(i, j int) bool { return snapshot.Consumers[i].Name < snapshot.Consumers[j].Name })
		state.Streams = append(state.Streams, snapshot)
	}
	content, err := marshalEvidenceJSON(state)
	return content, client.NC.ConnectedServerVersion(), err
}
