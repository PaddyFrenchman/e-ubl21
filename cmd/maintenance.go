package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/documentstore"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
)

const defaultTombstoneGracePeriod = 7 * 24 * time.Hour
const defaultTombstoneMaxItems = 100
const defaultObjectWriteClaimMaxItems = 100

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
	recoverClaims := &cobra.Command{
		Use:   "recover-object-write-claims",
		Short: "Recover a bounded batch of stale guarded object-write claims",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cfg.ObjectStore.Immutability != documentstore.ImmutabilityGuardedWrite || cfg.Database == nil {
				return fmt.Errorf("guarded-write object storage and YugabyteDB configuration are required")
			}
			age, err := cmd.Flags().GetDuration("min-age")
			if err != nil {
				return err
			}
			limit, err := cmd.Flags().GetInt("max-items")
			if err != nil {
				return err
			}
			store, err := documentstore.New(cfg.ObjectStore)
			if err != nil {
				return err
			}
			client, err := database.Connect(cmd.Context(), *cfg.Database)
			if err != nil {
				return err
			}
			defer client.Close()
			claims, err := documentstore.NewClaimStore(client)
			if err != nil {
				return err
			}
			report, recoverErr := claims.Recover(cmd.Context(), store, age, limit)
			if err := json.NewEncoder(cmd.OutOrStdout()).Encode(report); err != nil {
				return fmt.Errorf("write claim recovery report: %w", err)
			}
			return recoverErr
		},
	}
	recoverClaims.Flags().Duration("min-age", time.Minute, "Minimum claim age eligible for recovery")
	recoverClaims.Flags().Int("max-items", defaultObjectWriteClaimMaxItems, "Maximum stale claims handled in one run")
	maintenance.AddCommand(recoverClaims)
	releaseClaim := &cobra.Command{Use: "release-object-write-claim", Short: "Explicitly release an abandoned guarded-write claim after writers are quiesced", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if cfg.ObjectStore.Immutability != documentstore.ImmutabilityGuardedWrite || cfg.Database == nil {
			return fmt.Errorf("guarded-write object storage and YugabyteDB configuration are required")
		}
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("--confirm is required after quiescing writers for this object")
		}
		key, _ := cmd.Flags().GetString("object-key")
		sha, _ := cmd.Flags().GetString("sha256")
		age, _ := cmd.Flags().GetDuration("min-age")
		if key == "" || sha == "" {
			return fmt.Errorf("--object-key and --sha256 are required")
		}
		store, err := documentstore.New(cfg.ObjectStore)
		if err != nil {
			return err
		}
		client, err := database.Connect(cmd.Context(), *cfg.Database)
		if err != nil {
			return err
		}
		defer client.Close()
		claims, err := documentstore.NewClaimStore(client)
		if err != nil {
			return err
		}
		if err := claims.ReleaseAbsent(cmd.Context(), store, key, sha, age); err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"released": true, "objectKey": key, "sha256": sha})
	}}
	releaseClaim.Flags().String("object-key", "", "Exact claimed object key (writers must be quiesced first)")
	releaseClaim.Flags().String("sha256", "", "Exact claimed object SHA-256")
	releaseClaim.Flags().Duration("min-age", 5*time.Minute, "Minimum claim age eligible for explicit release")
	releaseClaim.Flags().Bool("confirm", false, "Confirm writers are quiesced and the absent object was investigated")
	maintenance.AddCommand(releaseClaim)
	return maintenance
}
