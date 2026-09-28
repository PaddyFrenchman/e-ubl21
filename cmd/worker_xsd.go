package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runXSDWorker = server.RunXSDWorker

func newXSDWorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	command := &cobra.Command{
		Use:   "worker-xsd",
		Short: "Consume only UBL 2.1 XSD stage requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return xsdWorker(cmd.Context(), cfg)
		},
	}
	command.Flags().String("worker-xsd-metrics-addr", ":9092", "Address the XSD worker's Prometheus /metrics and /healthz are served on")
	command.Flags().Duration("worker-xsd-timeout", 30*time.Second, "Maximum time for one XSD stage attempt, including Object Store streaming")
	command.Flags().Int("worker-xsd-concurrency", 1, "Maximum XSD stage requests validated concurrently per process")
	command.Flags().Int("worker-xsd-prefetch", 1, "Maximum XSD stage requests buffered locally while validators are busy")
	command.Flags().Duration("worker-xsd-progress-interval", 20*time.Second, "Interval for extending the acknowledgement deadline of active and prefetched requests")

	for _, name := range []string{
		"worker-xsd-metrics-addr", "worker-xsd-timeout", "worker-xsd-concurrency",
		"worker-xsd-prefetch", "worker-xsd-progress-interval",
	} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func xsdWorker(ctx context.Context, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runXSDWorker(ctx, server.XSDWorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace, ObjectStore: cfg.ObjectStore,
		MetricsAddr: cfg.XSDWorkerMetricsAddr, ValidationTimeout: cfg.XSDWorkerValidationTimeout,
		Concurrency: cfg.XSDWorkerConcurrency, Prefetch: cfg.XSDWorkerPrefetch,
		ProgressInterval: cfg.XSDWorkerProgressInterval, Logger: slog.Default(),
	})
}
