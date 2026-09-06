package validationassets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

const (
	schematronNamespace = "http://purl.oclc.org/dsdl/schematron"
	svrlNamespace       = "http://purl.oclc.org/dsdl/svrl"
	xsltNamespace       = "http://www.w3.org/1999/XSL/Transform"
	peppolSmokeExample  = "examples/base-example.xml"
	diagnosticProbeID   = "DIAGNOSTIC-PROBE"
	diagnosticProbeRef  = "DIAGNOSTIC-PROBE-CONTENT"
	diagnosticProbeText = "Diagnostic code: ABC"
)

const diagnosticProbeSchematron = `<?xml version="1.0" encoding="UTF-8"?>
<schema xmlns="http://purl.oclc.org/dsdl/schematron" queryBinding="xslt2">
  <title>Diagnostic compilation probe</title>
  <pattern id="diagnostic-probe">
    <rule context="probe">
      <assert id="DIAGNOSTIC-PROBE" flag="fatal" test="@ok = 'true'"
        diagnostics="DIAGNOSTIC-PROBE-CONTENT">Probe must pass.</assert>
    </rule>
  </pattern>
  <diagnostics>
    <diagnostic id="DIAGNOSTIC-PROBE-CONTENT">Diagnostic code: <value-of select="@code"/></diagnostic>
  </diagnostics>
</schema>`

const diagnosticProbeDocument = `<probe code="ABC"/>`

// PeppolCompilationConfig identifies the local, checksum-verified tools used
// for one deterministic compilation.
type PeppolCompilationConfig struct {
	JavaExecutable         string
	SaxonDirectory         string
	SchXsltDirectory       string
	SaxonConfigurationPath string
}

type peppolCompilerMetadata struct {
	SchemaVersion            int               `json:"schema_version"`
	Compiler                 string            `json:"compiler"`
	CompilerVersion          string            `json:"compiler_version"`
	CompilerReleaseURL       string            `json:"compiler_release_url"`
	CompilerArchiveURL       string            `json:"compiler_archive_url"`
	CompilerArchiveSHA256    string            `json:"compiler_archive_sha256"`
	CompilerManifestSHA256   string            `json:"compiler_distribution_manifest_sha256"`
	CompilerEntrypoint       string            `json:"compiler_entrypoint"`
	CompileParameters        map[string]string `json:"compile_parameters"`
	Processor                string            `json:"processor"`
	ProcessorVersion         string            `json:"processor_version"`
	ProcessorSHA256          string            `json:"processor_sha256"`
	ProcessorResolverVersion string            `json:"processor_resolver_version"`
	ProcessorResolverSHA256  string            `json:"processor_resolver_sha256"`
	ProcessorConfigSHA256    string            `json:"processor_configuration_sha256"`
	SourceSchematron         string            `json:"source_schematron"`
	SourceSchematronSHA256   string            `json:"source_schematron_sha256"`
	CompiledXSLT             string            `json:"compiled_xslt"`
	CompiledXSLTSHA256       string            `json:"compiled_xslt_sha256"`
}

