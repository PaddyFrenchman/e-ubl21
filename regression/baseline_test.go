package regression

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidationSplitBaselineMatchesPinnedManifests(t *testing.T) {
	root := filepath.Join("..")
	metadataBytes, err := os.ReadFile(filepath.Join("baselines", "validation-split", "baseline.json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		SchemaVersion int    `json:"schemaVersion"`
		Architecture  string `json:"architecture"`
		SourceCommit  string `json:"sourceCommit"`
		Corpora       []struct {
			Name           string `json:"name"`
			Manifest       string `json:"manifest"`
			ManifestSHA256 string `json:"manifestSha256"`
			Results        string `json:"results"`
			ResultsSHA256  string `json:"resultsSha256"`
			Fixtures       int    `json:"fixtures"`
		} `json:"corpora"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.SchemaVersion != 1 || metadata.Architecture != "monolithic-worker" || len(metadata.SourceCommit) != 40 || len(metadata.Corpora) != 2 {
		t.Fatalf("invalid baseline metadata: %+v", metadata)
	}

	for _, corpus := range metadata.Corpora {
		t.Run(corpus.Name, func(t *testing.T) {
			manifestPath := filepath.Join(root, filepath.FromSlash(corpus.Manifest))
			resultsPath := filepath.Join(root, filepath.FromSlash(corpus.Results))
			assertFileSHA256(t, manifestPath, corpus.ManifestSHA256)
			assertFileSHA256(t, resultsPath, corpus.ResultsSHA256)

			manifestBytes, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateFixtureManifest(manifestBytes); err != nil {
				t.Fatal(err)
			}
			var manifest Manifest
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				t.Fatal(err)
			}
			results, err := readNormalizedResultSet(resultsPath)
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Fixtures) != corpus.Fixtures || len(results) != corpus.Fixtures {
				t.Fatalf("fixtures manifest=%d results=%d metadata=%d", len(manifest.Fixtures), len(results), corpus.Fixtures)
			}
			for _, fixture := range manifest.Fixtures {
				result, exists := results[fixture.ID]
				if !exists {
					t.Errorf("baseline missing fixture %s", fixture.ID)
					continue
				}
				if result.SHA256 != fixture.SHA256 || result.Profile != fixture.Profile || result.ExpectedFinal != fixture.ExpectedFinal ||
					!reflect.DeepEqual(result.ExpectedStages, fixture.ExpectedStages) || result.Result != "pass" {
					t.Errorf("baseline contract differs for fixture %s", fixture.ID)
				}
			}
		})
	}
}

func assertFileSHA256(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if actual := hex.EncodeToString(digest[:]); actual != expected {
		t.Fatalf("%s SHA-256 = %s, want %s", path, actual, expected)
	}
}
