package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
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

const (
	corpusOwnershipMarkerName    = ".e-ubl21-generated-corpus"
	corpusOwnershipMarkerContent = "e-ubl21 createsamples generated corpus v1\n"
)

var generatorSourcePathspecs = []string{
	":(glob)createsamples/**/*.go",
	":(exclude,glob)createsamples/**/*_test.go",
	"regression/manifest.go",
	"regression/schemas/fixture-manifest.schema.json",
	":(glob)XSD/**/*.xsd",
}

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
	Mutation       *fixtureMutation       `json:"mutation,omitempty"`
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

type fixtureMutation struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type fixtureExpectedStage struct {
	Stage   string   `json:"stage"`
	Outcome string   `json:"outcome"`
	RuleIDs []string `json:"ruleIds,omitempty"`
}

type generatedFixture struct {
	entry              fixtureManifestEntry
	content            []byte
	schemaPath         string
	expectedDiagnostic string
}

type generatedCorpus struct {
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
	return resolveGeneratorVersionInRepo(strings.TrimSpace(string(rootOutput)))
}

func resolveGeneratorVersionInRepo(root string) (string, error) {
	statusArgs := append([]string{"status", "--porcelain", "--untracked-files=all", "--"}, generatorSourcePathspecs...)
	statusCommand := exec.Command("git", statusArgs...)
	statusCommand.Dir = root
	statusOutput, err := statusCommand.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("check generator inputs for local changes: %w: %s", err, strings.TrimSpace(string(statusOutput)))
	}
	if dirty := strings.TrimSpace(string(statusOutput)); dirty != "" {
		return "", fmt.Errorf("refuse automatic generator provenance with dirty generator, manifest-contract, or XSD inputs; commit them or deliberately use --generator-version to override:\n%s", dirty)
	}

	logArgs := append([]string{"log", "-1", "--format=%H", "--"}, generatorSourcePathspecs...)
	command := exec.Command("git", logArgs...)
	command.Dir = root
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

func generateUBLPositiveCorpus(report schemaInventory, index schemaIndex, xsdDir string, count int, generatorVersion string) (generatedCorpus, error) {
	fixtures := make([]generatedFixture, 0, len(report.MainFiles)*count)
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		for sample := 1; sample <= count; sample++ {
			content, err := generateCompleteSample(index, root, xsdDir, schema.File)
			if err != nil {
				return generatedCorpus{}, fmt.Errorf("generate %s sample %d: %w", root, sample, err)
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
	return buildGeneratedCorpus(fixtures)
}

func buildGeneratedCorpus(fixtures []generatedFixture) (generatedCorpus, error) {
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].entry.ID < fixtures[j].entry.ID })
	entries := make([]fixtureManifestEntry, len(fixtures))
	for index := range fixtures {
		entries[index] = fixtures[index].entry
	}
	manifestJSON, err := json.MarshalIndent(fixtureManifest{SchemaVersion: 1, Fixtures: entries}, "", "  ")
	if err != nil {
		return generatedCorpus{}, fmt.Errorf("marshal fixture manifest: %w", err)
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := regression.ValidateFixtureManifest(manifestJSON); err != nil {
		return generatedCorpus{}, fmt.Errorf("validate generated fixture manifest: %w", err)
	}
	return generatedCorpus{fixtures: fixtures, manifestJSON: manifestJSON}, nil
}

func combineGeneratedCorpora(corpora ...generatedCorpus) (generatedCorpus, error) {
	var fixtures []generatedFixture
	for _, corpus := range corpora {
		fixtures = append(fixtures, corpus.fixtures...)
	}
	return buildGeneratedCorpus(fixtures)
}

