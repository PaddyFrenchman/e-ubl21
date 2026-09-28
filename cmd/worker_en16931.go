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

var runEN16931Worker = server.RunEN16931Worker

func newEN16931WorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	saxon := validationassets.Saxon()
	profile := validationassets.EN16931()
	command := &cobra.Command{
		Use:   "worker-en16931",
		Short: "Consume only EN 16931 semantic-validation stage requests",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return en16931Worker(cmd.Context(), cfg)
		},
	}
	command.Flags().String("worker-en16931-metrics-addr", ":9093", "Address the EN 16931 worker's Prometheus /metrics and /healthz are served on")
	command.Flags().Duration("worker-en16931-timeout", 30*time.Second, "Maximum time for one EN 16931 stage attempt, including Object Store streaming")
	command.Flags().Int("worker-en16931-concurrency", 1, "Maximum EN 16931 stage requests validated concurrently per process")
	command.Flags().Int("worker-en16931-prefetch", 1, "Maximum EN 16931 stage requests buffered locally while validators are busy")
	command.Flags().Duration("worker-en16931-progress-interval", 20*time.Second, "Interval for extending the acknowledgement deadline of active and prefetched requests")
	command.Flags().String("worker-en16931-java-executable", "java", "Java executable used for the EN 16931 Saxon transform")
	command.Flags().String("worker-en16931-saxon-jar-path", saxon.SaxonJARPath, "Path to the pinned Saxon-HE JAR")
	command.Flags().String("worker-en16931-xmlresolver-jar-path", saxon.XMLResolverJARPath, "Path to the pinned xmlresolver JAR")
	command.Flags().String("worker-en16931-saxon-configuration-path", saxon.ConfigurationPath, "Path to the no-network Saxon configuration")
	command.Flags().String("worker-en16931-runtime-xslt-path", profile.RuntimeXSLTPath, "Path to the pinned EN 16931 UBL XSLT")
	command.Flags().Duration("worker-en16931-saxon-startup-timeout", 30*time.Second, "Maximum duration of the worker-startup Saxon smoke transform")

	for _, name := range []string{
		"worker-en16931-metrics-addr", "worker-en16931-timeout", "worker-en16931-concurrency",
		"worker-en16931-prefetch", "worker-en16931-progress-interval", "worker-en16931-java-executable",
		"worker-en16931-saxon-jar-path", "worker-en16931-xmlresolver-jar-path",
		"worker-en16931-saxon-configuration-path", "worker-en16931-runtime-xslt-path",
		"worker-en16931-saxon-startup-timeout",
	} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func en16931Worker(ctx context.Context, cfg *Config) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runEN16931Worker(ctx, server.EN16931WorkerConfig{
		NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace, ObjectStore: cfg.ObjectStore,
		MetricsAddr: cfg.ENWorkerMetricsAddr, ValidationTimeout: cfg.ENWorkerValidationTimeout,
		Concurrency: cfg.ENWorkerConcurrency, Prefetch: cfg.ENWorkerPrefetch,
		ProgressInterval: cfg.ENWorkerProgressInterval, JavaExecutable: cfg.ENWorkerJavaExecutable,
		SaxonJARPath: cfg.ENWorkerSaxonJARPath, XMLResolverJARPath: cfg.ENWorkerXMLResolverJARPath,
		SaxonConfigurationPath: cfg.ENWorkerSaxonConfigurationPath, EN16931XSLTPath: cfg.ENWorkerXSLTPath,
		SaxonStartupTimeout: cfg.ENWorkerSaxonStartupTimeout, Logger: slog.Default(),
	})
}
