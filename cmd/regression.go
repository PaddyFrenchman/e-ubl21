package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	"github.com/spf13/cobra"
)

func newRegressionCmd() *cobra.Command {
	var baseURL, token, manifest, outputDir, profile string
	var pollInterval, fixtureTimeout time.Duration
	command := &cobra.Command{
		Use:   "regression",
		Short: "Run a manifest corpus through the authenticated public API",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if token == "" {
				token = os.Getenv("EUBL21_REGRESSION_TOKEN")
			}
			summary, err := regression.RunCorpus(cmd.Context(), regression.RunnerConfig{
				BaseURL: baseURL, Token: token, ManifestPath: manifest, OutputDir: outputDir,
				Profile: profile, PollInterval: pollInterval, FixtureTimeout: fixtureTimeout,
			})
			fmt.Fprintf(cmd.OutOrStdout(), "regression total=%d passed=%d failed=%d artifacts=%s\n", summary.Total, summary.Passed, summary.Failed, outputDir)
			return err
		},
	}
	command.Flags().StringVar(&baseURL, "base-url", "http://127.0.0.1:8080", "Base URL of the e-ubl21 public API")
	command.Flags().StringVar(&token, "token", "", "OAuth bearer token (or set EUBL21_REGRESSION_TOKEN)")
	command.Flags().StringVar(&manifest, "manifest", "createsamples/samples-en16931/manifest.json", "Fixture manifest to execute")
	command.Flags().StringVar(&outputDir, "output", "regression-results/latest", "Directory for normalized regression artifacts")
	command.Flags().StringVar(&profile, "profile", "all", "Fixture profile selector (all, ubl21, or ubl21-en16931)")
	command.Flags().DurationVar(&pollInterval, "poll-interval", 100*time.Millisecond, "Delay between durable status polls")
	command.Flags().DurationVar(&fixtureTimeout, "fixture-timeout", 60*time.Second, "Maximum upload-to-terminal time per fixture")
	return command
}
