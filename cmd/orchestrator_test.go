package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
)

func TestOrchestratorStartsAndStopsOnContextCancellation(t *testing.T) {
	previous := runOrchestrator
	t.Cleanup(func() { runOrchestrator = previous })
	started := make(chan orchestrator.ServiceConfig, 1)
	runOrchestrator = func(ctx context.Context, cfg orchestrator.ServiceConfig) error {
		started <- cfg
		<-ctx.Done()
		return nil
	}
	cfg := &Config{
		OrchestratorMetricsAddr:       ":9494",
		OrchestratorRole:              "consumer",
		OrchestratorResultPrefetch:    13,
		OrchestratorReconcileInterval: 4 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- orchestratorService(ctx, cfg) }()
	select {
	case received := <-started:
		if received.MetricsAddr != ":9494" || received.Role != orchestrator.ServiceRoleConsumer || received.ResultPrefetch != 13 || received.ReconciliationInterval != 4*time.Second {
			t.Fatalf("orchestrator config = %+v", received)
		}
	case <-time.After(time.Second):
		t.Fatal("orchestrator runner was not called")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("orchestrator did not stop")
	}
}
