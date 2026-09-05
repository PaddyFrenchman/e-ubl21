package validationassets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

const (
	peppolReferencePath   = "peppol-reference/3.0.21.json"
	peppolReferenceSHA256 = "6b1b3d71cbe91eec12272281367fef01c2e7c1aaaa123c8d61cebfedb5292d50"
)

// PeppolReferenceManifest is the reviewed external-behaviour contract for the
// pinned release. It is deliberately separate from the generated corpus
// manifest so changing a generator cannot silently redefine expected results.
type PeppolReferenceManifest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Release       PeppolReferenceRelease `json:"release"`
	Inventory     PeppolRuleInventory    `json:"inventory"`
	Cases         []PeppolReferenceCase  `json:"cases"`
}

type PeppolReferenceRelease struct {
	BISVersion                 string `json:"bisVersion"`
	ValidationArtefactsVersion string `json:"validationArtefactsVersion"`
	SourceCommit               string `json:"sourceCommit"`
	SourceSchematronSHA256     string `json:"sourceSchematronSHA256"`
	CompiledXSLTSHA256         string `json:"compiledXSLTSHA256"`
}

// PeppolRuleInventory freezes the executable constructs and namespace bindings
// expected in both the official Schematron source and compiled stylesheet.
type PeppolRuleInventory struct {
	ActivePatterns int               `json:"activePatterns"`
	Rules          int               `json:"rules"`
	Asserts        int               `json:"asserts"`
	Reports        int               `json:"reports"`
	Namespaces     map[string]string `json:"namespaces"`
}

type PeppolReferenceCase struct {
	ID         string                 `json:"id"`
	Provenance string                 `json:"provenance"`
	Path       string                 `json:"path"`
	SHA256     string                 `json:"sha256"`
	Outcome    string                 `json:"outcome"`
	Issues     []PeppolReferenceIssue `json:"issues"`
}

type PeppolReferenceIssue struct {
	Kind     string `json:"kind"`
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity"`
}

// PeppolReference returns a fresh, strictly decoded copy of the reviewed
// reference contract after verifying its checksum and pinned release binding.
func PeppolReference() (PeppolReferenceManifest, error) {
	return readPeppolReferenceFS(files)
}

// VerifyPeppolReference verifies the frozen source inventory and proves that
// compilation retained the same active patterns, rules, assertions, reports,
// namespace bindings, rule IDs, severities, and diagnostic references.
func VerifyPeppolReference() error {
	manifest, err := readPeppolReferenceFS(files)
	if err != nil {
		return err
	}
	source, err := fs.ReadFile(files, peppol.SchematronPath)
	if err != nil {
		return fmt.Errorf("read Peppol source for reference inventory: %w", err)
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(source)); actual != manifest.Release.SourceSchematronSHA256 {
		return fmt.Errorf("verify Peppol reference source: SHA-256 mismatch: expected %s, got %s", manifest.Release.SourceSchematronSHA256, actual)
	}
	compiled, err := fs.ReadFile(files, peppol.EmbeddedXSLTPath)
	if err != nil {
		return fmt.Errorf("read Peppol XSLT for reference inventory: %w", err)
	}
	if err := verifyPeppolSemanticPreservation(source, compiled); err != nil {
		return fmt.Errorf("verify Peppol reference semantics: %w", err)
	}
	sourceInventory, err := readPeppolRuleInventory(source, true)
	if err != nil {
		return fmt.Errorf("read Peppol source inventory: %w", err)
	}
	if err := comparePeppolInventory("source", manifest.Inventory, sourceInventory); err != nil {
		return err
	}
	compiledInventory, err := readPeppolRuleInventory(compiled, false)
	if err != nil {
		return fmt.Errorf("read compiled Peppol inventory: %w", err)
	}
	return comparePeppolInventory("compiled XSLT", manifest.Inventory, compiledInventory)
}

