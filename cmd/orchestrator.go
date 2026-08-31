package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runOrchestrator = orchestrator.RunService

func newOrchestratorCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	command := &cobra.Command{
		Use:   "orchestrator",
		Short: "Advance durable validation plans from stage results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return orchestratorService(cmd.Context(), cfg)
		},
	}
	command.Flags().String("orchestrator-metrics-addr", ":9094", "Address for the orchestrator's Prometheus /metrics and /healthz endpoints")
	command.Flags().String("orchestrator-role", "combined", "Orchestration duties to run: combined, consumer, or reconciler")
	command.Flags().Int("orchestrator-result-prefetch", 8, "Maximum stage results buffered locally per stage consumer")
	command.Flags().Duration("orchestrator-reconcile-interval", 10*time.Second, "Interval for repairing pending stage dispatches and final results")
	for _, name := range []string{"orchestrator-metrics-addr", "orchestrator-role", "orchestrator-result-prefetch", "orchestrator-reconcile-interval"} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func orchestratorService(ctx context.Context, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runOrchestrator(ctx, orchestrator.ServiceConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
		MetricsAddr: cfg.OrchestratorMetricsAddr, ResultPrefetch: cfg.OrchestratorResultPrefetch,
		ReconciliationInterval: cfg.OrchestratorReconcileInterval, Role: orchestrator.ServiceRole(cfg.OrchestratorRole), Logger: slog.Default(),
	})
}
