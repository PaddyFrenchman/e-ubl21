package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
)

type peppolTestRuntime struct {
	javaPath, classpath, configuration, enXSLT, peppolXSLT string
}

func peppolRuntimeForTest(t *testing.T) peppolTestRuntime {
	t.Helper()
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
	return peppolTestRuntime{
		javaPath: javaPath, classpath: saxonJAR + string(os.PathListSeparator) + resolverJAR,
		configuration: filepath.Join("..", "validation-assets", "saxon", "saxon-config.xml"),
		enXSLT:        filepath.Join("..", "validation-assets", "en16931", "1.3.16", "EN16931-UBL-validation.xslt"),
		peppolXSLT:    filepath.Join("..", "validation-assets", "peppol", "3.0.21", "runtime", "PEPPOL-EN16931-UBL-validation.xslt"),
	}
}

func (runtime peppolTestRuntime) transform(t *testing.T, input []byte, stylesheet string) svrl.Report {
	t.Helper()
	command := exec.Command(runtime.javaPath, "-cp", runtime.classpath, "net.sf.saxon.Transform",
		"-config:"+runtime.configuration, "-ext:off", "-s:-", "-xsl:"+stylesheet)
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

func TestPeppolPositiveCorpusPassesENAndPeppolWithRealRuntime(t *testing.T) {
	runtime := peppolRuntimeForTest(t)
	corpus, err := generatePeppolPositiveCorpus(filepath.Join("..", "XSD"), testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range corpus.fixtures {
		fixture := fixture
		t.Run(fixture.entry.ID, func(t *testing.T) {
			if report := runtime.transform(t, fixture.content, runtime.enXSLT); report.FailedAssertions != 0 {
				t.Fatalf("positive fixture emitted %d EN 16931 failed assertions: %+v", report.FailedAssertions, report.Issues)
			}
			if report := runtime.transform(t, fixture.content, runtime.peppolXSLT); report.FailedAssertions != 0 {
				t.Fatalf("positive fixture emitted %d Peppol failed assertions: %+v", report.FailedAssertions, report.Issues)
			}
		})
	}
}

func TestPeppolNegativeCorpusPassesENAndTriggersOnlyCataloguedPeppolRule(t *testing.T) {
	runtime := peppolRuntimeForTest(t)
	corpus, err := generatePeppolNegativeCorpus(filepath.Join("..", "XSD"), testGeneratorVersion)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range corpus.fixtures {
		fixture := fixture
		t.Run(fixture.entry.Mutation.ID, func(t *testing.T) {
			enReport := runtime.transform(t, fixture.content, runtime.enXSLT)
			if enReport.FailedAssertions != 0 {
				t.Fatalf("mutation introduced %d EN 16931 failed assertions: %+v", enReport.FailedAssertions, enReport.Issues)
			}
			peppolReport := runtime.transform(t, fixture.content, runtime.peppolXSLT)
			wantRules := fixture.entry.ExpectedStages[2].RuleIDs
			var failedRules []string
			for _, issue := range peppolReport.Issues {
				if issue.Kind == queue.ValidationIssueFailedAssert {
					failedRules = append(failedRules, issue.Rule)
				}
			}
			if len(failedRules) != len(wantRules) {
				t.Fatalf("Peppol failed rules = %v, want %v", failedRules, wantRules)
			}
			for index, wantRule := range wantRules {
				if got := failedRules[index]; got != wantRule {
					t.Errorf("Peppol rule = %q, want %q", got, wantRule)
				}
			}
		})
	}
}
