package regression

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
)

func TestCollectEvidenceCapturesBoundedRedactedDiagnostics(t *testing.T) {
	manifestPath := writeRunnerManifest(t)
	logPath := filepath.Join(t.TempDir(), "worker.log")
	logContent := "starting\nAuthorization: Bearer secret-token\n{\"authorization\":\"Bearer json-secret\",\"message\":\"safe\"}\n<ubl:Invoice xmlns:ubl=\"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2\">\n<cbc:ID>private-invoice-id</cbc:ID>\n</ubl:Invoice>\n<CreditNote/>\n\\u003cInvoice/\\u003e\nfinished\n"
	if err := os.WriteFile(logPath, []byte(logContent), 0o600); err != nil {
		t.Fatal(err)
	}
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer metrics-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte("e_ubl21_test_total 1\n"))
	}))
	defer metricsServer.Close()
	output := t.TempDir()
	normalizedResult := []byte("normalized-correctness-result\n")
	if err := os.WriteFile(filepath.Join(output, "results.jsonl"), normalizedResult, 0o644); err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	ended := started.Add(time.Minute)
	report, err := CollectEvidence(t.Context(), EvidenceConfig{
		OutputDir: output, ManifestPath: manifestPath, Token: "metrics-token",
		LogFiles: map[string]string{"worker": logPath}, MetricsEndpoints: map[string]string{"api": metricsServer.URL},
		CollectNATS: true, StartedAt: started, EndedAt: ended,
		commandRunner: deterministicEvidenceCommand,
		natsSnapshotCollector: func(context.Context, EvidenceConfig) ([]byte, string, error) {
			return []byte("{\"schemaVersion\":1}\n"), "2.14.5", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Captures) < 6 {
		t.Fatalf("captures = %+v", report.Captures)
	}
	if actual, err := os.ReadFile(filepath.Join(output, "results.jsonl")); err != nil || !bytes.Equal(actual, normalizedResult) {
		t.Fatalf("normalized result changed: %q err=%v", actual, err)
	}
	redacted := readTestFile(t, filepath.Join(output, "logs", "worker.log"))
	for _, secret := range []string{"secret-token", "json-secret", "private-invoice-id", "<ubl:Invoice", "<CreditNote", `\u003cInvoice`} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted log contains %q: %s", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "Bearer [REDACTED]") || !strings.Contains(redacted, `"message":"safe"`) || !strings.Contains(redacted, "[REDACTED UBL XML]") || !strings.Contains(redacted, "finished") {
		t.Fatalf("redaction markers missing: %s", redacted)
	}
	if metrics := readTestFile(t, filepath.Join(output, "metrics", "api.prom")); metrics != "e_ubl21_test_total 1\n" {
		t.Fatalf("metrics = %q", metrics)
	}
	var environment Environment
	if err := json.Unmarshal([]byte(readTestFile(t, filepath.Join(output, "environment.json"))), &environment); err != nil {
		t.Fatal(err)
	}
	if environment.GitCommitSHA != strings.Repeat("a", 40) || environment.GitDirty || environment.NATSServer == nil || environment.NATSServer.Version != "2.14.5" {
		t.Fatalf("environment = %+v", environment)
	}
	if len(environment.UBL.SHA256) != 64 || len(environment.GeneratorVersions) != 1 {
		t.Fatalf("incomplete reproducibility metadata: %+v", environment)
	}
	if environment.StartedAt != started.Format(time.RFC3339Nano) || environment.EndedAt != ended.Format(time.RFC3339Nano) {
		t.Fatalf("environment times = %s/%s", environment.StartedAt, environment.EndedAt)
	}
	for _, path := range []string{"collection.json", "manifest-used.json", filepath.Join("nats", "state.json")} {
		if _, err := os.Stat(filepath.Join(output, path)); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
}

func TestCollectEvidenceRecordsRequestedSourceFailuresAfterOtherCaptures(t *testing.T) {
	manifestPath := writeRunnerManifest(t)
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer metricsServer.Close()
	output := t.TempDir()
	report, err := CollectEvidence(t.Context(), EvidenceConfig{
		OutputDir: output, ManifestPath: manifestPath,
		MetricsEndpoints: map[string]string{"api": metricsServer.URL}, commandRunner: deterministicEvidenceCommand,
	})
	if err == nil {
		t.Fatal("collector accepted a failed requested metrics source")
	}
	failed := false
	for _, entry := range report.Captures {
		if entry.Name == "metrics/api" && entry.Status == "failed" {
			failed = true
		}
	}
	if !failed {
		t.Fatalf("failure missing from report: %+v", report)
	}
	for _, path := range []string{"collection.json", "environment.json", "manifest-used.json"} {
		if _, statErr := os.Stat(filepath.Join(output, path)); statErr != nil {
			t.Errorf("partial evidence missing %s: %v", path, statErr)
		}
	}
}

func TestCollectKubernetesEvidenceSanitizesPodStateAndLogs(t *testing.T) {
	manifestPath := writeRunnerManifest(t)
	pods := `{"items":[{"metadata":{"name":"api-1","namespace":"regression"},"spec":{"containers":[{"name":"api","image":"e-ubl21:test","env":[{"name":"SECRET","value":"must-not-leak"}]}]},"status":{"phase":"Running","containerStatuses":[{"name":"api","ready":true,"restartCount":2,"imageID":"sha256:abc"}]}}]}`
	runner := func(_ context.Context, _ int, executable string, args ...string) ([]byte, bool, error) {
		joined := executable + " " + strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "get events"):
			return []byte(`{"items":[]}`), false, nil
		case strings.Contains(joined, "get pods"):
			return []byte(pods), false, nil
		case strings.Contains(joined, " logs "):
			return []byte("<Invoice>private</Invoice>\nworker failed BR-CO-15\n"), false, nil
		case strings.Contains(joined, "git rev-parse"):
			return []byte(strings.Repeat("b", 40)), false, nil
		case strings.Contains(joined, "git status"):
			return nil, false, nil
		default:
			return []byte("test-version"), false, nil
		}
	}
	output := t.TempDir()
	_, err := CollectEvidence(t.Context(), EvidenceConfig{
		OutputDir: output, ManifestPath: manifestPath, CollectKubernetes: true,
		KubernetesNamespace: "regression", KubernetesSelector: "app=e-ubl21", commandRunner: runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	podSnapshot := readTestFile(t, filepath.Join(output, "kubernetes", "pods.json"))
	if strings.Contains(podSnapshot, "must-not-leak") || !strings.Contains(podSnapshot, "sha256:abc") {
		t.Fatalf("unsafe or incomplete pod snapshot: %s", podSnapshot)
	}
	log := readTestFile(t, filepath.Join(output, "logs", "regression.api-1.api.log"))
	if strings.Contains(log, "private") || !strings.Contains(log, "BR-CO-15") {
		t.Fatalf("kubernetes log redaction = %q", log)
	}
	if _, err := os.Stat(filepath.Join(output, "logs", "regression.api-1.api.previous.log")); err != nil {
		t.Fatalf("missing previous container log: %v", err)
	}
}

func TestCollectNATSSnapshotContainsStateButNoMessages(t *testing.T) {
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.DefaultNamespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.EnsureInfra(t.Context()); err != nil {
		client.Close()
		t.Fatal(err)
	}
	if _, err := client.DocumentsConsumer(t.Context(), "evidence-worker", ""); err != nil {
		client.Close()
		t.Fatal(err)
	}
	client.Close()

	content, version, err := collectNATSSnapshot(t.Context(), EvidenceConfig{NATSURL: server.URL, NATSOptions: server.Options})
	if err != nil {
		t.Fatal(err)
	}
	if version == "" || bytes.Contains(content, []byte("<Invoice")) || !bytes.Contains(content, []byte("evidence-worker")) {
		t.Fatalf("version=%q snapshot=%s", version, content)
	}
}

func TestEvidenceBuffersAreBounded(t *testing.T) {
	content, truncated, err := readBounded(strings.NewReader(strings.Repeat("x", 11)), 10)
	if err != nil || !truncated || string(content) != strings.Repeat("x", 10) {
		t.Fatalf("readBounded content=%q truncated=%t err=%v", content, truncated, err)
	}
	var buffer boundedCommandBuffer
	buffer.limit = 5
	if written, err := buffer.Write([]byte("123456789")); err != nil || written != 9 {
		t.Fatalf("write=%d err=%v", written, err)
	}
	if buffer.String() != "12345" || !buffer.truncated {
		t.Fatalf("command buffer=%q truncated=%t", buffer.String(), buffer.truncated)
	}
}

func TestCollectEvidenceRejectsReversedRunTimes(t *testing.T) {
	ended := time.Now().UTC()
	_, err := CollectEvidence(t.Context(), EvidenceConfig{
		OutputDir: t.TempDir(), ManifestPath: writeRunnerManifest(t),
		StartedAt: ended.Add(time.Minute), EndedAt: ended,
	})
	if err == nil || !strings.Contains(err.Error(), "precedes") {
		t.Fatalf("error = %v", err)
	}
}

func deterministicEvidenceCommand(_ context.Context, _ int, executable string, args ...string) ([]byte, bool, error) {
	command := executable + " " + strings.Join(args, " ")
	switch {
	case strings.Contains(command, "git rev-parse"):
		return []byte(strings.Repeat("a", 40) + "\n"), false, nil
	case strings.Contains(command, "git status"):
		return nil, false, nil
	case strings.Contains(command, "java"):
		return []byte("openjdk test"), false, nil
	case strings.Contains(command, "xmllint"):
		return []byte("libxml test"), false, nil
	default:
		return nil, false, errors.New("unexpected command")
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
