package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/svrl"
)

func TestPeppolNegativeCorpusPassesENAndTriggersOnlyCataloguedPeppolRule(t *testing.T) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		t.Skip("Java is unavailable; the Peppol compilation gate supplies the real runtime")
	}
	saxonJAR := filepath.Join("..", "devsetup", "tools", "saxon", "saxon-he-12.7.jar")
	resolverJAR := filepath.Join("..", "devsetup", "tools", "saxon", "xmlresolver-6.0.17.jar")
	for _, runtimeAsset := range []string{saxonJAR, resolverJAR} {
		if info, err := os.Stat(runtimeAsset); err != nil || !info.Mode().IsRegular() {
			t.Skip("local Saxon build dependencies are unavailable; run make test-peppol-schematron")
		}
	}

	corpus, err := generatePeppolNegativeCorpus(filepath.Join("..", "XSD"), testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join("..", "validation-assets", "saxon", "saxon-config.xml")
	enXSLT := filepath.Join("..", "validation-assets", "en16931", "1.3.16", "EN16931-UBL-validation.xslt")
	peppolXSLT := filepath.Join("..", "validation-assets", "peppol", "3.0.21", "runtime", "PEPPOL-EN16931-UBL-validation.xslt")
	classpath := saxonJAR + string(os.PathListSeparator) + resolverJAR

	transform := func(t *testing.T, input []byte, stylesheet string) svrl.Report {
		t.Helper()
		command := exec.Command(javaPath, "-cp", classpath, "net.sf.saxon.Transform",
			"-config:"+configuration, "-ext:off", "-s:-", "-xsl:"+stylesheet)
		command.Stdin = bytes.NewReader(input)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("Saxon transform with %s: %v: %s", stylesheet, err, output)
		}
		report, err := svrl.Parse(bytes.NewReader(output))
		if err != nil {
			t.Fatalf("parse SVRL from %s: %v", stylesheet, err)
		}
		return report
	}

	for _, fixture := range corpus.fixtures {
		fixture := fixture
		t.Run(fixture.entry.Mutation.ID, func(t *testing.T) {
			enReport := transform(t, fixture.content, enXSLT)
			if enReport.FailedAssertions != 0 {
				t.Fatalf("mutation introduced %d EN 16931 failed assertions: %+v", enReport.FailedAssertions, enReport.Issues)
			}
			peppolReport := transform(t, fixture.content, peppolXSLT)
			wantRules := fixture.entry.ExpectedStages[2].RuleIDs
			if peppolReport.FailedAssertions != len(wantRules) {
				t.Fatalf("Peppol failed assertions = %d, want %d: %+v", peppolReport.FailedAssertions, len(wantRules), peppolReport.Issues)
			}
			for index, wantRule := range wantRules {
				if got := peppolReport.Issues[index].Rule; got != wantRule {
					t.Errorf("Peppol rule = %q, want %q", got, wantRule)
				}
			}
		})
	}
}
