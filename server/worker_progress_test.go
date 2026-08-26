package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type progressTestMsg struct{ calls atomic.Int32 }

func (*progressTestMsg) Metadata() (*jetstream.MsgMetadata, error) {
	return &jetstream.MsgMetadata{}, nil
}
func (*progressTestMsg) Data() []byte                     { return nil }
func (*progressTestMsg) Headers() nats.Header             { return nil }
func (*progressTestMsg) Subject() string                  { return "" }
func (*progressTestMsg) Reply() string                    { return "" }
func (*progressTestMsg) Ack() error                       { return nil }
func (*progressTestMsg) DoubleAck(context.Context) error  { return nil }
func (*progressTestMsg) Nak() error                       { return nil }
func (*progressTestMsg) NakWithDelay(time.Duration) error { return nil }
func (m *progressTestMsg) InProgress() error              { m.calls.Add(1); return nil }
func (*progressTestMsg) Term() error                      { return nil }
func (*progressTestMsg) TermWithReason(string) error      { return nil }

func TestProgressHeartbeatsResetAckDeadline(t *testing.T) {
	msg := &progressTestMsg{}
	stop := startProgressHeartbeats(msg, time.Millisecond)
	defer stop()
	deadline := time.Now().Add(100 * time.Millisecond)
	for msg.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if msg.calls.Load() == 0 {
		t.Fatal("no InProgress heartbeat was sent")
	}
	stop() // stopping is deliberately idempotent
	if stop := startProgressHeartbeats(msg, 0); stop == nil {
		t.Fatal("disabled heartbeat returned nil stop function")
	}
}

func TestPrefetchedJobStartsHeartbeatsBeforeProcessing(t *testing.T) {
	msg := &progressTestMsg{}
	job := newQueuedDocumentJob(msg, time.Millisecond)
	defer job.stopProgress()
	deadline := time.Now().Add(100 * time.Millisecond)
	for msg.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if msg.calls.Load() == 0 {
		t.Fatal("prefetched job sent no heartbeat while waiting for a worker")
	}
}
