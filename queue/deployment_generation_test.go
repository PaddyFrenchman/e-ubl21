package queue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestDeploymentGenerationTransitionsAreDurableAndForwardOnly(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("migration.state"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if _, err := client.DeploymentGeneration(ctx); !errors.Is(err, queue.ErrDeploymentGenerationNotProvisioned) {
		t.Fatalf("unprovisioned generation error = %v", err)
	}
	legacy, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if legacy.State.Mode != queue.DeploymentModeLegacy || legacy.State.Generation != 1 {
		t.Fatalf("initial generation = %+v", legacy.State)
	}
	if _, err := client.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeSplit, time.Now()); err == nil {
		t.Fatal("legacy mode transitioned directly to split")
	}
	draining, err := client.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeDraining, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.TransitionDeploymentMode(ctx, legacy, queue.DeploymentModeDraining, time.Now()); !errors.Is(err, queue.ErrDeploymentModeConflict) {
		t.Fatalf("stale transition error = %v", err)
	}
	rolledBack, err := client.TransitionDeploymentMode(ctx, draining, queue.DeploymentModeLegacy, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	draining, err = client.TransitionDeploymentMode(ctx, rolledBack, queue.DeploymentModeDraining, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	activating, err := client.TransitionDeploymentMode(ctx, draining, queue.DeploymentModeActivating, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := client.RequireLegacyDeployment(ctx); err == nil {
		t.Fatal("legacy runtime accepted activating deployment")
	}
	if _, err := client.TransitionDeploymentMode(ctx, activating, queue.DeploymentModeLegacy, time.Now()); err == nil {
		t.Fatal("activating deployment rolled back to legacy")
	}
	split, err := client.TransitionDeploymentMode(ctx, activating, queue.DeploymentModeSplit, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if split.State.Generation != 6 || split.State.Mode != queue.DeploymentModeSplit {
		t.Fatalf("split generation = %+v", split.State)
	}
	if err := client.RequireSplitDeployment(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.RequireLegacyDeployment(ctx); err == nil {
		t.Fatal("legacy runtime accepted split deployment")
	}
	if _, err := client.TransitionDeploymentMode(ctx, split, queue.DeploymentModeLegacy, time.Now()); err == nil {
		t.Fatal("split deployment rolled back to legacy")
	}
}

func TestLegacyDrainRequiresAnEmptyStream(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("migration.drain"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx := context.Background()
	if err := client.EnsureInfra(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := client.InspectLegacyDrain(ctx, "ubl-validator")
	if err != nil || !status.Ready {
		t.Fatalf("empty drain status = %+v, err=%v", status, err)
	}
	id, _ := queue.NewID()
	if err := client.PublishDocument(ctx, id, "Invoice", time.Now()); err != nil {
		t.Fatal(err)
	}
	status, err = client.InspectLegacyDrain(ctx, "ubl-validator")
	if err != nil {
		t.Fatal(err)
	}
	if status.Ready || status.StreamMessages != 1 {
		t.Fatalf("queued drain status = %+v", status)
	}
}
