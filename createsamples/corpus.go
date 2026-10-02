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
	":(glob)peppol/*.go",
	":(exclude,glob)peppol/*_test.go",
	"validation-assets/peppol.go",
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
	Provenance     fixtureProvenance      `json:"provenance"`
	Rulesets       *fixtureRulesets       `json:"rulesets,omitempty"`
	Mutation       *fixtureMutation       `json:"mutation,omitempty"`
	ExpectedFinal  string                 `json:"expectedFinal"`
	ExpectedStages []fixtureExpectedStage `json:"expectedStages"`
	Tags           []string               `json:"tags"`
	// StructuralTargets identifies the #161 schema targets this fixture is
	// intended to exercise. It is present only in the structural UBL corpus.
	StructuralTargets []string `json:"structuralTargets,omitempty"`
}

type fixtureStandards struct {
	UBL     string  `json:"ubl"`
	EN16931 *string `json:"en16931"`
	Peppol  *string `json:"peppol"`
}

type fixtureProvenance struct {
	Kind             string `json:"kind"`
	Generator        string `json:"generator,omitempty"`
	GeneratorVersion string `json:"generatorVersion,omitempty"`
	Publisher        string `json:"publisher,omitempty"`
	SourceURL        string `json:"sourceUrl,omitempty"`
	SourceVersion    string `json:"sourceVersion,omitempty"`
	SourceSHA256     string `json:"sourceSha256,omitempty"`
	SourceMember     any    `json:"sourceMember,omitempty"`
	Extraction       string `json:"extraction,omitempty"`
	License          string `json:"license,omitempty"`
}

type fixtureRulesets struct {
	EN16931 *fixtureRuleset `json:"en16931,omitempty"`
	Peppol  *fixtureRuleset `json:"peppol,omitempty"`
}

type fixtureRuleset struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
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
					Provenance:     fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
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

