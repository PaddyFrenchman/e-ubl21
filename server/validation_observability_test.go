package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func prometheusCounterValue(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	metric := &dto.Metric{}
	if err := counter.Write(metric); err != nil {
		t.Fatal(err)
	}
	return metric.GetCounter().GetValue()
}

func TestValidationStageMetricsAndStructuredLogs(t *testing.T) {
	metricValue := func(stage, outcome string) float64 {
		return prometheusCounterValue(t, validationStageTotal.WithLabelValues(stage, outcome))
	}
	xsdValidBefore := metricValue(validationMetricStageXSD, "valid")
	xsdInvalidBefore := metricValue(validationMetricStageXSD, "invalid")
	xsdErrorBefore := metricValue(validationMetricStageXSD, "error")
	enInvalidBefore := metricValue(validationMetricStageEN16931, "invalid")
	enErrorBefore := metricValue(validationMetricStageEN16931, "error")
	fatalBefore := prometheusCounterValue(t, en16931IssuesTotal.WithLabelValues("fatal"))
	otherBefore := prometheusCounterValue(t, en16931IssuesTotal.WithLabelValues("other"))
	svrlBefore := prometheusCounterValue(t, en16931RuntimeFailuresTotal.WithLabelValues("svrl"))

	const secretBody = `<Invoice><Note>TOP-SECRET-INVOICE-BODY</Note></Invoice>`
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	newPipeline := func(validator EN16931Validator) *documentValidationPipeline {
		pipeline := newDocumentValidationPipeline(&memoryValidationContentSource{content: []byte(secretBody)}, validator, time.Second)
		pipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return nil }
		pipeline.logger = logger
		return pipeline
	}

	invalidPipeline := newPipeline(en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{
			Layer: queue.ValidationLayerEN16931, Version: "profile-test", Valid: false,
			Issues: []queue.ValidationIssue{
				{Kind: queue.ValidationIssueFailedAssert, Severity: "fatal"},
				{Kind: queue.ValidationIssueSuccessfulReport, Severity: "attacker-controlled-severity"},
			},
			ValidatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}, nil
	}))
	if result, err := invalidPipeline.Validate(context.Background(), "upload-observed", "Invoice", "invoice.xsd"); err != nil || result.Status != queue.StatusInvalid {
		t.Fatalf("invalid result/error = %+v / %v", result, err)
	}

	runtimePipeline := newPipeline(en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
		return queue.ValidationStageResult{}, errEN16931RuntimeSVRL
	}))
	if _, err := runtimePipeline.Validate(context.Background(), "upload-runtime", "Invoice", "invoice.xsd"); !errors.Is(err, errEN16931RuntimeSVRL) {
		t.Fatalf("runtime error = %v", err)
	}

	xsdPipeline := newPipeline(nil)
	xsdPipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error { return errors.New("xmllint crashed") }
	if _, err := xsdPipeline.Validate(context.Background(), "upload-xsd", "Invoice", "invoice.xsd"); err == nil {
		t.Fatal("XSD operational failure returned nil")
	}
	xsdInvalidPipeline := newPipeline(nil)
	xsdInvalidPipeline.xsdValidate = func(context.Context, string, io.Reader, time.Duration) error {
		return errors.Join(errSchemaStandardsInvalid, errors.New("document does not match schema"))
	}
	if result, err := xsdInvalidPipeline.Validate(context.Background(), "upload-xsd-invalid", "Invoice", "invoice.xsd"); err != nil || result.Status != queue.StatusInvalid {
		t.Fatalf("XSD invalid result/error = %+v / %v", result, err)
	}

	if got := metricValue(validationMetricStageXSD, "valid") - xsdValidBefore; got != 2 {
		t.Fatalf("XSD valid metric delta = %v, want 2", got)
	}
	if got := metricValue(validationMetricStageXSD, "error") - xsdErrorBefore; got != 1 {
		t.Fatalf("XSD error metric delta = %v, want 1", got)
	}
	if got := metricValue(validationMetricStageXSD, "invalid") - xsdInvalidBefore; got != 1 {
		t.Fatalf("XSD invalid metric delta = %v, want 1", got)
	}
	if got := metricValue(validationMetricStageEN16931, "invalid") - enInvalidBefore; got != 1 {
		t.Fatalf("EN invalid metric delta = %v, want 1", got)
	}
	if got := metricValue(validationMetricStageEN16931, "error") - enErrorBefore; got != 1 {
		t.Fatalf("EN error metric delta = %v, want 1", got)
	}
	if got := prometheusCounterValue(t, en16931IssuesTotal.WithLabelValues("fatal")) - fatalBefore; got != 1 {
		t.Fatalf("fatal issue metric delta = %v, want 1", got)
	}
	if got := prometheusCounterValue(t, en16931IssuesTotal.WithLabelValues("other")) - otherBefore; got != 1 {
		t.Fatalf("other issue metric delta = %v, want 1", got)
	}
	if got := prometheusCounterValue(t, en16931RuntimeFailuresTotal.WithLabelValues("svrl")) - svrlBefore; got != 1 {
		t.Fatalf("SVRL failure metric delta = %v, want 1", got)
	}

	logText := logs.String()
	for _, field := range []string{
		`"upload_id":"upload-observed"`, `"document_type":"Invoice"`,
		`"stage":"en16931"`, `"profile_version":"profile-test"`,
		`"result":"invalid"`, `"issue_count":2`, `"duration_ms":`,
		`"upload_id":"upload-xsd"`, `"stage":"ubl21_xsd"`, `"result":"error"`,
		`"upload_id":"upload-xsd-invalid"`, `"result":"invalid"`,
		`"upload_id":"upload-runtime"`, `"runtime_failure_reason":"svrl"`,
	} {
		if !strings.Contains(logText, field) {
			t.Errorf("structured stage logs missing %s:\n%s", field, logText)
		}
	}
	if strings.Contains(logText, "TOP-SECRET-INVOICE-BODY") {
		t.Fatal("structured stage logs contained invoice body")
	}

	metrics := httptest.NewRecorder()
	metricsHandler().ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	for _, metric := range []string{
		"e_ubl21_validation_stage_total",
		"e_ubl21_validation_stage_duration_seconds_count",
		"e_ubl21_en16931_issues_total",
		"e_ubl21_en16931_runtime_failures_total",
	} {
		if !strings.Contains(metrics.Body.String(), metric) {
			t.Errorf("metrics output missing %s", metric)
		}
	}
}

