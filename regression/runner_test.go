package regression

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRunCorpusProducesDeterministicArtifacts(t *testing.T) {
	manifest := writeRunnerManifest(t)
	polls := 0
	uploads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ubl/upload":
			if r.URL.Query().Get("profile") != "ubl21" || r.URL.Query().Get("documentType") != "Invoice" {
				t.Fatalf("upload query = %s", r.URL.RawQuery)
			}
			uploads++
			id := "11111111111111111111111111111111"
			if uploads > 1 {
				id = "22222222222222222222222222222222"
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"profile":"ubl21","receivedAt":"volatile"}`, id)))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/ubl/"):
			polls++
			w.Header().Set("Content-Type", "application/json")
			if polls%2 == 1 {
				_, _ = w.Write([]byte(`{"status":"pending","profile":"ubl21","receivedAt":"volatile"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"valid","profile":"ubl21","validatedAt":"volatile","validationStages":[{"layer":"ubl21-xsd","valid":true,"validatedAt":"volatile","durationMs":999}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	first := filepath.Join(t.TempDir(), "first")
	second := filepath.Join(t.TempDir(), "second")
	config := RunnerConfig{BaseURL: server.URL, Token: "test-token", ManifestPath: manifest, OutputDir: first, PollInterval: time.Millisecond, FixtureTimeout: time.Second}
	summary, err := RunCorpus(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Total != 1 || summary.Submitted != 1 || summary.TerminallyEvaluated != 1 || len(summary.MissingFixtureIDs) != 0 || summary.Passed != 1 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	config.OutputDir = second
	if _, err := RunCorpus(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"results.jsonl", "summary.json", "junit.xml"} {
		firstBytes, err := os.ReadFile(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		secondBytes, err := os.ReadFile(filepath.Join(second, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(firstBytes) != string(secondBytes) {
			t.Errorf("%s is nondeterministic", name)
		}
	}
	firstMap := readRunMap(t, filepath.Join(first, "run-map.jsonl"))
	secondMap := readRunMap(t, filepath.Join(second, "run-map.jsonl"))
	if len(firstMap) != 1 || firstMap[0].FixtureID != "ubl21.invoice.runner" || firstMap[0].UploadID != "11111111111111111111111111111111" || firstMap[0].DocumentType != "Invoice" || firstMap[0].Profile != "ubl21" || firstMap[0].TerminalStatus != "valid" {
		t.Fatalf("first run map = %+v", firstMap)
	}
	if len(secondMap) != 1 || secondMap[0].UploadID != "22222222222222222222222222222222" {
		t.Fatalf("second run map = %+v", secondMap)
	}
}

func TestSelectFixturesSupportsExactFixtureIDsAndRejectsUnknownIDs(t *testing.T) {
	fixtures := []Fixture{
		{ID: "ubl21.invoice.one", Profile: "ubl21"},
		{ID: "ubl21.order.one", Profile: "ubl21"},
		{ID: "en16931.invoice.one", Profile: "ubl21-en16931"},
	}
	selected, err := selectFixtures(fixtures, "all", []string{"ubl21.order.one", "ubl21.invoice.one"})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 2 || selected[0].ID != "ubl21.invoice.one" || selected[1].ID != "ubl21.order.one" {
		t.Fatalf("selected fixtures = %+v", selected)
	}
	if _, err := selectFixtures(fixtures, "ubl21", []string{"en16931.invoice.one"}); err == nil || !strings.Contains(err.Error(), "en16931.invoice.one") {
		t.Fatalf("profile-filtered fixture selection error = %v", err)
	}
	if _, err := selectFixtures(fixtures, "all", []string{"missing.fixture"}); err == nil || !strings.Contains(err.Error(), "missing.fixture") {
		t.Fatalf("unknown fixture selection error = %v", err)
	}
}

func TestSummarizeReportsUnsubmittedAndUnprocessedFixtureIDs(t *testing.T) {
	results := []NormalizedResult{
		{FixtureID: "ubl21.invoice.submitted", Profile: "ubl21", Result: "pass"},
		{FixtureID: "ubl21.invoice.missing", Profile: "ubl21", Result: "fail"},
	}
	summary := summarize(results, []RunMapEntry{{FixtureID: "ubl21.invoice.submitted", UploadID: "upload", TerminalStatus: "valid"}, {FixtureID: "ubl21.invoice.missing", TerminalStatus: "operational-failure"}})
	if summary.Total != 2 || summary.Submitted != 1 || summary.TerminallyEvaluated != 1 || !reflect.DeepEqual(summary.MissingFixtureIDs, []string{"ubl21.invoice.missing"}) {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestRunCorpusWritesFailuresAndReturnsError(t *testing.T) {
	manifest := writeRunnerManifest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"0123456789abcdef0123456789abcdef","profile":"ubl21"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"invalid","profile":"ubl21","validationStages":[{"layer":"ubl21-xsd","valid":false,"issues":[{"kind":"xsd-report","severity":"error"}]}]}`))
	}))
	defer server.Close()
	output := t.TempDir()
	summary, err := RunCorpus(context.Background(), RunnerConfig{BaseURL: server.URL, Token: "token", ManifestPath: manifest, OutputDir: output, PollInterval: time.Millisecond, FixtureTimeout: time.Second})
	if err == nil || summary.Failed != 1 {
		t.Fatalf("summary=%+v error=%v", summary, err)
	}
	if !strings.Contains(err.Error(), "ubl21.invoice.runner") {
		t.Fatalf("error does not name failed fixture: %v", err)
	}
	for _, name := range []string{"results.jsonl", "run-map.jsonl", "summary.json", "junit.xml"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestRunCorpusRetainsUploadIDWhenPollingFails(t *testing.T) {
	manifest := writeRunnerManifest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"33333333333333333333333333333333","profile":"ubl21"}`))
			return
		}
		http.Error(w, "worker unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	output := t.TempDir()
	_, err := RunCorpus(t.Context(), RunnerConfig{BaseURL: server.URL, Token: "token", ManifestPath: manifest, OutputDir: output, PollInterval: time.Millisecond, FixtureTimeout: time.Second})
	if err == nil {
		t.Fatal("runner accepted polling failure")
	}
	runMap := readRunMap(t, filepath.Join(output, "run-map.jsonl"))
	if len(runMap) != 1 || runMap[0].UploadID != "33333333333333333333333333333333" || runMap[0].TerminalStatus != "operational-failure" {
		t.Fatalf("run map = %+v", runMap)
	}
}

func TestPollFixtureWaitsForCleanupReconciliation(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		polls++
		if polls == 1 {
			_, _ = w.Write([]byte(`{"status":"cleanup_pending","profile":"ubl21"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"acceptance_failed","profile":"ubl21"}`))
	}))
	defer server.Close()

	baseURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, err := pollFixture(t.Context(), server.Client(), baseURL, "token", "0123456789abcdef0123456789abcdef", time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "acceptance_failed" || polls != 2 {
		t.Fatalf("status=%q polls=%d, want reconciled acceptance_failed after two polls", response.Status, polls)
	}
}

func TestCompareResultRequiresExpectedRulesButAllowsRuleCascades(t *testing.T) {
	base := NormalizedResult{
		ExpectedFinal: "invalid", ActualFinal: "invalid",
		ExpectedStages: []ExpectedStage{{Stage: "en16931", Outcome: "standards-invalid", RuleIDs: []string{"BR-CO-15"}}},
		ActualStages:   []ActualStage{{Stage: "en16931", Outcome: "standards-invalid", RuleIDs: []string{"BR-CO-15", "BR-CO-16"}}},
	}
	if got := compareResult(base); got != "pass" {
		t.Fatalf("rule cascade result = %q", got)
	}
	base.ActualStages[0].RuleIDs = []string{"BR-CO-16"}
	if got := compareResult(base); got != "fail" {
		t.Fatalf("missing expected rule result = %q", got)
	}
}

func writeRunnerManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	content := []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`)
	digest := sha256.Sum256(content)
	manifest := map[string]any{
		"schemaVersion": 1,
		"fixtures": []any{map[string]any{
			"id": "ubl21.invoice.runner", "documentType": "Invoice", "profile": "ubl21", "file": "invoice.xml",
			"sha256":        fmtHash(digest[:]),
			"standards":     map[string]any{"ubl": "2.1", "en16931": nil, "peppol": nil},
			"provenance":    map[string]any{"kind": "generated", "generator": "runner-test", "generatorVersion": "git:0123456789abcdef0123456789abcdef01234567"},
			"expectedFinal": "valid", "expectedStages": []any{map[string]any{"stage": "ubl21-xsd", "outcome": "passed"}},
			"tags": []string{"runner"},
		}},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invoice.xml"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fmtHash(value []byte) string {
	const digits = "0123456789abcdef"
	encoded := make([]byte, len(value)*2)
	for index, b := range value {
		encoded[index*2] = digits[b>>4]
		encoded[index*2+1] = digits[b&15]
	}
	return string(encoded)
}

func readRunMap(t *testing.T, path string) []RunMapEntry {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entries []RunMapEntry
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		var entry RunMapEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}