// CompilePeppolSchematron compiles the pinned source twice, requires
// byte-identical output, verifies the accepted digest and rule metadata, and
// runs offline positive and diagnostic-content SVRL smoke transforms.
func CompilePeppolSchematron(ctx context.Context, cfg PeppolCompilationConfig) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg.SaxonDirectory == "" {
		return nil, fmt.Errorf("compile Peppol Schematron: Saxon directory is empty")
	}
	if cfg.SchXsltDirectory == "" {
		return nil, fmt.Errorf("compile Peppol Schematron: SchXslt directory is empty")
	}
	if cfg.SaxonConfigurationPath == "" {
		return nil, fmt.Errorf("compile Peppol Schematron: Saxon configuration path is empty")
	}
	if cfg.JavaExecutable == "" {
		cfg.JavaExecutable = "java"
	}
	javaPath, err := exec.LookPath(cfg.JavaExecutable)
	if err != nil {
		return nil, fmt.Errorf("compile Peppol Schematron: Java executable %q is unavailable: %w", cfg.JavaExecutable, err)
	}
	if err := VerifyPeppolSources(); err != nil {
		return nil, err
	}
	if err := VerifySaxonJARs(cfg.SaxonDirectory); err != nil {
		return nil, err
	}
	if err := verifyFileSHA256(cfg.SaxonConfigurationPath, Saxon().ConfigurationSHA256); err != nil {
		return nil, fmt.Errorf("verify Saxon configuration: %w", err)
	}
	if err := VerifySchXslt(cfg.SchXsltDirectory); err != nil {
		return nil, err
	}

	source, err := fs.ReadFile(files, peppol.SchematronPath)
	if err != nil {
		return nil, fmt.Errorf("read Peppol Schematron: %w", err)
	}
	smoke, err := fs.ReadFile(files, path.Join(peppol.EmbeddedRoot, peppolSmokeExample))
	if err != nil {
		return nil, fmt.Errorf("read Peppol smoke example: %w", err)
	}
	workDir, err := os.MkdirTemp("", "e-ubl21-peppol-compile-")
	if err != nil {
		return nil, fmt.Errorf("create Peppol compilation directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	sourcePath := filepath.Join(workDir, "PEPPOL-EN16931-UBL.sch")
	smokePath := filepath.Join(workDir, "base-example.xml")
	if err := os.WriteFile(sourcePath, source, 0o600); err != nil {
		return nil, fmt.Errorf("write Peppol Schematron input: %w", err)
	}
	if err := os.WriteFile(smokePath, smoke, 0o600); err != nil {
		return nil, fmt.Errorf("write Peppol smoke input: %w", err)
	}

	compilerPath := schxsltCompilerPath(cfg.SchXsltDirectory)
	outputs := []string{
		filepath.Join(workDir, "compiled-1.xslt"),
		filepath.Join(workDir, "compiled-2.xslt"),
	}
	for _, outputPath := range outputs {
		if err := runSaxon(ctx, javaPath, cfg.SaxonDirectory, cfg.SaxonConfigurationPath,
			sourcePath, compilerPath, outputPath, schxslt.CompileMetadataParameter); err != nil {
			return nil, fmt.Errorf("compile Peppol Schematron: %w", err)
		}
	}
	first, err := os.ReadFile(outputs[0])
	if err != nil {
		return nil, fmt.Errorf("read first compiled Peppol XSLT: %w", err)
	}
	second, err := os.ReadFile(outputs[1])
	if err != nil {
		return nil, fmt.Errorf("read second compiled Peppol XSLT: %w", err)
	}
	if !bytes.Equal(first, second) {
		return nil, fmt.Errorf("compile Peppol Schematron: consecutive outputs are not byte-identical")
	}
	actualSHA256 := fmt.Sprintf("%x", sha256.Sum256(first))
	if actualSHA256 != peppol.XSLTSHA256 {
		return nil, fmt.Errorf("compile Peppol Schematron: XSLT SHA-256 mismatch: expected %s, got %s", peppol.XSLTSHA256, actualSHA256)
	}
	if err := verifyPeppolSemanticPreservation(source, first); err != nil {
		return nil, fmt.Errorf("compile Peppol Schematron: %w", err)
	}

	smokeOutput := filepath.Join(workDir, "smoke.svrl.xml")
	if err := runSaxon(ctx, javaPath, cfg.SaxonDirectory, cfg.SaxonConfigurationPath,
		smokePath, outputs[0], smokeOutput, ""); err != nil {
		return nil, fmt.Errorf("run Peppol SVRL smoke transform: %w", err)
	}
	svrl, err := os.ReadFile(smokeOutput)
	if err != nil {
		return nil, fmt.Errorf("read Peppol SVRL smoke output: %w", err)
	}
	if err := verifyPassingSVRL(svrl); err != nil {
		return nil, fmt.Errorf("verify Peppol SVRL smoke output: %w", err)
	}
	if err := runDiagnosticConformanceProbe(ctx, javaPath, cfg, workDir, compilerPath); err != nil {
		return nil, err
	}
	return first, nil
}

func runDiagnosticConformanceProbe(ctx context.Context, javaPath string, cfg PeppolCompilationConfig, workDir, compilerPath string) error {
	schemaPath := filepath.Join(workDir, "diagnostic-probe.sch")
	documentPath := filepath.Join(workDir, "diagnostic-probe.xml")
	stylesheetPath := filepath.Join(workDir, "diagnostic-probe.xslt")
	svrlPath := filepath.Join(workDir, "diagnostic-probe.svrl.xml")
	if err := os.WriteFile(schemaPath, []byte(diagnosticProbeSchematron), 0o600); err != nil {
		return fmt.Errorf("write Schematron diagnostic probe: %w", err)
	}
	if err := os.WriteFile(documentPath, []byte(diagnosticProbeDocument), 0o600); err != nil {
		return fmt.Errorf("write diagnostic probe document: %w", err)
	}
	if err := runSaxon(ctx, javaPath, cfg.SaxonDirectory, cfg.SaxonConfigurationPath,
		schemaPath, compilerPath, stylesheetPath, schxslt.CompileMetadataParameter); err != nil {
		return fmt.Errorf("compile Schematron diagnostic probe: %w", err)
	}
	if err := runSaxon(ctx, javaPath, cfg.SaxonDirectory, cfg.SaxonConfigurationPath,
		documentPath, stylesheetPath, svrlPath, ""); err != nil {
		return fmt.Errorf("run Schematron diagnostic probe: %w", err)
	}
	content, err := os.ReadFile(svrlPath)
	if err != nil {
		return fmt.Errorf("read Schematron diagnostic probe SVRL: %w", err)
	}
	if err := verifyDiagnosticSVRL(content); err != nil {
		return fmt.Errorf("verify Schematron diagnostic probe SVRL: %w", err)
	}
	return nil
}

func runSaxon(ctx context.Context, javaPath, saxonDirectory, configurationPath, sourcePath, stylesheetPath, outputPath, parameter string) error {
	runtime := Saxon()
	classpath := filepath.Join(saxonDirectory, runtime.SaxonJARName) +
		string(os.PathListSeparator) +
		filepath.Join(saxonDirectory, runtime.XMLResolverJARName)
	args := []string{
		"-cp", classpath,
		"net.sf.saxon.Transform",
		"-config:" + configurationPath,
		"-ext:off",
		"-s:" + sourcePath,
		"-xsl:" + stylesheetPath,
		"-o:" + outputPath,
	}
	if parameter != "" {
		args = append(args, parameter)
	}
	cmd := exec.CommandContext(ctx, javaPath, args...)
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("Saxon timed out or was cancelled: %w", ctxErr)
		}
		return fmt.Errorf("Saxon failed: %w: %s", err, strings.TrimSpace(diagnostics.String()))
	}
	return nil
}

