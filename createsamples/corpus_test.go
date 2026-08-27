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
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/regression"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const testGeneratorVersion = "git:0000000000000000000000000000000000000000"

func TestCompleteUBLPositiveCorpusIsDeterministicManifestDrivenAndXSDValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	report, err := discoverInventory(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.MainSchemas != 65 || len(report.MainFiles) != 65 {
		t.Fatalf("main schema inventory = %d/%d, want 65/65", report.MainSchemas, len(report.MainFiles))
	}
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := generateUBLPositiveCorpus(report, index, xsdDir, 1, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateUBLPositiveCorpus(report, index, xsdDir, 1, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 65 {
		t.Fatalf("generated fixtures = %d, want 65", len(first.fixtures))
	}
	if err := regression.ValidateFixtureManifest(first.manifestJSON); err != nil {
		t.Fatalf("generated manifest rejected: %v", err)
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the UBL positive-corpus acceptance test")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}
	firstDir := filepath.Join(t.TempDir(), "first")
	secondDir := filepath.Join(t.TempDir(), "second")
	if err := writeGeneratedCorpus(firstDir, first); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedCorpus(secondDir, second); err != nil {
		t.Fatal(err)
	}
	firstFiles := snapshotGeneratedDirectory(t, firstDir)
	secondFiles := snapshotGeneratedDirectory(t, secondDir)
	if len(firstFiles) != 67 || len(secondFiles) != 67 {
		t.Fatalf("clean generation file counts = %d/%d, want 67 including manifest and ownership marker", len(firstFiles), len(secondFiles))
	}
	for path, firstBytes := range firstFiles {
		if !bytes.Equal(firstBytes, secondFiles[path]) {
			t.Fatalf("clean generation output differs at %s", path)
		}
	}

	var manifest fixtureManifest
	if err := json.Unmarshal(first.manifestJSON, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 65 {
		t.Fatalf("manifest fixtures = %d, want 65", len(manifest.Fixtures))
	}
	for index, fixture := range first.fixtures {
		if index > 0 && first.fixtures[index-1].entry.ID >= fixture.entry.ID {
			t.Fatalf("manifest order is not canonical at %q", fixture.entry.ID)
		}
		wantHash := fmt.Sprintf("%x", sha256.Sum256(fixture.content))
		if fixture.entry.SHA256 != wantHash {
			t.Errorf("fixture %s SHA-256 = %s, want %s", fixture.entry.ID, fixture.entry.SHA256, wantHash)
		}
	}
}

func TestUBLNegativeCorpusIsDeterministicManifestDrivenAndPredictablyXSDInvalid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the UBL negative-corpus acceptance test")
	}
	baseline := generatedCorpus{fixtures: []generatedFixture{{
		entry:   fixtureManifestEntry{ID: "ubl21.invoice.negative-baseline", ExpectedFinal: "valid"},
		content: []byte(minimalValidInvoice), schemaPath: filepath.Join(xsdDir, "maindoc", "UBL-Invoice-2.1.xsd"),
	}}}
	if err := validateGeneratedCorpus(xmllintPath, baseline); err != nil {
		t.Fatalf("negative mutation baseline is not XSD-valid: %v", err)
	}

	first, err := generateUBLNegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateUBLNegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 6 {
		t.Fatalf("negative fixtures = %d, want 6", len(first.fixtures))
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}
	wantMutations := []string{
		"invalid-datatype", "invalid-enumeration", "invalid-namespace",
		"malformed-xml", "missing-required-element", "wrong-element-order",
	}
	for index, fixture := range first.fixtures {
		if fixture.entry.Mutation == nil || fixture.entry.Mutation.ID != wantMutations[index] {
			t.Fatalf("fixture %d mutation = %+v, want %s", index, fixture.entry.Mutation, wantMutations[index])
		}
		if fixture.entry.Mutation.Description == "" {
			t.Errorf("fixture %s does not record why it must fail", fixture.entry.ID)
		}
		if fixture.entry.ExpectedFinal != "invalid" || len(fixture.entry.ExpectedStages) != 1 ||
			fixture.entry.ExpectedStages[0].Stage != "ubl21-xsd" ||
			fixture.entry.ExpectedStages[0].Outcome != "standards-invalid" {
			t.Errorf("fixture %s permits a later semantic stage: %+v", fixture.entry.ID, fixture.entry.ExpectedStages)
		}
	}
}

func TestEN16931PositiveCorpusIsDeterministicManifestDrivenAndXSDValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	first, err := generateEN16931PositiveCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateEN16931PositiveCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 10 {
		t.Fatalf("EN 16931 fixtures = %d, want 10", len(first.fixtures))
	}
	if err := regression.ValidateFixtureManifest(first.manifestJSON); err != nil {
		t.Fatalf("EN 16931 manifest rejected: %v", err)
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the EN 16931 positive-corpus acceptance test")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}

	generated, official, invoices, creditNotes := 0, 0, 0, 0
	profile := validationassets.EN16931()
	for _, fixture := range first.fixtures {
		if fixture.entry.Profile != "ubl21-en16931" || fixture.entry.ExpectedFinal != "valid" {
			t.Errorf("fixture %s has wrong profile/final expectation: %+v", fixture.entry.ID, fixture.entry)
		}
		if fixture.entry.Standards.EN16931 == nil || *fixture.entry.Standards.EN16931 != profile.Version ||
			fixture.entry.Rulesets == nil || fixture.entry.Rulesets.EN16931 == nil ||
			fixture.entry.Rulesets.EN16931.Version != profile.Version || fixture.entry.Rulesets.EN16931.SHA256 != profile.XSLTSHA256 {
			t.Errorf("fixture %s lacks pinned EN ruleset metadata", fixture.entry.ID)
		}
		if len(fixture.entry.ExpectedStages) != 2 || fixture.entry.ExpectedStages[0].Stage != "ubl21-xsd" ||
			fixture.entry.ExpectedStages[1].Stage != "en16931" || fixture.entry.ExpectedStages[1].Outcome != "passed" {
			t.Errorf("fixture %s has wrong stage contract: %+v", fixture.entry.ID, fixture.entry.ExpectedStages)
		}
		switch fixture.entry.Provenance.Kind {
		case "generated":
			generated++
		case "official-import":
			official++
			if fixture.entry.Provenance.SourceSHA256 != fixture.entry.SHA256 {
				t.Errorf("unmodified official fixture %s source/file hashes differ", fixture.entry.ID)
			}
		default:
			t.Errorf("fixture %s has unknown provenance %q", fixture.entry.ID, fixture.entry.Provenance.Kind)
		}
		switch fixture.entry.DocumentType {
		case "Invoice":
			invoices++
		case "CreditNote":
			creditNotes++
		default:
			t.Errorf("unexpected EN document type %q", fixture.entry.DocumentType)
		}
	}
	if generated != 8 || official != 2 || invoices != 9 || creditNotes != 1 {
		t.Fatalf("EN fixture coverage generated=%d official=%d invoices=%d creditNotes=%d", generated, official, invoices, creditNotes)
	}
}

func TestNegativeCorpusValidationRequiresTheCataloguedFailure(t *testing.T) {
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal(err)
	}
	schemaPath := filepath.Join("..", "XSD", "maindoc", "UBL-Invoice-2.1.xsd")
	accepted := generatedCorpus{fixtures: []generatedFixture{{
		entry:   fixtureManifestEntry{ID: "ubl21.invoice.accepted", ExpectedFinal: "invalid"},
		content: []byte(minimalValidInvoice), schemaPath: schemaPath, expectedDiagnostic: "must fail",
	}}}
	if err := validateGeneratedCorpus(xmllintPath, accepted); err == nil || !strings.Contains(err.Error(), "unexpectedly accepted") {
		t.Fatalf("accepted negative fixture error = %v", err)
	}
	wrongReason := generatedCorpus{fixtures: []generatedFixture{{
		entry:   fixtureManifestEntry{ID: "ubl21.invoice.wrong-reason", ExpectedFinal: "invalid"},
		content: []byte("<Invoice>"), schemaPath: schemaPath, expectedDiagnostic: "unrelated diagnostic",
	}}}
	if err := validateGeneratedCorpus(xmllintPath, wrongReason); err == nil || !strings.Contains(err.Error(), "without expected diagnostic") {
		t.Fatalf("wrong negative diagnostic error = %v", err)
	}
}

func TestTrackedUBLCorpusMatchesFreshGeneration(t *testing.T) {
	root := "samples"
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("tracked manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 71 {
		t.Fatalf("tracked manifest fixtures = %d, want 71", len(manifest.Fixtures))
	}
	generatorVersion := manifest.Fixtures[0].Provenance.GeneratorVersion
	for _, fixture := range manifest.Fixtures {
		if fixture.Provenance.GeneratorVersion != generatorVersion {
			t.Fatalf("tracked fixture %s generator version = %q, want %q", fixture.ID, fixture.Provenance.GeneratorVersion, generatorVersion)
		}
	}
	files := snapshotGeneratedDirectory(t, root)
	if len(files) != 73 {
		t.Fatalf("tracked corpus files = %d, want 73 including manifest and ownership marker", len(files))
	}
	for _, fixture := range manifest.Fixtures {
		content, ok := files[fixture.File]
		if !ok {
			t.Errorf("manifest fixture %s references missing file %s", fixture.ID, fixture.File)
			continue
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(content))
		if actual != fixture.SHA256 {
			t.Errorf("tracked fixture %s SHA-256 = %s, manifest has %s", fixture.ID, actual, fixture.SHA256)
		}
	}

	xsdDir := filepath.Join("..", "XSD")
	report, err := discoverInventory(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	positive, err := generateUBLPositiveCorpus(report, index, xsdDir, 1, generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := generateUBLNegativeCorpus(xsdDir, generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := combineGeneratedCorpora(positive, negative)
	if err != nil {
		t.Fatal(err)
	}
	freshDir := filepath.Join(t.TempDir(), "samples")
	if err := writeGeneratedCorpus(freshDir, fresh); err != nil {
		t.Fatal(err)
	}
	freshFiles := snapshotGeneratedDirectory(t, freshDir)
	if len(freshFiles) != len(files) {
		t.Fatalf("fresh/tracked corpus file counts = %d/%d", len(freshFiles), len(files))
	}
	for path, trackedBytes := range files {
		freshBytes, ok := freshFiles[path]
		if !ok {
			t.Errorf("fresh generation is missing tracked file %s", path)
			continue
		}
		if !bytes.Equal(freshBytes, trackedBytes) {
			t.Errorf("fresh generation differs from tracked corpus at %s", path)
		}
	}
}

func TestTrackedEN16931CorpusMatchesFreshGeneration(t *testing.T) {
	root := "samples-en16931"
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("tracked EN 16931 manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 10 {
		t.Fatalf("tracked EN 16931 manifest fixtures = %d, want 10", len(manifest.Fixtures))
	}
	generatorVersion := ""
	for _, fixture := range manifest.Fixtures {
		if fixture.Provenance.Kind != "generated" {
			continue
		}
		if generatorVersion == "" {
			generatorVersion = fixture.Provenance.GeneratorVersion
		}
		if fixture.Provenance.GeneratorVersion != generatorVersion {
			t.Fatalf("tracked fixture %s generator version = %q, want %q", fixture.ID, fixture.Provenance.GeneratorVersion, generatorVersion)
		}
	}
	if !immutableGeneratorVersion.MatchString(generatorVersion) {
		t.Fatalf("tracked EN 16931 generator version = %q", generatorVersion)
	}

	files := snapshotGeneratedDirectory(t, root)
	if len(files) != 12 {
		t.Fatalf("tracked EN 16931 corpus files = %d, want 12 including manifest and ownership marker", len(files))
	}
	for _, fixture := range manifest.Fixtures {
		content, ok := files[fixture.File]
		if !ok {
			t.Errorf("manifest fixture %s references missing file %s", fixture.ID, fixture.File)
			continue
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(content))
		if actual != fixture.SHA256 {
			t.Errorf("tracked fixture %s SHA-256 = %s, manifest has %s", fixture.ID, actual, fixture.SHA256)
		}
	}

	fresh, err := generateEN16931PositiveCorpus(filepath.Join("..", "XSD"), generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	freshDir := filepath.Join(t.TempDir(), "samples-en16931")
	if err := writeGeneratedCorpus(freshDir, fresh); err != nil {
		t.Fatal(err)
	}
	freshFiles := snapshotGeneratedDirectory(t, freshDir)
	if len(freshFiles) != len(files) {
		t.Fatalf("fresh/tracked EN 16931 corpus file counts = %d/%d", len(freshFiles), len(files))
	}
	for path, trackedBytes := range files {
		freshBytes, ok := freshFiles[path]
		if !ok {
			t.Errorf("fresh EN 16931 generation is missing tracked file %s", path)
			continue
		}
		if !bytes.Equal(freshBytes, trackedBytes) {
			t.Errorf("fresh EN 16931 generation differs from tracked corpus at %s", path)
		}
	}
}

func snapshotGeneratedDirectory(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestCorpusDeterminismDetectsChangedBytes(t *testing.T) {
	first := generatedCorpus{
		fixtures:     []generatedFixture{{entry: fixtureManifestEntry{ID: "ubl21.demo.minimal", File: "Demo/Demo-sample-01.xml"}, content: []byte("first")}},
		manifestJSON: []byte("same"),
	}
	second := first
	second.fixtures = append([]generatedFixture(nil), first.fixtures...)
	second.fixtures[0].content = []byte("second")
	if err := verifyCorpusDeterminism(first, second); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("changed generated bytes error = %v", err)
	}
	second = first
	second.manifestJSON = []byte("different")
	if err := verifyCorpusDeterminism(first, second); err == nil || !strings.Contains(err.Error(), "manifest differs") {
		t.Fatalf("changed manifest error = %v", err)
	}
}

func TestGeneratorVersionMustBeImmutable(t *testing.T) {
	for _, invalid := range []string{"main", "git:0123456", "git:ABCDEF0000000000000000000000000000000000"} {
		if _, err := resolveGeneratorVersion(invalid); err == nil {
			t.Errorf("resolveGeneratorVersion(%q) succeeded", invalid)
		}
	}
	if got, err := resolveGeneratorVersion(testGeneratorVersion); err != nil || got != testGeneratorVersion {
		t.Fatalf("resolveGeneratorVersion(valid) = %q, %v", got, err)
	}
	sha256Version := "git:" + strings.Repeat("a", 64)
	if got, err := resolveGeneratorVersion(sha256Version); err != nil || got != sha256Version {
		t.Fatalf("resolveGeneratorVersion(SHA-256) = %q, %v", got, err)
	}
}

func TestAutomaticGeneratorVersionRejectsDirtyRelevantInputs(t *testing.T) {
	repo := t.TempDir()
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(repo, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = repo
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
		}
	}
	runGit("init", "--quiet")
	runGit("config", "user.email", "tests@example.invalid")
	runGit("config", "user.name", "Corpus Tests")
	write("createsamples/generator.go", "package main\n")
	write("regression/manifest.go", "package regression\n")
	write("regression/schemas/fixture-manifest.schema.json", "{}\n")
	write("XSD/maindoc/UBL-Demo-2.1.xsd", "<schema/>\n")
	runGit("add", ".")
	runGit("commit", "--quiet", "-m", "initial")
	if version, err := resolveGeneratorVersionInRepo(repo); err != nil || !immutableGeneratorVersion.MatchString(version) {
		t.Fatalf("clean automatic generator version = %q, %v", version, err)
	}

	write("createsamples/generator.go", "package main\n// dirty\n")
	if _, err := resolveGeneratorVersionInRepo(repo); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty generator source error = %v", err)
	}
	if version, err := resolveGeneratorVersion(testGeneratorVersion); err != nil || version != testGeneratorVersion {
		t.Fatalf("explicit dirty-worktree override = %q, %v", version, err)
	}
	runGit("add", ".")
	runGit("commit", "--quiet", "-m", "generator change")
	write("XSD/maindoc/UBL-Demo-2.1.xsd", "<schema dirty='true'/>\n")
	if _, err := resolveGeneratorVersionInRepo(repo); err == nil || !strings.Contains(err.Error(), "XSD/maindoc") {
		t.Fatalf("dirty XSD input error = %v", err)
	}
	runGit("add", ".")
	runGit("commit", "--quiet", "-m", "XSD change")
	write("regression/manifest.go", "package regression\n// dirty\n")
	if _, err := resolveGeneratorVersionInRepo(repo); err == nil || !strings.Contains(err.Error(), "regression/manifest.go") {
		t.Fatalf("dirty manifest contract error = %v", err)
	}
}

func TestGeneratedCorpusRejectsUnsafeOutputDirectories(t *testing.T) {
	unowned := filepath.Join(t.TempDir(), "unowned")
	if err := os.Mkdir(unowned, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(unowned, symlink); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{"", ".", "..", string(filepath.Separator), filepath.Join("..", "createsamples"), filepath.Join("..", "XSD"), unowned, symlink} {
		if err := writeGeneratedCorpus(output, generatedCorpus{}); err == nil {
			t.Errorf("writeGeneratedCorpus(%q) accepted unsafe output", output)
		}
	}
}

func TestGeneratedCorpusReplacesOnlyMarkedOutput(t *testing.T) {
	output := filepath.Join(t.TempDir(), "samples")
	if err := writeGeneratedCorpus(output, generatedCorpus{manifestJSON: []byte("first")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "stale.xml"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeGeneratedCorpus(output, generatedCorpus{manifestJSON: []byte("second")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "stale.xml")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale generated file still exists: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil || string(manifest) != "second" {
		t.Fatalf("replacement manifest = %q, %v", manifest, err)
	}
}
