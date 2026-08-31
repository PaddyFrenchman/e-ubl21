package regression

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type failingEvidenceReader struct{}

func (failingEvidenceReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type failingRoundTripper struct{}

func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("transport failed")
}

func TestEvidenceCommandAndBoundedHelperBranches(t *testing.T) {
	content, truncated, err := runEvidenceCommand(context.Background(), 64, "sh", "-c", "printf version-1")
	if err != nil || truncated || string(content) != "version-1" {
		t.Fatalf("command output = %q, %t, %v", content, truncated, err)
	}
	if _, _, err := runEvidenceCommand(context.Background(), 10, "/definitely/missing/executable"); err == nil {
		t.Fatal("missing evidence executable succeeded")
	}

	var buffer boundedCommandBuffer
	buffer.limit = 4
	if n, err := buffer.Write([]byte("ab")); err != nil || n != 2 {
		t.Fatalf("bounded write = %d, %v", n, err)
	}
	if n, err := buffer.Write([]byte("cdef")); err != nil || n != 4 || !buffer.truncated || buffer.String() != "abcd" {
		t.Fatalf("truncating write = %d, %v, %t, %q", n, err, buffer.truncated, buffer.String())
	}
	if n, err := buffer.Write([]byte("ignored")); err != nil || n != 7 || buffer.String() != "abcd" {
		t.Fatalf("full write = %d, %v, %q", n, err, buffer.String())
	}

	if got := commandToolVersion(context.Background(), runEvidenceCommand, "sh", "-c", "printf '  v1  '"); got.Version != "v1" || got.Error != "" {
		t.Fatalf("tool version = %+v", got)
	}
	failingRunner := func(context.Context, int, string, ...string) ([]byte, bool, error) {
		return nil, false, errors.New("Bearer secret")
	}
	if got := commandToolVersion(context.Background(), failingRunner, "tool"); !strings.Contains(got.Error, "[REDACTED]") {
		t.Fatalf("failed tool version = %+v", got)
	}
	if got := toolVersion(" v2 ", nil); got.Version != "v2" {
		t.Fatalf("direct tool version = %+v", got)
	}
	if got := toolVersion("", errors.New("failed")); got.Error != "failed" {
		t.Fatalf("direct tool error = %+v", got)
	}
	if boundedError(nil) != "" {
		t.Fatal("nil bounded error was non-empty")
	}
	secret := errors.New("https://user:password@example.invalid Bearer token " + strings.Repeat("x", 2000))
	if got := boundedError(secret); len(got) != 1024 || strings.Contains(got, "password") || strings.Contains(got, "token") {
		t.Fatalf("bounded secret error = %q", got)
	}

	if got, limited, err := readBounded(strings.NewReader("abcdef"), 3); err != nil || !limited || string(got) != "abc" {
		t.Fatalf("bounded read = %q, %t, %v", got, limited, err)
	}
	if got, limited, err := readBounded(strings.NewReader("abc"), 3); err != nil || limited || string(got) != "abc" {
		t.Fatalf("exact read = %q, %t, %v", got, limited, err)
	}
	if _, _, err := readBounded(failingEvidenceReader{}, 3); err == nil {
		t.Fatal("reader failure was ignored")
	}
}