func TestValidationMetricLabelsAreBounded(t *testing.T) {
	for input, want := range map[string]string{
		"fatal": "fatal", " WARN ": "warning", "information": "info",
		"BR-CO-10": "other", "attacker-controlled": "other", "": "other",
	} {
		if got := normalizeEN16931MetricSeverity(input); got != want {
			t.Errorf("normalize severity %q = %q, want %q", input, got, want)
		}
	}
	if got := boundedValidationMetricStage(queue.ValidationLayer("attacker-controlled")); got != "other" {
		t.Fatalf("unrecognized stage label = %q", got)
	}
	if got := boundedValidationMetricOutcome("attacker-controlled"); got != "error" {
		t.Fatalf("unrecognized outcome label = %q", got)
	}
}

func TestEN16931RuntimeFailureReasonsAreBounded(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{context.DeadlineExceeded, "timeout"},
		{context.Canceled, "canceled"},
		{errEN16931RuntimeAsset, "asset"},
		{errEN16931RuntimeSaxon, "saxon"},
		{errEN16931RuntimeSVRL, "svrl"},
		{errEN16931SVRLOutputLimit, "svrl"},
		{errEN16931RuntimeInput, "input"},
		{errValidationContentStream, "input"},
		{errors.New("attacker-controlled"), "other"},
	}
	for _, test := range tests {
		if got := en16931RuntimeFailureReason(test.err); got != test.want {
			t.Errorf("reason for %v = %q, want %q", test.err, got, test.want)
		}
	}
}
