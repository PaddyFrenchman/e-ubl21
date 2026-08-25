package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
)

const defaultTombstoneGracePeriod = 7 * 24 * time.Hour
const defaultTombstoneMaxItems = 100

func newMaintenanceCmd(cfg *Config) *cobra.Command {
	maintenance := &cobra.Command{
		Use:   "maintenance",
		Short: "Run bounded operational maintenance",
	}
	purge := &cobra.Command{
		Use:   "purge-content-tombstones",
		Short: "Purge old raw-content Object Store delete markers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gracePeriod, err := cmd.Flags().GetDuration("grace-period")
			if err != nil {
				return err
			}
			maxItems, err := cmd.Flags().GetInt("max-items")
			if err != nil {
				return err
			}
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return err
			}

			q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				return err
			}
			defer q.Close()
			report, purgeErr := q.PurgeUploadContentTombstones(cmd.Context(), gracePeriod, maxItems, dryRun, time.Now())
			output := struct {
				queue.TombstonePurgeReport
				Namespace   string `json:"namespace"`
				DryRun      bool   `json:"dryRun"`
				GracePeriod string `json:"gracePeriod"`
				MaxItems    int    `json:"maxItems"`
			}{report, cfg.QueueNamespace.String(), dryRun, gracePeriod.String(), maxItems}
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(output); err != nil {
				return fmt.Errorf("write tombstone purge report: %w", err)
			}
			if purgeErr != nil {
				return purgeErr
			}
			if report.Failed > 0 {
				return fmt.Errorf("content tombstone cleanup incomplete: %d item(s) failed", report.Failed)
			}
			return nil
		},
	}
	purge.Flags().Duration("grace-period", defaultTombstoneGracePeriod, "Minimum age of delete markers eligible for purge")
	purge.Flags().Int("max-items", defaultTombstoneMaxItems, "Maximum eligible delete markers handled in one run")
	purge.Flags().Bool("dry-run", false, "Report eligible delete markers without purging them")
	maintenance.AddCommand(purge)
	return maintenance
}
