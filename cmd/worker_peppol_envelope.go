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

var runPeppolEnvelopeWorker = server.RunPeppolEnvelopeWorker

func newPeppolEnvelopeWorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	command := &cobra.Command{
		Use: "worker-peppol-envelope", Short: "Consume only Peppol SBDH envelope stage requests", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return peppolEnvelopeWorker(cmd.Context(), v, cfg) },
	}
	command.Flags().String("worker-peppol-envelope-metrics-addr", ":9096", "Address for Peppol envelope worker metrics and health")
	command.Flags().Duration("worker-peppol-envelope-timeout", 30*time.Second, "Maximum time for one Peppol envelope stage attempt")
	command.Flags().Int("worker-peppol-envelope-concurrency", 1, "Maximum concurrent Peppol envelope validations")
	command.Flags().Int("worker-peppol-envelope-prefetch", 1, "Maximum buffered Peppol envelope requests")
	command.Flags().Duration("worker-peppol-envelope-progress-interval", 20*time.Second, "Active request acknowledgement progress interval")
	for _, name := range []string{"worker-peppol-envelope-metrics-addr", "worker-peppol-envelope-timeout", "worker-peppol-envelope-concurrency", "worker-peppol-envelope-prefetch", "worker-peppol-envelope-progress-interval"} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func peppolEnvelopeWorker(ctx context.Context, v *viper.Viper, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runPeppolEnvelopeWorker(ctx, server.PeppolEnvelopeWorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace,
		MetricsAddr:       v.GetString("worker-peppol-envelope-metrics-addr"),
		ValidationTimeout: v.GetDuration("worker-peppol-envelope-timeout"),
		Concurrency:       v.GetInt("worker-peppol-envelope-concurrency"), Prefetch: v.GetInt("worker-peppol-envelope-prefetch"),
		ProgressInterval: v.GetDuration("worker-peppol-envelope-progress-interval"), Logger: slog.Default(),
	})
}
