package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func TestResolveSaxonRuntimeConfigUsesPinnedDefaults(t *testing.T) {
	got := resolveSaxonRuntimeConfig(WorkerConfig{})
	saxon := validationassets.Saxon()
	en16931 := validationassets.EN16931()
	if got.javaExecutable != "java" ||
		got.saxonJARPath != saxon.SaxonJARPath ||
		got.xmlResolverJARPath != saxon.XMLResolverJARPath ||
		got.configurationPath != saxon.ConfigurationPath ||
		got.xsltPath != en16931.RuntimeXSLTPath ||
		got.startupTimeout != defaultSaxonStartupTimeout {
		t.Fatalf("resolved Saxon runtime = %+v", got)
	}
}

func TestEnsureSaxonRuntimeAcceptsSuccessfulSVRLSmokeTransform(t *testing.T) {
	cfg := fakeSaxonRuntime(t, `
case "$*" in
  *-config:*" -ext:off -s:- -xsl:"*) ;;
  *) echo "missing secure Saxon arguments: $*" >&2; exit 9 ;;
esac
printf '%s\n' '<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl"/>'
`)
	if err := ensureSaxonRuntime(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSaxonRuntimeRejectsBrokenDependencies(t *testing.T) {
	t.Run("Java unavailable", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		cfg.javaExecutable = filepath.Join(t.TempDir(), "missing-java")
		if err := ensureSaxonRuntime(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "Java executable") {
			t.Fatalf("ensureSaxonRuntime() error = %v", err)
		}
	})

	t.Run("JAR checksum mismatch", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `exit 0`)
		if err := os.WriteFile(cfg.saxonJARPath, []byte("changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := ensureSaxonRuntime(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
			t.Fatalf("ensureSaxonRuntime() error = %v", err)
		}
	})

	t.Run("classes do not load", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `echo 'class load failed' >&2; exit 2`)
		if err := ensureSaxonRuntime(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "class load failed") {
			t.Fatalf("ensureSaxonRuntime() error = %v", err)
		}
	})

	t.Run("non-SVRL output", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `printf '%s\n' '<not-svrl/>'`)
		if err := ensureSaxonRuntime(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "want {"+svrlNamespace+"}schematron-output") {
			t.Fatalf("ensureSaxonRuntime() error = %v", err)
		}
	})

	t.Run("startup timeout", func(t *testing.T) {
		cfg := fakeSaxonRuntime(t, `while :; do :; done`)
		cfg.startupTimeout = 10 * time.Millisecond
		if err := ensureSaxonRuntime(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("ensureSaxonRuntime() error = %v", err)
		}
	})
}

func TestMandatoryEN16931RuntimeFailsBeforeNATSConnection(t *testing.T) {
	err := RunWorker(context.Background(), WorkerConfig{
		JavaExecutable: filepath.Join(t.TempDir(), "missing-java"),
	})
	if err == nil || !strings.Contains(err.Error(), "EN 16931 runtime: Java executable") {
		t.Fatalf("RunWorker() error = %v", err)
	}
}

func TestPinnedLocalSaxonRuntimeSmokeTransform(t *testing.T) {
	cfg, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	if err := ensureSaxonRuntime(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedSaxonConfigurationRejectsNetworkResources(t *testing.T) {
	cfg, ok := localSaxonRuntimeConfig(t)
	if !ok {
		return
	}
	maliciousXSLT := `<?xml version="1.0"?>
<xsl:stylesheet xmlns:xsl="http://www.w3.org/1999/XSL/Transform" version="3.0">
  <xsl:template match="/"><xsl:sequence select="doc('https://example.invalid/blocked.xml')"/></xsl:template>
</xsl:stylesheet>`
	path := filepath.Join(t.TempDir(), "network-access.xslt")
	if err := os.WriteFile(path, []byte(maliciousXSLT), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.xsltPath = path
	cfg.xsltSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(maliciousXSLT)))
	err := ensureSaxonRuntime(context.Background(), cfg)
	diagnostic := strings.ToLower(fmt.Sprint(err))
	if err == nil || (!strings.Contains(diagnostic, "disallowed") && !strings.Contains(diagnostic, "prohibited")) {
		t.Fatalf("network-reading stylesheet error = %v, want protocol prohibited", err)
	}
}

func localSaxonRuntimeConfig(t testing.TB) (saxonRuntimeConfig, bool) {
	t.Helper()
	required := os.Getenv("EUBL21_REQUIRE_EN16931_TEST_RUNTIME") == "1"
	java, err := exec.LookPath("java")
	if err != nil {
		if required {
			t.Fatalf("mandatory EN 16931 test runtime: Java is not installed: %v", err)
		}
		t.Skipf("Java is not installed: %v", err)
		return saxonRuntimeConfig{}, false
	}
	saxon := validationassets.Saxon()
	cfg := resolveSaxonRuntimeConfig(WorkerConfig{
		JavaExecutable:         java,
		SaxonJARPath:           filepath.Join("..", "devsetup", "tools", "saxon", saxon.SaxonJARName),
		XMLResolverJARPath:     filepath.Join("..", "devsetup", "tools", "saxon", saxon.XMLResolverJARName),
		SaxonConfigurationPath: filepath.Join("..", "validation-assets", "saxon", "saxon-config.xml"),
		EN16931XSLTPath:        filepath.Join("..", "validation-assets", validationassets.EN16931().EmbeddedXSLTPath),
		SaxonStartupTimeout:    10 * time.Second,
	})
	for name, path := range map[string]string{
		"Saxon":        cfg.saxonJARPath,
		"XML Resolver": cfg.xmlResolverJARPath,
	} {
		if _, err := os.Stat(path); err != nil {
			if required {
				t.Fatalf("mandatory EN 16931 test runtime: pinned %s JAR is unavailable at %s: %v", name, path, err)
			}
			t.Skipf("pinned local %s JAR is unavailable at %s: %v", name, path, err)
			return saxonRuntimeConfig{}, false
		}
	}
	return cfg, true
}

func fakeSaxonRuntime(t *testing.T, scriptBody string) saxonRuntimeConfig {
	t.Helper()
	dir := t.TempDir()
	javaPath := filepath.Join(dir, "java")
	if err := os.WriteFile(javaPath, []byte("#!/bin/sh\nset -eu\n"+scriptBody+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeAsset := func(name, content string) (string, string) {
		t.Helper()
		path := filepath.Join(dir, name)
		data := []byte(content)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return path, fmt.Sprintf("%x", sha256.Sum256(data))
	}
	saxonJAR, saxonHash := writeAsset("saxon.jar", "fake Saxon classes")
	resolverJAR, resolverHash := writeAsset("xmlresolver.jar", "fake resolver classes")
	configuration, configurationHash := writeAsset("saxon-config.xml", "fake local-only configuration")
	xslt, xsltHash := writeAsset("validator.xslt", "fake pinned stylesheet")
	return saxonRuntimeConfig{
		javaExecutable:      javaPath,
		saxonJARPath:        saxonJAR,
		xmlResolverJARPath:  resolverJAR,
		configurationPath:   configuration,
		xsltPath:            xslt,
		saxonJARSHA256:      saxonHash,
		xmlResolverSHA256:   resolverHash,
		configurationSHA256: configurationHash,
		xsltSHA256:          xsltHash,
		startupTimeout:      time.Second,
	}
}
