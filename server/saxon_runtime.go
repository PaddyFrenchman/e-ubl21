package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

const defaultSaxonStartupTimeout = 30 * time.Second

const saxonTransformClass = "net.sf.saxon.Transform"
const svrlNamespace = "http://purl.oclc.org/dsdl/svrl"

const saxonSmokeInvoice = `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`

type saxonRuntimeConfig struct {
	javaExecutable      string
	saxonJARPath        string
	xmlResolverJARPath  string
	configurationPath   string
	xsltPath            string
	saxonJARSHA256      string
	xmlResolverSHA256   string
	configurationSHA256 string
	xsltSHA256          string
	startupTimeout      time.Duration
}

func resolveSaxonRuntimeConfig(cfg WorkerConfig) saxonRuntimeConfig {
	return resolveSaxonRuntimeOverrides(saxonRuntimeOverrides{
		javaExecutable: cfg.JavaExecutable, saxonJARPath: cfg.SaxonJARPath,
		xmlResolverJARPath: cfg.XMLResolverJARPath, configurationPath: cfg.SaxonConfigurationPath,
		xsltPath: cfg.EN16931XSLTPath, startupTimeout: cfg.SaxonStartupTimeout,
	})
}

type saxonRuntimeOverrides struct {
	javaExecutable     string
	saxonJARPath       string
	xmlResolverJARPath string
	configurationPath  string
	xsltPath           string
	startupTimeout     time.Duration
}

func resolveSaxonRuntimeOverrides(overrides saxonRuntimeOverrides) saxonRuntimeConfig {
	profile := validationassets.EN16931()
	return resolveProfileSaxonRuntimeOverrides(overrides, profile.RuntimeXSLTPath, profile.XSLTSHA256)
}

// resolvePeppolSaxonRuntimeOverrides selects the pinned Peppol executable while
// retaining the same Java, Saxon, resolver and restricted configuration
// resolution used by EN 16931.
func resolvePeppolSaxonRuntimeOverrides(overrides saxonRuntimeOverrides) saxonRuntimeConfig {
	profile := validationassets.Peppol()
	return resolveProfileSaxonRuntimeOverrides(overrides, profile.RuntimeXSLTPath, profile.XSLTSHA256)
}

func resolvePeppolEnvelopeSaxonRuntimeOverrides(overrides saxonRuntimeOverrides) saxonRuntimeConfig {
	return resolveProfileSaxonRuntimeOverrides(overrides, validationassets.PeppolEnvelopeRuntimeXSLTPath, validationassets.PeppolEnvelopeXSLTSHA256)
}

func resolveProfileSaxonRuntimeOverrides(overrides saxonRuntimeOverrides, defaultXSLTPath, xsltSHA256 string) saxonRuntimeConfig {
	runtime := validationassets.Saxon()
	resolved := saxonRuntimeConfig{
		javaExecutable:      overrides.javaExecutable,
		saxonJARPath:        overrides.saxonJARPath,
		xmlResolverJARPath:  overrides.xmlResolverJARPath,
		configurationPath:   overrides.configurationPath,
		xsltPath:            overrides.xsltPath,
		saxonJARSHA256:      runtime.SaxonJARSHA256,
		xmlResolverSHA256:   runtime.XMLResolverSHA256,
		configurationSHA256: runtime.ConfigurationSHA256,
		xsltSHA256:          xsltSHA256,
		startupTimeout:      overrides.startupTimeout,
	}
	if resolved.javaExecutable == "" {
		resolved.javaExecutable = "java"
	}
	if resolved.saxonJARPath == "" {
		resolved.saxonJARPath = runtime.SaxonJARPath
	}
	if resolved.xmlResolverJARPath == "" {
		resolved.xmlResolverJARPath = runtime.XMLResolverJARPath
	}
	if resolved.configurationPath == "" {
		resolved.configurationPath = runtime.ConfigurationPath
	}
	if resolved.xsltPath == "" {
		resolved.xsltPath = defaultXSLTPath
	}
	if resolved.startupTimeout <= 0 {
		resolved.startupTimeout = defaultSaxonStartupTimeout
	}
	return resolved
}

