package main

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/regression"
)

func TestShippedSchemaInventoryIndexAndGeneration(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	report, err := discoverInventory(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	if report.MainSchemas == 0 || report.CommonSchemas == 0 {
		t.Fatalf("incomplete inventory: %+v", report)
	}
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}

	var invoice schemaInfo
	for _, info := range report.MainFiles {
		if info.RootElement == "Invoice" {
			invoice = info
			break
		}
	}
	if invoice.File == "" {
		t.Fatal("Invoice schema not found")
	}
	xml, err := generateCompleteSample(index, invoice.RootElement, xsdDir, invoice.File)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xml, `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"`) {
		t.Fatal("generated sample lacks Invoice root namespace")
	}
	if _, err := generateCompleteSample(index, "Missing", xsdDir, invoice.File); err == nil {
		t.Fatal("expected missing root error")
	}
}

func TestSplitQualifiedKeyPreservesFragmentNamespace(t *testing.T) {
	key := qualifiedKey("http://uri.etsi.org/01903/v1.3.2#", "AllDataObjectsTimeStamp")
	namespace, name, ok := splitQualifiedKey(key)
	if !ok || namespace != "http://uri.etsi.org/01903/v1.3.2#" || name != "AllDataObjectsTimeStamp" {
		t.Fatalf("splitQualifiedKey(%q) = %q, %q, %t", key, namespace, name, ok)
	}
}

func TestGenerateRestrictedXAdESTimestampRoot(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	const namespace = "http://uri.etsi.org/01903/v1.3.2#"
	doc := index.GlobalElementDocs[qualifiedKey(namespace, "AllDataObjectsTimeStamp")]
	if doc == nil {
		t.Fatal("AllDataObjectsTimeStamp declaration missing")
	}
	state := newTargetedStructuralGenState(nil, 1)
	state.signatureExtension = true
	content, err := generateSampleWithState(index, "AllDataObjectsTimeStamp", filepath.Join("..", "XSD"), doc.File, state)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "common", doc.File)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateXAdESCertificateValuesRoot(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	const namespace = "http://uri.etsi.org/01903/v1.3.2#"
	doc := index.GlobalElementDocs[qualifiedKey(namespace, "AttrAuthoritiesCertValues")]
	if doc == nil {
		t.Fatal("AttrAuthoritiesCertValues declaration missing")
	}
	state := newTargetedStructuralGenState(nil, 1)
	state.signatureExtension = true
	content, err := generateSampleWithState(index, "AttrAuthoritiesCertValues", filepath.Join("..", "XSD"), doc.File, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, ">U2FtcGxlRGF0YQ==</") {
		t.Fatalf("EncapsulatedX509Certificate is not base64: %s", content)
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "common", doc.File)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateXAdESAttributeCertificateRefsRoot(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	const namespace = "http://uri.etsi.org/01903/v1.3.2#"
	doc := index.GlobalElementDocs[qualifiedKey(namespace, "AttributeCertificateRefs")]
	if doc == nil {
		t.Fatal("AttributeCertificateRefs declaration missing")
	}
	state := newTargetedStructuralGenState(nil, 1)
	state.signatureExtension = true
	content, err := generateSampleWithState(index, "AttributeCertificateRefs", filepath.Join("..", "XSD"), doc.File, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "<ds:X509IssuerName>") {
		t.Fatalf("X509IssuerName lacks XMLDSig namespace: %s", content)
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "common", doc.File)); err != nil {
		t.Fatal(err)
	}
}

func TestCardinalityStructuralGenerationHonoursSchemaBounds(t *testing.T) {
	state := newStructuralGenState(0, 1, true)
	state.repeatCount = 5
	if got := occurrenceCount(xsdElementDecl{MaxOccurs: "2"}, state, 1); got != 2 {
		t.Fatalf("finite cardinality = %d, want 2", got)
	}
	if got := occurrenceCount(xsdElementDecl{MaxOccurs: "unbounded"}, state, 1); got != 5 {
		t.Fatalf("unbounded cardinality = %d, want 5", got)
	}
	if got := occurrenceCount(xsdElementDecl{MaxOccurs: "unbounded"}, state, 2); got != 1 {
		t.Fatalf("nested broad cardinality = %d, want 1", got)
	}
	state.repeatOverrides = map[string]int{"Line": 100}
	if got := occurrenceCount(xsdElementDecl{Name: "Line", MaxOccurs: "unbounded"}, state, 1); got != 100 {
		t.Fatalf("targeted cardinality = %d, want 100", got)
	}
}

