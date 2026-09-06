package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultResultPrefetch         = 8
	defaultReconciliationInterval = 10 * time.Second
)

// ServiceRole controls which independently scalable orchestration duties run
// in one process. Combined preserves the non-Kubernetes/default behavior;
// Kubernetes separates consumers from the singleton history-scanning repair
// role so scaling result throughput cannot multiply reconciliation scans.
type ServiceRole string

const (
	ServiceRoleCombined   ServiceRole = "combined"
	ServiceRoleConsumer   ServiceRole = "consumer"
	ServiceRoleReconciler ServiceRole = "reconciler"
)

func (role ServiceRole) consumesResults() bool {
	return role == ServiceRoleCombined || role == ServiceRoleConsumer
}

func (role ServiceRole) runsReconciliation() bool {
	return role == ServiceRoleCombined || role == ServiceRoleReconciler
}

// ServiceConfig configures the independently deployable orchestration role.
// Broker topology remains deployer-owned; the service verifies and consumes
// existing resources without creating or changing them.
type ServiceConfig struct {
	NATSURL                string
	NATSOptions            []nats.Option
	QueueNamespace         queue.Namespace
	MetricsAddr            string
	ResultPrefetch         int
	ReconciliationInterval time.Duration
	Role                   ServiceRole
	Logger                 *slog.Logger

	queueClient *queue.Client
	onStarted   func()
}

func resolveServiceConfig(cfg ServiceConfig) (ServiceConfig, error) {
	if cfg.Role == "" {
		cfg.Role = ServiceRoleCombined
	}
	switch cfg.Role {
	case ServiceRoleCombined, ServiceRoleConsumer, ServiceRoleReconciler:
	default:
		return ServiceConfig{}, fmt.Errorf("invalid orchestrator role %q: want combined, consumer, or reconciler", cfg.Role)
	}
	if cfg.ResultPrefetch == 0 {
		cfg.ResultPrefetch = defaultResultPrefetch
	}
	if cfg.ResultPrefetch < 0 {
		return ServiceConfig{}, errors.New("orchestrator result prefetch must be positive")
	}
	if cfg.ReconciliationInterval == 0 {
		cfg.ReconciliationInterval = defaultReconciliationInterval
	}
	if cfg.ReconciliationInterval < 0 {
		return ServiceConfig{}, errors.New("orchestrator reconciliation interval must be positive")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return cfg, nil
}

// RunService performs the result-consumer and/or reconciliation duties selected
// by ServiceConfig.Role until ctx is canceled.
func RunService(ctx context.Context, raw ServiceConfig) error {
	cfg, err := resolveServiceConfig(raw)
	if err != nil {
		return err
	}
	client := cfg.queueClient
	ownedClient := false
	if client == nil {
		client, err = queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
		if err != nil {
			return fmt.Errorf("orchestrator connect to NATS: %w", err)
		}
		ownedClient = true
	}
	if ownedClient {
		defer client.Close()
		if err := client.RequireSplitDeployment(ctx); err != nil {
			return fmt.Errorf("orchestrator deployment barrier: %w", err)
		}
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err != nil {
		return fmt.Errorf("orchestrator verify execution infrastructure: %w", err)
	}

	stages := make([]queue.ValidationLayer, 0, 3)
	if cfg.Role.consumesResults() {
		stages = append(stages,
			queue.ValidationLayerUBL21XSD,
			queue.ValidationLayerEN16931,
			queue.ValidationLayerPeppolBISBilling,
		)
	}
	consumers := make([]jetstream.Consumer, 0, len(stages))
	for _, stage := range stages {
		consumer, consumerErr := client.StageResultConsumer(ctx, stage)
		if consumerErr != nil {
			return fmt.Errorf("orchestrator open %s result consumer: %w", stage, consumerErr)
		}
		consumers = append(consumers, consumer)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	machine := New(client)
	errCh := make(chan error, len(consumers)+3)
	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{Addr: cfg.MetricsAddr, Handler: serviceMetricsMux(client)}
		go func() {
			if serveErr := metricsServer.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				errCh <- fmt.Errorf("orchestrator metrics server: %w", serveErr)
			}
		}()
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			_ = metricsServer.Shutdown(shutdownCtx)
		}()
	}

	var workers sync.WaitGroup
	for index, consumer := range consumers {
		stage := stages[index]
		workers.Add(1)
		go func() {
			defer workers.Done()
			if consumeErr := consumeStageResults(runCtx, machine, consumer, stage, cfg.ResultPrefetch, cfg.Logger); consumeErr != nil {
				select {
				case errCh <- consumeErr:
				case <-runCtx.Done():
				}
			}
		}()
	}
	if cfg.Role.runsReconciliation() {
		workers.Add(2)
		go func() {
			defer workers.Done()
			reconcileErr := machine.RunDispatchReconciler(runCtx, cfg.ReconciliationInterval, func(report DispatchReconcileReport, reconcileErr error) {
				if reconcileErr != nil {
					cfg.Logger.Error("orchestrator dispatch reconciliation failed", "error", reconcileErr, "scanned", report.Scanned, "reconciled", report.Reconciled)
				}
			})
			if reconcileErr != nil {
				select {
				case errCh <- fmt.Errorf("orchestrator dispatch reconciler: %w", reconcileErr):
				case <-runCtx.Done():
				}
			}
		}()
		go func() {
			defer workers.Done()
			reconcileErr := machine.RunFinalResultReconciler(runCtx, cfg.ReconciliationInterval, func(report FinalResultReconcileReport, reconcileErr error) {
				if reconcileErr != nil {
					cfg.Logger.Error("orchestrator final-result reconciliation failed", "error", reconcileErr, "scanned", report.Scanned, "reconciled", report.Reconciled)
				}
			})
			if reconcileErr != nil {
				select {
				case errCh <- fmt.Errorf("orchestrator final-result reconciler: %w", reconcileErr):
				case <-runCtx.Done():
				}
			}
		}()
	}

	if cfg.onStarted != nil {
		cfg.onStarted()
	}
	cfg.Logger.Info("orchestrator started", "role", cfg.Role, "queueNamespace", cfg.QueueNamespace.String(), "resultPrefetch", cfg.ResultPrefetch, "reconciliationInterval", cfg.ReconciliationInterval)
	select {
	case <-ctx.Done():
		cancel()
		workers.Wait()
		cfg.Logger.Info("orchestrator shutting down")
		return nil
	case runErr := <-errCh:
		cancel()
		workers.Wait()
		return runErr
	}
}

func consumeStageResults(ctx context.Context, machine *Machine, consumer jetstream.Consumer, stage queue.ValidationLayer, prefetch int, logger *slog.Logger) error {
	messages, err := consumer.Messages(jetstream.PullMaxMessages(prefetch))
	if err != nil {
		return fmt.Errorf("orchestrator start %s result consumer: %w", stage, err)
	}
	defer messages.Stop()
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			messages.Stop()
		case <-stop:
		}
	}()
	defer close(stop)
	for {
		message, nextErr := messages.Next()
		if nextErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("orchestrator pull %s result: %w", stage, nextErr)
		}
		if handleErr := machine.HandleStageResultMessage(ctx, message); handleErr != nil {
			logger.Error("orchestrator stage result failed", "stage", stage, "error", handleErr)
		}
	}
}

func serviceMetricsMux(client *queue.Client) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if client.NC.Status() != nats.CONNECTED {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unavailable\n"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}
