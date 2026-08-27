package regression

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunCorpusProducesDeterministicArtifacts(t *testing.T) {
	manifest := writeRunnerManifest(t)
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ubl/upload":
			if r.URL.Query().Get("profile") != "ubl21" || r.URL.Query().Get("documentType") != "Invoice" {
				t.Fatalf("upload query = %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"id":"0123456789abcdef0123456789abcdef","profile":"ubl21","receivedAt":"volatile"}`))
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
	if summary.Total != 1 || summary.Passed != 1 || summary.Failed != 0 {
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
	for _, name := range []string{"results.jsonl", "summary.json", "junit.xml"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
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