func TestLargeInvoiceLineFixtureIsDeterministicAndValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	first, err := generateLargeCollectionStructuralSample(index, "Invoice", xsdDir, "UBL-Invoice-2.1.xsd", "InvoiceLine", 100)
	if err != nil {
		t.Fatal(err)
	}
	second, err := generateLargeCollectionStructuralSample(index, "Invoice", xsdDir, "UBL-Invoice-2.1.xsd", "InvoiceLine", 100)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("large Invoice fixture is not deterministic")
	}
	if got, err := directChildCount(first, "InvoiceLine"); err != nil || got != 100 {
		t.Fatalf("direct InvoiceLine count = %d, err = %v, want 100", got, err)
	}
	if err := validateStructuralCandidate([]byte(first), filepath.Join(xsdDir, "maindoc", "UBL-Invoice-2.1.xsd")); err != nil {
		t.Fatal(err)
	}
}

func TestMultiCollectionInvoiceFixtureIsValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := generateLargeCollectionsStructuralSample(index, "Invoice", xsdDir, "UBL-Invoice-2.1.xsd", map[string]int{
		"InvoiceLine":     100,
		"AllowanceCharge": 50,
		"TaxTotal":        50,
	})
	if err != nil {
		t.Fatal(err)
	}
	for element, want := range map[string]int{"InvoiceLine": 100, "AllowanceCharge": 50, "TaxTotal": 50} {
		got, err := directChildCount(content, element)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("direct %s count = %d, want %d", element, got, want)
		}
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join(xsdDir, "maindoc", "UBL-Invoice-2.1.xsd")); err != nil {
		t.Fatal(err)
	}
}

