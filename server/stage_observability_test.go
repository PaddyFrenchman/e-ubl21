package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestPeppolStageMetricsAndStructuredLogUseBoundedDimensions(t *testing.T) {
	stage := string(queue.ValidationLayerPeppolBISBilling)
	outcome := string(queue.StageOutcomeOperationalFailure)
	reason := string(queue.StageReasonResourceLimit)
	completed := stageRequestsTotal.WithLabelValues(stage, outcome)
	runtimeErrors := stageRuntimeErrorsTotal.WithLabelValues(stage, reason)
	retries := stageRetriesTotal.WithLabelValues(stage, reason)
	duration := validationStageDuration.WithLabelValues(stage)
	queueWait := stageQueueWait.WithLabelValues(stage)
	publication := stageResultPublicationLatency.WithLabelValues(stage, outcome)
	completedBefore := prometheusCounterValue(t, completed)
	runtimeBefore := prometheusCounterValue(t, runtimeErrors)
	retriesBefore := prometheusCounterValue(t, retries)
	durationBefore := histogramSampleCount(t, duration)
	queueWaitBefore := histogramSampleCount(t, queueWait)
	publicationBefore := histogramSampleCount(t, publication)

	result := queue.StageResult{
		UploadID: "upload-peppol-observed", ValidationID: "validation-peppol-observed",
		RequestID: "request-peppol-observed", DocumentType: "Invoice",
		Plan: "peppol-bis-billing", PlanVersion: "1", Generation: 2,
		Stage: queue.ValidationLayerPeppolBISBilling, RulesetVersion: validationassets.Peppol().ValidationArtefactsVersion,
		Attempt: 1, Outcome: queue.StageOutcomeOperationalFailure, Reason: queue.StageReasonResourceLimit,
		Issues: []queue.ValidationIssue{{Kind: queue.ValidationIssueFailedAssert}}, DurationMS: 1250,
	}
	recordSplitStageExecution(result)
	recordSplitStagePublished(result, 3)
	recordStageRequestTiming(stage, .75, 1)
	observeStageResultPublication(result.Stage, result.Outcome, time.Now())
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logSplitStageResult(context.Background(), logger, result, false)

	if got := prometheusCounterValue(t, completed) - completedBefore; got != 1 {
		t.Fatalf("completed metric delta = %v, want 1", got)
	}
	if got := prometheusCounterValue(t, runtimeErrors) - runtimeBefore; got != 1 {
		t.Fatalf("runtime error metric delta = %v, want 1", got)
	}
	if got := prometheusCounterValue(t, retries) - retriesBefore; got != 1 {
		t.Fatalf("retry metric delta = %v, want 1", got)
	}
	if got := histogramSampleCount(t, duration) - durationBefore; got != 1 {
		t.Fatalf("duration metric samples = %d, want 1", got)
	}
	if got := histogramSampleCount(t, queueWait) - queueWaitBefore; got != 1 {
		t.Fatalf("queue-wait metric samples = %d, want 1", got)
	}
	if got := histogramSampleCount(t, publication) - publicationBefore; got != 1 {
		t.Fatalf("publication metric samples = %d, want 1", got)
	}
	logText := output.String()
	for _, field := range []string{
		`"upload_id":"upload-peppol-observed"`, `"document_type":"Invoice"`,
		`"validation_profile":"peppol-bis-billing"`, `"profile_version":"1"`,
		`"stage":"peppol-bis-billing"`, `"ruleset_version":"1.3.16"`,
		`"outcome":"operational-failure"`, `"reason":"resource-limit"`,
		`"issue_count":1`, `"duration_ms":1250`, `"bis_version":"3.0.21"`,
	} {
		if !strings.Contains(logText, field) {
			t.Errorf("Peppol structured log missing %s: %s", field, logText)
		}
	}
	if strings.Contains(logText, "invoice-body") {
		t.Fatal("Peppol structured log contains invoice content")
	}
}

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

func histogramSampleSum(t *testing.T, observer prometheus.Observer) float64 {
	t.Helper()
	metric, ok := observer.(prometheus.Metric)
	if !ok {
		t.Fatal("histogram observer does not expose a Prometheus metric")
	}
	value := &dto.Metric{}
	if err := metric.Write(value); err != nil {
		t.Fatal(err)
	}
	return value.GetHistogram().GetSampleSum()
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

func TestXSDQueueWaitStopsBeforeAuthorization(t *testing.T) {
	client, ctx := newXSDWorkerTestClient(t)
	request := startXSDWorkerTestRequest(t, ctx, client)
	data, err := request.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	created, err := time.Parse(time.RFC3339Nano, request.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	handlerTime := created.Add(500 * time.Millisecond)
	currentTime := handlerTime
	observer := stageQueueWait.WithLabelValues(string(request.Stage))
	before := histogramSampleSum(t, observer)
	worker := &xsdStageWorker{
		queue:    client,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		nakDelay: time.Millisecond,
		now:      func() time.Time { return currentTime },
		authorizeRequest: func(context.Context, queue.StageRequest) (queue.ValidationExecution, error) {
			currentTime = currentTime.Add(4 * time.Second)
			return queue.ValidationExecution{}, errors.New("delayed authorization failed")
		},
	}
	message := &workerStateMsg{
		data:     data,
		metadata: &jetstream.MsgMetadata{NumDelivered: 1},
	}
	if err := worker.handle(context.Background(), message); err == nil {
		t.Fatal("handle succeeded, want authorization error")
	}
	if got := histogramSampleSum(t, observer) - before; got < .499 || got > .501 {
		t.Fatalf("queue wait = %v seconds, want handler-entry age 0.5 seconds", got)
	}
}
