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
	"reflect"
	"strings"
	"testing"

	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
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

func TestPeppolPositiveCorpusIsDeterministicManifestDrivenAndXSDValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	first, err := generatePeppolPositiveCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generatePeppolPositiveCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 14 {
		t.Fatalf("Peppol fixtures = %d, want 14", len(first.fixtures))
	}
	if err := regression.ValidateFixtureManifest(first.manifestJSON); err != nil {
		t.Fatalf("Peppol manifest rejected: %v", err)
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the Peppol positive-corpus acceptance test")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}

	enProfile := validationassets.EN16931()
	peppolProfile := validationassets.Peppol()
	profiles := map[queue.PeppolProcessProfile]int{}
	documentTypes := map[string]int{}
	provenanceKinds := map[string]int{}
	coverage := map[string]bool{}
	for _, fixture := range first.fixtures {
		entry := fixture.entry
		if entry.Profile != "peppol-bis-billing" || entry.ExpectedFinal != "valid" {
			t.Errorf("fixture %s has incorrect profile/provenance: %+v", entry.ID, entry)
		}
		provenanceKinds[entry.Provenance.Kind]++
		switch entry.Provenance.Kind {
		case "generated":
			if entry.Provenance.Generator != "createsamples/peppol" || entry.Provenance.GeneratorVersion != testGeneratorVersion {
				t.Errorf("fixture %s has incorrect generated provenance: %+v", entry.ID, entry.Provenance)
			}
		case "official-import":
			if entry.Provenance.Publisher != "OpenPeppol" || entry.Provenance.SourceVersion != peppolProfile.SourceCommit ||
				entry.Provenance.SourceSHA256 != entry.SHA256 || entry.Provenance.SourceMember == nil {
				t.Errorf("fixture %s has incorrect official provenance: %+v", entry.ID, entry.Provenance)
			}
		default:
			t.Errorf("fixture %s has unexpected provenance kind %q", entry.ID, entry.Provenance.Kind)
		}
		if entry.Standards.EN16931 == nil || *entry.Standards.EN16931 != enProfile.Version ||
			entry.Standards.Peppol == nil || *entry.Standards.Peppol != peppolProfile.Version ||
			entry.Rulesets == nil || entry.Rulesets.EN16931 == nil || entry.Rulesets.Peppol == nil ||
			entry.Rulesets.EN16931.Version != enProfile.Version ||
			entry.Rulesets.EN16931.SHA256 != enProfile.XSLTSHA256 ||
			entry.Rulesets.Peppol.Version != peppolProfile.ValidationArtefactsVersion ||
			entry.Rulesets.Peppol.SHA256 != peppolProfile.XSLTSHA256 {
			t.Errorf("fixture %s lacks pinned EN/Peppol metadata", entry.ID)
		}
		if len(entry.ExpectedStages) != 3 ||
			entry.ExpectedStages[0].Stage != "ubl21-xsd" || entry.ExpectedStages[0].Outcome != "passed" ||
			entry.ExpectedStages[1].Stage != "en16931" || entry.ExpectedStages[1].Outcome != "passed" ||
			entry.ExpectedStages[2].Stage != "peppol-bis-billing" || entry.ExpectedStages[2].Outcome != "passed" {
			t.Errorf("fixture %s has wrong stage contract: %+v", entry.ID, entry.ExpectedStages)
		}
		resolution, err := peppolprofile.Resolve(
			entry.DocumentType, queue.ValidationProfileAuto, fixture.content,
		)
		if err != nil {
			t.Fatalf("resolve generated fixture %s: %v", entry.ID, err)
		}
		if !resolution.PeppolIdentifiersValid() {
			t.Errorf("fixture %s does not positively resolve to Peppol: %+v", entry.ID, resolution)
		}
		profiles[resolution.Identifiers.ProcessProfile]++
		documentTypes[entry.DocumentType]++
		for _, tag := range entry.Tags {
			coverage[tag] = true
		}
	}
	if profiles[queue.PeppolProcessProfile01] != 11 || profiles[queue.PeppolProcessProfile02] != 3 ||
		documentTypes["Invoice"] != 11 || documentTypes["CreditNote"] != 3 ||
		provenanceKinds["generated"] != 4 || provenanceKinds["official-import"] != 10 {
		t.Fatalf("Peppol coverage profiles=%v documents=%v provenance=%v", profiles, documentTypes, provenanceKinds)
	}
	for _, required := range []string{
		"allowance", "attachment", "billing-reference", "charge", "country-dk", "country-no",
		"country-se", "eas-0002", "eas-0088", "eas-0184", "eas-0192", "order-reference",
		"payment-30", "profile-01", "profile-02", "reference", "vat-e", "vat-o", "vat-s", "vat-z",
	} {
		if !coverage[required] {
			t.Errorf("Peppol positive corpus lacks %s coverage", required)
		}
	}
}

