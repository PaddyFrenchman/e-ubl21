// This file defines the Prometheus metrics exposed by the server (via the
// GET /metrics endpoint, registered in server.go) using
// github.com/prometheus/client_golang, the de facto standard Go metrics
// client. Metrics are updated by the shared request-observability
// middleware in logging.go, which records both structured logs and these
// metrics from a single response-status/duration observation per request.
package server

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	workerDeadLetterDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_dead_letter_depth",
		Help: "Current number of terminal failures retained in the dead-letter stream.",
	})
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

// metricsHandler exposes all registered metrics in the Prometheus text
// exposition format.
func metricsHandler() http.HandlerFunc {
	handler := promhttp.Handler()
	return func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r)
	}
}