func TestLargeAttachmentReferenceAndShipmentFixturesAreValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		root, schema, element string
	}{
		{"Invoice", "UBL-Invoice-2.1.xsd", "AdditionalDocumentReference"},
		{"FreightInvoice", "UBL-FreightInvoice-2.1.xsd", "Shipment"},
	} {
		content, err := generateLargeCollectionStructuralSample(index, fixture.root, xsdDir, fixture.schema, fixture.element, 50)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := directChildCount(content, fixture.element); err != nil || got != 50 {
			t.Fatalf("%s direct %s count = %d, err = %v, want 50", fixture.root, fixture.element, got, err)
		}
		if err := validateStructuralCandidate([]byte(content), filepath.Join(xsdDir, "maindoc", fixture.schema)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLargeItemBearingCatalogueFixtureIsValid(t *testing.T) {
	xsdDir := filepath.Join("..", "XSD")
	index, err := buildSchemaIndex(xsdDir)
	if err != nil {
		t.Fatal(err)
	}
	content, err := generateLargeCollectionStructuralSample(index, "Catalogue", xsdDir, "UBL-Catalogue-2.1.xsd", "CatalogueLine", 100)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := directChildCount(content, "CatalogueLine"); err != nil || got != 100 {
		t.Fatalf("direct CatalogueLine count = %d, err = %v, want 100", got, err)
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join(xsdDir, "maindoc", "UBL-Catalogue-2.1.xsd")); err != nil {
		t.Fatal(err)
	}
}

func directChildCount(content, local string) (int, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))
	depth, count := 0, 0
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return 0, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 1 && token.Name.Local == local {
				count++
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

func TestCommandsAndInventoryErrors(t *testing.T) {
	if err := runGenerate([]string{"--count", "0"}); err == nil {
		t.Fatal("expected invalid count error")
	}
	if err := runInventory([]string{"--bad-flag"}); err == nil {
		t.Fatal("expected flag parse error")
	}
	if err := runInventory([]string{"--xsd", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("inventory accepted a missing XSD tree")
	}
	if err := runGenerate([]string{"--xsd", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("generate accepted a missing XSD tree")
	}
	if err := runGenerate([]string{"--profile", "peppol"}); err == nil {
		t.Fatal("generate accepted an unsupported profile")
	}
	if err := runGenerate([]string{"--set", "stress"}); err == nil {
		t.Fatal("generate accepted an unsupported fixture set")
	}
	for _, args := range [][]string{
		{"--profile", "en16931", "--count", "2"},
	} {
		args = append(args, "--xsd", filepath.Join("..", "XSD"), "--output", filepath.Join(t.TempDir(), "samples"), "--generator-version", testGeneratorVersion)
		if err := runGenerate(args); err == nil {
			t.Fatalf("generate accepted unsupported EN 16931 options %v", args)
		}
	}
	emptyTree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(emptyTree, "maindoc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(emptyTree, "common"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runGenerate([]string{"--xsd", emptyTree, "--output", filepath.Join(t.TempDir(), "samples")}); err == nil || !strings.Contains(err.Error(), "no main document") {
		t.Fatalf("empty XSD tree error = %v", err)
	}
	blockedParent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedParent, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runInventory([]string{"--xsd", filepath.Join("..", "XSD"), "--output", filepath.Join(blockedParent, "inventory.json")}); err == nil {
		t.Fatal("inventory wrote beneath a regular file")
	}
	if _, err := listXSDs(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing directory error")
	}

	dir := t.TempDir()
	mainDir := filepath.Join(dir, "maindoc")
	commonDir := filepath.Join(dir, "common")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, "bad.xsd"), []byte("<bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := discoverInventory(dir); err == nil {
		t.Fatal("expected malformed schema error")
	}
	if _, err := parseSchema(filepath.Join(dir, "missing.xsd")); err == nil {
		t.Fatal("expected read error")
	}
}

func TestInventoryGenerateAndValidateCommands(t *testing.T) {
	dir := t.TempDir()
	mainDir := filepath.Join(dir, "maindoc")
	commonDir := filepath.Join(dir, "common")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	schema := `<?xml version="1.0"?><xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns="urn:test" elementFormDefault="qualified"><xsd:element name="Demo" type="xsd:string"/></xsd:schema>`
	if err := os.WriteFile(filepath.Join(mainDir, "UBL-Demo-2.1.xsd"), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	reportPath := filepath.Join(dir, "reports", "inventory.json")
	if err := run([]string{"inventory", "--xsd", dir, "--output", reportPath}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "samples")
	if err := run([]string{"generate", "--xsd", dir, "--output", out, "--count", "2", "--generator-version", testGeneratorVersion}); err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifest); err != nil {
		t.Fatalf("generated manifest rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(out, "Demo", "Demo-sample-02.xml"), []byte(`<Wrong/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(out, "Unknown"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "Unknown", "sample.xml"), []byte(`<Unknown/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"validate", "--xsd", dir, "--samples", out}); err != nil {
		t.Fatal(err)
	}
	if err := runValidate([]string{"--xsd", dir, "--samples", filepath.Join(dir, "empty")}); err == nil {
		t.Fatal("expected missing samples error")
	}
}

func TestTrackedStructuralCorpusManifestIsValid(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("samples-structural", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifest); err != nil {
		t.Fatalf("tracked structural manifest rejected: %v", err)
	}
}

func TestGenerateEN16931UsesProfileDefaultOutput(t *testing.T) {
	xsdDir, err := filepath.Abs(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	workingDir := t.TempDir()
	ublOutput := filepath.Join(workingDir, "createsamples", "samples")
	if err := os.MkdirAll(ublOutput, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ublOutput, corpusOwnershipMarkerName), []byte(corpusOwnershipMarkerContent), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(ublOutput, "ubl-corpus-sentinel")
	if err := os.WriteFile(sentinel, []byte("preserve UBL corpus"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(workingDir)

	if err := run([]string{
		"generate", "--profile", "en16931", "--xsd", xsdDir,
		"--generator-version", testGeneratorVersion,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join("createsamples", "samples-en16931", "manifest.json")); err != nil {
		t.Fatalf("EN 16931 corpus was not written to its profile default: %v", err)
	}
	if content, err := os.ReadFile(sentinel); err != nil || string(content) != "preserve UBL corpus" {
		t.Fatalf("UBL corpus was modified: content=%q err=%v", content, err)
	}
}

func TestGeneratePeppolUsesProfileDefaultOutput(t *testing.T) {
	xsdDir, err := filepath.Abs(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	if err := run([]string{
		"generate", "--profile", "peppol-bis-billing", "--xsd", xsdDir,
		"--generator-version", testGeneratorVersion,
	}); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join("createsamples", "samples-peppol", "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Peppol corpus was not written to its profile default: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 14 {
		t.Fatalf("Peppol generated fixtures = %d, want 14", len(manifest.Fixtures))
	}
	if err := run([]string{
		"generate", "--profile", "peppol-bis-billing", "--set", "negative",
		"--xsd", xsdDir, "--output", filepath.Join(workingDir, "negative"),
		"--generator-version", testGeneratorVersion,
	}); err != nil {
		t.Fatal(err)
	}
	negativeManifest, err := os.ReadFile(filepath.Join(workingDir, "negative", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var negative fixtureManifest
	if err := json.Unmarshal(negativeManifest, &negative); err != nil {
		t.Fatal(err)
	}
	if len(negative.Fixtures) != 16 {
		t.Fatalf("Peppol negative generated fixtures = %d, want 16", len(negative.Fixtures))
	}
}

func TestGenerateUBLNegativeSet(t *testing.T) {
	output := filepath.Join(t.TempDir(), "negative")
	if err := run([]string{
		"generate", "--profile", "ubl21", "--set", "negative",
		"--xsd", filepath.Join("..", "XSD"), "--output", output,
		"--generator-version", testGeneratorVersion,
	}); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("negative manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 6 {
		t.Fatalf("negative manifest fixtures = %d, want 6", len(manifest.Fixtures))
	}
}

func TestGenerateEN16931NegativeSet(t *testing.T) {
	output := filepath.Join(t.TempDir(), "negative")
	if err := run([]string{
		"generate", "--profile", "en16931", "--set", "negative",
		"--xsd", filepath.Join("..", "XSD"), "--output", output,
		"--generator-version", testGeneratorVersion,
	}); err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := regression.ValidateFixtureManifest(manifestBytes); err != nil {
		t.Fatalf("EN 16931 negative manifest rejected: %v", err)
	}
	var manifest fixtureManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Fixtures) != 22 {
		t.Fatalf("EN 16931 negative manifest fixtures = %d, want 22", len(manifest.Fixtures))
	}
}

func TestSyntheticContentModelBranches(t *testing.T) {
	primitive := xsdElementDecl{Name: "Value", Type: "xsd:string"}
	unsupported := xsdElementDecl{Name: "Any", MinOccurs: "0", Sequence: &xsdSequence{Any: &xsdAny{}}}
	index := schemaIndex{GlobalElements: map[string]xsdElementDecl{}, GlobalTypes: map[string]xsdComplexType{}, GlobalSimpleTypes: map[string]xsdSimpleType{}}
	state := newGenState()

	if nodes, ok := flattenChoice(xsdChoice{Elements: []xsdElementDecl{unsupported, primitive}}, index, "urn:test", state, 0); !ok || len(nodes) != 1 {
		t.Fatalf("choice = %#v, %v", nodes, ok)
	}
	if _, ok := flattenChoice(xsdChoice{}, index, "urn:test", state, 0); ok {
		t.Fatal("empty choice should be unsupported")
	}
	all := xsdAll{Elements: []xsdElementDecl{{Name: "Optional", Type: "xsd:string", MinOccurs: "0"}, primitive}}
	if nodes, ok := flattenAll(all, index, "urn:test", state.demoted(), 0); !ok || len(nodes) != 1 {
		t.Fatalf("required-only all = %#v, %v", nodes, ok)
	}
	if _, ok := flattenContentModel(nil, nil, nil, index, "urn:test", state, 0); !ok {
		t.Fatal("empty model should be supported")
	}
	if _, ok := flattenContentModel(nil, nil, &all, index, "urn:test", state, 0); !ok {
		t.Fatal("all model should be supported")
	}
	unsupportedAll := xsdAll{Elements: []xsdElementDecl{{}, {Name: "RequiredAny", Sequence: &xsdSequence{Any: &xsdAny{}}}}}
	if _, ok := flattenAll(unsupportedAll, index, "urn:test", state, 0); ok {
		t.Fatal("all group with a required wildcard should be unsupported")
	}
	if nodes, ok := flattenSequence(xsdSequence{Elements: []xsdElementDecl{{}, primitive}}, index, "urn:test", state, 0); !ok || len(nodes) != 1 {
		t.Fatalf("sequence with unnamed element = %#v, %v", nodes, ok)
	}
	if _, ok := flattenChoice(xsdChoice{Elements: []xsdElementDecl{{}}}, index, "urn:test", state, 0); ok {
		t.Fatal("choice with only unnamed elements should be unsupported")
	}
	if name, prefix := elementNameAndPrefix(xsdElementDecl{Name: "Local"}); name != "Local" || prefix != "" {
		t.Fatalf("local element = %q/%q", prefix, name)
	}

	simple := xsdSimpleType{Restriction: &xsdRestriction{Enumerations: []xsdEnumeration{{Value: "A"}, {Value: ""}}}}
	index.GlobalSimpleTypes[qualifiedKey("urn:test", "Enum")] = simple
	if got := sampleScalarForType(index, "Code", "Enum", "urn:test", 0); got != "A" {
		t.Fatalf("enumerated scalar = %q", got)
	}
	complexEnum := xsdComplexType{SimpleContent: &xsdSimpleContent{Restriction: &xsdRestriction{Enumerations: []xsdEnumeration{{Value: "B"}}}}}
	index.GlobalTypes[qualifiedKey("urn:test", "ComplexEnum")] = complexEnum
	if got := enumerationValuesForType(index, "ComplexEnum", "urn:test"); len(got) != 1 || got[0] != "B" {
		t.Fatalf("complex enumeration = %#v", got)
	}
}

func TestTargetedChoiceSelectionIsPerSite(t *testing.T) {
	state := newTargetedStructuralGenState(map[int]int{0: 1, 1: 0}, 1)
	choice := xsdChoice{Elements: []xsdElementDecl{{Name: "First", Type: "xsd:string"}, {Name: "Second", Type: "xsd:string"}}}
	if nodes, ok := flattenChoice(choice, schemaIndex{}, "urn:test", state, 0); !ok || nodes[0].Name != "Second" {
		t.Fatalf("outer choice = %#v, %v", nodes, ok)
	}
	if nodes, ok := flattenChoice(choice, schemaIndex{}, "urn:test", state, 0); !ok || nodes[0].Name != "First" {
		t.Fatalf("next choice = %#v, %v", nodes, ok)
	}
}

func TestGenerateSignatureExtensionStructuralSampleIsXSDValid(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := generateSignatureExtensionStructuralSample(index, "Invoice", filepath.Join("..", "XSD"), "UBL-Invoice-2.1.xsd", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<sig:UBLDocumentSignatures>", "<ds:KeyInfo>", "<ds:XPath>/*</ds:XPath>"} {
		if !strings.Contains(content, want) {
			t.Errorf("signature extension missing %q", want)
		}
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "maindoc", "UBL-Invoice-2.1.xsd")); err != nil {
		t.Fatal(err)
	}
	matched, err := matchFixtureBytes(index, []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"KeyInfoType/choice", "TransformType/choice", "SignaturePolicyIdentifierType", "CommitmentTypeIndicationType", "UnsignedSignaturePropertiesType", "XAdESTimeStampType"} {
		found := false
		for target := range matched {
			if strings.Contains(target, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("signature extension did not exercise %s", want)
		}
	}
}

func TestGenerateCounterSignatureRootIsXSDValid(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	doc := index.GlobalElementDocs[qualifiedKey("http://uri.etsi.org/01903/v1.3.2#", "CounterSignature")]
	if doc == nil {
		t.Fatal("CounterSignature declaration not found")
	}
	signatureMethod := index.GlobalTypes[qualifiedKey("http://www.w3.org/2000/09/xmldsig#", "SignatureMethodType")]
	if signatureMethod.Sequence == nil || len(signatureMethod.Sequence.Elements) == 0 {
		t.Fatal("SignatureMethod HMACOutputLength declaration not found")
	}
	if got := signatureMethod.Sequence.Elements[0].Type; got != "ds:HMACOutputLengthType" {
		t.Fatalf("HMACOutputLength type = %q", got)
	}
	if _, ok := index.GlobalSimpleTypes[qualifiedKey("http://www.w3.org/2000/09/xmldsig#", "HMACOutputLengthType")]; !ok {
		t.Fatal("HMACOutputLengthType not indexed as a simple type")
	}
	if signatureMethod.Sequence.Elements[0].ComplexType != nil {
		t.Fatal("HMACOutputLength unexpectedly has an inline complex type")
	}
	if got := sampleScalarForType(index, "HMACOutputLength", "ds:HMACOutputLengthType", "http://www.w3.org/2000/09/xmldsig#", 1); got != "1" {
		t.Fatalf("direct HMACOutputLength = %q", got)
	}
	if node, ok := expandElement(index, signatureMethod.Sequence.Elements[0], "HMACOutputLength", "ds", "http://www.w3.org/2000/09/xmldsig#", newTargetedStructuralGenState(nil, 1), 0); !ok || node.Value != "1" {
		t.Fatalf("generated HMACOutputLength = %#v, supported=%v", node, ok)
	}
	state := newTargetedStructuralGenState(nil, 1)
	state.signatureExtension = true
	content, err := generateSampleWithState(index, "CounterSignature", filepath.Join("..", "XSD"), doc.File, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, ">1</ds:HMACOutputLength>") {
		t.Fatalf("HMACOutputLength is not a valid integer:\n%s", content)
	}
	if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "common", doc.File)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateSignatureExtensionStructuralVariantsAreDistinctAndXSDValid(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for variant := 0; variant < 32; variant++ {
		content, err := generateSignatureExtensionStructuralSample(index, "Invoice", filepath.Join("..", "XSD"), "UBL-Invoice-2.1.xsd", variant)
		if err != nil {
			t.Fatalf("variant %d: %v", variant, err)
		}
		if seen[content] {
			t.Fatalf("variant %d duplicates an earlier signature fixture", variant)
		}
		seen[content] = true
		if err := validateStructuralCandidate([]byte(content), filepath.Join("..", "XSD", "maindoc", "UBL-Invoice-2.1.xsd")); err != nil {
			t.Fatalf("variant %d: %v", variant, err)
		}
	}
}

func TestStructuralChoiceSiteCountIsRootLocal(t *testing.T) {
	ns := "urn:test"
	index := schemaIndex{
		GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): {Name: "Root", Type: "RootType"}},
		GlobalTypes:    map[string]xsdComplexType{qualifiedKey(ns, "RootType"): {Sequence: &xsdSequence{Choices: []xsdChoice{{Elements: []xsdElementDecl{{Name: "A", Type: "xsd:string"}, {Name: "B", Type: "xsd:string"}}}}}}},
		ByTargetNS: map[string]*schemaDoc{ns: {TargetNamespace: ns, Types: map[string]xsdComplexType{
			"RootType":      {Sequence: &xsdSequence{Choices: []xsdChoice{{Elements: []xsdElementDecl{{Name: "A", Type: "xsd:string"}, {Name: "B", Type: "xsd:string"}}}}}},
			"UnrelatedType": {Choice: &xsdChoice{Elements: []xsdElementDecl{{Name: "C", Type: "xsd:string"}, {Name: "D", Type: "xsd:string"}}}},
		}}},
	}
	xsdDir := t.TempDir()
	path := filepath.Join(xsdDir, "maindoc", "root.xsd")
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	count, err := structuralChoiceSiteCount(index, "Root", xsdDir, filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("choice sites = %d, want 1", count)
	}
}

func TestStructuralChoicePlansDiscoverNestedAlternativeSites(t *testing.T) {
	ns := "urn:test"
	inner := xsdChoice{Elements: []xsdElementDecl{{Name: "One", Type: "xsd:string"}, {Name: "Two", Type: "xsd:string"}}}
	rootType := xsdComplexType{Choice: &xsdChoice{Sequences: []xsdSequence{
		{Choices: []xsdChoice{inner}},
		{Choices: []xsdChoice{inner}},
	}}}
	index := schemaIndex{
		GlobalElements: map[string]xsdElementDecl{qualifiedKey(ns, "Root"): {Name: "Root", Type: "RootType"}},
		GlobalTypes:    map[string]xsdComplexType{qualifiedKey(ns, "RootType"): rootType},
		ByTargetNS:     map[string]*schemaDoc{ns: {TargetNamespace: ns, Types: map[string]xsdComplexType{"RootType": rootType}}},
	}
	xsdDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(xsdDir, "maindoc"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(xsdDir, "maindoc", "root.xsd"), []byte(`<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	plans, err := structuralChoicePlans(index, "Root", xsdDir, "root.xsd")
	if err != nil {
		t.Fatal(err)
	}
	foundNested := false
	for _, plan := range plans {
		if plan[0] == 1 && plan[1] == 1 {
			foundNested = true
		}
	}
	if !foundNested {
		t.Fatalf("plans did not include nested alternate after parent alternate: %#v", plans)
	}
}

func TestStructuralWildcardHonorsPolicy(t *testing.T) {
	if _, ok := structuralWildcardNode(xsdAny{Namespace: "##other", ProcessContents: "strict"}); ok {
		t.Fatal("strict wildcard must not invent an undeclared node")
	}
	if _, ok := structuralWildcardNode(xsdAny{Namespace: "##other"}); ok {
		t.Fatal("an omitted wildcard policy defaults to strict")
	}
	if n, ok := structuralWildcardNode(xsdAny{Namespace: "##local", ProcessContents: "skip"}); !ok || n.Attributes["xmlns"] != "" {
		t.Fatalf("local wildcard = %#v, %v", n, ok)
	}
}

func TestFacetBoundaryValues(t *testing.T) {
	ns := "urn:test"
	index := schemaIndex{GlobalSimpleTypes: map[string]xsdSimpleType{
		qualifiedKey(ns, "Range"):  {Restriction: &xsdRestriction{MinInclusive: &xsdFacet{Value: "2"}, MaxExclusive: &xsdFacet{Value: "5"}}},
		qualifiedKey(ns, "Digits"): {Restriction: &xsdRestriction{TotalDigits: &xsdFacet{Value: "5"}, FractionDigits: &xsdFacet{Value: "2"}}},
	}}
	if got := facetBoundaryValuesForType(index, "Range", ns); len(got) != 2 || got[0] != "2" || got[1] != "4" {
		t.Fatalf("range boundaries = %#v", got)
	}
	if got := facetBoundaryValuesForType(index, "Digits", ns); len(got) != 1 || got[0] != "999.99" {
		t.Fatalf("digit boundary = %#v", got)
	}
}

func TestExpandElementResolutionBranches(t *testing.T) {
	ns := "urn:test"
	index := schemaIndex{
		GlobalElements: map[string]xsdElementDecl{
			qualifiedKey(ns, "Target"): {Name: "Target", Type: "xsd:string"},
		},
		GlobalTypes: map[string]xsdComplexType{
			qualifiedKey(ns, "Container"): {Name: "Container", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "Child", Type: "xsd:string"}}}},
		},
		GlobalSimpleTypes: map[string]xsdSimpleType{
			qualifiedKey(ns, "Enum"): {Restriction: &xsdRestriction{Enumerations: []xsdEnumeration{{Value: "ONE"}}}},
		},
	}
	state := newGenState()
	cases := []xsdElementDecl{
		{Ref: "Target"},
		{Name: "Simple", Type: "Enum"},
		{Name: "Container", Type: "Container"},
		{Name: "Inline", Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "Child", Type: "xsd:string"}}}},
		{Name: "Fallback"},
		{Name: "SimpleContent", ComplexType: &xsdComplexType{SimpleContent: &xsdSimpleContent{Extension: &xsdExtension{Base: "xsd:string"}}}},
	}
	for _, element := range cases {
		name := element.Name
		if name == "" {
			name = "Target"
		}
		if _, ok := expandElement(index, element, name, "", ns, state, 0); !ok {
			t.Errorf("expandElement(%s) unsupported", name)
		}
	}
	deep, ok := expandElement(index, xsdElementDecl{Name: "Deep", Type: "xsd:string"}, "Deep", "", ns, state, maxDepth+1)
	if !ok || deep.Value == "" {
		t.Fatalf("depth fallback = %#v, %v", deep, ok)
	}
	if _, ok := expandElement(index, xsdElementDecl{Name: "RequiredAny", Sequence: &xsdSequence{Any: &xsdAny{}}}, "RequiredAny", "", ns, state, 0); ok {
		t.Fatal("required wildcard should be unsupported")
	}
}

func TestStructuralExpansionPreservesRequiredChildrenAndExternalNamespaces(t *testing.T) {
	const extensionNS = "urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2"
	index := schemaIndex{
		GlobalElements: map[string]xsdElementDecl{
			qualifiedKey(extensionNS, "UBLExtension"): {Name: "UBLExtension", Type: "xsd:string"},
		},
		GlobalTypes: map[string]xsdComplexType{
			qualifiedKey("urn:test", "Container"): {
				Name:     "Container",
				Sequence: &xsdSequence{Elements: []xsdElementDecl{{Name: "ID", Type: "xsd:string"}}},
			},
		},
		GlobalSimpleTypes: map[string]xsdSimpleType{},
	}

	extension, ok := expandElement(index, xsdElementDecl{Ref: "UBLExtension"}, "UBLExtension", "", extensionNS, newStructuralGenState(0, 1, true), 0)
	if !ok || extension.Prefix != "ext" {
		t.Fatalf("unprefixed external ref = %#v, supported=%v", extension, ok)
	}

	state := newStructuralGenState(0, 1, true)
	state.typeDepth[qualifiedKey("urn:test", "Container")] = 1
	container, ok := expandElement(index, xsdElementDecl{Name: "Container", Type: "Container"}, "Container", "", "urn:test", state, 0)
	if !ok || len(container.Children) != 1 || container.Children[0].Name != "ID" {
		t.Fatalf("recursive cutoff removed required children: %#v, supported=%v", container, ok)
	}
}

func TestRunDispatchAndUsage(t *testing.T) {
	printUsage()
	if err := run(nil); err == nil {
		t.Fatal("missing command accepted")
	}
	if err := run([]string{"unknown"}); err == nil {
		t.Fatal("unknown command accepted")
	}
	if err := run([]string{"generate", "--count", "0"}); err == nil {
		t.Fatal("generate error not propagated")
	}
	if err := run([]string{"inventory", "--bad"}); err == nil {
		t.Fatal("inventory error not propagated")
	}
	if err := run([]string{"validate", "--samples", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("validate error not propagated")
	}
	if err := runGenerate([]string{"--bad"}); err == nil {
		t.Fatal("generate flag error not propagated")
	}
	if err := runValidate([]string{"--bad"}); err == nil {
		t.Fatal("validate flag error not propagated")
	}
	if localPart("") != "" {
		t.Fatal("empty local part should remain empty")
	}
}

func TestMainRunsSuccessfulInventoryCommand(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"createsamples", "inventory", "--xsd", "../XSD"}
	main()
}

func TestSchemaIndexReadErrorsAndNamespaceFallback(t *testing.T) {
	dir := t.TempDir()
	if ns, name := qualifyRef("", "context"); ns != "" || name != "" {
		t.Fatalf("empty ref = %q/%q", ns, name)
	}
	if _, err := loadSchemaDoc(filepath.Join(dir, "missing.xsd")); err == nil {
		t.Fatal("missing schema accepted")
	}
	bad := filepath.Join(dir, "bad.xsd")
	if err := os.WriteFile(bad, []byte(`<schema`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSchemaDoc(bad); err == nil {
		t.Fatal("malformed schema accepted")
	}
	if _, err := rootNamespaceForSchema(dir, "missing.xsd"); err == nil {
		t.Fatal("missing main schema accepted")
	}
	mainDir := filepath.Join(dir, "maindoc")
	if err := os.MkdirAll(mainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(mainDir, "UBL-Fallback-2.1.xsd")
	if err := os.WriteFile(fallback, []byte(`<schema><element name="Fallback"/></schema>`), 0o600); err != nil {
		t.Fatal(err)
	}
	ns, err := rootNamespaceForSchema(dir, filepath.Base(fallback))
	if err != nil || !strings.Contains(ns, "Fallback-2") {
		t.Fatalf("fallback namespace = %q, %v", ns, err)
	}
}

func TestQNameValuesAndGenerationHelpers(t *testing.T) {
	index, err := buildSchemaIndex(filepath.Join("..", "XSD"))
	if err != nil {
		t.Fatal(err)
	}
	if localPart("cbc:ID") != "ID" || prefixPart("cbc:ID") != "cbc" || trimUBLDocumentType("UBL-Invoice-2.1.xsd") != "Invoice" {
		t.Fatal("QName helpers returned unexpected values")
	}
	if ns, name := qualifyRef("unknown:Thing", "context"); ns != "" || name != "Thing" {
		t.Fatalf("qualifyRef unknown = %q, %q", ns, name)
	}
	for name, want := range map[string]string{
		"IssueDate": "2026-01-01", "IssueTime": "12:00:00", "CreatedDateTime": "2026-01-01T12:00:00Z",
		"Duration": "P0D", "TaxAmount": "100.00", "Percent": "10.00", "Quantity": "1", "Indicator": "true",
		"CurrencyCode": "USD", "ID": "ID-001", "Status": "Active", "BinaryObject": "U2FtcGxlRGF0YQ==",
	} {
		if got := sampleValueForType(name, "", 0); got != want {
			t.Errorf("sampleValueForType(%q) = %q, want %q", name, got, want)
		}
	}
	if sampleValueForType("Rate", "", 2) != "1.15" || sampleValueForType("Text", "", 2) != "Value of Text 2" {
		t.Error("indexed sample values incorrect")
	}
	if got := sampleValueForType("IssueTime", "xsd:dateTime", 0); got != "2026-01-01T12:00:00Z" {
		t.Errorf("xsd:dateTime IssueTime = %q, want a valid date-time", got)
	}
	if got := sampleScalarForType(index, "HMACOutputLength", "ds:HMACOutputLengthType", "http://www.w3.org/2000/09/xmldsig#", 0); got != "1" {
		t.Errorf("HMACOutputLength = %q, want a valid integer", got)
	}
	if !isPrimitiveType("xsd:string") || isPrimitiveType("custom") {
		t.Error("primitive detection incorrect")
	}
	for input, want := range map[string]int{"": 2, "3": 3, "bad": 2, "-1": 2} {
		if got := parseOccursInt(input, 2); got != want {
			t.Errorf("parseOccursInt(%q) = %d", input, got)
		}
	}
	if occurrenceCount(xsdElementDecl{MinOccurs: "99"}, newGenState(), 1) != maxOccursSample || occurrenceCount(xsdElementDecl{MinOccurs: "0"}, newGenState(), 1) != 1 {
		t.Error("occurrence cap/default incorrect")
	}
	if sequenceRequiresAny(nil) || sequenceRequiresAny(&xsdSequence{}) || !sequenceRequiresAny(&xsdSequence{Any: &xsdAny{}}) {
		t.Error("sequenceRequiresAny incorrect")
	}
}