func TestPeppolNegativeCorpusIsDeterministicManifestDrivenAndXSDValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	first, err := generatePeppolNegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generatePeppolNegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 16 {
		t.Fatalf("Peppol negative fixtures = %d, want 16", len(first.fixtures))
	}
	if err := regression.ValidateFixtureManifest(first.manifestJSON); err != nil {
		t.Fatalf("Peppol negative manifest rejected: %v", err)
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the Peppol negative-corpus acceptance test")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}

	peppolFamilies := map[string]bool{}
	controls := map[string]bool{}
	enControls := map[string][]string{
		"control-en-missing-customization-id":    {"BR-01"},
		"control-en-unsupported-attachment-mime": {"BR-CL-24"},
	}
	for _, fixture := range first.fixtures {
		entry := fixture.entry
		if entry.Mutation == nil || entry.Mutation.ID == "" || entry.Mutation.Description == "" {
			t.Errorf("fixture %s lacks mutation provenance: %+v", entry.ID, entry.Mutation)
		}
		if entry.ExpectedFinal != "invalid" || len(entry.ExpectedStages) == 0 ||
			entry.ExpectedStages[len(entry.ExpectedStages)-1].Outcome != "standards-invalid" {
			t.Errorf("fixture %s has wrong negative-stage contract: %+v", entry.ID, entry)
		}
		if entry.Mutation.ID == "control-xsd-missing-invoice-id" {
			controls[entry.Mutation.ID] = true
			if len(entry.ExpectedStages) != 1 || entry.ExpectedStages[0].Stage != "ubl21-xsd" {
				t.Errorf("XSD control reaches a later stage: %+v", entry.ExpectedStages)
			}
		} else if wantRules, ok := enControls[entry.Mutation.ID]; ok {
			controls[entry.Mutation.ID] = true
			if len(entry.ExpectedStages) != 2 || entry.ExpectedStages[1].Stage != "en16931" ||
				!reflect.DeepEqual(entry.ExpectedStages[1].RuleIDs, wantRules) {
				t.Errorf("EN control has wrong stop contract: %+v", entry.ExpectedStages)
			}
		} else if len(entry.ExpectedStages) != 3 || entry.ExpectedStages[0].Outcome != "passed" ||
			entry.ExpectedStages[1].Stage != "en16931" || entry.ExpectedStages[1].Outcome != "passed" ||
			entry.ExpectedStages[2].Stage != "peppol-bis-billing" || len(entry.ExpectedStages[2].RuleIDs) == 0 {
			t.Errorf("Peppol target does not reach the Peppol stage: %+v", entry.ExpectedStages)
		} else {
			for _, family := range []string{
				"arithmetic", "cardinality", "code-list", "common", "country", "currency",
				"endpoint", "identifier", "party-scheme", "payment", "profile",
			} {
				for _, tag := range entry.Tags {
					if tag == family {
						peppolFamilies[family] = true
					}
				}
			}
		}
	}
	if len(peppolFamilies) != 11 {
		t.Fatalf("Peppol-targeted negative family coverage = %v", peppolFamilies)
	}
	if len(controls) != 3 {
		t.Fatalf("earlier-stage controls = %v", controls)
	}
}