func verifyPassingSVRL(content []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	rootSeen := false
	firedRules := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !rootSeen {
			rootSeen = true
			if start.Name.Space != svrlNamespace || start.Name.Local != "schematron-output" {
				return fmt.Errorf("root is {%s}%s, want {%s}schematron-output", start.Name.Space, start.Name.Local, svrlNamespace)
			}
		}
		if start.Name.Space == svrlNamespace && start.Name.Local == "failed-assert" {
			return fmt.Errorf("official positive example emitted failed assertion %q", xmlAttribute(start.Attr, "id"))
		}
		if start.Name.Space == svrlNamespace && start.Name.Local == "fired-rule" {
			firedRules++
		}
	}
	if !rootSeen {
		return fmt.Errorf("SVRL document is empty")
	}
	if firedRules == 0 {
		return fmt.Errorf("SVRL document contains no fired rules")
	}
	return nil
}

func verifyDiagnosticSVRL(content []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	rootSeen := false
	firedRuleSeen := false
	assertionSeen := false
	diagnosticSeen := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !rootSeen {
			rootSeen = true
			if start.Name.Space != svrlNamespace || start.Name.Local != "schematron-output" {
				return fmt.Errorf("root is {%s}%s, want {%s}schematron-output", start.Name.Space, start.Name.Local, svrlNamespace)
			}
		}
		if start.Name.Space != svrlNamespace {
			continue
		}
		switch start.Name.Local {
		case "fired-rule":
			firedRuleSeen = true
		case "failed-assert":
			if xmlAttribute(start.Attr, "id") == diagnosticProbeID {
				assertionSeen = true
			}
		case "diagnostic-reference":
			if xmlAttribute(start.Attr, "diagnostic") != diagnosticProbeRef {
				continue
			}
			text, err := xmlElementText(decoder)
			if err != nil {
				return fmt.Errorf("read diagnostic reference content: %w", err)
			}
			if normalizedXMLText(text) != diagnosticProbeText {
				return fmt.Errorf("diagnostic %q content = %q, want %q", diagnosticProbeRef, normalizedXMLText(text), diagnosticProbeText)
			}
			diagnosticSeen = true
		}
	}
	if !rootSeen {
		return fmt.Errorf("SVRL document is empty")
	}
	if !firedRuleSeen {
		return fmt.Errorf("diagnostic probe contains no fired rule")
	}
	if !assertionSeen {
		return fmt.Errorf("diagnostic probe assertion %q did not fail", diagnosticProbeID)
	}
	if !diagnosticSeen {
		return fmt.Errorf("diagnostic probe reference %q is missing", diagnosticProbeRef)
	}
	return nil
}

