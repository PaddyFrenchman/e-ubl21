package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go/jetstream"
)

func init() {
	stages := []queue.ValidationLayer{queue.ValidationLayerUBL21XSD, queue.ValidationLayerEN16931, queue.ValidationLayerPeppolBISBilling}
	outcomes := []queue.StageOutcome{queue.StageOutcomePassed, queue.StageOutcomeStandardsInvalid, queue.StageOutcomeOperationalFailure}
	reasons := []queue.StageOperationalReason{queue.StageReasonTimeout, queue.StageReasonCancelled, queue.StageReasonRuntimeUnavailable, queue.StageReasonRuntimeFailure, queue.StageReasonContentUnavailable, queue.StageReasonResourceLimit, queue.StageReasonMalformedOutput, queue.StageReasonInternalError}
	for _, stage := range stages {
		stageName := string(stage)
		stageInFlight.WithLabelValues(stageName).Set(0)
		stageDLQDepth.WithLabelValues(stageName).Set(0)
		stageQueueDepth.WithLabelValues(stageName).Set(0)
		stageRedeliveriesTotal.WithLabelValues(stageName).Add(0)
		validationStageDuration.WithLabelValues(stageName)
		stageQueueWait.WithLabelValues(stageName)
		stageRequestAge.WithLabelValues(stageName)
		for _, outcome := range outcomes {
			stageRequestsTotal.WithLabelValues(stageName, string(outcome)).Add(0)
			stageResultPublicationLatency.WithLabelValues(stageName, string(outcome))
		}
		for _, reason := range reasons {
			stageRetriesTotal.WithLabelValues(stageName, string(reason)).Add(0)
			stageRuntimeErrorsTotal.WithLabelValues(stageName, string(reason)).Add(0)
		}
		for _, component := range []string{"queue", "content_store", "execution_store", "assets", "runtime"} {
			stageStartupErrorsTotal.WithLabelValues(stageName, component).Add(0)
		}
	}
}

func beginStageObservation(request queue.StageRequest, message jetstream.Msg, started time.Time) func() {
	stage := string(request.Stage)
	var deliveries uint64
	if metadata, err := message.Metadata(); err == nil {
		deliveries = metadata.NumDelivered
	}
	if created, err := time.Parse(time.RFC3339Nano, request.CreatedAt); err == nil {
		age := started.Sub(created).Seconds()
		recordStageRequestTiming(stage, age, deliveries)
	}
	if deliveries > 1 {
		stageRedeliveriesTotal.WithLabelValues(stage).Inc()
	}
	stageInFlight.WithLabelValues(stage).Inc()
	return func() { stageInFlight.WithLabelValues(stage).Dec() }
}

func recordStageRequestTiming(stage string, age float64, deliveries uint64) {
	if age < 0 {
		age = 0
	}
	stageRequestAge.WithLabelValues(stage).Observe(age)
	if deliveries == 1 {
		stageQueueWait.WithLabelValues(stage).Observe(age)
	}
}

func recordSplitStageExecution(result queue.StageResult) {
	stage := string(result.Stage)
	validationStageDuration.WithLabelValues(stage).Observe(float64(result.DurationMS) / 1000)
	if result.Outcome == queue.StageOutcomeOperationalFailure {
		reason := string(result.Reason)
		stageRuntimeErrorsTotal.WithLabelValues(stage, reason).Inc()
	}
}

func recordSplitStagePublished(result queue.StageResult, maxAttempts uint32) {
	stage, outcome := string(result.Stage), string(result.Outcome)
	stageRequestsTotal.WithLabelValues(stage, outcome).Inc()
	if result.Outcome == queue.StageOutcomeOperationalFailure && result.Attempt < maxAttempts {
		stageRetriesTotal.WithLabelValues(stage, string(result.Reason)).Inc()
	}
}

func observeStageResultPublication(stage queue.ValidationLayer, outcome queue.StageOutcome, started time.Time) {
	stageResultPublicationLatency.WithLabelValues(string(stage), string(outcome)).Observe(time.Since(started).Seconds())
}

func logSplitStageResult(ctx context.Context, logger *slog.Logger, result queue.StageResult, recovered bool) {
	traceID, spanID := observability.CorrelationFields(ctx)
	attributes := []any{
		"trace_id", traceID, "span_id", spanID,
		"upload_id", result.UploadID, "validation_id", result.ValidationID,
		"document_type", result.DocumentType, "validation_profile", result.Plan,
		"profile_version", result.PlanVersion, "ruleset_version", result.RulesetVersion,
		"generation", result.Generation, "request_id", result.RequestID,
		"stage", result.Stage, "attempt", result.Attempt, "outcome", result.Outcome,
		"reason", result.Reason, "issue_count", len(result.Issues),
		"duration_ms", result.DurationMS, "recovered_from_dlq", recovered,
	}
	if result.Stage == queue.ValidationLayerPeppolBISBilling {
		profile := validationassets.Peppol()
		attributes = append(attributes,
			"bis_version", profile.Version, "bis_release_date", profile.PublicationDate,
			"bis_mandatory_from", profile.MandatoryFrom)
	}
	logger.Info("validation stage request completed", attributes...)
}
