package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestProvisionInfrastructureAppliesIndependentStageDeliveryLimits(t *testing.T) {
	server := queuetest.StartServer(t)
	namespace := queue.MustNamespace("eubl21.autoscaling")
	cfg := &Config{
		NATSURL: server.URL, NATSOptions: server.Options, QueueNamespace: namespace,
		UploadContentMaxBytes: 64 << 20,
		StageXSDAckWait:       41 * time.Second, StageXSDMaxAckPending: 19,
		StageENAckWait: 73 * time.Second, StageENMaxAckPending: 11,
		StagePeppolAckWait: 89 * time.Second, StagePeppolMaxAckPending: 7,
	}
	if err := provisionInfrastructure(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}

	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.VerifyUploadContentInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.VerifyValidationExecutionInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	generation, err := client.DeploymentGeneration(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if generation.State.Mode != queue.DeploymentModeLegacy || generation.State.Generation != 1 {
		t.Fatalf("provisioned deployment generation = %+v", generation.State)
	}
	want := queue.StageQueueInfraConfig{Stages: []queue.StageQueueConfig{
		provisionStage(queue.ValidationLayerUBL21XSD, 41*time.Second, 19),
		provisionStage(queue.ValidationLayerEN16931, 73*time.Second, 11),
		provisionStage(queue.ValidationLayerPeppolBISBilling, 89*time.Second, 7),
	}}
	if err := client.VerifyStageQueueInfra(context.Background(), want); err != nil {
		t.Fatal(err)
	}
}