func xmlElementText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder
	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			text.Write(value)
		}
	}
	return text.String(), nil
}

func normalizedXMLText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func verifyPeppolCompilationFS(fsys fs.FS, profile PeppolProfile) error {
	entries, err := fs.ReadDir(fsys, path.Join(profile.EmbeddedRoot, peppolRuntimeDirectory))
	if err != nil {
		return fmt.Errorf("read %s profile %s runtime inventory: %w", profile.Name, profile.Version, err)
	}
	expectedInventory := map[string]bool{
		path.Base(profile.EmbeddedXSLTPath):     false,
		path.Base(profile.CompilerMetadataPath): false,
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return fmt.Errorf("verify %s profile %s runtime inventory: unexpected directory %q", profile.Name, profile.Version, entry.Name())
		}
		if _, ok := expectedInventory[entry.Name()]; !ok {
			return fmt.Errorf("verify %s profile %s runtime inventory: unexpected file %q", profile.Name, profile.Version, entry.Name())
		}
		expectedInventory[entry.Name()] = true
	}
	for name, seen := range expectedInventory {
		if !seen {
			return fmt.Errorf("verify %s profile %s runtime inventory: missing file %q", profile.Name, profile.Version, name)
		}
	}

	source, err := fs.ReadFile(fsys, profile.SchematronPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s Schematron: %w", profile.Name, profile.Version, err)
	}
	actualSourceSHA256 := fmt.Sprintf("%x", sha256.Sum256(source))
	if actualSourceSHA256 != profile.SchematronSHA256 {
		return fmt.Errorf("verify %s profile %s Schematron: SHA-256 mismatch: expected %s, got %s", profile.Name, profile.Version, profile.SchematronSHA256, actualSourceSHA256)
	}
	compiled, err := fs.ReadFile(fsys, profile.EmbeddedXSLTPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s compiled XSLT: %w", profile.Name, profile.Version, err)
	}
	actualSHA256 := fmt.Sprintf("%x", sha256.Sum256(compiled))
	if actualSHA256 != profile.XSLTSHA256 {
		return fmt.Errorf("verify %s profile %s compiled XSLT: SHA-256 mismatch: expected %s, got %s", profile.Name, profile.Version, profile.XSLTSHA256, actualSHA256)
	}
	if err := verifyPeppolSemanticPreservation(source, compiled); err != nil {
		return fmt.Errorf("verify %s profile %s compiled XSLT semantics: %w", profile.Name, profile.Version, err)
	}
	return verifyPeppolCompilerMetadata(fsys, profile)
}

