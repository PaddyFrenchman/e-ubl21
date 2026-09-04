package validationassets

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

const (
	peppolSchematronRelativePath = "rules/PEPPOL-EN16931-UBL.sch"
	peppolProvenanceRelativePath = "provenance.json"
	peppolManifestRelativePath   = "sha256.txt"
	peppolRuntimeDirectory       = "runtime"
)

// PeppolProfile identifies the complete immutable source bundle and accepted
// executable XSLT for Peppol BIS Billing. Compilation remains a separate,
// reproducible lifecycle step whose inputs and output are frozen here.
type PeppolProfile struct {
	Name                               string
	Version                            string
	PublicationDate                    string
	MandatoryFrom                      string
	ValidationArtefactsVersion         string
	ValidationArtefactsPublicationDate string
	CodeListPublicationDate            string
	SourceRepository                   string
	SourceCommit                       string
	SourceTree                         string
	SourceArchiveURL                   string
	SourceArchiveSHA256                string
	ManifestSHA256                     string
	EmbeddedRoot                       string
	SchematronPath                     string
	EmbeddedXSLTPath                   string
	RuntimeXSLTPath                    string
	XSLTSHA256                         string
	CompilerMetadataPath               string
}

// PeppolExample is one byte-for-byte official example from the pinned
// OpenPeppol source commit. The examples are validation inputs, not locally
// authored substitutes for external conformity evidence.
type PeppolExample struct {
	Name          string
	XML           []byte
	SourceURL     string
	SourceVersion string
	SourceSHA256  string
	License       string
}

var peppolExampleNames = []string{
	"Allowance-example.xml",
	"Vat-category-S.xml",
	"base-creditnote-correction.xml",
	"base-example.xml",
	"base-example_profile02.xml",
	"base-negative-inv-correction.xml",
	"sales-order-example.xml",
	"vat-category-E.xml",
	"vat-category-O.xml",
	"vat-category-Z.xml",
}

var peppol = newPeppolProfile(
	"3.0.21",
	"705e83fa494c052254c5cd4cbd2fb42baa975d559774f6cc928dbdddd10ab29e",
)

func newPeppolProfile(version, manifestSHA256 string) PeppolProfile {
	const sourceCommit = "806866bd2bd91d7e9623b68f08164e8fbe9e67a0"
	embeddedRoot := path.Join("peppol", version)
	return PeppolProfile{
		Name:                               "peppol-bis-billing",
		Version:                            version,
		PublicationDate:                    "2026-05-20",
		MandatoryFrom:                      "2026-08-17",
		ValidationArtefactsVersion:         "1.3.16",
		ValidationArtefactsPublicationDate: "2026-04-10",
		CodeListPublicationDate:            "2026-03-06",
		SourceRepository:                   "OpenPEPPOL/peppol-bis-invoice-3",
		SourceCommit:                       sourceCommit,
		SourceTree:                         "a8acc94f1216279839547e1670585fac62dfc122",
		SourceArchiveURL:                   "https://codeload.github.com/OpenPEPPOL/peppol-bis-invoice-3/tar.gz/" + sourceCommit,
		SourceArchiveSHA256:                "a843cd3808c6e95565eded1a535a5a40a88ad6f92e009d2b1816a4818a82299e",
		ManifestSHA256:                     manifestSHA256,
		EmbeddedRoot:                       embeddedRoot,
		SchematronPath:                     path.Join(embeddedRoot, peppolSchematronRelativePath),
		EmbeddedXSLTPath:                   path.Join(embeddedRoot, peppolRuntimeDirectory, "PEPPOL-EN16931-UBL-validation.xslt"),
		RuntimeXSLTPath:                    path.Join("/opt/e-ubl21/validation-assets", embeddedRoot, peppolRuntimeDirectory, "PEPPOL-EN16931-UBL-validation.xslt"),
		XSLTSHA256:                         "56bbd05ea26b4ffb859e0c15fcd2c1c8782870b530aae06a828ddb0bdf9b66aa",
		CompilerMetadataPath:               path.Join(embeddedRoot, peppolRuntimeDirectory, "compiler.json"),
	}
}

// Peppol returns a copy of the immutable Peppol asset metadata.
func Peppol() PeppolProfile {
	return peppol
}

// OpenPeppolSchematron opens the exact ISO Schematron source represented by
// Peppol. The future deterministic compiler consumes this verified input.
func OpenPeppolSchematron() (fs.File, error) {
	return files.Open(peppol.SchematronPath)
}

// OpenPeppolXSLT opens the accepted deterministic runtime artefact.
func OpenPeppolXSLT() (fs.File, error) {
	return files.Open(peppol.EmbeddedXSLTPath)
}

