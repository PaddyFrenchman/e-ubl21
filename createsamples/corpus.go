package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/regression"
)

var immutableGeneratorVersion = regexp.MustCompile(`^git:(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

type fixtureManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Fixtures      []fixtureManifestEntry `json:"fixtures"`
}

type fixtureManifestEntry struct {
	ID             string                 `json:"id"`
	DocumentType   string                 `json:"documentType"`
	Profile        string                 `json:"profile"`
	File           string                 `json:"file"`
	SHA256         string                 `json:"sha256"`
	Standards      fixtureStandards       `json:"standards"`
	Provenance     generatedProvenance    `json:"provenance"`
	ExpectedFinal  string                 `json:"expectedFinal"`
	ExpectedStages []fixtureExpectedStage `json:"expectedStages"`
	Tags           []string               `json:"tags"`
}

type fixtureStandards struct {
	UBL     string  `json:"ubl"`
	EN16931 *string `json:"en16931"`
	Peppol  *string `json:"peppol"`
}

type generatedProvenance struct {
	Kind             string `json:"kind"`
	Generator        string `json:"generator"`
	GeneratorVersion string `json:"generatorVersion"`
}

type fixtureExpectedStage struct {
	Stage   string   `json:"stage"`
	Outcome string   `json:"outcome"`
	RuleIDs []string `json:"ruleIds,omitempty"`
}

type generatedFixture struct {
	entry      fixtureManifestEntry
	content    []byte
	schemaPath string
}

type generatedPositiveCorpus struct {
	fixtures     []generatedFixture
	manifestJSON []byte
}

func resolveGeneratorVersion(explicit string) (string, error) {
	if explicit != "" {
		if !immutableGeneratorVersion.MatchString(explicit) {
			return "", fmt.Errorf("generator version must be git:<40-or-64-lowercase-hex-commit>")
		}
		return explicit, nil
	}
	rootCommand := exec.Command("git", "rev-parse", "--show-toplevel")
	rootOutput, err := rootCommand.Output()
	if err != nil {
		return "", fmt.Errorf("locate generator Git checkout (use --generator-version to provide it explicitly): %w", err)
	}
	command := exec.Command("git", "log", "-1", "--format=%H", "--",
		":(glob)createsamples/**/*.go",
		":(exclude,glob)createsamples/**/*_test.go",
		"regression/manifest.go",
		"regression/schemas/fixture-manifest.schema.json",
	)
	command.Dir = strings.TrimSpace(string(rootOutput))
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve generator implementation Git commit (use --generator-version to provide it explicitly): %w", err)
	}
	version := "git:" + strings.TrimSpace(string(output))
	if !immutableGeneratorVersion.MatchString(version) {
		return "", fmt.Errorf("resolved invalid generator Git commit %q", version)
	}
	return version, nil
}

func generateUBLPositiveCorpus(report schemaInventory, index schemaIndex, xsdDir string, count int, generatorVersion string) (generatedPositiveCorpus, error) {
	fixtures := make([]generatedFixture, 0, len(report.MainFiles)*count)
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		for sample := 1; sample <= count; sample++ {
			content, err := generateCompleteSample(index, root, xsdDir, schema.File)
			if err != nil {
				return generatedPositiveCorpus{}, fmt.Errorf("generate %s sample %d: %w", root, sample, err)
			}
			fileName := fmt.Sprintf("%s-sample-%02d.xml", root, sample)
			relativePath := filepath.ToSlash(filepath.Join(root, fileName))
			fixtureID := "ubl21." + strings.ToLower(root) + ".minimal"
			if sample > 1 {
				fixtureID = fmt.Sprintf("ubl21.%s.sample-%02d", strings.ToLower(root), sample)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
			fixtures = append(fixtures, generatedFixture{
				entry: fixtureManifestEntry{
					ID: fixtureID, DocumentType: root, Profile: "ubl21", File: relativePath, SHA256: digest,
					Standards:      fixtureStandards{UBL: "2.1"},
					Provenance:     generatedProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
					ExpectedFinal:  "valid",
					ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags:           []string{"positive", "ubl21"},
				},
				content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File),
			})
		}
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].entry.ID < fixtures[j].entry.ID })
	entries := make([]fixtureManifestEntry, len(fixtures))
	for index := range fixtures {
		entries[index] = fixtures[index].entry
	}
	manifestJSON, err := json.MarshalIndent(fixtureManifest{SchemaVersion: 1, Fixtures: entries}, "", "  ")
	if err != nil {
		return generatedPositiveCorpus{}, fmt.Errorf("marshal fixture manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := regression.ValidateFixtureManifest(manifestJSON); err != nil {
		return generatedPositiveCorpus{}, fmt.Errorf("validate generated fixture manifest: %w", err)
	}
	return generatedPositiveCorpus{fixtures: fixtures, manifestJSON: manifestJSON}, nil
}

func verifyCorpusDeterminism(first, second generatedPositiveCorpus) error {
	if !bytes.Equal(first.manifestJSON, second.manifestJSON) {
		return fmt.Errorf("generated manifest differs between consecutive clean generations")
	}
	if len(first.fixtures) != len(second.fixtures) {
		return fmt.Errorf("generated fixture count differs between consecutive clean generations: %d != %d", len(first.fixtures), len(second.fixtures))
	}
	for index := range first.fixtures {
		if first.fixtures[index].entry.File != second.fixtures[index].entry.File || !bytes.Equal(first.fixtures[index].content, second.fixtures[index].content) {
			return fmt.Errorf("generated fixture %q differs between consecutive clean generations", first.fixtures[index].entry.ID)
		}
	}
	return nil
}

func validateGeneratedCorpus(xmllintPath string, corpus generatedPositiveCorpus) error {
	for _, fixture := range corpus.fixtures {
		command := exec.Command(xmllintPath, "--nonet", "--noout", "--schema", fixture.schemaPath, "-")
		command.Stdin = bytes.NewReader(fixture.content)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("xmllint validate %s against %s: %w: %s", fixture.entry.ID, fixture.schemaPath, err, strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func writeGeneratedCorpus(outputDir string, corpus generatedPositiveCorpus) error {
	if err := validateCorpusOutputDirectory(outputDir); err != nil {
		return err
	}
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("clear existing output %s: %w", outputDir, err)
	}
	for _, fixture := range corpus.fixtures {
		path := filepath.Join(outputDir, filepath.FromSlash(fixture.entry.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create sample dir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, fixture.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		fmt.Println(path)
	}
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create manifest directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, corpus.manifestJSON, 0o644); err != nil {
		return fmt.Errorf("write fixture manifest: %w", err)
	}
	return nil
}

func validateCorpusOutputDirectory(outputDir string) error {
	if outputDir == "" || filepath.Clean(outputDir) == "." {
		return fmt.Errorf("generated corpus output must be a dedicated non-empty directory")
	}
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve generated corpus output: %w", err)
	}
	volumeRoot := filepath.VolumeName(absolute) + string(filepath.Separator)
	if absolute == volumeRoot {
		return fmt.Errorf("generated corpus output must not be a filesystem root")
	}
	return nil
}
