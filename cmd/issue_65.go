package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	"github.com/spf13/cobra"
)

func newIssue65Cmd(cfg *Config) *cobra.Command {
	var baseURL, token, manifest, outputDir string
	var natsURL, queueNamespace, kubectlBin, namespace string
	var pollInterval, fixtureTimeout time.Duration

	command := &cobra.Command{
		Use:   "issue-65",
		Short: "Issue #65 split-deployment end-to-end validation gate",
		Long: `Prove issue #65 acceptance criteria:
- Split architecture deploys independently (API, orchestrator, XSD worker, EN worker)
- EN scale-to-zero with queue accumulation and recovery
- All validation outcomes preserved (valid/invalid/errors)
- Semantic equivalence from frozen corpus`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if token == "" {
				token = os.Getenv("EUBL21_REGRESSION_TOKEN")
			}
			if token == "" {
				return errors.New("token required (--token or EUBL21_REGRESSION_TOKEN)")
			}
			if manifest == "" {
				return errors.New("manifest required (--manifest)")
			}
			if natsURL == "" {
				return errors.New("NATS URL required (--nats-url)")
			}
			if kubectlBin == "" {
				return errors.New("kubectl binary path required (--kubectl-bin)")
			}
			if namespace == "" {
				return errors.New("Kubernetes namespace required (--namespace)")
			}

			// Invoke issue #65 test suite
			summary, err := regression.RunIssue65Gate(cmd.Context(), regression.Issue65GateConfig{
				BaseURL:           baseURL,
				Token:             token,
				ManifestPath:      manifest,
				OutputDir:         outputDir,
				NATSURL:           natsURL,
				QueueNamespace:    queueNamespace,
				KubectlBin:        kubectlBin,
				Namespace:         namespace,
				PollInterval:      pollInterval,
				FixtureTimeout:    fixtureTimeout,
			})
			if summary != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "issue-65 total=%d passed=%d failed=%d artifacts=%s\n",
					summary.Total, summary.Passed, summary.Failed, outputDir)
			}
			return err
		},
	}

	command.Flags().StringVar(&baseURL, "base-url", "http://127.0.0.1:8080", "Base URL of the e-ubl21 public API")
	command.Flags().StringVar(&token, "token", "", "OAuth bearer token (or set EUBL21_REGRESSION_TOKEN)")
	command.Flags().StringVar(&manifest, "manifest", "", "Fixture manifest to execute (required)")
	command.Flags().StringVar(&outputDir, "output", "regression-results/issue-65/latest", "Directory for test artifacts")
	command.Flags().StringVar(&natsURL, "nats-url", "", "NATS connection URL (required)")
	command.Flags().StringVar(&queueNamespace, "queue-namespace", "eubl21.i65", "Queue namespace for validation requests")
	command.Flags().StringVar(&kubectlBin, "kubectl-bin", "", "Path to kubectl binary (required)")
	command.Flags().StringVar(&namespace, "namespace", "", "Kubernetes namespace with split deployment (required)")
	command.Flags().DurationVar(&pollInterval, "poll-interval", 100*time.Millisecond, "Delay between durable status polls")
	command.Flags().DurationVar(&fixtureTimeout, "fixture-timeout", 60*time.Second, "Maximum upload-to-terminal time per fixture")

	return command
}
