package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/server"
	"github.com/spf13/cobra"
)

const defaultLegacyConsumerName = "ubl-validator"

type migrationOutput struct {
	State          queue.DeploymentGeneration           `json:"state"`
	Drain          *queue.LegacyDrainStatus             `json:"drain,omitempty"`
	Reconciliation *server.LegacyPendingMigrationReport `json:"reconciliation,omitempty"`
}

func newMigrationCmd(cfg *Config) *cobra.Command {
	command := &cobra.Command{Use: "migration", Short: "Control the legacy-to-split deployment barrier"}
	command.PersistentFlags().String("legacy-consumer-name", defaultLegacyConsumerName, "Legacy combined-worker durable consumer to drain")
	command.PersistentFlags().Bool("allow-no-legacy-consumer", false, "Explicitly allow the named legacy consumer to be absent while still checking all stream consumers")
	command.AddCommand(
		&cobra.Command{
			Use: "status", Short: "Show the durable deployment generation and legacy drain state", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				consumer, _ := cmd.Flags().GetString("legacy-consumer-name")
				allowMissing, _ := cmd.Flags().GetBool("allow-no-legacy-consumer")
				return migrationStatus(cmd.Context(), cmd.OutOrStdout(), cfg, consumer, allowMissing)
			},
		},
		&cobra.Command{
			Use: "begin-drain", Short: "Stop admission and begin draining the legacy combined worker", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return migrationTransition(cmd.Context(), cmd.OutOrStdout(), cfg, queue.DeploymentModeDraining)
			},
		},
		&cobra.Command{
			Use: "activate", Short: "Reconcile pending uploads and activate split validation", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				consumer, _ := cmd.Flags().GetString("legacy-consumer-name")
				allowMissing, _ := cmd.Flags().GetBool("allow-no-legacy-consumer")
				return migrationActivate(cmd.Context(), cmd.OutOrStdout(), cfg, consumer, allowMissing)
			},
		},
		&cobra.Command{
			Use: "rollback", Short: "Return an unactivated drain to legacy mode", Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return migrationTransition(cmd.Context(), cmd.OutOrStdout(), cfg, queue.DeploymentModeLegacy)
			},
		},
	)
	return command
}

func migrationClient(cfg *Config) (*queue.Client, error) {
	client, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		return nil, fmt.Errorf("migration connect to NATS: %w", err)
	}
	return client, nil
}

func migrationStatus(ctx context.Context, output io.Writer, cfg *Config, consumer string, allowMissing bool) error {
	client, err := migrationClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	current, err := client.DeploymentGeneration(ctx)
	if err != nil {
		return fmt.Errorf("load deployment generation: %w", err)
	}
	drain, err := client.InspectLegacyDrainWithPolicy(ctx, consumer, allowMissing)
	if err != nil {
		return err
	}
	return writeMigrationOutput(output, migrationOutput{State: current.State, Drain: &drain})
}

func migrationTransition(ctx context.Context, output io.Writer, cfg *Config, target queue.DeploymentMode) error {
	client, err := migrationClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	current, err := client.DeploymentGeneration(ctx)
	if err != nil {
		return fmt.Errorf("load deployment generation: %w", err)
	}
	if current.State.Mode == target {
		return writeMigrationOutput(output, migrationOutput{State: current.State})
	}
	if target == queue.DeploymentModeLegacy && current.State.Mode == queue.DeploymentModeSplit {
		return errors.New("split activation is forward-only; the legacy combined worker cannot interpret split execution state")
	}
	if target == queue.DeploymentModeLegacy && current.State.Mode == queue.DeploymentModeActivating {
		return errors.New("activation reconciliation has begun and is forward-only; repair the reported error and resume activation")
	}
	next, err := client.TransitionDeploymentMode(ctx, current, target, time.Now())
	if err != nil {
		return err
	}
	return writeMigrationOutput(output, migrationOutput{State: next.State})
}

func migrationActivate(ctx context.Context, output io.Writer, cfg *Config, consumer string, allowMissing bool) error {
	client, err := migrationClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()
	current, err := client.DeploymentGeneration(ctx)
	if err != nil {
		return fmt.Errorf("load deployment generation: %w", err)
	}
	if current.State.Mode == queue.DeploymentModeSplit {
		return writeMigrationOutput(output, migrationOutput{State: current.State})
	}
	if current.State.Mode != queue.DeploymentModeDraining && current.State.Mode != queue.DeploymentModeActivating {
		return fmt.Errorf("split activation requires deployment mode %q or %q, found %q", queue.DeploymentModeDraining, queue.DeploymentModeActivating, current.State.Mode)
	}
	drain, err := client.InspectLegacyDrainWithPolicy(ctx, consumer, allowMissing)
	if err != nil {
		return err
	}
	if !drain.Ready {
		return fmt.Errorf("legacy validation is not drained: stream=%d pending=%d ackPending=%d waiting=%d", drain.StreamMessages, drain.ConsumerPending, drain.ConsumerAckPending, drain.ConsumerWaiting)
	}
	if err := client.VerifyValidationExecutionInfra(ctx); err != nil {
		return fmt.Errorf("verify execution infrastructure: %w", err)
	}
	for _, stage := range []queue.ValidationLayer{
		queue.ValidationLayerUBL21XSD,
		queue.ValidationLayerEN16931,
		queue.ValidationLayerPeppolBISBilling,
		queue.ValidationLayerPeppolEnvelope,
	} {
		if _, err := client.VerifyStageWorkerInfra(ctx, stage); err != nil {
			return fmt.Errorf("verify %s stage infrastructure: %w", stage, err)
		}
	}
	if current.State.Mode == queue.DeploymentModeDraining {
		current, err = client.TransitionDeploymentMode(ctx, current, queue.DeploymentModeActivating, time.Now())
		if err != nil {
			return fmt.Errorf("claim forward-only activation reconciliation: %w", err)
		}
	}
	reconciliation, err := server.MigrateLegacyPendingUploads(ctx, client)
	if err != nil {
		return err
	}
	// Re-check after reconciliation closes the window for a late legacy
	// descriptor that was already in flight when draining began.
	drain, err = client.InspectLegacyDrainWithPolicy(ctx, consumer, allowMissing)
	if err != nil {
		return err
	}
	if !drain.Ready {
		return errors.New("legacy validation became active during reconciliation; split activation aborted")
	}
	current, err = client.DeploymentGeneration(ctx)
	if err != nil {
		return fmt.Errorf("reload deployment generation after reconciliation: %w", err)
	}
	if current.State.Mode != queue.DeploymentModeActivating {
		return fmt.Errorf("deployment mode changed during reconciliation: found %q", current.State.Mode)
	}
	next, err := client.TransitionDeploymentMode(ctx, current, queue.DeploymentModeSplit, time.Now())
	if err != nil {
		return err
	}
	return writeMigrationOutput(output, migrationOutput{State: next.State, Drain: &drain, Reconciliation: &reconciliation})
}

func writeMigrationOutput(output io.Writer, value migrationOutput) error {
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
