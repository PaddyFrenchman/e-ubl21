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
	for _, input := range []ServiceConfig{{ResultPrefetch: -1}, {ReconciliationInterval: -time.Second}} {
		if _, err := resolveServiceConfig(input); err == nil {
			t.Fatalf("resolveServiceConfig(%+v) unexpectedly succeeded", input)
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
		done <- RunService(runCtx, ServiceConfig{queueClient: client, MetricsAddr: ""})
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
