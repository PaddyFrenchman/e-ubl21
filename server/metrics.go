// This file defines the Prometheus metrics exposed by the server (via the
// GET /metrics endpoint, registered in server.go) using
// github.com/prometheus/client_golang, the de facto standard Go metrics
// client. Metrics are updated by the shared request-observability
// middleware in logging.go, which records both structured logs and these
// metrics from a single response-status/duration observation per request.
package server

import (
	"net/http"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	stageServiceDurationBuckets = []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20, 30, 60}
	stageQueueAgeBuckets        = []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1800, 3600}
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_http_requests_total",
		Help: "Total HTTP requests handled, labeled by method, route pattern and status code.",
	}, []string{"method", "route", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "e_ubl21_http_request_duration_seconds",
		Help:    "HTTP request handling duration in seconds, labeled by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route"})

	uploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_uploads_total",
		Help: "Total UBL upload attempts handled by the API, labeled by outcome (queued, or the rejection reason).",
	}, []string{"outcome"})
	uploadReconciliations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_upload_reconciliations_total",
		Help: "Upload acceptance records handled by reconciliation, labeled by outcome.",
	}, []string{"outcome"})
	objectWriteClaimReconciliations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_object_write_claim_reconciliations_total",
		Help: "Guarded object-write claims handled by recovery, labeled by outcome.",
	}, []string{"outcome"})
	uploadContentExpirations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_upload_content_expirations_total",
		Help: "Raw upload-content records handled by retention sweeping, labeled by outcome.",
	}, []string{"outcome"})
	uploadContentExpiredBytes = promauto.NewCounter(prometheus.CounterOpts{
		Name: "e_ubl21_upload_content_expired_bytes_total",
		Help: "Raw XML payload bytes reclaimed by retention sweeping, excluding Object Store metadata.",
	})
	uploadContentStorageBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_upload_content_storage_bytes",
		Help: "Current physical size of the raw XML Object Store, including JetStream object metadata.",
	})
	uploadContentStorageLimitBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_upload_content_storage_limit_bytes",
		Help: "Configured raw XML Object Store byte limit; zero means unlimited.",
	})

	validationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validations_total",
		Help: "Total UBL documents processed by the worker, labeled by outcome (valid, invalid, or a processing failure reason).",
	}, []string{"outcome"})

	validationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "e_ubl21_validation_duration_seconds",
		Help:    "Worker document validation duration in seconds, labeled by document type.",
		Buckets: prometheus.DefBuckets,
	}, []string{"document_type"})
	validationStageTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_total",
		Help: "Total validation stage executions, labeled by bounded stage and outcome values.",
	}, []string{"stage", "outcome"})
	validationStageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "e_ubl21_validation_stage_duration_seconds",
		Help:    "Validation stage execution duration in seconds, labeled by bounded stage values.",
		Buckets: stageServiceDurationBuckets,
	}, []string{"stage"})
	en16931IssuesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_en16931_issues_total",
		Help: "Total EN 16931 SVRL issues, labeled by normalized bounded severity values.",
	}, []string{"severity"})
	en16931RuntimeFailuresTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_en16931_runtime_failures_total",
		Help: "Total EN 16931 operational failures, labeled by a bounded reason classification.",
	}, []string{"reason"})

	workerValidationsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_validations_in_flight",
		Help: "Current number of documents actively being processed by this worker process.",
	})

	workerJobsChannelDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_jobs_channel_depth",
		Help: "Current number of pulled documents waiting in the bounded Go jobs channel; excludes the NATS iterator's separately bounded internal buffer.",
	})

	workerJobsChannelCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_jobs_channel_capacity",
		Help: "Configured capacity of the bounded Go jobs channel.",
	})

	workerNATSIteratorCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_nats_iterator_capacity",
		Help: "Configured per-process NATS iterator message limit, equal to concurrency plus prefetch.",
	})

	workerRedeliveries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "e_ubl21_worker_redeliveries_total",
		Help: "Total documents observed with a JetStream delivery count greater than one.",
	})
	workerTerminalFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "e_ubl21_worker_terminal_failures_total",
		Help: "Total documents moved to processing_failed after exhausting application processing attempts.",
	})
	workerCompletionReconciliations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_worker_completion_reconciliations_total",
		Help: "Pending worker completion records handled without a work descriptor, labeled by outcome.",
	}, []string{"outcome"})
	workerDeadLetterDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_dead_letter_depth",
		Help: "Current number of terminal failures retained in the dead-letter stream.",
	})
	stageRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_requests_total", Help: "Completed split-stage requests by bounded stage and outcome.",
	}, []string{"stage", "outcome"})
	stageQueueWait = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "e_ubl21_validation_stage_queue_wait_seconds", Help: "Time from durable request creation until first worker handling.", Buckets: stageQueueAgeBuckets,
	}, []string{"stage"})
	stageRequestAge = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "e_ubl21_validation_stage_request_age_seconds", Help: "Age of a stage request on every broker delivery observed by a worker.", Buckets: stageQueueAgeBuckets,
	}, []string{"stage"})
	stageInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "e_ubl21_validation_stage_inflight", Help: "Stage requests currently being handled.",
	}, []string{"stage"})
	stageRetriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_retries_total", Help: "Operational stage results eligible for another logical attempt.",
	}, []string{"stage", "reason"})
	stageRedeliveriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_redeliveries_total", Help: "Stage requests observed after their first broker delivery.",
	}, []string{"stage"})
	stageDLQDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "e_ubl21_validation_stage_dlq_depth", Help: "Permanent operational dead letters retained by stage.",
	}, []string{"stage"})
	stageQueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "e_ubl21_validation_stage_queue_depth", Help: "Stage request messages retained by the broker.",
	}, []string{"stage"})
	stageResultPublicationLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "e_ubl21_validation_stage_result_publication_latency_seconds", Help: "Duration of stage result publication attempts.", Buckets: prometheus.DefBuckets,
	}, []string{"stage", "outcome"})
	stageRuntimeErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_runtime_errors_total", Help: "Operational validator failures by bounded reason.",
	}, []string{"stage", "reason"})
	stageStartupErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "e_ubl21_validation_stage_startup_errors_total", Help: "Stage process startup failures by bounded component.",
	}, []string{"stage", "component"})
)

