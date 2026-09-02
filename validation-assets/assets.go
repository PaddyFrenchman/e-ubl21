// Package validationassets contains the immutable validation artefacts and
// provenance metadata shipped by e-ubl21.
package validationassets

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

// Profile identifies one pinned semantic-validation ruleset. Treat profile
// values as immutable release metadata: changing one requires the documented
// asset upgrade review and fixture suite.
type Profile struct {
	Name             string
	Version          string
	XSLTName         string
	EmbeddedXSLTPath string
	RuntimeXSLTPath  string
	SourceRepository string
	SourceRelease    string
	XSLTSHA256       string
}

// en16931 is the single source of truth for the packaged core EN 16931 UBL
// ruleset. The release tag and digest deliberately do not follow a moving
// upstream branch.
var en16931 = newEN16931Profile(
	"1.3.16",
	"39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd",
)

func newEN16931Profile(version, checksum string) Profile {
	const (
		name       = "en16931"
		xsltName   = "EN16931-UBL-validation.xslt"
		runtimeDir = "/opt/e-ubl21/validation-assets"
	)
	embeddedPath := path.Join(name, version, xsltName)
	return Profile{
		Name:             name,
		Version:          version,
		XSLTName:         xsltName,
		EmbeddedXSLTPath: embeddedPath,
		RuntimeXSLTPath:  path.Join(runtimeDir, embeddedPath),
		SourceRepository: "ConnectingEurope/eInvoicing-EN16931",
		SourceRelease:    "validation-" + version,
		XSLTSHA256:       checksum,
	}
}

// EN16931 returns the immutable release metadata for the packaged core EN
// 16931 UBL ruleset. Returning a value prevents callers from changing the
// package's source of truth.
func EN16931() Profile {
	return en16931
}

// Embed versioned stylesheets without selecting a version here. The profile
// above is the only version selector and verification rejects any mismatch.
//
//go:embed en16931/*/*.xslt peppol saxon/saxon-config.xml schematron
var files embed.FS

// OpenEN16931XSLT opens the exact stylesheet represented by EN16931. The
// embedded copy makes the artefact available to Go verification without any
// network or machine-local dependency.
func OpenEN16931XSLT() (fs.File, error) {
	return files.Open(en16931.EmbeddedXSLTPath)
}

// VerifyEN16931 proves that the checked-in runtime artefact has the digest
// recorded in the EN16931 profile.
func VerifyEN16931() error {
	content, err := files.ReadFile(en16931.EmbeddedXSLTPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s stylesheet: %w", en16931.Name, en16931.Version, err)
	}
	actual := fmt.Sprintf("%x", sha256.Sum256(content))
	if actual != en16931.XSLTSHA256 {
		return fmt.Errorf("verify %s profile %s stylesheet: SHA-256 mismatch: expected %s, got %s", en16931.Name, en16931.Version, en16931.XSLTSHA256, actual)
	}
	return nil
}

// StageEN16931 verifies and writes only the selected stylesheet beneath root,
// preserving its profile-relative path. Container builds use a fixed staging
// root, so their COPY instruction never declares an EN 16931 version.
func StageEN16931(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("stage %s profile: destination root is empty", en16931.Name)
	}
	if err := VerifyEN16931(); err != nil {
		return "", err
	}
	content, err := files.ReadFile(en16931.EmbeddedXSLTPath)
	if err != nil {
		return "", fmt.Errorf("read staged %s stylesheet: %w", en16931.Name, err)
	}
	destination := filepath.Join(root, filepath.FromSlash(en16931.EmbeddedXSLTPath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", fmt.Errorf("create %s asset directory: %w", en16931.Name, err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		return "", fmt.Errorf("stage %s stylesheet: %w", en16931.Name, err)
	}
	return destination, nil
}

// StagePeppol writes the checksum-verified compiled Peppol XSLT beneath root.
func StagePeppol(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("stage %s profile: destination root is empty", peppol.Name)
	}
	if err := VerifyPeppol(); err != nil {
		return "", err
	}
	content, err := files.ReadFile(peppol.EmbeddedXSLTPath)
	if err != nil {
		return "", fmt.Errorf("read staged %s stylesheet: %w", peppol.Name, err)
	}
	destination := filepath.Join(root, filepath.FromSlash(peppol.EmbeddedXSLTPath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return "", fmt.Errorf("create %s asset directory: %w", peppol.Name, err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		return "", fmt.Errorf("stage %s stylesheet: %w", peppol.Name, err)
	}
	return destination, nil
}

// StageRuntimeAssets stages every source-controlled file needed by the
// semantic runtime. Downloaded JARs are handled separately and are also
// checksum-verified before entering the image.
func StageRuntimeAssets(root string) ([]string, error) {
	configPath := "saxon/saxon-config.xml"
	content, err := files.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("read Saxon configuration: %w", err)
	}
	actualConfigSHA256 := fmt.Sprintf("%x", sha256.Sum256(content))
	if actualConfigSHA256 != saxon.ConfigurationSHA256 {
		return nil, fmt.Errorf("verify Saxon configuration: SHA-256 mismatch: expected %s, got %s", saxon.ConfigurationSHA256, actualConfigSHA256)
	}
	xslt, err := StageEN16931(root)
	if err != nil {
		return nil, err
	}
	peppolXSLT, err := StagePeppol(root)
	if err != nil {
		return nil, err
	}
	destination := filepath.Join(root, filepath.FromSlash(configPath))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nil, fmt.Errorf("create Saxon configuration directory: %w", err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		return nil, fmt.Errorf("stage Saxon configuration: %w", err)
	}
	return []string{xslt, peppolXSLT, destination}, nil
}
