package regression

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestPeppolReleaseBaselineMatchesPinnedCorpusAndReference(t *testing.T) {
	root := filepath.Join("..")
	metadataPath := filepath.Join("baselines", "peppol", "baseline.json")
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		SchemaVersion int    `json:"schemaVersion"`
		Architecture  string `json:"architecture"`
		Release       struct {
			BISVersion                 string `json:"bisVersion"`
			ValidationArtefactsVersion string `json:"validationArtefactsVersion"`
			SourceCommit               string `json:"sourceCommit"`
			SourceSchematronSHA256     string `json:"sourceSchematronSHA256"`
			CompiledXSLTSHA256         string `json:"compiledXSLTSHA256"`
		} `json:"release"`
		Manifest struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		} `json:"manifest"`
		Reference struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Cases  int    `json:"cases"`
		} `json:"reference"`
		Results struct {
			Path     string `json:"path"`
			SHA256   string `json:"sha256"`
			Fixtures int    `json:"fixtures"`
		} `json:"results"`
	}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	profile := validationassets.Peppol()
	reference, err := validationassets.PeppolReference()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SchemaVersion != 1 || metadata.Architecture != "split-stage" ||
		metadata.Release.BISVersion != profile.Version ||
		metadata.Release.ValidationArtefactsVersion != profile.ValidationArtefactsVersion ||
		metadata.Release.SourceCommit != profile.SourceCommit ||
		metadata.Release.SourceSchematronSHA256 != reference.Release.SourceSchematronSHA256 ||
		metadata.Release.CompiledXSLTSHA256 != profile.XSLTSHA256 {
		t.Fatalf("baseline release metadata does not match pinned Peppol assets: %+v", metadata.Release)
	}
	if metadata.Reference.Cases != len(reference.Cases) || metadata.Results.Fixtures == 0 {
		t.Fatalf("baseline evidence counts are incomplete: reference=%d results=%d", metadata.Reference.Cases, metadata.Results.Fixtures)
	}

	manifestPath := filepath.Join(root, filepath.FromSlash(metadata.Manifest.Path))
	referencePath := filepath.Join(root, filepath.FromSlash(metadata.Reference.Path))
	resultsPath := filepath.Join(root, filepath.FromSlash(metadata.Results.Path))
	assertPeppolBaselineDigest(t, manifestPath, metadata.Manifest.SHA256)
	assertPeppolBaselineDigest(t, referencePath, metadata.Reference.SHA256)
	assertPeppolBaselineDigest(t, resultsPath, metadata.Results.SHA256)

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
	if len(manifest.Fixtures) != metadata.Results.Fixtures || len(results) != metadata.Results.Fixtures {
		t.Fatalf("baseline fixture counts manifest=%d results=%d metadata=%d", len(manifest.Fixtures), len(results), metadata.Results.Fixtures)
	}
	fixturesByDigest := make(map[string]Fixture, len(manifest.Fixtures))
	for _, fixture := range manifest.Fixtures {
		fixturesByDigest[fixture.SHA256] = fixture
		result, ok := results[fixture.ID]
		if !ok {
			t.Errorf("baseline missing fixture %s", fixture.ID)
			continue
		}
		if result.SHA256 != fixture.SHA256 || result.Profile != fixture.Profile || result.ExpectedFinal != fixture.ExpectedFinal ||
			!reflect.DeepEqual(result.ExpectedStages, fixture.ExpectedStages) || result.Result != "pass" {
			t.Errorf("baseline contract differs for fixture %s", fixture.ID)
		}
	}

	// The reviewed reference contract is independent from the corpus manifest
	// and runtime. Bind every reference case to the accepted normalized result
	// by immutable fixture digest and require exact official Peppol rule
	// equality. Product applicability failures are frozen in the normalized
	// baseline too, but are deliberately excluded from the official reference
	// comparison because EUBL21-* identifiers are not Schematron rule IDs.
	for _, testCase := range reference.Cases {
		fixture, ok := fixturesByDigest[testCase.SHA256]
		if !ok {
			t.Errorf("reference case %s has no corpus fixture with digest %s", testCase.ID, testCase.SHA256)
			continue
		}
		result := results[fixture.ID]
		var stage ActualStage
		for _, candidate := range result.ActualStages {
			if candidate.Stage == "peppol-bis-billing" {
				stage = candidate
				break
			}
		}
		wantOutcome := testCase.Outcome
		wantRules := make([]string, 0, len(testCase.Issues))
		for _, issue := range testCase.Issues {
			wantRules = append(wantRules, issue.RuleID)
		}
		sort.Strings(wantRules)
		officialRules := make([]string, 0, len(stage.RuleIDs))
		for _, ruleID := range stage.RuleIDs {
			if !strings.HasPrefix(ruleID, "EUBL21-PEPPOL-") {
				officialRules = append(officialRules, ruleID)
			}
		}
		if stage.Outcome != wantOutcome || !slices.Equal(officialRules, wantRules) {
			t.Errorf("reference case %s maps to %s Peppol outcome/rules %q/%v, want %q/%v", testCase.ID, fixture.ID, stage.Outcome, officialRules, wantOutcome, wantRules)
		}
	}
}

func assertPeppolBaselineDigest(t *testing.T, path, expected string) {
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
