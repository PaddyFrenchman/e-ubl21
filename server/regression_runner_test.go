package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/regression"
)

func TestNormalizedRegressionRunnerUsesAuthenticatedDurablePublicPath(t *testing.T) {
	runtime, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureXMLLint(); err != nil {
		t.Skip(err)
	}
	manifestPath := buildRegressionRunnerAcceptanceManifest(t)
	cfg := testConfig(t, func(cfg *Config) {
		cfg.RateLimitRPS = 1000
		cfg.RateLimitBurst = 1000
	})
	stopWorker := startComplianceWorker(t, cfg, func(worker *WorkerConfig) {
		configureRealSaxonWorker(worker, runtime)
	})
	defer stopWorker()
	application, err := NewApplication(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()
	httpServer := httptest.NewServer(application.Handler)
	defer httpServer.Close()
	token := signedJWT(t, cfg.OAuthIssuer, cfg.OAuthAudience, testSecret, time.Hour)

	output := t.TempDir()
	summary, err := regression.RunCorpus(t.Context(), regression.RunnerConfig{
		BaseURL: httpServer.URL, Token: token, ManifestPath: manifestPath, OutputDir: output,
		PollInterval: 20 * time.Millisecond, FixtureTimeout: 30 * time.Second,
	})
	if err != nil {
		results, _ := os.ReadFile(filepath.Join(output, "results.jsonl"))
		t.Fatalf("%v\n%s", err, results)
	}
	if summary.Total != 2 || summary.Passed != 2 || summary.Failed != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	for _, name := range []string{"results.jsonl", "summary.json", "junit.xml"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func buildRegressionRunnerAcceptanceManifest(t *testing.T) string {
	t.Helper()
	destination := t.TempDir()
	type manifestDocument struct {
		SchemaVersion int               `json:"schemaVersion"`
		Fixtures      []json.RawMessage `json:"fixtures"`
	}
	var selected []json.RawMessage
	for _, source := range []struct {
		manifest string
		wantID   string
	}{
		{filepath.Join("..", "createsamples", "samples", "manifest.json"), "ubl21.invoice.minimal"},
		{filepath.Join("..", "createsamples", "samples-en16931", "manifest.json"), "en16931.invoice.monetary-total-reconciliation"},
	} {
		content, err := os.ReadFile(source.manifest)
		if err != nil {
			t.Fatal(err)
		}
		var document manifestDocument
		if err := json.Unmarshal(content, &document); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, raw := range document.Fixtures {
			var fixture struct {
				ID   string `json:"id"`
				File string `json:"file"`
			}
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			if fixture.ID != source.wantID {
				continue
			}
			fixtureSource := filepath.Join(filepath.Dir(source.manifest), filepath.FromSlash(fixture.File))
			fixtureDestination := filepath.Join(destination, filepath.FromSlash(fixture.File))
			body, err := os.ReadFile(fixtureSource)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(fixtureDestination), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fixtureDestination, body, 0o644); err != nil {
				t.Fatal(err)
			}
			selected = append(selected, raw)
			found = true
			break
		}
		if !found {
			t.Fatalf("fixture %s not found", source.wantID)
		}
	}
	encoded, err := json.MarshalIndent(manifestDocument{SchemaVersion: 1, Fixtures: selected}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(destination, "manifest.json")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