// generateUBLStructuralCorpus produces purpose-specific fixtures instead of
// trying to force every legal structural variation into one document. The
// variants share the ordinary manifest/provenance contract and remain wholly
// generated from the pinned schema graph.
func generateUBLStructuralCorpus(report schemaInventory, index schemaIndex, xsdDir, generatorVersion string) (generatedCorpus, error) {
	choiceVariants := maxChoiceAlternatives(index)
	if choiceVariants < 1 {
		choiceVariants = 1
	}
	fixtures := make([]generatedFixture, 0, len(report.MainFiles)*(choiceVariants+3))
	for _, schema := range report.MainFiles {
		root := schema.RootElement
		if root == "" {
			root = strings.TrimSuffix(filepath.Base(schema.File), filepath.Ext(schema.File))
		}
		choicePlans, err := structuralChoicePlans(index, root, xsdDir, schema.File)
		if err != nil {
			return generatedCorpus{}, fmt.Errorf("count %s choice sites: %w", root, err)
		}
		add := func(family string, choices map[int]int, recursiveDepth, facetBoundary, repeatCount int, repeatElement string, repeatElements map[string]int, tags ...string) error {
			var content string
			var err error
			if len(repeatElements) > 0 {
				content, err = generateLargeCollectionsStructuralSample(index, root, xsdDir, schema.File, repeatElements)
			} else if repeatElement != "" {
				content, err = generateLargeCollectionStructuralSample(index, root, xsdDir, schema.File, repeatElement, repeatCount)
			} else if repeatCount > 0 {
				content, err = generateCardinalityStructuralSample(index, root, xsdDir, schema.File, repeatCount)
			} else if facetBoundary > 0 {
				content, err = generateFacetBoundaryStructuralSample(index, root, xsdDir, schema.File, facetBoundary)
			} else {
				content, err = generateTargetedStructuralSample(index, root, xsdDir, schema.File, choices, recursiveDepth)
			}
			if err != nil {
				return fmt.Errorf("generate %s %s: %w", root, family, err)
			}
			if err := validateStructuralCandidate([]byte(content), filepath.Join(xsdDir, "maindoc", schema.File)); err != nil {
				return fmt.Errorf("validate %s %s: %w", root, family, err)
			}
			fixtures = append(fixtures, generatedFixture{entry: fixtureManifestEntry{
				ID:             fmt.Sprintf("ubl21.%s.%s", strings.ToLower(root), family),
				DocumentType:   root,
				Profile:        "ubl21",
				File:           filepath.ToSlash(filepath.Join(root, fmt.Sprintf("%s-%s.xml", root, family))),
				SHA256:         fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
				Standards:      fixtureStandards{UBL: "2.1"},
				Provenance:     fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
				ExpectedFinal:  "valid",
				ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
				Tags:           append([]string{"positive", "ubl21", "structural", family}, tags...),
				// The manifest records the stable intended schema-root/family
				// targets.  The complete fixture-to-schema-target mapping belongs
				// in coverage.json; duplicating it in every manifest entry makes a
				// versioned corpus needlessly enormous.
				StructuralTargets: []string{"schema-root:" + root, "family:" + family},
			}, content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)})
			return nil
		}
		minimal, err := generateMinimalSample(index, root, xsdDir, schema.File)
		if err != nil {
			return generatedCorpus{}, fmt.Errorf("generate %s minimal: %w", root, err)
		}
		fixtures = append(fixtures, generatedFixture{entry: fixtureManifestEntry{
			ID:                fmt.Sprintf("ubl21.%s.minimal", strings.ToLower(root)),
			DocumentType:      root,
			Profile:           "ubl21",
			File:              filepath.ToSlash(filepath.Join(root, fmt.Sprintf("%s-minimal.xml", root))),
			SHA256:            fmt.Sprintf("%x", sha256.Sum256([]byte(minimal))),
			Standards:         fixtureStandards{UBL: "2.1"},
			Provenance:        fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
			ExpectedFinal:     "valid",
			ExpectedStages:    []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
			Tags:              []string{"positive", "ubl21", "structural", "minimal", "optional-absent", "cardinality", "occurrences-0"},
			StructuralTargets: []string{"schema-root:" + root, "family:minimal"},
		}, content: []byte(minimal), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)})
		if err := add("optional-rich", nil, 1, 0, 0, "", nil, "optional-elements", "optional-attributes", "cardinality", "occurrences-1"); err != nil {
			return generatedCorpus{}, err
		}
		// XMLDSig/XAdES is reachable only through UBL's extension wildcard.
		// Keep a focused declared-signature fixture alongside the generic foreign
		// extension payload so that schema choices are measured against real UBL
		// extension content rather than an invented private namespace.
		if root == "Invoice" {
			for variant, family := range []string{
				"key-name", "key-name-transform-wildcard",
				"rsa-key-value", "rsa-key-value-transform-wildcard",
				"retrieval-method", "retrieval-method-transform-wildcard",
				"x509-subject-name", "x509-subject-name-transform-wildcard",
				"pgp-key-id", "pgp-key-id-transform-wildcard",
				"pgp-key-packet", "pgp-key-packet-transform-wildcard",
				"spki-data", "spki-data-transform-wildcard",
				"management-data", "management-data-transform-wildcard",
				"key-info-wildcard", "key-info-wildcard-transform-wildcard",
				"dsa-key-value", "dsa-key-value-transform-wildcard",
				"key-value-wildcard", "key-value-wildcard-transform-wildcard",
				"x509-issuer-serial", "x509-issuer-serial-transform-wildcard",
				"x509-ski", "x509-ski-transform-wildcard",
				"x509-certificate", "x509-certificate-transform-wildcard",
				"x509-crl", "x509-crl-transform-wildcard",
				"x509-wildcard", "x509-wildcard-transform-wildcard",
			} {
				content, err := generateSignatureExtensionStructuralSample(index, root, xsdDir, schema.File, variant)
				if err != nil {
					return generatedCorpus{}, fmt.Errorf("generate %s signature extension %s: %w", root, family, err)
				}
				if err := validateStructuralCandidate([]byte(content), filepath.Join(xsdDir, "maindoc", schema.File)); err != nil {
					return generatedCorpus{}, fmt.Errorf("validate %s signature extension %s: %w", root, family, err)
				}
				fixtures = append(fixtures, generatedFixture{entry: fixtureManifestEntry{
					ID:                "ubl21.invoice.signature-extension-" + family,
					DocumentType:      root,
					Profile:           "ubl21",
					File:              filepath.ToSlash(filepath.Join(root, "Invoice-signature-extension-"+family+".xml")),
					SHA256:            fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
					Standards:         fixtureStandards{UBL: "2.1"},
					Provenance:        fixtureProvenance{Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion},
					ExpectedFinal:     "valid",
					ExpectedStages:    []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "passed"}},
					Tags:              []string{"positive", "ubl21", "structural", "signature-extension", "choice-alternative", "wildcard", family},
					StructuralTargets: []string{"schema-root:" + root, "family:signature-extension", "signature-variant:" + family},
				}, content: []byte(content), schemaPath: filepath.Join(xsdDir, "maindoc", schema.File)})
			}
		}
		for planIndex, choices := range choicePlans[1:] {
			if err := add(fmt.Sprintf("choice-path-%03d", planIndex+1), choices, 1, 0, 0, "", nil, "choice-alternative", "targeted"); err != nil {
				return generatedCorpus{}, err
			}
		}
		for boundary, family := range []string{"facet-boundary-low", "facet-boundary-high"} {
			if err := add(family, nil, 1, boundary+1, 0, "", nil, "facet-boundary"); err != nil {
				return generatedCorpus{}, err
			}
		}
		for _, recursiveDepth := range []int{2, 3} {
			if err := add(fmt.Sprintf("recursive-depth-%d", recursiveDepth), nil, recursiveDepth, 0, 0, "", nil, "recursive"); err != nil {
				return generatedCorpus{}, err
			}
		}
		for _, count := range []int{2, 5} {
			if err := add(fmt.Sprintf("cardinality-%d", count), nil, 1, 0, count, "", nil, "cardinality", fmt.Sprintf("occurrences-%d", count)); err != nil {
				return generatedCorpus{}, err
			}
		}
		// These fixtures exercise the progressive high-volume collection tiers
		// seen by the real service without making every repeatable branch of a
		// document enormous.  The smaller tiers make it possible to locate
		// cardinality regressions before the several-hundred-line fixtures.
		for _, large := range []struct {
			document string
			element  string
			count    int
			kind     string
		}{
			{"Invoice", "InvoiceLine", 10, "line-collection"}, {"Invoice", "InvoiceLine", 50, "line-collection"}, {"Invoice", "InvoiceLine", 100, "line-collection"}, {"Invoice", "InvoiceLine", 500, "line-collection"},
			{"CreditNote", "CreditNoteLine", 10, "line-collection"}, {"CreditNote", "CreditNoteLine", 50, "line-collection"}, {"CreditNote", "CreditNoteLine", 100, "line-collection"}, {"CreditNote", "CreditNoteLine", 250, "line-collection"},
			{"Order", "OrderLine", 10, "line-collection"}, {"Order", "OrderLine", 50, "line-collection"}, {"Order", "OrderLine", 100, "line-collection"}, {"Order", "OrderLine", 250, "line-collection"},
			// Non-line collections exercise the aggregate-heavy paths that have
			// historically been easy to overlook in document-size testing.
			{"Invoice", "AllowanceCharge", 50, "aggregate-collection"}, {"Invoice", "TaxTotal", 50, "aggregate-collection"}, {"Invoice", "BillingReference", 50, "aggregate-collection"},
			{"CreditNote", "AllowanceCharge", 50, "aggregate-collection"}, {"CreditNote", "TaxTotal", 50, "aggregate-collection"}, {"CreditNote", "BillingReference", 50, "aggregate-collection"},
			{"Order", "AllowanceCharge", 50, "aggregate-collection"}, {"Order", "TaxTotal", 50, "aggregate-collection"}, {"Order", "ProjectReference", 50, "aggregate-collection"},
			// AdditionalDocumentReference carries an Attachment; FreightInvoice
			// is the main-document root with a repeatable Shipment collection.
			{"Invoice", "AdditionalDocumentReference", 50, "attachment-collection"}, {"Order", "AdditionalDocumentReference", 50, "attachment-collection"},
			{"FreightInvoice", "Shipment", 50, "shipment-collection"},
			// A CatalogueLine carries its Item content, making this a practical
			// high-cardinality item-bearing document without violating Item's
			// own single-occurrence particle in each line.
			{"Catalogue", "CatalogueLine", 100, "item-collection"},
		} {
			if root != large.document {
				continue
			}
			if err := add(fmt.Sprintf("%s-%03d", strings.ToLower(large.element), large.count), nil, 1, 0, large.count, large.element, nil, "cardinality", "large-document", large.kind); err != nil {
				return generatedCorpus{}, err
			}
		}
		if root == "Invoice" {
			if err := add("multi-collection-100", nil, 1, 0, 0, "", map[string]int{"InvoiceLine": 100, "AllowanceCharge": 50, "TaxTotal": 50}, "cardinality", "large-document", "line-collection", "aggregate-collection", "multi-collection"); err != nil {
				return generatedCorpus{}, err
			}
		}
	}
	return buildGeneratedCorpus(fixtures)
}

