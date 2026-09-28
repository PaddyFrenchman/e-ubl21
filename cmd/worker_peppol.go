package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/server"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var runPeppolWorker = server.RunPeppolWorker

func newPeppolWorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	saxon := validationassets.Saxon()
	profile := validationassets.Peppol()
	command := &cobra.Command{
		Use: "worker-peppol", Short: "Consume only Peppol BIS Billing stage requests", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return peppolWorker(cmd.Context(), cfg) },
	}
	command.Flags().String("worker-peppol-metrics-addr", ":9095", "Address the Peppol worker's Prometheus /metrics and /healthz are served on")
	command.Flags().Duration("worker-peppol-timeout", 30*time.Second, "Maximum time for one Peppol stage attempt, including Object Store streaming")
	command.Flags().Int("worker-peppol-concurrency", 1, "Maximum Peppol stage requests validated concurrently per process")
	command.Flags().Int("worker-peppol-prefetch", 1, "Maximum Peppol stage requests buffered locally while validators are busy")
	command.Flags().Duration("worker-peppol-progress-interval", 20*time.Second, "Interval for extending the acknowledgement deadline of active and prefetched requests")
	command.Flags().String("worker-peppol-java-executable", "java", "Java executable used for the pinned Peppol Saxon runtime")
	command.Flags().String("worker-peppol-saxon-jar-path", saxon.SaxonJARPath, "Path to the pinned Saxon-HE JAR")
	command.Flags().String("worker-peppol-xmlresolver-jar-path", saxon.XMLResolverJARPath, "Path to the pinned xmlresolver JAR")
	command.Flags().String("worker-peppol-saxon-configuration-path", saxon.ConfigurationPath, "Path to the no-network Saxon configuration")
	command.Flags().String("worker-peppol-runtime-xslt-path", profile.RuntimeXSLTPath, "Path to the pinned Peppol Billing UBL XSLT")
	command.Flags().Duration("worker-peppol-saxon-startup-timeout", 30*time.Second, "Maximum duration of the worker-startup Peppol Saxon smoke transform")
	for _, name := range []string{
		"worker-peppol-metrics-addr", "worker-peppol-timeout", "worker-peppol-concurrency",
		"worker-peppol-prefetch", "worker-peppol-progress-interval", "worker-peppol-java-executable",
		"worker-peppol-saxon-jar-path", "worker-peppol-xmlresolver-jar-path",
		"worker-peppol-saxon-configuration-path", "worker-peppol-runtime-xslt-path",
		"worker-peppol-saxon-startup-timeout",
	} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func peppolWorker(ctx context.Context, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runPeppolWorker(ctx, server.PeppolWorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace, ObjectStore: cfg.ObjectStore,
		MetricsAddr: cfg.PeppolWorkerMetricsAddr, ValidationTimeout: cfg.PeppolWorkerValidationTimeout,
		Concurrency: cfg.PeppolWorkerConcurrency, Prefetch: cfg.PeppolWorkerPrefetch,
		ProgressInterval: cfg.PeppolWorkerProgressInterval, JavaExecutable: cfg.PeppolWorkerJavaExecutable,
		SaxonJARPath: cfg.PeppolWorkerSaxonJARPath, XMLResolverJARPath: cfg.PeppolWorkerXMLResolverJARPath,
		SaxonConfigurationPath: cfg.PeppolWorkerSaxonConfigurationPath, PeppolXSLTPath: cfg.PeppolWorkerXSLTPath,
		SaxonStartupTimeout: cfg.PeppolWorkerSaxonStartupTimeout, Logger: slog.Default(),
	})
}
