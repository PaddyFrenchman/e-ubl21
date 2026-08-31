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
	if err := migrationActivate(ctx, &output, cfg, defaultLegacyConsumerName); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"mode": "split"`) {
		t.Fatalf("activation output = %s", output.String())
	}
	if err := migrationTransition(ctx, &output, cfg, queue.DeploymentModeLegacy); err == nil {
		t.Fatal("split deployment rolled back to legacy")
	}
}
