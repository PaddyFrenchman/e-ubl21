package validationassets

import (
	"bytes"
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
		EmbeddedXSLTPath:                   "peppol/3.0.21/runtime/PEPPOL-EN16931-UBL-validation.xslt",
		RuntimeXSLTPath:                    "/opt/e-ubl21/validation-assets/peppol/3.0.21/runtime/PEPPOL-EN16931-UBL-validation.xslt",
		XSLTSHA256:                         "56bbd05ea26b4ffb859e0c15fcd2c1c8782870b530aae06a828ddb0bdf9b66aa",
		CompilerMetadataPath:               "peppol/3.0.21/runtime/compiler.json",
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

func TestOpenPeppolXSLT(t *testing.T) {
	file, err := OpenPeppolXSLT()
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

func TestVerifyPeppolRejectsChangedCompiledXSLT(t *testing.T) {
	mapFS := copyPeppolFS(t)
	mapFS[Peppol().EmbeddedXSLTPath].Data = append(mapFS[Peppol().EmbeddedXSLTPath].Data, '\n')
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "compiled XSLT: SHA-256 mismatch") {
		t.Fatalf("verifyPeppolFS() error = %v, want compiled XSLT SHA-256 mismatch", err)
	}
}

func TestVerifyPeppolRejectsMissingCompilerMetadata(t *testing.T) {
	mapFS := copyPeppolFS(t)
	delete(mapFS, Peppol().CompilerMetadataPath)
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "missing file \"compiler.json\"") {
		t.Fatalf("verifyPeppolFS() error = %v, want missing compiler metadata", err)
	}
}

func TestVerifyPeppolRejectsCompilerMetadataMismatch(t *testing.T) {
	mapFS := copyPeppolFS(t)
	metadata := mapFS[Peppol().CompilerMetadataPath]
	metadata.Data = bytes.Replace(metadata.Data, []byte(`"compiler_version": "1.10.1"`), []byte(`"compiler_version": "changed"`), 1)
	err := verifyPeppolFS(mapFS, Peppol())
	if err == nil || !strings.Contains(err.Error(), "values do not match pinned compiler profile") {
		t.Fatalf("verifyPeppolFS() error = %v, want compiler metadata mismatch", err)
	}
}

func TestCompiledPeppolXSLTContainsNoVolatileTimestamp(t *testing.T) {
	compiled, err := fs.ReadFile(files, Peppol().EmbeddedXSLTPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("current-dateTime()"), []byte("<svrl:metadata")} {
		if bytes.Contains(compiled, forbidden) {
			t.Fatalf("compiled XSLT contains volatile metadata marker %q", forbidden)
		}
	}
}

func TestVerifyPassingSVRLRequiresRuleExecution(t *testing.T) {
	valid := []byte(`<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">
  <svrl:fired-rule context="Invoice"/>
</svrl:schematron-output>`)
	if err := verifyPassingSVRL(valid); err != nil {
		t.Fatal(err)
	}

	empty := []byte(`<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"/>`)
	if err := verifyPassingSVRL(empty); err == nil || !strings.Contains(err.Error(), "no fired rules") {
		t.Fatalf("empty SVRL error = %v, want no fired rules", err)
	}

	failed := []byte(`<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">
  <svrl:fired-rule context="Invoice"/>
  <svrl:failed-assert id="PEPPOL-TEST"/>
</svrl:schematron-output>`)
	if err := verifyPassingSVRL(failed); err == nil || !strings.Contains(err.Error(), "PEPPOL-TEST") {
		t.Fatalf("failed SVRL error = %v, want failed assertion ID", err)
	}
}

func TestVerifyDiagnosticSVRLRequiresRenderedDiagnostic(t *testing.T) {
	valid := []byte(`<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">
  <svrl:fired-rule context="probe"/>
  <svrl:failed-assert id="DIAGNOSTIC-PROBE">
    <svrl:diagnostic-reference diagnostic="DIAGNOSTIC-PROBE-CONTENT">
      Diagnostic code: ABC
    </svrl:diagnostic-reference>
  </svrl:failed-assert>
</svrl:schematron-output>`)
	if err := verifyDiagnosticSVRL(valid); err != nil {
		t.Fatal(err)
	}

	wrongContent := bytes.Replace(valid, []byte("Diagnostic code: ABC"), []byte("Diagnostic code: WRONG"), 1)
	if err := verifyDiagnosticSVRL(wrongContent); err == nil || !strings.Contains(err.Error(), "Diagnostic code: WRONG") {
		t.Fatalf("wrong diagnostic content error = %v, want rendered-content mismatch", err)
	}
	missingReference := bytes.Replace(valid,
		[]byte(`<svrl:diagnostic-reference diagnostic="DIAGNOSTIC-PROBE-CONTENT">
      Diagnostic code: ABC
    </svrl:diagnostic-reference>`), nil, 1)
	if err := verifyDiagnosticSVRL(missingReference); err == nil || !strings.Contains(err.Error(), "reference") {
		t.Fatalf("missing diagnostic reference error = %v, want missing reference", err)
	}
}

func TestSemanticPreservationChecksRulesDiagnosticsAndNamespaces(t *testing.T) {
	source := []byte(`<schema xmlns="http://purl.oclc.org/dsdl/schematron">
	  <ns prefix="cbc" uri="urn:example:cbc"/>
	  <pattern><rule context="/">
	    <assert id="R-1" flag="fatal" role="error" diagnostics="D-1">required</assert>
	  </rule></pattern>
	  <diagnostics><diagnostic id="D-1">detail</diagnostic></diagnostics>
	</schema>`)
	compiled := []byte(`<xsl:transform xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
	  xmlns:svrl="http://purl.oclc.org/dsdl/svrl" xmlns:cbc="urn:example:cbc" version="2.0">
	  <svrl:failed-assert id="R-1" flag="fatal" role="error">
	    <svrl:diagnostic-reference diagnostic="D-1">detail</svrl:diagnostic-reference>
	  </svrl:failed-assert>
	</xsl:transform>`)
	if err := verifyPeppolSemanticPreservation(source, compiled); err != nil {
		t.Fatal(err)
	}

	withoutSeverity := bytes.Replace(compiled, []byte(` flag="fatal"`), nil, 1)
	if err := verifyPeppolSemanticPreservation(source, withoutSeverity); err == nil || !strings.Contains(err.Error(), "rules differ") {
		t.Fatalf("severity-loss error = %v, want rules differ", err)
	}
	withoutDiagnostic := bytes.Replace(compiled, []byte(`<svrl:diagnostic-reference diagnostic="D-1">detail</svrl:diagnostic-reference>`), nil, 1)
	if err := verifyPeppolSemanticPreservation(source, withoutDiagnostic); err == nil || !strings.Contains(err.Error(), "diagnostic references differ") {
		t.Fatalf("diagnostic-loss error = %v, want diagnostic references differ", err)
	}
	withoutNamespace := bytes.Replace(compiled, []byte(` xmlns:cbc="urn:example:cbc"`), nil, 1)
	if err := verifyPeppolSemanticPreservation(source, withoutNamespace); err == nil || !strings.Contains(err.Error(), "namespace") {
		t.Fatalf("namespace-loss error = %v, want namespace mismatch", err)
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
