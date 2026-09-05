// Command check-peppol-reference executes the reviewed Peppol reference cases
// with both the accepted runtime and a freshly compiled stylesheet.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

func main() {
	java := flag.String("java", "java", "Java executable")
	saxonDir := flag.String("saxon-dir", "", "directory containing pinned Saxon and XML Resolver JARs")
	schxsltDir := flag.String("schxslt-dir", "", "directory containing the pinned SchXslt distribution")
	config := flag.String("saxon-config", "validation-assets/saxon/saxon-config.xml", "restricted Saxon configuration")
	timeout := flag.Duration("timeout", 3*time.Minute, "complete cross-check timeout")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := validationassets.VerifyPeppolReference(); err != nil {
		exitf("%v", err)
	}
	manifest, err := validationassets.PeppolReference()
	if err != nil {
		exitf("%v", err)
	}
	fresh, err := validationassets.CompilePeppolSchematron(ctx, validationassets.PeppolCompilationConfig{
		JavaExecutable: *java, SaxonDirectory: *saxonDir, SchXsltDirectory: *schxsltDir,
		SaxonConfigurationPath: *config,
	})
	if err != nil {
		exitf("compile independent Peppol reference runtime: %v", err)
	}
	temporary, err := os.CreateTemp("", "e-ubl21-peppol-reference-*.xslt")
	if err != nil {
		exitf("create reference stylesheet: %v", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(fresh); err != nil {
		temporary.Close()
		exitf("write reference stylesheet: %v", err)
	}
	if err := temporary.Close(); err != nil {
		exitf("close reference stylesheet: %v", err)
	}

	javaPath, err := exec.LookPath(*java)
	if err != nil {
		exitf("find Java: %v", err)
	}
	runtime := validationassets.Saxon()
	classpath := filepath.Join(*saxonDir, runtime.SaxonJARName) + string(os.PathListSeparator) + filepath.Join(*saxonDir, runtime.XMLResolverJARName)
	accepted := filepath.Join("validation-assets", filepath.FromSlash(validationassets.Peppol().EmbeddedXSLTPath))
	for _, testCase := range manifest.Cases {
		content, err := os.ReadFile(filepath.Clean(testCase.Path))
		if err != nil {
			exitf("case %s: read fixture: %v", testCase.ID, err)
		}
		if digest := fmt.Sprintf("%x", sha256.Sum256(content)); digest != testCase.SHA256 {
			exitf("case %s: fixture SHA-256 = %s, want %s", testCase.ID, digest, testCase.SHA256)
		}
		want := expectedIssues(testCase)
		acceptedIssues, err := transform(ctx, javaPath, classpath, *config, accepted, content)
		if err != nil {
			exitf("case %s: accepted runtime: %v", testCase.ID, err)
		}
		freshIssues, err := transform(ctx, javaPath, classpath, *config, temporaryPath, content)
		if err != nil {
			exitf("case %s: fresh reference runtime: %v", testCase.ID, err)
		}
		if !reflect.DeepEqual(acceptedIssues, want) {
			exitf("case %s: accepted semantic issues = %+v, want %+v", testCase.ID, acceptedIssues, want)
		}
		if !reflect.DeepEqual(freshIssues, want) {
			exitf("case %s: fresh-reference semantic issues = %+v, want %+v", testCase.ID, freshIssues, want)
		}
	}
	fmt.Printf("Peppol reference cross-check passed: release=%s source=%s cases=%d patterns=%d rules=%d asserts=%d reports=%d\n",
		manifest.Release.BISVersion, manifest.Release.SourceCommit, len(manifest.Cases),
		manifest.Inventory.ActivePatterns, manifest.Inventory.Rules, manifest.Inventory.Asserts, manifest.Inventory.Reports)
}

type semanticIssue struct {
	Kind, RuleID, Severity string
}

func expectedIssues(testCase validationassets.PeppolReferenceCase) []semanticIssue {
	issues := make([]semanticIssue, 0, len(testCase.Issues))
	for _, issue := range testCase.Issues {
		issues = append(issues, semanticIssue{Kind: issue.Kind, RuleID: issue.RuleID, Severity: issue.Severity})
	}
	return issues
}

func transform(ctx context.Context, javaPath, classpath, config, stylesheet string, input []byte) ([]semanticIssue, error) {
	command := exec.CommandContext(ctx, javaPath, "-cp", classpath, "net.sf.saxon.Transform",
		"-config:"+config, "-ext:off", "-s:-", "-xsl:"+stylesheet)
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Saxon: %w: %s", err, stderr.String())
	}
	report, err := svrl.ParseForLayer(bytes.NewReader(stdout.Bytes()), queue.ValidationLayerPeppolBISBilling)
	if err != nil {
		return nil, err
	}
	issues := make([]semanticIssue, 0, len(report.Issues))
	for _, issue := range report.Issues {
		issues = append(issues, semanticIssue{Kind: string(issue.Kind), RuleID: issue.Rule, Severity: issue.Severity})
	}
	return issues, nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
