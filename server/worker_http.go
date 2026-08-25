// This file implements the worker's small unauthenticated operational HTTP
// surface (GET /metrics, GET /healthz), separate from the API's routes in
// server.go since the worker does not serve the UBL upload/OpenAPI/Swagger
// endpoints at all.
package server

import (
	"net/http"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// workerMetricsMux builds the worker's metrics/health HTTP handler. Both
// routes are intentionally unauthenticated (unlike the API's routes): the
// worker has no bearer-token validator configured and this address is meant
// to be reachable only from an internal monitoring network.
func workerMetricsMux(q *queue.Client) http.Handler {
	mux := http.NewServeMux()
	metrics := promhttp.Handler()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if depth, err := q.DeadLetterDepth(r.Context()); err == nil {
			workerDeadLetterDepth.Set(float64(depth))
		}
		metrics.ServeHTTP(w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if q.NC.Status() != nats.CONNECTED {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"nats":   q.NC.Status().String(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}