func TestEN16931NegativeCorpusIsDeterministicManifestDrivenAndXSDValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	first, err := generateEN16931NegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateEN16931NegativeCorpus(xsdDir, testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyCorpusDeterminism(first, second); err != nil {
		t.Fatal(err)
	}
	if len(first.fixtures) != 22 {
		t.Fatalf("EN 16931 negative fixtures = %d, want 22", len(first.fixtures))
	}
	if err := regression.ValidateFixtureManifest(first.manifestJSON); err != nil {
		t.Fatalf("EN 16931 negative manifest rejected: %v", err)
	}
	xmllintPath, err := exec.LookPath("xmllint")
	if err != nil {
		t.Fatal("xmllint is required for the EN 16931 negative-corpus acceptance test")
	}
	if err := validateGeneratedCorpus(xmllintPath, first); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range first.fixtures {
		if fixture.entry.Mutation == nil || fixture.entry.Mutation.ID == "" || fixture.entry.Mutation.Description == "" {
			t.Errorf("fixture %s lacks mutation provenance: %+v", fixture.entry.ID, fixture.entry.Mutation)
		}
		if fixture.entry.ExpectedFinal != "invalid" || len(fixture.entry.ExpectedStages) != 2 ||
			fixture.entry.ExpectedStages[0].Stage != "ubl21-xsd" || fixture.entry.ExpectedStages[0].Outcome != "passed" ||
			fixture.entry.ExpectedStages[1].Stage != "en16931" || fixture.entry.ExpectedStages[1].Outcome != "standards-invalid" ||
			len(fixture.entry.ExpectedStages[1].RuleIDs) == 0 {
			t.Errorf("fixture %s has wrong negative-stage contract: %+v", fixture.entry.ID, fixture.entry)
		}
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

func TestTrackedStructuralCorpusHasValidHashes(t *testing.T) {
	root := "samples-structural"
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("tracked structural manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) < 300 {
		t.Fatalf("tracked structural corpus fixtures = %d, want at least 300", len(manifest.Fixtures))
	}
	for _, fixture := range manifest.Fixtures {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fixture.File)))
		if err != nil {
			t.Errorf("read fixture %s: %v", fixture.ID, err)
			continue
		}
		if actual := fmt.Sprintf("%x", sha256.Sum256(content)); actual != fixture.SHA256 {
			t.Errorf("tracked structural fixture %s SHA-256 = %s, manifest has %s", fixture.ID, actual, fixture.SHA256)
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
	if len(manifest.Fixtures) != 32 {
		t.Fatalf("tracked EN 16931 manifest fixtures = %d, want 32", len(manifest.Fixtures))
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
	if len(files) != 34 {
		t.Fatalf("tracked EN 16931 corpus files = %d, want 34 including manifest and ownership marker", len(files))
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

	positive, err := generateEN16931PositiveCorpus(filepath.Join("..", "XSD"), generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := generateEN16931NegativeCorpus(filepath.Join("..", "XSD"), generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := combineGeneratedCorpora(positive, negative)
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

func TestTrackedPeppolCorpusMatchesFreshGeneration(t *testing.T) {
	root := "samples-peppol"
	manifestBytes, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("tracked Peppol manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 30 {
		t.Fatalf("tracked Peppol manifest fixtures = %d, want 30", len(manifest.Fixtures))
	}
	generatorVersion := manifest.Fixtures[0].Provenance.GeneratorVersion
	if !immutableGeneratorVersion.MatchString(generatorVersion) {
		t.Fatalf("tracked Peppol generator version = %q", generatorVersion)
	}
	for _, fixture := range manifest.Fixtures {
		if fixture.Provenance.Kind == "generated" && fixture.Provenance.GeneratorVersion != generatorVersion {
			t.Fatalf("tracked fixture %s provenance = %+v", fixture.ID, fixture.Provenance)
		}
		if fixture.Provenance.Kind != "generated" && fixture.Provenance.Kind != "official-import" {
			t.Fatalf("tracked fixture %s provenance kind = %q", fixture.ID, fixture.Provenance.Kind)
		}
	}

	files := snapshotGeneratedDirectory(t, root)
	if len(files) != 32 {
		t.Fatalf("tracked Peppol corpus files = %d, want 32 including manifest and ownership marker", len(files))
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

	positive, err := generatePeppolPositiveCorpus(filepath.Join("..", "XSD"), generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	negative, err := generatePeppolNegativeCorpus(filepath.Join("..", "XSD"), generatorVersion)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := combineGeneratedCorpora(positive, negative)
	if err != nil {
		t.Fatal(err)
	}
	freshDir := filepath.Join(t.TempDir(), "samples-peppol")
	if err := writeGeneratedCorpus(freshDir, fresh); err != nil {
		t.Fatal(err)
	}
	freshFiles := snapshotGeneratedDirectory(t, freshDir)
	if len(freshFiles) != len(files) {
		t.Fatalf("fresh/tracked Peppol corpus file counts = %d/%d", len(freshFiles), len(files))
	}
	for path, trackedBytes := range files {
		freshBytes, ok := freshFiles[path]
		if !ok {
			t.Errorf("fresh Peppol generation is missing tracked file %s", path)
			continue
		}
		if !bytes.Equal(freshBytes, trackedBytes) {
			t.Errorf("fresh Peppol generation differs from tracked corpus at %s", path)
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

func TestCorpusFileWritingAndValidationFailureBranches(t *testing.T) {
	if _, err := buildGeneratedCorpus([]generatedFixture{{entry: fixtureManifestEntry{
		ID: "invalid", Provenance: fixtureProvenance{SourceMember: make(chan int)},
	}}}); err == nil {
		t.Fatal("unmarshalable fixture provenance was accepted")
	}
	if err := writeCorpusFiles(filepath.Join(t.TempDir(), "missing", "child"), generatedCorpus{}); err == nil {
		t.Fatal("missing output directory unexpectedly succeeded")
	}

	dir := t.TempDir()
	blocking := filepath.Join(dir, "blocking")
	if err := os.WriteFile(blocking, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixtureCorpus := generatedCorpus{fixtures: []generatedFixture{{
		entry: fixtureManifestEntry{File: "blocking/sample.xml"}, content: []byte("<Invoice/>")},
	}}
	if err := writeCorpusFiles(dir, fixtureCorpus); err == nil {
		t.Fatal("fixture below ordinary file unexpectedly succeeded")
	}

	manifestDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(manifestDir, "manifest.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeCorpusFiles(manifestDir, generatedCorpus{manifestJSON: []byte("{}")}); err == nil {
		t.Fatal("manifest overwrite of directory unexpectedly succeeded")
	}

	negative := generatedCorpus{fixtures: []generatedFixture{{
		entry:              fixtureManifestEntry{ID: "negative", ExpectedFinal: "invalid"},
		expectedDiagnostic: "wanted", content: []byte("<Invoice/>"), schemaPath: "missing.xsd",
	}}}
	if err := validateGeneratedCorpus("/bin/true", negative); err == nil {
		t.Fatal("negative fixture accepted by validator was ignored")
	}
	if err := validateGeneratedCorpus("/bin/false", negative); err == nil {
		t.Fatal("negative fixture without expected diagnostic was accepted")
	}
	if err := validateGeneratedCorpus("/definitely/missing/xmllint", negative); err == nil {
		t.Fatal("missing validator executable was ignored")
	}
}

func TestCorpusDeterminismRejectsFixtureCountAndIdentityChanges(t *testing.T) {
	manifest := []byte("same")
	first := generatedCorpus{manifestJSON: manifest, fixtures: []generatedFixture{{entry: fixtureManifestEntry{ID: "one", File: "one.xml"}, content: []byte("one")}}}
	if err := verifyCorpusDeterminism(first, generatedCorpus{manifestJSON: manifest}); err == nil {
		t.Fatal("fixture count drift was accepted")
	}
	second := first
	second.fixtures = append([]generatedFixture(nil), first.fixtures...)
	second.fixtures[0].entry.File = "two.xml"
	if err := verifyCorpusDeterminism(first, second); err == nil {
		t.Fatal("fixture identity drift was accepted")
	}
}

func TestParseStructuralResumeCursor(t *testing.T) {
	for cursor, want := range map[string][2]string{
		"":                           {"", ""},
		"main:Invoice":               {"main", "Invoice"},
		"signature:PGPData":          {"signature", "PGPData"},
		"cardinality:OtherTimeStamp": {"cardinality", "OtherTimeStamp"},
	} {
		phase, root, err := parseStructuralResumeCursor(cursor)
		if err != nil || phase != want[0] || root != want[1] {
			t.Errorf("parseStructuralResumeCursor(%q) = %q, %q, %v", cursor, phase, root, err)
		}
	}
	for _, cursor := range []string{"Invoice", "unknown:Invoice", "main:"} {
		if _, _, err := parseStructuralResumeCursor(cursor); err == nil {
			t.Errorf("parseStructuralResumeCursor(%q) accepted invalid cursor", cursor)
		}
	}
}
