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

func TestVerifyEN16931RejectsChecksumMismatch(t *testing.T) {
	original := en16931
	en16931.XSLTSHA256 = strings.Repeat("0", 64)
	t.Cleanup(func() { en16931 = original })

	err := VerifyEN16931()
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("VerifyEN16931() error = %v, want SHA-256 mismatch", err)
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
