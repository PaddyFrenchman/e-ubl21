package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestMigrationCommandsActivateOnlyAfterDrainAndCannotRollBackSplit(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("migration.command")
	cfg := &Config{NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace}
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DocumentsConsumer(ctx, defaultLegacyConsumerName, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := migrationTransition(ctx, &output, cfg, queue.DeploymentModeDraining); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	if err := migrationActivate(ctx, &output, cfg, defaultLegacyConsumerName, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"mode": "split"`) {
		t.Fatalf("activation output = %s", output.String())
	}
	if err := migrationTransition(ctx, &output, cfg, queue.DeploymentModeLegacy); err == nil {
		t.Fatal("split deployment rolled back to legacy")
	}
}

func TestMigrationActivationClaimPreventsUnsafeRollback(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("migration.rollback.barrier")
	cfg := &Config{NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace}
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.DocumentsConsumer(ctx, defaultLegacyConsumerName, ""); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	id, _ := queue.NewID()
	if err := client.PutUploadRecord(ctx, queue.UploadRecord{
		ID: id, DocumentType: "Invoice", Status: queue.StatusPending,
		ReceivedAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := migrationTransition(ctx, &output, cfg, queue.DeploymentModeDraining); err != nil {
		t.Fatal(err)
	}
	if err := migrationActivate(ctx, &output, cfg, defaultLegacyConsumerName, false); err == nil {
		t.Fatal("activation with missing pending content succeeded")
	}
	current, err := client.DeploymentGeneration(ctx)
	if err != nil || current.State.Mode != queue.DeploymentModeActivating {
		t.Fatalf("activation claim = %+v, err=%v", current.State, err)
	}
	if err := migrationTransition(ctx, &output, cfg, queue.DeploymentModeLegacy); err == nil || !strings.Contains(err.Error(), "forward-only") {
		t.Fatalf("unsafe rollback error = %v", err)
	}
}

func TestMigrationStatusRequiresExpectedLegacyConsumerUnlessExplicitlyAllowed(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("migration.consumer.policy")
	cfg := &Config{NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace}
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := migrationStatus(ctx, &output, cfg, defaultLegacyConsumerName, false); !errors.Is(err, queue.ErrLegacyConsumerNotFound) {
		t.Fatalf("missing expected consumer status error = %v", err)
	}
	output.Reset()
	if err := migrationStatus(ctx, &output, cfg, defaultLegacyConsumerName, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"ready": true`) || !strings.Contains(output.String(), `"consumerCount": 0`) {
		t.Fatalf("explicit no-consumer status output = %s", output.String())
	}
}