func verifyPeppolCompilerMetadata(fsys fs.FS, profile PeppolProfile) error {
	content, err := fs.ReadFile(fsys, profile.CompilerMetadataPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s compiler metadata: %w", profile.Name, profile.Version, err)
	}
	actualMetadataSHA256 := fmt.Sprintf("%x", sha256.Sum256(content))
	if actualMetadataSHA256 != profile.CompilerMetadataSHA256 {
		return fmt.Errorf("verify %s profile %s compiler metadata: SHA-256 mismatch: expected %s, got %s", profile.Name, profile.Version, profile.CompilerMetadataSHA256, actualMetadataSHA256)
	}
	var metadata peppolCompilerMetadata
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return fmt.Errorf("parse %s profile %s compiler metadata: %w", profile.Name, profile.Version, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("parse %s profile %s compiler metadata: multiple JSON values", profile.Name, profile.Version)
		}
		return fmt.Errorf("parse %s profile %s compiler metadata trailer: %w", profile.Name, profile.Version, err)
	}
	runtime := Saxon()
	compiler := SchXslt()
	source, err := fs.ReadFile(fsys, profile.SchematronPath)
	if err != nil {
		return fmt.Errorf("read %s profile %s Schematron for compiler metadata: %w", profile.Name, profile.Version, err)
	}
	sourceSHA256 := fmt.Sprintf("%x", sha256.Sum256(source))
	expected := peppolCompilerMetadata{
		SchemaVersion:            1,
		Compiler:                 compiler.Name,
		CompilerVersion:          compiler.Version,
		CompilerReleaseURL:       compiler.ReleaseURL,
		CompilerArchiveURL:       compiler.ArchiveURL,
		CompilerArchiveSHA256:    compiler.ArchiveSHA256,
		CompilerManifestSHA256:   compiler.DistributionManifestSHA256,
		CompilerEntrypoint:       path.Join(compiler.DistributionDirectory, compiler.CompilerRelativePath),
		CompileParameters:        map[string]string{"schxslt.compile.metadata": "false()"},
		Processor:                "Saxon-HE",
		ProcessorVersion:         runtime.SaxonVersion,
		ProcessorSHA256:          runtime.SaxonJARSHA256,
		ProcessorResolverVersion: runtime.XMLResolverVersion,
		ProcessorResolverSHA256:  runtime.XMLResolverSHA256,
		ProcessorConfigSHA256:    runtime.ConfigurationSHA256,
		SourceSchematron:         peppolSchematronRelativePath,
		SourceSchematronSHA256:   sourceSHA256,
		CompiledXSLT:             strings.TrimPrefix(profile.EmbeddedXSLTPath, profile.EmbeddedRoot+"/"),
		CompiledXSLTSHA256:       profile.XSLTSHA256,
	}
	if !reflect.DeepEqual(metadata, expected) {
		return fmt.Errorf("verify %s profile %s compiler metadata: values do not match pinned compiler profile", profile.Name, profile.Version)
	}
	return nil
}

type schematronSemantics struct {
	rules          map[string]int
	diagnosticRefs map[string]int
	namespaces     map[string]string
}

func verifyPeppolSemanticPreservation(source, compiled []byte) error {
	sourceSemantics, err := readSchematronSemantics(source, true)
	if err != nil {
		return fmt.Errorf("parse source semantics: %w", err)
	}
	compiledSemantics, err := readSchematronSemantics(compiled, false)
	if err != nil {
		return fmt.Errorf("parse compiled semantics: %w", err)
	}
	if err := compareStringCounts("rules", sourceSemantics.rules, compiledSemantics.rules); err != nil {
		return err
	}
	if err := compareStringCounts("diagnostic references", sourceSemantics.diagnosticRefs, compiledSemantics.diagnosticRefs); err != nil {
		return err
	}
	for prefix, uri := range sourceSemantics.namespaces {
		if compiledSemantics.namespaces[prefix] != uri {
			return fmt.Errorf("namespace %q = %q, want %q", prefix, compiledSemantics.namespaces[prefix], uri)
		}
	}
	return nil
}

