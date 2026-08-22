// This file implements request observability: a single middleware that
// captures each request's response status and duration once, then both logs
// it (via the standard library's structured log/slog package) and records it
// as Prometheus metrics (see metrics.go). route is a low-cardinality label
// (the registered mux pattern, not the raw URL, which could contain
// unbounded path parameters like upload IDs) used for metrics only; logs
// include the full request path.
package server

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps an http.ResponseWriter to capture the status code
// that was written, defaulting to 200 if WriteHeader is never called
// explicitly (matching net/http's own default).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// observabilityMiddleware wraps next, recording structured logs and
// Prometheus metrics for every request. route should be the low-cardinality
// mux pattern the handler was registered under (e.g. "/api/v1/ubl/upload"),
// not r.URL.Path, to keep metric label cardinality bounded.
func observabilityMiddleware(logger *slog.Logger, route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		duration := time.Since(start)

		logger.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", duration.Milliseconds(),
			"remote_addr", clientIP(r),
		)
		status := statusLabel(rec.status)
		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(duration.Seconds())
	}
}

func statusLabel(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500:
		return "5xx"
	default:
		return "unknown"
	}
}
