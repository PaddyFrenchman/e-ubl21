package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestMaintenancePurgeContentTombstonesDryRunAndApply(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("eubl21.maintenance")
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
	now := time.Now().UTC()
	if err := q.PutUploadRecord(ctx, queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusValid,
		ReceivedAt:       now.Add(-time.Hour).Format(time.RFC3339),
		ValidatedAt:      now.Add(-time.Hour).Format(time.RFC3339),
		ContentExpiredAt: now.Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.PutUploadContent(ctx, id, []byte("expired")); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteUploadContent(ctx, id); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace}
	run := func(dryRun bool) map[string]any {
		t.Helper()
		var stdout bytes.Buffer
		command := newMaintenanceCmd(cfg)
		command.SetOut(&stdout)
		args := []string{"purge-content-tombstones", "--grace-period=0", "--max-items=1"}
		if dryRun {
			args = append(args, "--dry-run")
		}
		command.SetArgs(args)
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		var output map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
			t.Fatalf("decode structured output %q: %v", stdout.String(), err)
		}
		return output
	}

	preview := run(true)
	if preview["dryRun"] != true || preview["eligible"] != float64(1) || preview["purged"] != float64(0) || preview["namespace"] != namespace.String() {
		t.Fatalf("dry-run output = %v", preview)
	}
	store, err := q.JS.ObjectStore(ctx, namespace.UploadContentBucketName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetInfo(ctx, id, jetstream.GetObjectInfoShowDeleted()); err != nil {
		t.Fatalf("dry-run removed tombstone: %v", err)
	}

	applied := run(false)
	if applied["eligible"] != float64(1) || applied["purged"] != float64(1) || applied["failed"] != float64(0) {
		t.Fatalf("apply output = %v", applied)
	}
	if _, err := store.GetInfo(ctx, id, jetstream.GetObjectInfoShowDeleted()); !errors.Is(err, jetstream.ErrObjectNotFound) {
		t.Fatalf("applied command left tombstone: %v", err)
	}
}

func TestMaintenancePurgeContentTombstonesFailsWhenCleanupIsIncomplete(t *testing.T) {
	server := queuetest.StartServer(t)
	q, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	ctx := context.Background()
	if err := q.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	if err := q.PutUploadContent(ctx, id, []byte("orphaned")); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteUploadContent(ctx, id); err != nil {
		t.Fatal(err)
	}
	kv, err := q.JS.KeyValue(ctx, q.Namespace.UploadsBucketName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kv.Put(ctx, id, []byte("{")); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	command := newMaintenanceCmd(&Config{NATSURL: server.URL, NATSOptions: server.Options})
	command.SetOut(&stdout)
	command.SilenceUsage = true
	command.SilenceErrors = true
	command.SetArgs([]string{"purge-content-tombstones", "--grace-period=0", "--max-items=10"})
	if err := command.Execute(); err == nil {
		t.Fatal("incomplete tombstone cleanup returned success")
	}
	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode structured failure output %q: %v", stdout.String(), err)
	}
	if output["failed"] != float64(1) || output["purged"] != float64(0) {
		t.Fatalf("failure output = %v", output)
	}
}
