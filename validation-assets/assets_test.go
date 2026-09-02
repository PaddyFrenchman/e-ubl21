package validationassets

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEN16931PinnedMetadata(t *testing.T) {
	want := Profile{
		Name:             "en16931",
		Version:          "1.3.16",
		XSLTName:         "EN16931-UBL-validation.xslt",
		EmbeddedXSLTPath: "en16931/1.3.16/EN16931-UBL-validation.xslt",
		RuntimeXSLTPath:  "/opt/e-ubl21/validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt",
		SourceRepository: "ConnectingEurope/eInvoicing-EN16931",
		SourceRelease:    "validation-1.3.16",
		XSLTSHA256:       "39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd",
	}
	if EN16931() != want {
		t.Fatalf("EN16931 metadata = %#v, want %#v", EN16931(), want)
	}

	changed := EN16931()
	changed.Version = "changed-by-caller"
	if EN16931() != want {
		t.Fatal("caller mutated the pinned EN16931 profile")
	}
}

func TestVerifyEN16931(t *testing.T) {
	if err := VerifyEN16931(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenEN16931XSLT(t *testing.T) {
	file, err := OpenEN16931XSLT()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyEN16931RejectsChecksumMismatch(t *testing.T) {
	original := en16931
	en16931.XSLTSHA256 = strings.Repeat("0", 64)
	t.Cleanup(func() { en16931 = original })

	err := VerifyEN16931()
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("VerifyEN16931() error = %v, want SHA-256 mismatch", err)
	}
}

func TestEN16931MissingEmbeddedAssetErrors(t *testing.T) {
	original := en16931
	en16931.EmbeddedXSLTPath = "en16931/missing.xslt"
	t.Cleanup(func() { en16931 = original })

	if err := VerifyEN16931(); err == nil || !strings.Contains(err.Error(), "read en16931 profile") {
		t.Fatalf("VerifyEN16931() error = %v", err)
	}
	if _, err := StageEN16931(t.TempDir()); err == nil {
		t.Fatal("StageEN16931 accepted a missing embedded asset")
	}
}

func TestStageRuntimeAssetsRejectsConfigurationChecksumMismatch(t *testing.T) {
	original := saxon
	saxon.ConfigurationSHA256 = strings.Repeat("0", 64)
	t.Cleanup(func() { saxon = original })

	if _, err := StageRuntimeAssets(t.TempDir()); err == nil || !strings.Contains(err.Error(), "Saxon configuration") {
		t.Fatalf("StageRuntimeAssets() error = %v", err)
	}
}

func TestAssetStagingReportsFilesystemFailures(t *testing.T) {
	blockedRoot := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedRoot, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StageEN16931(filepath.Join(blockedRoot, "child")); err == nil || !strings.Contains(err.Error(), "asset directory") {
		t.Fatalf("StageEN16931 mkdir error = %v", err)
	}

	root := t.TempDir()
	destination := filepath.Join(root, filepath.FromSlash(EN16931().EmbeddedXSLTPath))
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := StageEN16931(root); err == nil || !strings.Contains(err.Error(), "stage en16931 stylesheet") {
		t.Fatalf("StageEN16931 write error = %v", err)
	}

	root = t.TempDir()
	if _, err := StageEN16931(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "saxon"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := StageRuntimeAssets(root); err == nil || !strings.Contains(err.Error(), "configuration directory") {
		t.Fatalf("StageRuntimeAssets mkdir error = %v", err)
	}
}

func TestStageEN16931UsesSelectedProfilePath(t *testing.T) {
	root := t.TempDir()
	got, err := StageEN16931(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, filepath.FromSlash(EN16931().EmbeddedXSLTPath))
	if got != want {
		t.Fatalf("StageEN16931() path = %q, want %q", got, want)
	}
	staged, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := files.ReadFile(EN16931().EmbeddedXSLTPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, embedded) {
		t.Fatal("staged stylesheet differs from verified embedded stylesheet")
	}
}

func TestStageEN16931RejectsEmptyRoot(t *testing.T) {
	if _, err := StageEN16931(""); err == nil {
		t.Fatal("StageEN16931() accepted an empty destination root")
	}
}

func TestStagePeppolUsesSelectedProfilePath(t *testing.T) {
	root := t.TempDir()
	got, err := StagePeppol(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, filepath.FromSlash(Peppol().EmbeddedXSLTPath))
	if got != want {
		t.Fatalf("StagePeppol() path = %q, want %q", got, want)
	}
	staged, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := files.ReadFile(Peppol().EmbeddedXSLTPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(staged, embedded) {
		t.Fatal("staged Peppol stylesheet differs from verified embedded stylesheet")
	}
}

func TestStagePeppolRejectsEmptyRoot(t *testing.T) {
	if _, err := StagePeppol(""); err == nil {
		t.Fatal("StagePeppol() accepted an empty destination root")
	}
}

func TestStageRuntimeAssetsIncludesRestrictedSaxonConfiguration(t *testing.T) {
	root := t.TempDir()
	staged, err := StageRuntimeAssets(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 3 {
		t.Fatalf("StageRuntimeAssets() staged %d files, want 3", len(staged))
	}
	configuration, err := os.ReadFile(filepath.Join(root, "saxon", "saxon-config.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`allowedProtocols="file"`, `allowExternalFunctions="false"`, `disableXslEvaluate="true"`} {
		if !bytes.Contains(configuration, []byte(required)) {
			t.Errorf("Saxon configuration does not contain %s", required)
		}
	}
}

func TestRecordedEN16931ChecksumMatchesProfile(t *testing.T) {
	profile := EN16931()
	manifest, err := os.ReadFile(filepath.Join("en16931", profile.Version, "sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%s  %s", profile.XSLTSHA256, profile.XSLTName)
	if strings.TrimSpace(string(manifest)) != want {
		t.Fatalf("sha256.txt = %q, want %q", strings.TrimSpace(string(manifest)), want)
	}
}

func TestRecordedEN16931ProvenanceMatchesProfile(t *testing.T) {
	profile := EN16931()
	provenance, err := os.ReadFile(filepath.Join("en16931", profile.Version, "source-url.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(provenance)
	for _, required := range []string{profile.SourceRepository, profile.SourceRelease} {
		if !strings.Contains(text, required) {
			t.Errorf("source-url.txt does not contain %q", required)
		}
	}
}