// recordUploadOutcome increments the upload counter for outcome, e.g.
// "queued", "invalid_xml", "unsupported_document_type", "bad_request",
// "internal_error".
func recordUploadOutcome(outcome string) {
	uploadsTotal.WithLabelValues(outcome).Inc()
}

// recordValidationOutcome increments the worker's validation counter for
// outcome, e.g. "valid", "invalid", "validator_unavailable",
// "unsupported_document_type", "internal_error".
func recordValidationOutcome(outcome string) {
	validationsTotal.WithLabelValues(outcome).Inc()
}

// recordValidationDuration observes how long the worker took to validate a
// document of the given type against its XSD.
func recordValidationDuration(documentType string, seconds float64) {
	validationDuration.WithLabelValues(documentType).Observe(seconds)
}

const (
	validationMetricStageXSD     = "ubl21_xsd"
	validationMetricStageEN16931 = "en16931"
)

func recordValidationStage(stage, outcome string, seconds float64) {
	validationStageTotal.WithLabelValues(stage, outcome).Inc()
	validationStageDuration.WithLabelValues(stage).Observe(seconds)
}

func recordEN16931Issues(issues []queue.ValidationIssue) {
	for _, issue := range issues {
		en16931IssuesTotal.WithLabelValues(normalizeEN16931MetricSeverity(issue.Severity)).Inc()
	}
}

func normalizeEN16931MetricSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "fatal":
		return "fatal"
	case "error":
		return "error"
	case "warning", "warn":
		return "warning"
	case "info", "information":
		return "info"
	default:
		return "other"
	}
}

func recordEN16931RuntimeFailure(reason string) {
	en16931RuntimeFailuresTotal.WithLabelValues(reason).Inc()
}

// metricsHandler exposes all registered metrics in the Prometheus text
// exposition format.
func metricsHandler() http.HandlerFunc {
	handler := promhttp.Handler()
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}
