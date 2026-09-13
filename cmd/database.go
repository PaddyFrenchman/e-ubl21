package cmd

import (
	"fmt"
	"time"

	appdb "github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/spf13/cobra"
)

func newDatabaseCmd(cfg *Config) *cobra.Command {
	databaseCommand := &cobra.Command{
		Use:   "database",
		Short: "Manage the relational database schema",
	}

	var timeout time.Duration
	migrateCommand := &cobra.Command{
		Use:   "migrate",
		Short: "Apply pending embedded schema migrations",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if cfg.Database == nil {
				return fmt.Errorf("database migrate: --database-url or EUBL21_DATABASE_URL is required")
			}
			migrator, err := appdb.NewMigrator(timeout)
			if err != nil {
				return fmt.Errorf("database migrate: %w", err)
			}
			client, err := appdb.Connect(command.Context(), *cfg.Database)
			if err != nil {
				return fmt.Errorf("database migrate: %w", err)
			}
			defer client.Close()

			result, err := migrator.Run(command.Context(), client)
			if err != nil {
				return fmt.Errorf("database migrate: %w", err)
			}
			if len(result.AppliedVersions) == 0 {
				fmt.Fprintf(command.OutOrStdout(), "Database schema is current at version %06d.\n", result.CurrentVersion)
				return nil
			}
			fmt.Fprintf(command.OutOrStdout(), "Database schema migrated from version %06d to %06d (%d applied).\n",
				result.PreviousVersion, result.CurrentVersion, len(result.AppliedVersions))
			return nil
		},
	}
	migrateCommand.Flags().DurationVar(&timeout, "timeout", appdb.DefaultMigrationTimeout, "Maximum time to wait for the migration lock and apply pending migrations")
	databaseCommand.AddCommand(migrateCommand)
	return databaseCommand
}
