// This file implements the worker's small unauthenticated operational HTTP
// surface (GET /metrics, GET /healthz), separate from the API's routes in
// server.go since the worker does not serve the UBL upload/OpenAPI/Swagger
// endpoints at all.
package server

import (
	"net/http"

	appdb "github.com/PaddyFrenchman/e-ubl21/database"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// workerMetricsMux builds the worker's metrics/health HTTP handler. Both
// routes are intentionally unauthenticated (unlike the API's routes): the
// worker has no bearer-token validator configured and this address is meant
// to be reachable only from an internal monitoring network.
func workerMetricsMux(q *queue.Client, database *appdb.Client) http.Handler {
	mux := http.NewServeMux()
	metrics := promhttp.Handler()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if depth, err := q.DeadLetterDepth(r.Context()); err == nil {
			workerDeadLetterDepth.Set(float64(depth))
		}
		for _, stage := range []queue.ValidationLayer{queue.ValidationLayerUBL21XSD, queue.ValidationLayerEN16931, queue.ValidationLayerPeppolBISBilling, queue.ValidationLayerPeppolEnvelope} {
			if depth, err := q.StageDeadLetterDepth(r.Context(), stage); err == nil {
				stageDLQDepth.WithLabelValues(string(stage)).Set(float64(depth))
			}
			if depth, err := q.StageRequestQueueDepth(r.Context(), stage); err == nil {
				stageQueueDepth.WithLabelValues(string(stage)).Set(float64(depth))
			}
		}
		metrics.ServeHTTP(w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if q.NC.Status() != nats.CONNECTED {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"nats":   q.NC.Status().String(),
			})
			return
		}
		if database != nil {
			if err := database.Probe(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "unavailable"})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}
