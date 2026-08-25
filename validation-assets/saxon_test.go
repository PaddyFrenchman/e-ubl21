package validationassets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaxonRuntimePinnedMetadataMatchesDownloader(t *testing.T) {
	runtime := Saxon()
	if runtime.SaxonVersion != "12.7" || runtime.XMLResolverVersion != "6.0.17" {
		t.Fatalf("Saxon runtime versions = %s/%s", runtime.SaxonVersion, runtime.XMLResolverVersion)
	}
	downloader, err := os.ReadFile(filepath.Join("..", "devsetup", "download_saxon_he.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(downloader)
	for _, declaration := range []string{
		`SAXON_VERSION="` + runtime.SaxonVersion + `"`,
		`RESOLVER_VERSION="` + runtime.XMLResolverVersion + `"`,
		`EXPECTED_SHA256="` + runtime.SaxonJARSHA256 + `"`,
		`EXPECTED_RESOLVER_SHA256="` + runtime.XMLResolverSHA256 + `"`,
	} {
		if !strings.Contains(text, declaration) {
			t.Errorf("download_saxon_he.sh does not contain %s", declaration)
		}
	}
}

func TestVerifySaxonJARsRejectsMissingAndChangedFiles(t *testing.T) {
	if err := VerifySaxonJARs(t.TempDir()); err == nil {
		t.Fatal("VerifySaxonJARs() accepted missing JARs")
	}
	dir := t.TempDir()
	runtime := Saxon()
	for _, name := range []string{runtime.SaxonJARName, runtime.XMLResolverJARName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("not the pinned JAR"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := VerifySaxonJARs(dir); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("VerifySaxonJARs() error = %v", err)
	}
}
