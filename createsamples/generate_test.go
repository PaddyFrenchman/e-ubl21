package main

import (
	"encoding/json"
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
		{"--profile", "en16931", "--set", "negative"},
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
	if !isPrimitiveType("xsd:string") || isPrimitiveType("custom") {
		t.Error("primitive detection incorrect")
	}
	for input, want := range map[string]int{"": 2, "3": 3, "bad": 2, "-1": 2} {
		if got := parseOccursInt(input, 2); got != want {
			t.Errorf("parseOccursInt(%q) = %d", input, got)
		}
	}
	if occurrenceCount(xsdElementDecl{MinOccurs: "99"}) != maxOccursSample || occurrenceCount(xsdElementDecl{MinOccurs: "0"}) != 1 {
		t.Error("occurrence cap/default incorrect")
	}
	if sequenceRequiresAny(nil) || sequenceRequiresAny(&xsdSequence{}) || !sequenceRequiresAny(&xsdSequence{Any: &xsdAny{}}) {
		t.Error("sequenceRequiresAny incorrect")
	}
}
