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

var runWorker = server.RunWorker

func newWorkerCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	saxon := validationassets.Saxon()
	en16931 := validationassets.EN16931()
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
	workerCmd.Flags().Duration("worker-validation-timeout", 30*time.Second, "Maximum total validation time per document")
	workerCmd.Flags().Duration("worker-xsd-validation-timeout", 0, "Maximum UBL 2.1 XSD stage time per document (0 inherits worker-validation-timeout)")
	workerCmd.Flags().Duration("worker-en16931-validation-timeout", 0, "Maximum EN 16931 semantic stage time per Invoice or CreditNote (0 inherits worker-validation-timeout)")
	workerCmd.Flags().Int("worker-concurrency", 1, "Maximum documents validated concurrently per worker process")
	workerCmd.Flags().Int("worker-prefetch", 1, "Maximum pulled documents buffered locally while validations are busy")
	workerCmd.Flags().Duration("worker-ack-wait", 60*time.Second, "JetStream acknowledgement deadline; must exceed validation timeout")
	workerCmd.Flags().Int("worker-max-deliver", 5, "Maximum application processing attempts before a durable dead-letter transition")
	workerCmd.Flags().Int("worker-max-ack-pending", 0, "Fleet-wide outstanding limit on the shared durable consumer (0 uses this replica's concurrency + prefetch)")
	workerCmd.Flags().String("worker-java-executable", "java", "Java executable used for Saxon")
	workerCmd.Flags().String("worker-saxon-jar-path", saxon.SaxonJARPath, "Path to the pinned Saxon-HE JAR")
	workerCmd.Flags().String("worker-xmlresolver-jar-path", saxon.XMLResolverJARPath, "Path to the pinned xmlresolver JAR")
	workerCmd.Flags().String("worker-saxon-configuration-path", saxon.ConfigurationPath, "Path to the no-network Saxon configuration")
	workerCmd.Flags().String("worker-en16931-xslt-path", en16931.RuntimeXSLTPath, "Path to the pinned EN 16931 UBL XSLT")
	workerCmd.Flags().Duration("worker-saxon-startup-timeout", 30*time.Second, "Maximum duration of the worker-startup Saxon smoke transform")

	mustBindLocalFlag(v, workerCmd, "worker-consumer-name")
	mustBindLocalFlag(v, workerCmd, "worker-subject-filter")
	mustBindLocalFlag(v, workerCmd, "worker-metrics-addr")
	mustBindLocalFlag(v, workerCmd, "worker-validation-timeout")
	mustBindLocalFlag(v, workerCmd, "worker-xsd-validation-timeout")
	mustBindLocalFlag(v, workerCmd, "worker-en16931-validation-timeout")
	mustBindLocalFlag(v, workerCmd, "worker-concurrency")
	mustBindLocalFlag(v, workerCmd, "worker-prefetch")
	mustBindLocalFlag(v, workerCmd, "worker-ack-wait")
	mustBindLocalFlag(v, workerCmd, "worker-max-deliver")
	mustBindLocalFlag(v, workerCmd, "worker-max-ack-pending")
	mustBindLocalFlag(v, workerCmd, "worker-java-executable")
	mustBindLocalFlag(v, workerCmd, "worker-saxon-jar-path")
	mustBindLocalFlag(v, workerCmd, "worker-xmlresolver-jar-path")
	mustBindLocalFlag(v, workerCmd, "worker-saxon-configuration-path")
	mustBindLocalFlag(v, workerCmd, "worker-en16931-xslt-path")
	mustBindLocalFlag(v, workerCmd, "worker-saxon-startup-timeout")

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

	return runWorker(ctx, server.WorkerConfig{
		NATSURL:                  cfg.NATSURL,
		NATSOptions:              cfg.NATSOptions,
		QueueNamespace:           cfg.QueueNamespace,
		ObjectStore:              cfg.ObjectStore,
		ConsumerName:             cfg.WorkerConsumerName,
		SubjectFilter:            cfg.WorkerSubjectFilter,
		MetricsAddr:              cfg.WorkerMetricsAddr,
		ValidationTimeout:        cfg.WorkerValidationTimeout,
		XSDValidationTimeout:     cfg.WorkerXSDValidationTimeout,
		EN16931ValidationTimeout: cfg.WorkerEN16931ValidationTimeout,
		Concurrency:              cfg.WorkerConcurrency,
		Prefetch:                 cfg.WorkerPrefetch,
		AckWait:                  cfg.WorkerAckWait,
		MaxDeliver:               cfg.WorkerMaxDeliver,
		MaxAckPending:            cfg.WorkerMaxAckPending,
		JavaExecutable:           cfg.WorkerJavaExecutable,
		SaxonJARPath:             cfg.WorkerSaxonJARPath,
		XMLResolverJARPath:       cfg.WorkerXMLResolverJARPath,
		SaxonConfigurationPath:   cfg.WorkerSaxonConfigurationPath,
		EN16931XSLTPath:          cfg.WorkerEN16931XSLTPath,
		SaxonStartupTimeout:      cfg.WorkerSaxonStartupTimeout,
		UploadContentRetention:   cfg.UploadContentRetention,
		UploadContentMaxBytes:    cfg.UploadContentMaxBytes,
		Logger:                   slog.Default(),
	})
}