// PeppolExamples returns verified independent copies of every official UBL
// example packaged from the pinned release. Redistribution terms remain under
// review and are recorded explicitly rather than inferred from public access.
func PeppolExamples() ([]PeppolExample, error) {
	if err := VerifyPeppolSources(); err != nil {
		return nil, err
	}
	examples := make([]PeppolExample, 0, len(peppolExampleNames))
	for _, name := range peppolExampleNames {
		examplePath := path.Join(peppol.EmbeddedRoot, "examples", name)
		content, err := files.ReadFile(examplePath)
		if err != nil {
			return nil, fmt.Errorf("read official Peppol example %s: %w", name, err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(content))
		examples = append(examples, PeppolExample{
			Name: name, XML: append([]byte(nil), content...),
			SourceURL:     "https://github.com/" + peppol.SourceRepository + "/blob/" + peppol.SourceCommit + "/rules/examples/" + name,
			SourceVersion: peppol.SourceCommit, SourceSHA256: digest,
			License: "Redistribution terms under review; see standard/peppol/Peppol-licensing.md.",
		})
	}
	return examples, nil
}

// VerifyPeppolSources verifies the immutable official source bundle without
// considering the separately generated runtime artefact.
func VerifyPeppolSources() error {
	return verifyPeppolSourcesFS(files, peppol)
}

// VerifyPeppol verifies both the official source bundle and the deterministic
// executable XSLT derived from it.
func VerifyPeppol() error {
	return verifyPeppolFS(files, peppol)
}

type peppolProvenance struct {
	Profile                            string            `json:"profile"`
	BISVersion                         string            `json:"bis_version"`
	PublicationDate                    string            `json:"publication_date"`
	MandatoryFrom                      string            `json:"mandatory_from"`
	ValidationArtefactsVersion         string            `json:"validation_artefacts_version"`
	ValidationArtefactsPublicationDate string            `json:"validation_artefacts_publication_date"`
	CodeListPublicationDate            string            `json:"code_list_publication_date"`
	SourceRepository                   string            `json:"source_repository"`
	SourceCommit                       string            `json:"source_commit"`
	SourceTree                         string            `json:"source_tree"`
	SourceArchiveURL                   string            `json:"source_archive_url"`
	SourceArchiveSHA256                string            `json:"source_archive_sha256"`
	SourcePaths                        map[string]string `json:"source_paths"`
	AssetCounts                        map[string]int    `json:"asset_counts"`
	Excluded                           []string          `json:"excluded"`
}

func verifyPeppolFS(fsys fs.FS, profile PeppolProfile) error {
	if err := verifyPeppolSourcesFS(fsys, profile); err != nil {
		return err
	}
	return verifyPeppolCompilationFS(fsys, profile)
}

func verifyPeppolSourcesFS(fsys fs.FS, profile PeppolProfile) error {
	manifestPath := path.Join(profile.EmbeddedRoot, peppolManifestRelativePath)
	manifest, err := fs.ReadFile(fsys, manifestPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s manifest: %w", profile.Name, profile.Version, err)
	}
	manifestDigest := fmt.Sprintf("%x", sha256.Sum256(manifest))
	if manifestDigest != profile.ManifestSHA256 {
		return fmt.Errorf("verify %s profile %s manifest: SHA-256 mismatch: expected %s, got %s", profile.Name, profile.Version, profile.ManifestSHA256, manifestDigest)
	}

	entries := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(manifest)))
	for scanner.Scan() {
		line := scanner.Text()
		checksum, relativePath, ok := strings.Cut(line, "  ")
		if !ok || len(checksum) != sha256.Size*2 {
			return fmt.Errorf("verify %s profile %s manifest: malformed line %q", profile.Name, profile.Version, line)
		}
		if _, err := hex.DecodeString(checksum); err != nil {
			return fmt.Errorf("verify %s profile %s manifest checksum for %q: %w", profile.Name, profile.Version, relativePath, err)
		}
		if relativePath == "" || relativePath != path.Clean(relativePath) || relativePath == ".." || strings.HasPrefix(relativePath, "../") || strings.HasPrefix(relativePath, "/") || strings.Contains(relativePath, "\\") {
			return fmt.Errorf("verify %s profile %s manifest: unsafe asset path %q", profile.Name, profile.Version, relativePath)
		}
		if _, exists := entries[relativePath]; exists {
			return fmt.Errorf("verify %s profile %s manifest: duplicate asset %q", profile.Name, profile.Version, relativePath)
		}
		content, err := fs.ReadFile(fsys, path.Join(profile.EmbeddedRoot, relativePath))
		if err != nil {
			return fmt.Errorf("read %s profile %s asset %q: %w", profile.Name, profile.Version, relativePath, err)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(content))
		if actual != checksum {
			return fmt.Errorf("verify %s profile %s asset %q: SHA-256 mismatch: expected %s, got %s", profile.Name, profile.Version, relativePath, checksum, actual)
		}
		entries[relativePath] = checksum
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s profile %s manifest: %w", profile.Name, profile.Version, err)
	}

	if err := verifyPeppolInventory(entries); err != nil {
		return fmt.Errorf("verify %s profile %s inventory: %w", profile.Name, profile.Version, err)
	}
	if err := verifyPeppolEmbeddedInventory(fsys, profile, entries); err != nil {
		return err
	}
	return verifyPeppolProvenance(fsys, profile)
}

