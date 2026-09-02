package validationassets

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPeppolPinnedMetadata(t *testing.T) {
	want := PeppolProfile{
		Name:                               "peppol-bis-billing",
		Version:                            "3.0.21",
		PublicationDate:                    "2026-05-20",
		MandatoryFrom:                      "2026-08-17",
		ValidationArtefactsVersion:         "1.3.16",
		ValidationArtefactsPublicationDate: "2026-04-10",
		CodeListPublicationDate:            "2026-03-06",
		SourceRepository:                   "OpenPEPPOL/peppol-bis-invoice-3",
		SourceCommit:                       "806866bd2bd91d7e9623b68f08164e8fbe9e67a0",
		SourceTree:                         "a8acc94f1216279839547e1670585fac62dfc122",
		SourceArchiveURL:                   "https://codeload.github.com/OpenPEPPOL/peppol-bis-invoice-3/tar.gz/806866bd2bd91d7e9623b68f08164e8fbe9e67a0",
		SourceArchiveSHA256:                "a843cd3808c6e95565eded1a535a5a40a88ad6f92e009d2b1816a4818a82299e",
		ManifestSHA256:                     "705e83fa494c052254c5cd4cbd2fb42baa975d559774f6cc928dbdddd10ab29e",
		EmbeddedRoot:                       "peppol/3.0.21",
		SchematronPath:                     "peppol/3.0.21/rules/PEPPOL-EN16931-UBL.sch",
	}
	if Peppol() != want {
		t.Fatalf("Peppol metadata = %#v, want %#v", Peppol(), want)
	}

	changed := Peppol()
	changed.Version = "changed-by-caller"
	if Peppol() != want {
		t.Fatal("caller mutated the pinned Peppol profile")
	}
}

func TestVerifyPeppol(t *testing.T) {
	if err := VerifyPeppol(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenPeppolSchematron(t *testing.T) {
	file, err := OpenPeppolSchematron()
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPeppolRejectsManifestChecksumMismatch(t *testing.T) {
	profile := Peppol()
	profile.ManifestSHA256 = strings.Repeat("0", 64)
	err := verifyPeppolFS(files, profile)
	if err == nil || !strings.Contains(err.Error(), "manifest: SHA-256 mismatch") {
		t.Fatalf("verifyPeppolFS() error = %v, want manifest SHA-256 mismatch", err)
	}
}

func TestVerifyPeppolRejectsChangedAsset(t *testing.T) {
	mapFS := copyPeppolFS(t)
	assetPath := path.Join(Peppol().EmbeddedRoot, peppolSchematronRelativePath)
	mapFS[assetPath].Data = append(mapFS[assetPath].Data, '\n')
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "asset \"rules/PEPPOL-EN16931-UBL.sch\": SHA-256 mismatch") {
		t.Fatalf("verifyPeppolFS() error = %v, want asset SHA-256 mismatch", err)
	}
}

func TestVerifyPeppolRejectsMissingAsset(t *testing.T) {
	mapFS := copyPeppolFS(t)
	delete(mapFS, path.Join(Peppol().EmbeddedRoot, "examples/base-example.xml"))
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "read peppol-bis-billing profile 3.0.21 asset") {
		t.Fatalf("verifyPeppolFS() error = %v, want missing asset error", err)
	}
}

func TestVerifyPeppolRejectsWrongVersionMetadata(t *testing.T) {
	profile := Peppol()
	profile.Version = "3.0.22"
	err := verifyPeppolFS(files, profile)
	if err == nil || !strings.Contains(err.Error(), "bis_version") {
		t.Fatalf("verifyPeppolFS() error = %v, want provenance version mismatch", err)
	}
}

func TestVerifyPeppolRejectsProvenanceMismatch(t *testing.T) {
	profile := Peppol()
	profile.SourceCommit = strings.Repeat("0", 40)
	err := verifyPeppolFS(files, profile)
	if err == nil || !strings.Contains(err.Error(), "source_commit") {
		t.Fatalf("verifyPeppolFS() error = %v, want source commit mismatch", err)
	}
}

func TestVerifyPeppolRejectsUnmanifestedAsset(t *testing.T) {
	mapFS := copyPeppolFS(t)
	mapFS[path.Join(Peppol().EmbeddedRoot, "examples/unexpected.xml")] = &fstest.MapFile{Data: []byte("unexpected")}
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "unmanifested asset") {
		t.Fatalf("verifyPeppolFS() error = %v, want unmanifested asset error", err)
	}
}

func TestVerifyPeppolRejectsMalformedManifestEntry(t *testing.T) {
	mapFS := copyPeppolFS(t)
	manifestPath := path.Join(Peppol().EmbeddedRoot, peppolManifestRelativePath)
	mapFS[manifestPath].Data = append(mapFS[manifestPath].Data, []byte("not-a-checksum  examples/unexpected.xml\n")...)
	profile := Peppol()
	profile.ManifestSHA256 = fmt.Sprintf("%x", sha256.Sum256(mapFS[manifestPath].Data))
	err := verifyPeppolFS(mapFS, profile)
	if err == nil || !strings.Contains(err.Error(), "malformed line") {
		t.Fatalf("verifyPeppolFS() error = %v, want malformed manifest error", err)
	}
}

func copyPeppolFS(t *testing.T) fstest.MapFS {
	t.Helper()
	mapFS := fstest.MapFS{}
	err := fs.WalkDir(files, Peppol().EmbeddedRoot, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		content, err := fs.ReadFile(files, filePath)
		if err != nil {
			return err
		}
		mapFS[filePath] = &fstest.MapFile{Data: append([]byte(nil), content...)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return mapFS
}
