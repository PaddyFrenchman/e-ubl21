package server

import (
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func histogramSampleCount(t *testing.T, observer prometheus.Observer) uint64 {
	t.Helper()
	metric, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not expose a Prometheus metric")
	}
	value := &dto.Metric{}
	if err := metric.Write(value); err != nil {
		t.Fatal(err)
	}
	return value.GetHistogram().GetSampleCount()
}

func TestStageQueueWaitIsFirstDeliveryOnlyWhileRequestAgeTracksRedelivery(t *testing.T) {
	stage := string(queue.ValidationLayerUBL21XSD)
	wait := stageQueueWait.WithLabelValues(stage)
	age := stageRequestAge.WithLabelValues(stage)
	waitBefore, ageBefore := histogramSampleCount(t, wait), histogramSampleCount(t, age)

	recordStageRequestTiming(stage, .8, 1)
	recordStageRequestTiming(stage, 47, 2)

	if got := histogramSampleCount(t, wait) - waitBefore; got != 1 {
		t.Fatalf("queue-wait samples = %d, want one first-delivery sample", got)
	}
	if got := histogramSampleCount(t, age) - ageBefore; got != 2 {
		t.Fatalf("request-age samples = %d, want both delivery samples", got)
	}
}

func TestStageHistogramBucketsCoverValidatorAndBacklogHorizons(t *testing.T) {
	if last := stageServiceDurationBuckets[len(stageServiceDurationBuckets)-1]; last < 60 {
		t.Fatalf("service histogram ceiling = %v seconds, want at least 60", last)
	}
	if last := stageQueueAgeBuckets[len(stageQueueAgeBuckets)-1]; last < 3600 {
		t.Fatalf("queue/request-age histogram ceiling = %v seconds, want at least 3600", last)
	}
	for name, buckets := range map[string][]float64{"service": stageServiceDurationBuckets, "queue-age": stageQueueAgeBuckets} {
		for index := 1; index < len(buckets); index++ {
			if buckets[index] <= buckets[index-1] {
				t.Fatalf("%s buckets are not strictly increasing: %v", name, buckets)
			}
		}
	}
}
