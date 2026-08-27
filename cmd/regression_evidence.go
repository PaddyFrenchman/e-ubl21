package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	"github.com/spf13/cobra"
)

func newRegressionEvidenceCmd(cfg *Config) *cobra.Command {
	var outputDir, manifest, token, namespace, selector, imageDigest, startedAt string
	var logs, metrics []string
	var collectNATS, collectKubernetes bool
	var sourceTimeout time.Duration
	command := &cobra.Command{
		Use:   "collect-evidence",
		Short: "Collect bounded diagnostics beside normalized regression results",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			logFiles, err := parseNamedEvidenceSources(logs)
			if err != nil {
				return err
			}
			metricEndpoints, err := parseNamedEvidenceSources(metrics)
			if err != nil {
				return err
			}
			if token == "" {
				token = os.Getenv("EUBL21_REGRESSION_TOKEN")
			}
			if imageDigest == "" {
				imageDigest = os.Getenv("EUBL21_IMAGE_DIGEST")
			}
			var started time.Time
			if startedAt != "" {
				started, err = time.Parse(time.RFC3339Nano, startedAt)
				if err != nil {
					return fmt.Errorf("parse --started-at: %w", err)
				}
			}
			report, collectErr := regression.CollectEvidence(cmd.Context(), regression.EvidenceConfig{
				OutputDir: outputDir, ManifestPath: manifest, Token: token,
				LogFiles: logFiles, MetricsEndpoints: metricEndpoints,
				NATSURL: cfg.NATSURL, NATSOptions: cfg.NATSOptions, QueueNamespace: cfg.QueueNamespace, CollectNATS: collectNATS,
				CollectKubernetes: collectKubernetes, KubernetesNamespace: strings.TrimSpace(namespace), KubernetesSelector: selector,
				JavaExecutable: cfg.WorkerJavaExecutable, DockerImageDigest: imageDigest,
				StartedAt: started, EndedAt: time.Now().UTC(), SourceTimeout: sourceTimeout,
			})
			failed := 0
			for _, capture := range report.Captures {
				if capture.Status == "failed" {
					failed++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "evidence captures=%d failed=%d artifacts=%s\n", len(report.Captures), failed, outputDir)
			return collectErr
		},
	}
	command.Flags().StringVar(&outputDir, "output", "regression-results/latest", "Regression artifact directory")
	command.Flags().StringVar(&manifest, "manifest", "createsamples/samples-en16931/manifest.json", "Exact fixture manifest used by the run")
	command.Flags().StringVar(&token, "token", "", "Bearer token for protected metrics endpoints (or EUBL21_REGRESSION_TOKEN)")
	command.Flags().StringArrayVar(&logs, "log", nil, "Service log source as NAME=PATH (repeatable)")
	command.Flags().StringArrayVar(&metrics, "metrics", nil, "Prometheus endpoint as NAME=URL (repeatable)")
	command.Flags().BoolVar(&collectNATS, "nats-state", false, "Capture selected namespaced JetStream stream and consumer state")
	command.Flags().BoolVar(&collectKubernetes, "kubernetes", false, "Capture Kubernetes events, pod state, and bounded container logs")
	command.Flags().StringVar(&namespace, "kubernetes-namespace", "", "Kubernetes namespace (empty captures selected pods/events across namespaces)")
	command.Flags().StringVar(&selector, "kubernetes-selector", "app.kubernetes.io/part-of=e-ubl21", "Label selector for service pods")
	command.Flags().StringVar(&imageDigest, "image-digest", "", "Tested Docker image digest (or EUBL21_IMAGE_DIGEST)")
	command.Flags().StringVar(&startedAt, "started-at", "", "Regression start time in RFC3339; collection time is used when omitted")
	command.Flags().DurationVar(&sourceTimeout, "source-timeout", 15*time.Second, "Maximum time for each remote endpoint or diagnostic command")
	return command
}

func parseNamedEvidenceSources(values []string) (map[string]string, error) {
	parsed := make(map[string]string, len(values))
	for _, value := range values {
		name, source, ok := strings.Cut(value, "=")
		name, source = strings.TrimSpace(name), strings.TrimSpace(source)
		if !ok || name == "" || source == "" {
			return nil, fmt.Errorf("evidence source %q must use NAME=VALUE", value)
		}
		if _, duplicate := parsed[name]; duplicate {
			return nil, fmt.Errorf("duplicate evidence source name %q", name)
		}
		parsed[name] = source
	}
	return parsed, nil
}
