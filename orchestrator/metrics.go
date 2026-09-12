package orchestrator

import (
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	orchestratorTransitionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "e_ubl21_orchestrator_transition_duration_seconds",
		Help:    "Duration of orchestrator state-machine transitions by bounded transition and stage.",
		Buckets: prometheus.DefBuckets,
	}, []string{"transition", "stage"})
	orchestratorResultPublicationLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "e_ubl21_orchestrator_result_publication_latency_seconds",
		Help:    "Duration of canonical validation-result publication attempts.",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	for _, transition := range []string{"handle_stage_result", "dispatch_stage_request"} {
		for _, stage := range []queue.ValidationLayer{queue.ValidationLayerUBL21XSD, queue.ValidationLayerEN16931, queue.ValidationLayerPeppolBISBilling, queue.ValidationLayerPeppolEnvelope} {
			orchestratorTransitionDuration.WithLabelValues(transition, string(stage))
		}
	}
}

func observeOrchestratorTransition(transition string, stage queue.ValidationLayer, started time.Time) {
	orchestratorTransitionDuration.WithLabelValues(transition, string(stage)).Observe(time.Since(started).Seconds())
}