// ensureSaxonRuntime validates the complete local EN 16931 processor once at
// worker startup. The smoke transform proves that Java can load Saxon and
// xmlresolver, compile the official stylesheet, and emit SVRL under the
// no-network Saxon configuration.
func ensureSaxonRuntime(ctx context.Context, cfg saxonRuntimeConfig) error {
	return ensureProfileSaxonRuntime(ctx, cfg, "EN 16931")
}

func ensurePeppolSaxonRuntime(ctx context.Context, cfg saxonRuntimeConfig) error {
	if err := validationassets.VerifyPeppol(); err != nil {
		return fmt.Errorf("Peppol runtime: verify embedded source, provenance and compilation chain: %w", err)
	}
	return ensureProfileSaxonRuntime(ctx, cfg, "Peppol")
}

func ensurePeppolEnvelopeSaxonRuntime(ctx context.Context, cfg saxonRuntimeConfig) error {
	if err := validationassets.VerifyPeppolEnvelope(); err != nil {
		return fmt.Errorf("Peppol envelope runtime: verify embedded rules: %w", err)
	}
	return ensureProfileSaxonRuntime(ctx, cfg, "Peppol envelope")
}

func ensureProfileSaxonRuntime(ctx context.Context, cfg saxonRuntimeConfig, profileName string) error {
	javaPath, err := exec.LookPath(cfg.javaExecutable)
	if err != nil {
		return fmt.Errorf("%s runtime: Java executable %q is unavailable: %w", profileName, cfg.javaExecutable, err)
	}
	for _, asset := range []struct {
		name, path, sha256 string
	}{
		{"Saxon-HE JAR", cfg.saxonJARPath, cfg.saxonJARSHA256},
		{"xmlresolver JAR", cfg.xmlResolverJARPath, cfg.xmlResolverSHA256},
		{"Saxon configuration", cfg.configurationPath, cfg.configurationSHA256},
		{profileName + " XSLT", cfg.xsltPath, cfg.xsltSHA256},
	} {
		if err := verifyReadableFileSHA256(asset.path, asset.sha256); err != nil {
			return fmt.Errorf("%s runtime: verify %s: %w", profileName, asset.name, err)
		}
	}

	smokeCtx, cancel := context.WithTimeout(ctx, cfg.startupTimeout)
	defer cancel()
	classpath := cfg.saxonJARPath + string(os.PathListSeparator) + cfg.xmlResolverJARPath
	cmd := exec.CommandContext(smokeCtx, javaPath,
		"-cp", classpath,
		saxonTransformClass,
		"-config:"+cfg.configurationPath,
		"-ext:off",
		"-s:-",
		"-xsl:"+cfg.xsltPath,
	)
	cmd.Stdin = bytes.NewBufferString(saxonSmokeInvoice)
	var output bytes.Buffer
	diagnostics := &boundedDiagnosticBuffer{}
	cmd.Stdout = &output
	cmd.Stderr = diagnostics
	if err := cmd.Run(); err != nil {
		if smokeCtx.Err() != nil {
			return fmt.Errorf("%s runtime: Saxon smoke transform timed out after %s: %w", profileName, cfg.startupTimeout, smokeCtx.Err())
		}
		return fmt.Errorf("%s runtime: Saxon smoke transform failed: %w: %s", profileName, err, diagnostics.String())
	}
	decoder := xml.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("%s runtime: Saxon smoke transform emitted no SVRL document", profileName)
			}
			return fmt.Errorf("%s runtime: parse Saxon smoke output: %w", profileName, err)
		}
		if start, ok := token.(xml.StartElement); ok {
			if start.Name.Space != svrlNamespace || start.Name.Local != "schematron-output" {
				return fmt.Errorf("%s runtime: Saxon smoke transform emitted root {%s}%s, want {%s}schematron-output", profileName, start.Name.Space, start.Name.Local, svrlNamespace)
			}
			return nil
		}
	}
}

func verifyReadableFileSHA256(path, expected string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if expected != "" && actual != expected {
		return fmt.Errorf("SHA-256 mismatch for %s: expected %s, got %s", path, expected, actual)
	}
	return nil
}