func readSchematronSemantics(content []byte, source bool) (schematronSemantics, error) {
	result := schematronSemantics{
		rules:          make(map[string]int),
		diagnosticRefs: make(map[string]int),
		namespaces:     make(map[string]string),
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	rootSeen := false
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
		if !rootSeen {
			rootSeen = true
			if source {
				if start.Name.Space != schematronNamespace || start.Name.Local != "schema" {
					return result, fmt.Errorf("root is {%s}%s, want Schematron schema", start.Name.Space, start.Name.Local)
				}
			} else {
				if start.Name.Space != xsltNamespace || (start.Name.Local != "transform" && start.Name.Local != "stylesheet") {
					return result, fmt.Errorf("root is {%s}%s, want XSLT transform", start.Name.Space, start.Name.Local)
				}
				for _, attr := range start.Attr {
					switch {
					case attr.Name.Space == "xmlns":
						result.namespaces[attr.Name.Local] = attr.Value
					case attr.Name.Space == "" && attr.Name.Local == "xmlns":
						result.namespaces[""] = attr.Value
					}
				}
			}
		}
		if source && start.Name.Space == schematronNamespace && start.Name.Local == "ns" {
			result.namespaces[xmlAttribute(start.Attr, "prefix")] = xmlAttribute(start.Attr, "uri")
			continue
		}
		if source && start.Name.Space == schematronNamespace && (start.Name.Local == "assert" || start.Name.Local == "report") {
			id := xmlAttribute(start.Attr, "id")
			if id == "" {
				return result, fmt.Errorf("%s has no id", start.Name.Local)
			}
			kind := "failed-assert"
			if start.Name.Local == "report" {
				kind = "successful-report"
			}
			result.rules[ruleKey(kind, id, xmlAttribute(start.Attr, "flag"), xmlAttribute(start.Attr, "role"))]++
			for _, diagnostic := range strings.Fields(xmlAttribute(start.Attr, "diagnostics")) {
				result.diagnosticRefs[diagnostic]++
			}
			continue
		}
		if !source && start.Name.Space == svrlNamespace && (start.Name.Local == "failed-assert" || start.Name.Local == "successful-report") {
			id := xmlAttribute(start.Attr, "id")
			if id == "" {
				return result, fmt.Errorf("compiled %s has no id", start.Name.Local)
			}
			result.rules[ruleKey(start.Name.Local, id, xmlAttribute(start.Attr, "flag"), xmlAttribute(start.Attr, "role"))]++
			continue
		}
		if !source && start.Name.Space == svrlNamespace && start.Name.Local == "diagnostic-reference" {
			result.diagnosticRefs[xmlAttribute(start.Attr, "diagnostic")]++
		}
	}
	if !rootSeen {
		return result, fmt.Errorf("document is empty")
	}
	return result, nil
}

func ruleKey(kind, id, flag, role string) string {
	return strings.Join([]string{kind, id, flag, role}, "\x00")
}

func xmlAttribute(attributes []xml.Attr, name string) string {
	for _, attribute := range attributes {
		if attribute.Name.Space == "" && attribute.Name.Local == name {
			return attribute.Value
		}
	}
	return ""
}

func compareStringCounts(label string, expected, actual map[string]int) error {
	keys := make([]string, 0, len(expected)+len(actual))
	seen := make(map[string]struct{})
	for key := range expected {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range actual {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if expected[key] != actual[key] {
			return fmt.Errorf("%s differ for %q: got %d, want %d", label, strings.ReplaceAll(key, "\x00", "/"), actual[key], expected[key])
		}
	}
	return nil
}
