package standard_test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const peppolCommit = "806866bd2bd91d7e9623b68f08164e8fbe9e67a0"

func TestPeppolChecksumManifest(t *testing.T) {
	manifest, err := os.Open(filepath.Join("peppol", "sha256.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer manifest.Close()

	want := map[string]bool{
		"archive/peppol-bis-invoice-3-" + peppolCommit + ".tar.gz": false,
		"source/rules/sch/CEN-EN16931-UBL.sch":                     false,
		"source/rules/sch/PEPPOL-EN16931-UBL.sch":                  false,
		"source/structure/codelist/ISO3166-1_Alpha2.xml":           false,
		"source/structure/codelist/ISO4217_2015.xml":               false,
		"source/structure/codelist/MimeCode.xml":                   false,
		"source/structure/codelist/SEPA.xml":                       false,
		"source/structure/codelist/UNCL1001-cn.xml":                false,
		"source/structure/codelist/UNCL1001-inv.xml":               false,
		"source/structure/codelist/UNCL1153.xml":                   false,
		"source/structure/codelist/UNCL2005.xml":                   false,
		"source/structure/codelist/UNCL4461.xml":                   false,
		"source/structure/codelist/UNCL5189.xml":                   false,
		"source/structure/codelist/UNCL5305.xml":                   false,
		"source/structure/codelist/UNCL7143.xml":                   false,
		"source/structure/codelist/UNCL7161.xml":                   false,
		"source/structure/codelist/UNECERec20-11e.xml":             false,
		"source/structure/codelist/VATEX.xml":                      false,
		"source/structure/codelist/eas.xml":                        false,
		"source/structure/codelist/icd.xml":                        false,
	}
	digestLine := regexp.MustCompile(`^([0-9a-f]{64})  ([^/][^ ]*)$`)
	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(manifest)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		match := digestLine.FindStringSubmatch(line)
		if match == nil {
			t.Fatalf("malformed checksum line %q", line)
		}
		name := match[2]
		if filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "..") {
			t.Fatalf("unsafe checksum path %q", name)
		}
		if _, duplicate := seen[name]; duplicate {
			t.Fatalf("duplicate checksum path %q", name)
		}
		seen[name] = struct{}{}
		if _, required := want[name]; !required {
			t.Fatalf("unexpected checksum path %q", name)
		}
		want[name] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing checksum for %s", name)
		}
	}
}

func TestPeppolDownloaderPinsFullCommit(t *testing.T) {
	scriptPath := filepath.Join("peppol", "download.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(content)
	if !strings.Contains(script, "SOURCE_COMMIT="+peppolCommit) {
		t.Fatalf("download script does not pin source commit %s", peppolCommit)
	}
	if !strings.Contains(script, "ARCHIVE_URL=https://codeload.github.com/OpenPEPPOL/peppol-bis-invoice-3/tar.gz/$SOURCE_COMMIT") {
		t.Fatal("download script does not derive its immutable archive URL from SOURCE_COMMIT")
	}
	if strings.Contains(script, "docs.peppol.eu") {
		t.Fatal("download script reads mutable rendered documentation")
	}
	for _, moving := range []string{"/main/", "/master/", "/upcoming/"} {
		if strings.Contains(script, moving) {
			t.Errorf("download script contains moving source path %q", moving)
		}
	}
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatal("download script is not executable")
	}
}

func TestPeppolOfflineIndexCoversCompleteSourceAreas(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("peppol", "offline-index.html"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(content)
	for _, sourceArea := range []string{
		"source/guide/bis/main.adoc",
		"source/guide/transaction-spec/",
		"source/guide/syntax/",
		"source/guide/cius/",
		"source/guide/national-rules/",
		"source/structure/codelist/",
		"source/rules/sch/PEPPOL-EN16931-UBL.sch",
		"source/rules/examples/",
		"source/mapping/",
		"source/stylesheet/",
	} {
		if !strings.Contains(index, sourceArea) {
			t.Errorf("offline index does not link to %s", sourceArea)
		}
	}
}

func TestPeppolProvenanceMatchesDownloader(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("peppol", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), peppolCommit) {
		t.Fatalf("Peppol provenance does not record source commit %s", peppolCommit)
	}
}