func readPeppolReferenceFS(fsys fs.FS) (PeppolReferenceManifest, error) {
	content, err := fs.ReadFile(fsys, peppolReferencePath)
	if err != nil {
		return PeppolReferenceManifest{}, fmt.Errorf("read Peppol reference manifest: %w", err)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != peppolReferenceSHA256 {
		return PeppolReferenceManifest{}, fmt.Errorf("verify Peppol reference manifest: SHA-256 mismatch: expected %s, got %s", peppolReferenceSHA256, actual)
	}
	var manifest PeppolReferenceManifest
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("parse Peppol reference manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return manifest, fmt.Errorf("parse Peppol reference manifest trailer: %v", err)
	}
	if err := validatePeppolReference(manifest); err != nil {
		return manifest, fmt.Errorf("validate Peppol reference manifest: %w", err)
	}
	return manifest, nil
}

func validatePeppolReference(manifest PeppolReferenceManifest) error {
	if manifest.SchemaVersion != 1 {
		return fmt.Errorf("schemaVersion = %d, want 1", manifest.SchemaVersion)
	}
	wantRelease := PeppolReferenceRelease{
		BISVersion: peppol.Version, ValidationArtefactsVersion: peppol.ValidationArtefactsVersion,
		SourceCommit: peppol.SourceCommit, CompiledXSLTSHA256: peppol.XSLTSHA256,
	}
	if manifest.Release.BISVersion != wantRelease.BISVersion ||
		manifest.Release.ValidationArtefactsVersion != wantRelease.ValidationArtefactsVersion ||
		manifest.Release.SourceCommit != wantRelease.SourceCommit ||
		manifest.Release.CompiledXSLTSHA256 != wantRelease.CompiledXSLTSHA256 {
		return fmt.Errorf("release metadata does not match the pinned Peppol profile")
	}
	if !validSHA256(manifest.Release.SourceSchematronSHA256) {
		return fmt.Errorf("sourceSchematronSHA256 is invalid")
	}
	if len(manifest.Cases) == 0 {
		return fmt.Errorf("cases are empty")
	}
	seen := make(map[string]bool, len(manifest.Cases))
	wantOfficial := make(map[string]bool, len(peppolExampleNames))
	for _, name := range peppolExampleNames {
		wantOfficial[name] = false
	}
	official := 0
	for _, testCase := range manifest.Cases {
		if testCase.ID == "" || seen[testCase.ID] {
			return fmt.Errorf("case ID %q is empty or duplicated", testCase.ID)
		}
		seen[testCase.ID] = true
		if testCase.Provenance == "official-example" {
			official++
			name := strings.TrimPrefix(path.Base(testCase.Path), "OpenPeppol-")
			if _, ok := wantOfficial[name]; !ok || wantOfficial[name] {
				return fmt.Errorf("case %s does not identify one unique pinned official example", testCase.ID)
			}
			wantOfficial[name] = true
		} else if testCase.Provenance != "reviewed-mutation" {
			return fmt.Errorf("case %s has unsupported provenance %q", testCase.ID, testCase.Provenance)
		}
		if testCase.Path == "" || path.IsAbs(testCase.Path) || path.Clean(testCase.Path) != testCase.Path || strings.HasPrefix(testCase.Path, "../") {
			return fmt.Errorf("case %s has unsafe path %q", testCase.ID, testCase.Path)
		}
		if !validSHA256(testCase.SHA256) {
			return fmt.Errorf("case %s has invalid SHA-256", testCase.ID)
		}
		if testCase.Outcome != "passed" && testCase.Outcome != "standards-invalid" {
			return fmt.Errorf("case %s has unsupported outcome %q", testCase.ID, testCase.Outcome)
		}
		if (testCase.Outcome == "passed") != (len(testCase.Issues) == 0) {
			return fmt.Errorf("case %s outcome and issues disagree", testCase.ID)
		}
		for _, issue := range testCase.Issues {
			if issue.Kind != "failed-assert" || issue.RuleID == "" || issue.Severity == "" {
				return fmt.Errorf("case %s has incomplete semantic issue", testCase.ID)
			}
		}
	}
	if official != len(peppolExampleNames) {
		return fmt.Errorf("official example cases = %d, want %d", official, len(peppolExampleNames))
	}
	return nil
}

func validSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func readPeppolRuleInventory(content []byte, source bool) (PeppolRuleInventory, error) {
	result := PeppolRuleInventory{Namespaces: make(map[string]string)}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return result, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if source && start.Name.Space == schematronNamespace {
			switch start.Name.Local {
			case "pattern":
				result.ActivePatterns++
			case "rule":
				result.Rules++
			case "assert":
				result.Asserts++
			case "report":
				result.Reports++
			case "ns":
				result.Namespaces[xmlAttribute(start.Attr, "prefix")] = xmlAttribute(start.Attr, "uri")
			}
		}
		if !source && start.Name.Space == svrlNamespace {
			switch start.Name.Local {
			case "active-pattern":
				result.ActivePatterns++
			case "fired-rule":
				result.Rules++
			case "failed-assert":
				result.Asserts++
			case "successful-report":
				result.Reports++
			}
		}
		if !source && start.Name.Space == xsltNamespace && (start.Name.Local == "transform" || start.Name.Local == "stylesheet") {
			for _, attr := range start.Attr {
				if attr.Name.Space == "xmlns" {
					result.Namespaces[attr.Name.Local] = attr.Value
				}
			}
		}
	}
	return result, nil
}

func comparePeppolInventory(label string, want, got PeppolRuleInventory) error {
	if got.ActivePatterns != want.ActivePatterns || got.Rules != want.Rules || got.Asserts != want.Asserts || got.Reports != want.Reports {
		return fmt.Errorf("%s Peppol inventory patterns/rules/asserts/reports = %d/%d/%d/%d, want %d/%d/%d/%d",
			label, got.ActivePatterns, got.Rules, got.Asserts, got.Reports,
			want.ActivePatterns, want.Rules, want.Asserts, want.Reports)
	}
	for prefix, uri := range want.Namespaces {
		if got.Namespaces[prefix] != uri {
			return fmt.Errorf("%s Peppol namespace %q = %q, want %q", label, prefix, got.Namespaces[prefix], uri)
		}
	}
	return nil
}