func TestRunnerInputHTTPAndNormalizationBranches(t *testing.T) {
	if _, err := RunCorpus(nil, RunnerConfig{}); err == nil {
		t.Fatal("nil runner context was accepted")
	}
	if _, err := RunCorpus(context.Background(), RunnerConfig{}); err == nil {
		t.Fatal("missing runner configuration was accepted")
	}
	if _, err := RunCorpus(context.Background(), RunnerConfig{BaseURL: "://bad", Token: "token", ManifestPath: "missing", OutputDir: "out"}); err == nil {
		t.Fatal("invalid runner URL was accepted")
	}
	if _, err := RunCorpus(context.Background(), RunnerConfig{BaseURL: "http://example.invalid", Token: "token", ManifestPath: "missing", OutputDir: "out"}); err == nil {
		t.Fatal("missing runner manifest was accepted")
	}
	manifest := writeRunnerManifest(t)
	if _, err := RunCorpus(context.Background(), RunnerConfig{BaseURL: "http://example.invalid", Token: "token", ManifestPath: manifest, OutputDir: t.TempDir(), Profile: "absent"}); err == nil {
		t.Fatal("empty profile selection was accepted")
	}

	request, err := http.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := doJSON(&http.Client{Transport: failingRoundTripper{}}, request, http.StatusOK, &map[string]any{}); err == nil {
		t.Fatal("HTTP transport failure was ignored")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad-status" {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()
	badStatus, _ := http.NewRequest(http.MethodGet, server.URL+"/bad-status", nil)
	if err := doJSON(server.Client(), badStatus, http.StatusOK, &map[string]any{}); err == nil {
		t.Fatal("unexpected HTTP status was accepted")
	}
	badJSON, _ := http.NewRequest(http.MethodGet, server.URL+"/bad-json", nil)
	if err := doJSON(server.Client(), badJSON, http.StatusOK, &map[string]any{}); err == nil {
		t.Fatal("malformed HTTP JSON was accepted")
	}

	final, stages := normalizeAPIResult(apiResponse{Status: "invalid", Stages: []apiStage{{
		Layer: "en16931", Valid: false, Issues: []apiIssue{
			{Kind: "failed-assert", Rule: "BR-2", Severity: "error"},
			{Kind: "failed-assert", Rule: "BR-1", Severity: "ERROR"},
			{Kind: "failed-assert", Rule: "BR-W", Severity: "warning"},
			{Kind: "report", Rule: "ignored", Severity: "error"},
		},
	}}}, nil)
	if final != "invalid" || len(stages) != 1 || len(stages[0].RuleIDs) != 2 || stages[0].RuleIDs[0] != "BR-1" {
		t.Fatalf("normalized invalid result = %q %+v", final, stages)
	}
	final, stages = normalizeAPIResult(apiResponse{Status: "processing_failed", Stages: []apiStage{{Layer: "ubl21-xsd", Valid: true}}},
		[]ExpectedStage{{Stage: "ubl21-xsd"}, {Stage: "en16931"}})
	if final != "operational-failure" || len(stages) != 2 || stages[1].Stage != "en16931" {
		t.Fatalf("normalized operational result = %q %+v", final, stages)
	}
	final, stages = normalizeAPIResult(apiResponse{Status: "acceptance_failed"}, nil)
	if final != "operational-failure" || len(stages) != 1 || stages[0].Stage != "ubl21-xsd" {
		t.Fatalf("normalized acceptance result = %q %+v", final, stages)
	}

	baseURL, _ := url.Parse(server.URL)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := pollFixture(canceled, server.Client(), baseURL, "token", "id", time.Millisecond); err == nil {
		t.Fatal("canceled poll succeeded")
	}
}

func TestEvidenceSerializationAndArtifactHelperBranches(t *testing.T) {
	content, err := marshalEvidenceJSON(map[string]string{"ok": "yes"})
	if err != nil || !strings.HasSuffix(string(content), "\n") {
		t.Fatalf("evidence JSON = %q, %v", content, err)
	}
	if _, err := marshalEvidenceJSON(make(chan int)); err == nil {
		t.Fatal("unsupported evidence JSON value succeeded")
	}
	if err := validEvidenceName("api.log"); err != nil {
		t.Fatal(err)
	}
	if err := validEvidenceName("../secret"); err == nil {
		t.Fatal("unsafe evidence name was accepted")
	}
	if got := sortedMapKeys(map[string]string{"z": "1", "a": "2"}); len(got) != 2 || got[0] != "a" || got[1] != "z" {
		t.Fatalf("sorted keys = %v", got)
	}
	entry := failedEvidence("source", errors.New("Bearer hidden"))
	if entry.Status != "failed" || !strings.Contains(entry.Error, "[REDACTED]") {
		t.Fatalf("failed evidence = %+v", entry)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")
	if err := writeFileAtomically(path, []byte("payload")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "payload" {
		t.Fatalf("artifact = %q, %v", got, err)
	}
	if err := writeFileAtomically(filepath.Join(dir, "missing", "artifact"), []byte("payload")); err == nil {
		t.Fatal("artifact in missing directory succeeded")
	}

	values := uniqueStrings([]string{"a", "a", "b", "b"})
	if len(values) != 2 || values[0] != "a" || values[1] != "b" {
		t.Fatalf("unique strings = %v", values)
	}
	if got := uniqueStrings(nil); got != nil {
		t.Fatalf("empty unique strings = %v", got)
	}
	if !containsAll([]string{"a", "b"}, nil) || containsAll([]string{"a"}, []string{"b"}) {
		t.Fatal("containsAll returned an inconsistent subset result")
	}
}

func TestKubernetesEvidenceFailureBranches(t *testing.T) {
	allFail := func(context.Context, int, string, ...string) ([]byte, bool, error) {
		return nil, false, errors.New("kubectl unavailable")
	}
	entries, failures := collectKubernetesEvidence(context.Background(), EvidenceConfig{OutputDir: t.TempDir()}, allFail)
	if len(entries) != 2 || len(failures) != 2 {
		t.Fatalf("command failure evidence = %+v, failures=%v", entries, failures)
	}

	malformedPods := func(_ context.Context, _ int, _ string, args ...string) ([]byte, bool, error) {
		if strings.Contains(strings.Join(args, " "), "get pods") {
			return []byte("{"), false, nil
		}
		return []byte(`{"items":[]}`), false, nil
	}
	entries, failures = collectKubernetesEvidence(context.Background(), EvidenceConfig{OutputDir: t.TempDir()}, malformedPods)
	if len(entries) != 2 || len(failures) != 1 {
		t.Fatalf("malformed pod evidence = %+v, failures=%v", entries, failures)
	}

	pods := `{"items":[{"metadata":{"name":"api","namespace":"test"},"spec":{"containers":[{"name":"api","image":"image"}]},"status":{"phase":"Running","containerStatuses":[{"name":"api","ready":false,"restartCount":1}]}}]}`
	logFail := func(_ context.Context, _ int, _ string, args ...string) ([]byte, bool, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "get pods"):
			return []byte(pods), false, nil
		case strings.Contains(joined, " logs "):
			return nil, false, errors.New("log unavailable")
		default:
			return []byte(`{"items":[]}`), true, nil
		}
	}
	entries, failures = collectKubernetesEvidence(context.Background(), EvidenceConfig{OutputDir: t.TempDir(), KubernetesNamespace: "test", KubernetesSelector: "app=e-ubl21"}, logFail)
	if len(entries) != 4 || len(failures) != 2 {
		t.Fatalf("log failure evidence = %+v, failures=%v", entries, failures)
	}
	if got := kubernetesScopeArgs("  "); len(got) != 1 || got[0] != "--all-namespaces" {
		t.Fatalf("all-namespace args = %v", got)
	}
	if got := kubernetesScopeArgs("test"); len(got) != 2 || got[1] != "test" {
		t.Fatalf("namespace args = %v", got)
	}
}