func verifyCorpusDeterminism(first, second generatedCorpus) error {
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

func validateGeneratedCorpus(xmllintPath string, corpus generatedCorpus) error {
	for _, fixture := range corpus.fixtures {
		command := exec.Command(xmllintPath, "--nonet", "--noout", "--schema", fixture.schemaPath, "-")
		command.Stdin = bytes.NewReader(fixture.content)
		output, err := command.CombinedOutput()
		diagnostics := strings.TrimSpace(string(output))
		if fixture.entry.ExpectedFinal == "valid" {
			if err != nil {
				return fmt.Errorf("xmllint validate %s against %s: %w: %s", fixture.entry.ID, fixture.schemaPath, err, diagnostics)
			}
			continue
		}
		if err == nil {
			return fmt.Errorf("xmllint unexpectedly accepted negative fixture %s against %s", fixture.entry.ID, fixture.schemaPath)
		}
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			return fmt.Errorf("run xmllint for negative fixture %s: %w", fixture.entry.ID, err)
		}
		if fixture.expectedDiagnostic == "" || !strings.Contains(diagnostics, fixture.expectedDiagnostic) {
			return fmt.Errorf("negative fixture %s failed without expected diagnostic %q: %s", fixture.entry.ID, fixture.expectedDiagnostic, diagnostics)
		}
	}
	return nil
}

func writeGeneratedCorpus(outputDir string, corpus generatedCorpus) error {
	if err := validateCorpusOutputDirectory(outputDir); err != nil {
		return err
	}
	absolute, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve generated corpus output: %w", err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create generated corpus parent %s: %w", parent, err)
	}
	staging, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".staging-*")
	if err != nil {
		return fmt.Errorf("create generated corpus staging directory: %w", err)
	}
	stagingExists := true
	defer func() {
		if stagingExists {
			_ = os.RemoveAll(staging)
		}
	}()
	if err := writeCorpusFiles(staging, corpus); err != nil {
		return err
	}
	if err := validateCorpusOutputDirectory(absolute); err != nil {
		return err
	}

	_, statErr := os.Lstat(absolute)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := os.Rename(staging, absolute); err != nil {
			return fmt.Errorf("install generated corpus at %s: %w", absolute, err)
		}
		stagingExists = false
	} else if statErr != nil {
		return fmt.Errorf("inspect generated corpus output %s: %w", absolute, statErr)
	} else {
		backup, err := os.MkdirTemp(parent, "."+filepath.Base(absolute)+".backup-*")
		if err != nil {
			return fmt.Errorf("reserve generated corpus backup path: %w", err)
		}
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("prepare generated corpus backup path: %w", err)
		}
		if err := os.Rename(absolute, backup); err != nil {
			return fmt.Errorf("move existing generated corpus to backup: %w", err)
		}
		if err := os.Rename(staging, absolute); err != nil {
			if restoreErr := os.Rename(backup, absolute); restoreErr != nil {
				return fmt.Errorf("install generated corpus: %w (also failed to restore previous corpus: %v)", err, restoreErr)
			}
			return fmt.Errorf("install generated corpus: %w", err)
		}
		stagingExists = false
		if err := os.RemoveAll(backup); err != nil {
			return fmt.Errorf("remove replaced generated corpus backup %s: %w", backup, err)
		}
	}
	for _, fixture := range corpus.fixtures {
		fmt.Println(filepath.Join(outputDir, filepath.FromSlash(fixture.entry.File)))
	}
	return nil
}

func writeCorpusFiles(outputDir string, corpus generatedCorpus) error {
	if err := os.WriteFile(filepath.Join(outputDir, corpusOwnershipMarkerName), []byte(corpusOwnershipMarkerContent), 0o644); err != nil {
		return fmt.Errorf("write generated corpus ownership marker: %w", err)
	}
	for _, fixture := range corpus.fixtures {
		path := filepath.Join(outputDir, filepath.FromSlash(fixture.entry.File))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create sample dir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, fixture.content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
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
	info, err := os.Lstat(absolute)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect generated corpus output %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("generated corpus output %s must be a directory and not a symlink", absolute)
	}
	marker, err := os.ReadFile(filepath.Join(absolute, corpusOwnershipMarkerName))
	if err != nil || string(marker) != corpusOwnershipMarkerContent {
		return fmt.Errorf("refuse to replace %s because it is not recognizably generator-owned (missing or invalid %s)", absolute, corpusOwnershipMarkerName)
	}
	return nil
}
