package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newProvisionCmd(v *viper.Viper, cfg *Config) *cobra.Command {
	command := &cobra.Command{
		Use:   "provision",
		Short: "Provision namespace-scoped durable queue infrastructure",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return provisionInfrastructure(cmd.Context(), cfg)
		},
	}
	command.Flags().Duration("stage-xsd-ack-wait", 90*time.Second, "XSD request/result acknowledgement window")
	command.Flags().Int("stage-xsd-max-ack-pending", 64, "Fleet-wide XSD pending-delivery limit")
	command.Flags().Duration("stage-en16931-ack-wait", 120*time.Second, "EN 16931 request/result acknowledgement window")
	command.Flags().Int("stage-en16931-max-ack-pending", 32, "Fleet-wide EN 16931 pending-delivery limit")
	command.Flags().Duration("stage-peppol-ack-wait", 120*time.Second, "Reserved Peppol request/result acknowledgement window")
	command.Flags().Int("stage-peppol-max-ack-pending", 32, "Reserved Peppol pending-delivery limit")
	command.Flags().Duration("stage-peppol-envelope-ack-wait", 120*time.Second, "Peppol envelope request/result acknowledgement window")
	command.Flags().Int("stage-peppol-envelope-max-ack-pending", 32, "Peppol envelope pending-delivery limit")
	for _, name := range []string{
		"stage-xsd-ack-wait", "stage-xsd-max-ack-pending", "stage-en16931-ack-wait",
		"stage-en16931-max-ack-pending", "stage-peppol-ack-wait", "stage-peppol-max-ack-pending",
		"stage-peppol-envelope-ack-wait", "stage-peppol-envelope-max-ack-pending",
	} {
		mustBindLocalFlag(v, command, name)
	}
	return command
}

func provisionInfrastructure(ctx context.Context, cfg *Config) error {
	client, err := queue.ConnectWithOptions(cfg.NATSURL, cfg.QueueNamespace, cfg.NATSOptions...)
	if err != nil {
		return fmt.Errorf("provision connect to NATS: %w", err)
	}
	defer client.Close()
	if err := client.EnsureInfraWithConfig(ctx, queue.InfraConfig{UploadContentMaxBytes: cfg.UploadContentMaxBytes}); err != nil {
		return fmt.Errorf("provision upload infrastructure: %w", err)
	}
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		return fmt.Errorf("provision execution infrastructure: %w", err)
	}
	if _, err := client.ProvisionDeploymentGenerationInfra(ctx, time.Now()); err != nil {
		return fmt.Errorf("provision deployment generation infrastructure: %w", err)
	}
	envelopeAckWait, envelopeMaxAckPending := cfg.StagePeppolEnvelopeAckWait, cfg.StagePeppolEnvelopeMaxAckPending
	if envelopeAckWait <= 0 {
		envelopeAckWait = cfg.StagePeppolAckWait
	}
	if envelopeMaxAckPending <= 0 {
		envelopeMaxAckPending = cfg.StagePeppolMaxAckPending
	}
	stageConfig := queue.StageQueueInfraConfig{Stages: []queue.StageQueueConfig{
		provisionStage(queue.ValidationLayerUBL21XSD, cfg.StageXSDAckWait, cfg.StageXSDMaxAckPending),
		provisionStage(queue.ValidationLayerEN16931, cfg.StageENAckWait, cfg.StageENMaxAckPending),
		provisionStage(queue.ValidationLayerPeppolBISBilling, cfg.StagePeppolAckWait, cfg.StagePeppolMaxAckPending),
		provisionStage(queue.ValidationLayerPeppolEnvelope, envelopeAckWait, envelopeMaxAckPending),
	}}
	if err := client.ProvisionStageQueueInfra(ctx, stageConfig); err != nil {
		return fmt.Errorf("provision stage infrastructure: %w", err)
	}
	return nil
}

func provisionStage(stage queue.ValidationLayer, ackWait time.Duration, maxAckPending int) queue.StageQueueConfig {
	delivery := queue.StageDeliveryConfig{AckWait: ackWait, MaxAckPending: maxAckPending}
	return queue.StageQueueConfig{Stage: stage, RequestDelivery: delivery, ResultDelivery: delivery}
}
