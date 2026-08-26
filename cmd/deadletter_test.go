package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestDeadLetterRetryCommandRequeuesFailure(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("eubl21.deadletter.command")
	q, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	now := time.Now().UTC().Format(time.RFC3339)
	record := queue.UploadRecord{ID: id, DocumentType: "Invoice", Status: queue.StatusProcessingFailed, ReceivedAt: now, Error: "failed", LastError: "failed", ProcessingAttempts: 5, ProcessingFailedAt: now}
	if err := q.PutUploadRecord(ctx, record); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadContent(ctx, id, []byte(`<Invoice/>`)); err != nil {
		t.Fatal(err)
	}
	if err := q.PublishDeadLetter(ctx, queue.DeadLetter{ID: id, DocumentType: "Invoice", ReceivedAt: now, LastError: "failed", Attempts: 5, FailedAt: now}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	command := newDeadLetterCmd(&Config{NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace})
	command.SetOut(&output)
	command.SetArgs([]string{"retry", id})
	if err := command.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "requeued "+id) {
		t.Fatalf("output = %q", output.String())
	}
	stored, err := q.GetUploadRecord(ctx, id)
	if err != nil || stored.Status != queue.StatusPending || !stored.ReplayReady {
		t.Fatalf("record=%+v err=%v", stored, err)
	}
}

func TestDeadLetterRetryCommandReportsConnectionFailure(t *testing.T) {
	command := newDeadLetterCmd(&Config{NATSURL: "://bad"})
	command.SilenceErrors = true
	command.SilenceUsage = true
	command.SetArgs([]string{"retry", "00000000000000000000000000000000"})
	if err := command.Execute(); err == nil {
		t.Fatal("malformed NATS URL returned success")
	}
}
