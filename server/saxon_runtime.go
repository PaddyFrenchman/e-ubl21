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
	runtime := validationassets.Saxon()
	profile := validationassets.EN16931()
	resolved := saxonRuntimeConfig{
		javaExecutable:      cfg.JavaExecutable,
		saxonJARPath:        cfg.SaxonJARPath,
		xmlResolverJARPath:  cfg.XMLResolverJARPath,
		configurationPath:   cfg.SaxonConfigurationPath,
		xsltPath:            cfg.EN16931XSLTPath,
		saxonJARSHA256:      runtime.SaxonJARSHA256,
		xmlResolverSHA256:   runtime.XMLResolverSHA256,
		configurationSHA256: runtime.ConfigurationSHA256,
		xsltSHA256:          profile.XSLTSHA256,
		startupTimeout:      cfg.SaxonStartupTimeout,
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
		resolved.xsltPath = profile.RuntimeXSLTPath
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
	javaPath, err := exec.LookPath(cfg.javaExecutable)
	if err != nil {
		return fmt.Errorf("EN 16931 runtime: Java executable %q is unavailable: %w", cfg.javaExecutable, err)
	}
	for _, asset := range []struct {
		name, path, sha256 string
	}{
		{"Saxon-HE JAR", cfg.saxonJARPath, cfg.saxonJARSHA256},
		{"xmlresolver JAR", cfg.xmlResolverJARPath, cfg.xmlResolverSHA256},
		{"Saxon configuration", cfg.configurationPath, cfg.configurationSHA256},
		{"EN 16931 XSLT", cfg.xsltPath, cfg.xsltSHA256},
	} {
		if err := verifyReadableFileSHA256(asset.path, asset.sha256); err != nil {
			return fmt.Errorf("EN 16931 runtime: verify %s: %w", asset.name, err)
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
			return fmt.Errorf("EN 16931 runtime: Saxon smoke transform timed out after %s: %w", cfg.startupTimeout, smokeCtx.Err())
		}
		return fmt.Errorf("EN 16931 runtime: Saxon smoke transform failed: %w: %s", err, diagnostics.String())
	}
	decoder := xml.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("EN 16931 runtime: Saxon smoke transform emitted no SVRL document")
			}
			return fmt.Errorf("EN 16931 runtime: parse Saxon smoke output: %w", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			if start.Name.Space != svrlNamespace || start.Name.Local != "schematron-output" {
				return fmt.Errorf("EN 16931 runtime: Saxon smoke transform emitted root {%s}%s, want {%s}schematron-output", start.Name.Space, start.Name.Local, svrlNamespace)
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
