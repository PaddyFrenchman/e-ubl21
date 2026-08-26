package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func (pipeline *documentValidationPipeline) observeStage(uploadID, documentType string, stage queue.ValidationStageResult, outcome string, duration time.Duration, stageErr error) {
	metricStage := boundedValidationMetricStage(stage.Layer)
	metricOutcome := boundedValidationMetricOutcome(outcome)
	seconds := duration.Seconds()
	if seconds < 0 {
		seconds = 0
	}
	recordValidationStage(metricStage, metricOutcome, seconds)
	runtimeFailureReason := ""
	if stage.Layer == queue.ValidationLayerEN16931 {
		recordEN16931Issues(stage.Issues)
		if stageErr != nil {
			runtimeFailureReason = en16931RuntimeFailureReason(stageErr)
			recordEN16931RuntimeFailure(runtimeFailureReason)
		}
	}
	if pipeline.logger == nil {
		return
	}

	attrs := []slog.Attr{
		slog.String("upload_id", uploadID),
		slog.String("document_type", documentType),
		slog.String("stage", metricStage),
		slog.String("profile_version", stage.Version),
		slog.String("result", metricOutcome),
		slog.Int("issue_count", len(stage.Issues)),
		slog.Int64("duration_ms", duration.Milliseconds()),
	}
	level := slog.LevelInfo
	message := "validation stage completed"
	if stageErr != nil {
		level = slog.LevelError
		message = "validation stage failed"
		if runtimeFailureReason != "" {
			attrs = append(attrs, slog.String("runtime_failure_reason", runtimeFailureReason))
		}
		attrs = append(attrs, slog.Any("error", stageErr))
	}
	pipeline.logger.LogAttrs(context.Background(), level, message, attrs...)
}

func boundedValidationMetricStage(layer queue.ValidationLayer) string {
	switch layer {
	case queue.ValidationLayerUBL21XSD:
		return validationMetricStageXSD
	case queue.ValidationLayerEN16931:
		return validationMetricStageEN16931
	default:
		return "other"
	}
}

func boundedValidationMetricOutcome(outcome string) string {
	switch outcome {
	case "valid", "invalid", "error":
		return outcome
	default:
		return "error"
	}
}

func en16931RuntimeFailureReason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, errEN16931RuntimeAsset):
		return "asset"
	case errors.Is(err, errEN16931RuntimeSaxon):
		return "saxon"
	case errors.Is(err, errEN16931RuntimeSVRL), errors.Is(err, errEN16931SVRLOutputLimit):
		return "svrl"
	case errors.Is(err, errEN16931RuntimeInput), errors.Is(err, errValidationContentStream):
		return "input"
	default:
		return "other"
	}
}
