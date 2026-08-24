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

	workerBufferedMessages = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_buffered_messages",
		Help: "Current number of pulled documents waiting in the bounded local worker buffer.",
	})

	workerPullBufferCapacity = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "e_ubl21_worker_pull_buffer_capacity",
		Help: "Configured maximum number of documents waiting in the local worker buffer.",
	})

	workerRedeliveries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "e_ubl21_worker_redeliveries_total",
		Help: "Total documents observed with a JetStream delivery count greater than one.",
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
