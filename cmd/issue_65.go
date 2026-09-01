package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	"github.com/spf13/cobra"
)

func newIssue65Cmd(cfg *Config) *cobra.Command {
	var gate regression.Issue65GateConfig
	command := &cobra.Command{
		Use:   "issue-65",
		Short: "Run the issue #65 split-deployment end-to-end validation gate",
		Long: `Prove issue #65 acceptance against a live split deployment:
- valid EN 16931 plan: public upload -> XSD PASS -> EN PASS -> final valid
- XSD-invalid document finalizes invalid with zero EN requests
- EN-invalid document: XSD PASS -> EN standards-invalid -> final invalid
- EN scale-to-zero: durable accumulation, restore, drain, terminal results
- frozen semantic-equivalence corpus against the monolithic baseline`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if gate.Token == "" {
				gate.Token = os.Getenv("EUBL21_REGRESSION_TOKEN")
			}
			summary, err := regression.RunIssue65Gate(cmd.Context(), gate)
			if summary != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "issue-65 total=%d passed=%d failed=%d artifacts=%s\n",
					summary.Total, summary.Passed, summary.Failed, gate.OutputDir)
			}
			return err
		},
	}

	command.Flags().StringVar(&gate.BaseURL, "base-url", "http://127.0.0.1:8080", "Base URL of the e-ubl21 public API")
	command.Flags().StringVar(&gate.Token, "token", "", "OAuth bearer token (or set EUBL21_REGRESSION_TOKEN)")
	command.Flags().StringVar(&gate.ManifestPath, "manifest", "", "Fixture manifest to execute (required)")
	command.Flags().StringVar(&gate.XSDInvalidManifestPath, "xsd-invalid-manifest", "", "Secondary manifest searched for an XSD-invalid fixture when the primary has none")
	command.Flags().StringVar(&gate.OutputDir, "output", "regression-results/issue-65/latest", "Directory for gate artifacts")
	command.Flags().StringVar(&gate.NATSURL, "nats-url", "", "NATS URL for JetStream stream assertions (required)")
	command.Flags().StringVar(&gate.QueueNamespace, "queue-namespace", "", "Queue namespace of the deployed topology (required)")
	command.Flags().StringVar(&gate.KubectlBin, "kubectl-bin", "kubectl", "kubectl executable used for scaling operations")
	command.Flags().StringVar(&gate.Namespace, "namespace", "", "Kubernetes namespace of the split deployment (required)")
	command.Flags().StringVar(&gate.ENDeployment, "en-deployment", "e-ubl21-worker-en16931", "Name of the EN 16931 worker deployment")
	command.Flags().StringVar(&gate.EquivalenceBaseline, "equivalence-baseline", "", "Frozen monolithic results.jsonl baseline (required)")
	command.Flags().DurationVar(&gate.PollInterval, "poll-interval", 100*time.Millisecond, "Delay between durable status polls")
	command.Flags().DurationVar(&gate.FixtureTimeout, "fixture-timeout", 120*time.Second, "Maximum upload-to-terminal time per fixture")

	return command
}
