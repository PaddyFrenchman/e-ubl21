package svrl_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
)

const prefix = `<svrl:schematron-output xmlns:svrl="http://purl.oclc.org/dsdl/svrl">`
const suffix = `</svrl:schematron-output>`

func TestParsePassingReport(t *testing.T) {
	report, err := svrl.Parse(strings.NewReader(prefix + `<svrl:active-pattern id="EN16931-UBL"/>` + suffix))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 0 || report.FailedAssertions != 0 || report.SuccessfulReports != 0 {
		t.Fatalf("passing report = %+v", report)
	}
}

func TestParseFatalFailurePreservesOfficialFields(t *testing.T) {
	document := prefix + `
<svrl:failed-assert id="BR-CO-10" flag="fatal"
 location=" /ubl:Invoice/cac:LegalMonetaryTotal " test=" sum(cac:InvoiceLine) = cbc:LineExtensionAmount ">
  <svrl:text> Sum of invoice line <em>net amounts</em>
      must equal the total. </svrl:text>
  <svrl:diagnostic-reference diagnostic="BT-106"> Line total mismatch. </svrl:diagnostic-reference>
</svrl:failed-assert>` + suffix
	report, err := svrl.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if report.FailedAssertions != 1 || report.SuccessfulReports != 0 || len(report.Issues) != 1 {
		t.Fatalf("report counts = %+v", report)
	}
	want := queue.ValidationIssue{
		Kind:  queue.ValidationIssueFailedAssert,
		Layer: queue.ValidationLayerEN16931, Rule: "BR-CO-10", Severity: "fatal",
		Location: "/ubl:Invoice/cac:LegalMonetaryTotal", Test: "sum(cac:InvoiceLine) = cbc:LineExtensionAmount",
		Message: "Sum of invoice line net amounts must equal the total.", Diagnostic: "BT-106: Line total mismatch.",
	}
	if got := report.Issues[0]; got != want {
		t.Fatalf("issue = %#v, want %#v", got, want)
	}
}

func TestParseWarningSuccessfulReportAndRoleFallback(t *testing.T) {
	document := prefix + `
<svrl:successful-report id="UBL-SR-53" role="warning" location="/ubl:Invoice" test="cbc:Note">
  <svrl:text> Optional note was supplied. </svrl:text>
</svrl:successful-report>` + suffix
	report, err := svrl.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if report.SuccessfulReports != 1 || len(report.Issues) != 1 {
		t.Fatalf("report = %+v", report)
	}
	issue := report.Issues[0]
	if issue.Kind != queue.ValidationIssueSuccessfulReport || issue.Rule != "UBL-SR-53" || issue.Severity != "warning" || issue.Message != "Optional note was supplied." {
		t.Fatalf("successful report issue = %+v", issue)
	}
}

func TestParseMultipleIssuesRetainsDocumentOrderAndRuleFamilies(t *testing.T) {
	document := prefix + `
<svrl:failed-assert id="BR-DEC-01" flag="warning"><svrl:text> First </svrl:text></svrl:failed-assert>
<svrl:successful-report id="BR-S-08" flag="error"><svrl:text> Second </svrl:text></svrl:successful-report>
<svrl:failed-assert id="UBL-CR-660" flag="fatal"><svrl:text> Third </svrl:text></svrl:failed-assert>` + suffix
	report, err := svrl.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	wantRules := []string{"BR-DEC-01", "BR-S-08", "UBL-CR-660"}
	wantKinds := []queue.ValidationIssueKind{queue.ValidationIssueFailedAssert, queue.ValidationIssueSuccessfulReport, queue.ValidationIssueFailedAssert}
	wantSeverities := []string{"warning", "error", "fatal"}
	if report.FailedAssertions != 2 || report.SuccessfulReports != 1 || len(report.Issues) != len(wantRules) {
		t.Fatalf("report = %+v", report)
	}
	for index, want := range wantRules {
		if report.Issues[index].Rule != want {
			t.Fatalf("issue %d rule = %q, want %q", index, report.Issues[index].Rule, want)
		}
		if report.Issues[index].Kind != wantKinds[index] {
			t.Fatalf("issue %d kind = %q, want %q", index, report.Issues[index].Kind, wantKinds[index])
		}
		if report.Issues[index].Severity != wantSeverities[index] {
			t.Fatalf("issue %d severity = %q, want %q", index, report.Issues[index].Severity, wantSeverities[index])
		}
	}
}

