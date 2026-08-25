package cmd

import (
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
)

func newDeadLetterCmd(cfg *Config) *cobra.Command {
	deadLetter := &cobra.Command{Use: "dead-letter", Short: "Inspect and replay terminal processing failures"}
	deadLetter.AddCommand(&cobra.Command{
		Use:   "retry UPLOAD_ID",
		Short: "Safely requeue one processing_failed upload and remove its dead letter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				return err
			}
			defer q.Close()
			if err := q.EnsureInfraWithConfig(cmd.Context(), queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
				return err
			}
			if err := q.ReplayProcessingFailure(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "requeued %s\n", args[0])
			return nil
		},
	})
	return deadLetter
}
