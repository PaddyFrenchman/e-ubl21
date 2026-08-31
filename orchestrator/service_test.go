package orchestrator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestResolveServiceConfigDefaultsAndRejectsInvalidLimits(t *testing.T) {
	cfg, err := resolveServiceConfig(ServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResultPrefetch != defaultResultPrefetch || cfg.ReconciliationInterval != defaultReconciliationInterval || cfg.Logger == nil {
		t.Fatalf("resolved config = %+v", cfg)
	}
	if cfg.Role != ServiceRoleCombined {
		t.Fatalf("default role = %q, want %q", cfg.Role, ServiceRoleCombined)
	}
	for _, input := range []ServiceConfig{{ResultPrefetch: -1}, {ReconciliationInterval: -time.Second}} {
		if _, err := resolveServiceConfig(input); err == nil {
			t.Fatalf("resolveServiceConfig(%+v) unexpectedly succeeded", input)
		}
	}
	if _, err := resolveServiceConfig(ServiceConfig{Role: "leaderish"}); err == nil {
		t.Fatal("invalid orchestrator role unexpectedly succeeded")
	}
}

func TestServiceRolesKeepScaledConsumersSeparateFromHistoryScans(t *testing.T) {
	tests := []struct {
		role       ServiceRole
		consumes   bool
		reconciles bool
	}{
		{role: ServiceRoleCombined, consumes: true, reconciles: true},
		{role: ServiceRoleConsumer, consumes: true, reconciles: false},
		{role: ServiceRoleReconciler, consumes: false, reconciles: true},
	}
	for _, test := range tests {
		if got := test.role.consumesResults(); got != test.consumes {
			t.Fatalf("role %q consumesResults = %t, want %t", test.role, got, test.consumes)
		}
		if got := test.role.runsReconciliation(); got != test.reconciles {
			t.Fatalf("role %q runsReconciliation = %t, want %t", test.role, got, test.reconciles)
		}
	}
}

func TestServiceHealthReflectsNATSConnection(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	handler := serviceMetricsMux(client)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("connected health status = %d, body %q", recorder.Code, recorder.Body.String())
	}
	client.Close()
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed health status = %d, body %q", recorder.Code, recorder.Body.String())
	}
}

func TestRunServiceOpensProvisionedConsumersAndStops(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
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
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- RunService(runCtx, ServiceConfig{queueClient: client, MetricsAddr: "", Role: ServiceRoleConsumer})
	}()
	consumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		info, infoErr := consumer.Info(ctx)
		if infoErr != nil {
			t.Fatal(infoErr)
		}
		if info.NumWaiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("orchestrator did not begin pulling stage results")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("orchestrator did not stop after cancellation")
	}
}

func TestReconcilerRoleDoesNotOpenStageConsumers(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.ProvisionValidationExecutionInfra(context.Background()); err != nil {
		t.Fatal(err)
	}

	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunService(runCtx, ServiceConfig{queueClient: client, MetricsAddr: "", Role: ServiceRoleReconciler, ReconciliationInterval: 10 * time.Millisecond})
	}()
	select {
	case err := <-done:
		t.Fatalf("reconciler stopped before cancellation: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reconciler did not stop after cancellation")
	}
}