func validateStructuralCandidate(content []byte, schemaPath string) error {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		return fmt.Errorf("xmllint is required for structural candidate validation: %w", err)
	}
	cmd := exec.Command(xmllint, "--nonet", "--noout", "--schema", schemaPath, "-")
	cmd.Stdin = bytes.NewReader(content)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("XSD validation failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func structuralTargetIDs(index schemaIndex, content []byte) ([]string, error) {
	matches, err := matchFixtureBytes(index, content)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(matches))
	for target := range matches {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets, nil
}

func countChoiceSites(index schemaIndex) int {
	count := 0
	var sequence func(*xsdSequence)
	var choice func(*xsdChoice)
	var all func(*xsdAll)
	sequence = func(p *xsdSequence) {
		if p != nil {
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	choice = func(p *xsdChoice) {
		if p != nil {
			count++
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	all = func(p *xsdAll) {
		if p != nil {
			for i := range p.Sequences {
				sequence(&p.Sequences[i])
			}
			for i := range p.Choices {
				choice(&p.Choices[i])
			}
			for i := range p.Alls {
				all(&p.Alls[i])
			}
		}
	}
	for _, typ := range index.GlobalTypes {
		sequence(typ.Sequence)
		choice(typ.Choice)
		all(typ.All)
	}
	return count
}

func maxChoiceAlternatives(index schemaIndex) int {
	max := 0
	var visitSequence func(*xsdSequence)
	var visitAll func(*xsdAll)
	var visitChoice func(*xsdChoice)
	visitSequence = func(sequence *xsdSequence) {
		if sequence == nil {
			return
		}
		for i := range sequence.Sequences {
			visitSequence(&sequence.Sequences[i])
		}
		for i := range sequence.Choices {
			visitChoice(&sequence.Choices[i])
		}
		for i := range sequence.Alls {
			visitAll(&sequence.Alls[i])
		}
	}
	visitAll = func(all *xsdAll) {
		if all == nil {
			return
		}
		for i := range all.Sequences {
			visitSequence(&all.Sequences[i])
		}
		for i := range all.Choices {
			visitChoice(&all.Choices[i])
		}
		for i := range all.Alls {
			visitAll(&all.Alls[i])
		}
	}
	visitChoice = func(choice *xsdChoice) {
		if choice == nil {
			return
		}
		alternatives := len(choice.Elements) + len(choice.Sequences) + len(choice.Choices) + len(choice.Alls)
		if alternatives > max {
			max = alternatives
		}
		for i := range choice.Sequences {
			visitSequence(&choice.Sequences[i])
		}
		for i := range choice.Choices {
			visitChoice(&choice.Choices[i])
		}
		for i := range choice.Alls {
			visitAll(&choice.Alls[i])
		}
	}
	for _, typ := range index.GlobalTypes {
		visitSequence(typ.Sequence)
		visitChoice(typ.Choice)
		visitAll(typ.All)
	}
	return max
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
		expectsXSDPass := fixture.entry.ExpectedFinal == "valid"
		if len(fixture.entry.ExpectedStages) > 0 {
			expectsXSDPass = fixture.entry.ExpectedStages[0].Outcome == "passed"
		}
		if expectsXSDPass {
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
