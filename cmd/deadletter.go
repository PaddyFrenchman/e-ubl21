package cmd

import (
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
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
	deadLetter.AddCommand(&cobra.Command{
		Use:   "stage-retry STAGE DEAD_LETTER_ID",
		Short: "Replay one exhausted validation stage without rerunning passing prerequisites",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			stage := queue.ValidationLayer(args[0])
			if _, err := cfg.QueueNamespace.StageQueue(stage); err != nil {
				return err
			}
			q, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
			if err != nil {
				return err
			}
			defer q.Close()
			dead, err := q.GetStageDeadLetter(cmd.Context(), stage, args[1])
			if err != nil {
				return err
			}
			replayed, err := orchestrator.New(q).ReplayStage(cmd.Context(), dead)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "replayed %s generation %d stage %s\n", replayed.Execution.ValidationID, replayed.Execution.Generation, stage)
			return nil
		},
	})
	return deadLetter
}
