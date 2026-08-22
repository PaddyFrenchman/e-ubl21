package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/PaddyFrenchman/e-ubl21/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newWorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	workerCmd := &cobra.Command{
		Use:   "worker",
		Short: "Consume queued UBL documents from NATS JetStream and validate them",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return worker(cmd.Context(), cfg)
		},
	}

	workerCmd.Flags().String("worker-consumer-name", "ubl-validator", "Durable JetStream consumer name (run multiple replicas with the same name to load-balance)")
	workerCmd.Flags().String("worker-subject-filter", "", "Restrict this worker to a single document type's queue (e.g. ubl.documents.Invoice); empty processes every document type")
	workerCmd.Flags().String("worker-metrics-addr", ":9091", "Address the worker's Prometheus /metrics and /healthz are served on")

	mustBindLocalFlag(v, workerCmd, "worker-consumer-name")
	mustBindLocalFlag(v, workerCmd, "worker-subject-filter")
	mustBindLocalFlag(v, workerCmd, "worker-metrics-addr")

	return workerCmd
}

// worker runs the validation worker until it receives SIGINT/SIGTERM, at
// which point it stops pulling new messages and lets any in-flight message
// finish being acknowledged before returning - unlike `serve`, this
// graceful-shutdown handling is essential here so a `docker stop` (or local
// Ctrl-C) doesn't abandon a message mid-processing and force an avoidable
// redelivery.
func worker(ctx context.Context, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	return server.RunWorker(ctx, server.WorkerConfig{
		NATSURL:       cfg.NATSURL,
		ConsumerName:  cfg.WorkerConsumerName,
		SubjectFilter: cfg.WorkerSubjectFilter,
		MetricsAddr:   cfg.WorkerMetricsAddr,
		Logger:        slog.Default(),
	})
}