func TestParseJoinsMultipleDiagnosticReferences(t *testing.T) {
	document := prefix + `<svrl:failed-assert id="BR-1" flag="error">
<svrl:text>Failure</svrl:text>
<svrl:diagnostic-reference diagnostic="D1"> First diagnostic. </svrl:diagnostic-reference>
<svrl:diagnostic-reference> Second diagnostic. </svrl:diagnostic-reference>
<svrl:diagnostic-reference diagnostic="D3"/>
</svrl:failed-assert>` + suffix
	report, err := svrl.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := report.Issues[0].Diagnostic, "D1: First diagnostic.; Second diagnostic.; D3"; got != want {
		t.Fatalf("diagnostic = %q, want %q", got, want)
	}
}

func TestParseRejectsMalformedOrNonSVRLDocuments(t *testing.T) {
	for _, document := range []string{
		``,
		`<not-svrl/>`,
		prefix + `<svrl:failed-assert>` + suffix,
		prefix + `<svrl:failed-assert>&unknown;</svrl:failed-assert>` + suffix,
	} {
		_, err := svrl.Parse(strings.NewReader(document))
		if err == nil || !errors.Is(err, svrl.ErrMalformed) {
			t.Errorf("Parse(%q) error = %v, want ErrMalformed", document, err)
		}
	}
}

func TestParseDoesNotReturnPartialReportOnOperationalFailure(t *testing.T) {
	document := prefix + `<svrl:failed-assert id="BR-1" flag="fatal"><svrl:text>Found first</svrl:text></svrl:failed-assert><broken>`
	report, err := svrl.Parse(strings.NewReader(document))
	if err == nil || !errors.Is(err, svrl.ErrMalformed) {
		t.Fatalf("Parse() error = %v, want ErrMalformed", err)
	}
	if len(report.Issues) != 0 || report.FailedAssertions != 0 {
		t.Fatalf("malformed SVRL returned a partial semantic report: %+v", report)
	}
}

func TestParseFailsClosedAtEveryBound(t *testing.T) {
	base := svrl.DefaultLimits()
	tests := []struct {
		name     string
		document string
		limits   svrl.Limits
	}{
		{"input", prefix + suffix, with(base, func(l *svrl.Limits) { l.MaxInputBytes = 10 })},
		{"issues", prefix + `<svrl:failed-assert/><svrl:failed-assert/>` + suffix, with(base, func(l *svrl.Limits) { l.MaxIssues = 1 })},
		{"rule", prefix + `<svrl:failed-assert id="TOO-LONG"/>` + suffix, with(base, func(l *svrl.Limits) { l.MaxRuleBytes = 3 })},
		{"message", prefix + `<svrl:failed-assert><svrl:text>TOO-LONG</svrl:text></svrl:failed-assert>` + suffix, with(base, func(l *svrl.Limits) { l.MaxMessageBytes = 3 })},
		{"diagnostic", prefix + `<svrl:failed-assert><svrl:diagnostic-reference>TOO-LONG</svrl:diagnostic-reference></svrl:failed-assert>` + suffix, with(base, func(l *svrl.Limits) { l.MaxDiagnosticBytes = 3 })},
		{"total", prefix + `<svrl:failed-assert id="1234" flag="error"/>` + suffix, with(base, func(l *svrl.Limits) { l.MaxTotalTextBytes = 4 })},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := svrl.ParseWithLimits(strings.NewReader(test.document), test.limits)
			if err == nil || !errors.Is(err, svrl.ErrLimitExceeded) {
				t.Fatalf("ParseWithLimits() error = %v, want ErrLimitExceeded", err)
			}
		})
	}
}

func TestParseRejectsInvalidLimits(t *testing.T) {
	limits := svrl.DefaultLimits()
	limits.MaxIssues = 0
	if _, err := svrl.ParseWithLimits(strings.NewReader(prefix+suffix), limits); err == nil {
		t.Fatal("ParseWithLimits() accepted an unbounded/invalid limit configuration")
	}
}

func TestDefaultLimitsAcceptPinnedLongRuleExpressions(t *testing.T) {
	// The official VAT reconciliation assertions exceed the former 2 KiB
	// field limit. They must remain bounded, but must not become operational
	// failures merely because a non-compliant document causes them to appear.
	document := prefix + `<svrl:failed-assert id="BR-S-08" flag="fatal" test="` + strings.Repeat("x", 3<<10) + `"><svrl:text>Mismatch</svrl:text></svrl:failed-assert>` + suffix
	report, err := svrl.Parse(strings.NewReader(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || len(report.Issues[0].Test) != 3<<10 {
		t.Fatalf("long official-style test expression was not retained: %+v", report)
	}
}

func TestParseRejectsNilReader(t *testing.T) {
	if _, err := svrl.Parse(nil); err == nil {
		t.Fatal("Parse(nil) returned no error")
	}
}

func with(limits svrl.Limits, change func(*svrl.Limits)) svrl.Limits {
	change(&limits)
	return limits
}