func verifyPeppolInventory(entries map[string]string) error {
	counts := map[string]int{"codelists/": 0, "examples/": 0, "rules/": 0}
	for relativePath := range entries {
		switch {
		case relativePath == peppolProvenanceRelativePath:
		case strings.HasPrefix(relativePath, "codelists/") && strings.HasSuffix(relativePath, ".xml"):
			counts["codelists/"]++
		case strings.HasPrefix(relativePath, "examples/") && strings.HasSuffix(relativePath, ".xml"):
			counts["examples/"]++
		case relativePath == peppolSchematronRelativePath:
			counts["rules/"]++
		default:
			return fmt.Errorf("unexpected asset %q", relativePath)
		}
	}
	if len(entries) != 29 || counts["codelists/"] != 17 || counts["examples/"] != 10 || counts["rules/"] != 1 {
		return fmt.Errorf("asset counts = total:%d schematron:%d code_lists:%d examples:%d, want total:29 schematron:1 code_lists:17 examples:10", len(entries), counts["rules/"], counts["codelists/"], counts["examples/"])
	}
	if _, ok := entries[peppolProvenanceRelativePath]; !ok {
		return fmt.Errorf("missing %s", peppolProvenanceRelativePath)
	}
	return nil
}

func verifyPeppolEmbeddedInventory(fsys fs.FS, profile PeppolProfile, entries map[string]string) error {
	seen := 0
	err := fs.WalkDir(fsys, profile.EmbeddedRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath := strings.TrimPrefix(filePath, profile.EmbeddedRoot+"/")
		if relativePath == peppolManifestRelativePath || strings.HasPrefix(relativePath, peppolRuntimeDirectory+"/") {
			return nil
		}
		seen++
		if _, ok := entries[relativePath]; !ok {
			return fmt.Errorf("unmanifested asset %q", relativePath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("verify %s profile %s embedded inventory: %w", profile.Name, profile.Version, err)
	}
	if seen != len(entries) {
		return fmt.Errorf("verify %s profile %s embedded inventory: found %d assets, manifest records %d", profile.Name, profile.Version, seen, len(entries))
	}
	return nil
}

func verifyPeppolProvenance(fsys fs.FS, profile PeppolProfile) error {
	content, err := fs.ReadFile(fsys, path.Join(profile.EmbeddedRoot, peppolProvenanceRelativePath))
	if err != nil {
		return fmt.Errorf("read %s profile %s provenance: %w", profile.Name, profile.Version, err)
	}
	var provenance peppolProvenance
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&provenance); err != nil {
		return fmt.Errorf("parse %s profile %s provenance: %w", profile.Name, profile.Version, err)
	}
	required := map[string]string{
		"profile":                               provenance.Profile,
		"bis_version":                           provenance.BISVersion,
		"publication_date":                      provenance.PublicationDate,
		"mandatory_from":                        provenance.MandatoryFrom,
		"validation_artefacts_version":          provenance.ValidationArtefactsVersion,
		"validation_artefacts_publication_date": provenance.ValidationArtefactsPublicationDate,
		"code_list_publication_date":            provenance.CodeListPublicationDate,
		"source_repository":                     provenance.SourceRepository,
		"source_commit":                         provenance.SourceCommit,
		"source_tree":                           provenance.SourceTree,
		"source_archive_url":                    provenance.SourceArchiveURL,
		"source_archive_sha256":                 provenance.SourceArchiveSHA256,
	}
	expected := map[string]string{
		"profile":                               profile.Name,
		"bis_version":                           profile.Version,
		"publication_date":                      profile.PublicationDate,
		"mandatory_from":                        profile.MandatoryFrom,
		"validation_artefacts_version":          profile.ValidationArtefactsVersion,
		"validation_artefacts_publication_date": profile.ValidationArtefactsPublicationDate,
		"code_list_publication_date":            profile.CodeListPublicationDate,
		"source_repository":                     profile.SourceRepository,
		"source_commit":                         profile.SourceCommit,
		"source_tree":                           profile.SourceTree,
		"source_archive_url":                    profile.SourceArchiveURL,
		"source_archive_sha256":                 profile.SourceArchiveSHA256,
	}
	for field, want := range expected {
		if required[field] != want {
			return fmt.Errorf("verify %s profile %s provenance: %s = %q, want %q", profile.Name, profile.Version, field, required[field], want)
		}
	}
	if provenance.SourcePaths["schematron"] != "rules/sch/PEPPOL-EN16931-UBL.sch" ||
		provenance.SourcePaths["code_lists"] != "structure/codelist/*.xml" ||
		provenance.SourcePaths["examples"] != "rules/examples/*.xml" {
		return fmt.Errorf("verify %s profile %s provenance: unexpected source paths", profile.Name, profile.Version)
	}
	if provenance.AssetCounts["schematron"] != 1 || provenance.AssetCounts["code_lists"] != 17 || provenance.AssetCounts["examples"] != 10 || len(provenance.AssetCounts) != 3 {
		return fmt.Errorf("verify %s profile %s provenance: unexpected asset counts", profile.Name, profile.Version)
	}
	return nil
}
