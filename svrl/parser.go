// Package svrl decodes Schematron Validation Report Language documents into
// the stable validation model used by queue persistence and result events.
package svrl

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

const Namespace = "http://purl.oclc.org/dsdl/svrl"

// Limits bounds both parser memory and the normalized result stored in KV or
// published to JetStream. Exceeding a limit fails parsing; returning a partial
// report could hide a standards-invalid assertion.
type Limits struct {
	MaxInputBytes      int64
	MaxIssues          int
	MaxRuleBytes       int
	MaxSeverityBytes   int
	MaxLocationBytes   int
	MaxTestBytes       int
	MaxMessageBytes    int
	MaxDiagnosticBytes int
	MaxTotalTextBytes  int
}

// DefaultLimits returns an independent copy of the production parser bounds.
func DefaultLimits() Limits {
	return Limits{
		MaxInputBytes:    4 << 20,
		MaxIssues:        256,
		MaxRuleBytes:     256,
		MaxSeverityBytes: 64,
		MaxLocationBytes: 2 << 10,
		// The pinned ruleset contains legitimate failed-assert test expressions
		// above 2 KiB (notably VAT breakdown reconciliation). Keep them bounded
		// without turning a real semantic failure into an operational retry.
		MaxTestBytes:       16 << 10,
		MaxMessageBytes:    4 << 10,
		MaxDiagnosticBytes: 4 << 10,
		MaxTotalTextBytes:  256 << 10,
	}
}

var ErrMalformed = errors.New("malformed SVRL")
var ErrLimitExceeded = errors.New("SVRL limit exceeded")

// Report is the deterministic normalized representation of an SVRL document.
// Issues retain document order across failed-assert and successful-report
// elements.
type Report struct {
	Issues            []queue.ValidationIssue
	FailedAssertions  int
	SuccessfulReports int
}

// Parse decodes SVRL using production bounds.
func Parse(reader io.Reader) (Report, error) {
	return ParseForLayer(reader, queue.ValidationLayerEN16931)
}

// ParseForLayer decodes SVRL and attributes every normalized issue to the
// semantic validation layer that executed the Schematron. Only the two SVRL
// backed layers are accepted so callers cannot manufacture ambiguous issue
// provenance.
func ParseForLayer(reader io.Reader, layer queue.ValidationLayer) (Report, error) {
	return ParseWithLimitsForLayer(reader, DefaultLimits(), layer)
}

// ParseWithLimits decodes one complete SVRL document. Malformed XML, an
// unexpected root, read failures, and limit violations return errors and no
// partial report, allowing callers to classify them as operational failures.
func ParseWithLimits(reader io.Reader, limits Limits) (Report, error) {
	return ParseWithLimitsForLayer(reader, limits, queue.ValidationLayerEN16931)
}

// ParseWithLimitsForLayer is ParseForLayer with caller-supplied resource
// bounds. The legacy Parse and ParseWithLimits entry points retain EN 16931 as
// their default layer for compatibility.
func ParseWithLimitsForLayer(reader io.Reader, limits Limits, layer queue.ValidationLayer) (Report, error) {
	if reader == nil {
		return Report{}, fmt.Errorf("read SVRL: reader is nil")
	}
	if layer != queue.ValidationLayerEN16931 && layer != queue.ValidationLayerPeppolBISBilling {
		return Report{}, fmt.Errorf("parse SVRL: unsupported validation layer %q", layer)
	}
	if err := validateLimits(limits); err != nil {
		return Report{}, err
	}
	content, err := io.ReadAll(io.LimitReader(reader, limits.MaxInputBytes+1))
	if err != nil {
		return Report{}, fmt.Errorf("read SVRL: %w", err)
	}
	if int64(len(content)) > limits.MaxInputBytes {
		return Report{}, fmt.Errorf("%w: input exceeds %d bytes", ErrLimitExceeded, limits.MaxInputBytes)
	}

	decoder := xml.NewDecoder(bytes.NewReader(content))
	var report Report
	rootSeen := false
	totalTextBytes := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return Report{}, fmt.Errorf("%w: %v", ErrMalformed, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if !rootSeen {
			rootSeen = true
			if start.Name.Space != Namespace || start.Name.Local != "schematron-output" {
				return Report{}, fmt.Errorf("%w: root is {%s}%s, want {%s}schematron-output", ErrMalformed, start.Name.Space, start.Name.Local, Namespace)
			}
			continue
		}
		if start.Name.Space != Namespace || (start.Name.Local != "failed-assert" && start.Name.Local != "successful-report") {
			continue
		}
		if len(report.Issues) >= limits.MaxIssues {
			return Report{}, fmt.Errorf("%w: issue count exceeds %d", ErrLimitExceeded, limits.MaxIssues)
		}
		assertion, err := decodeAssertion(decoder, start)
		if err != nil {
			return Report{}, fmt.Errorf("%w: decode %s: %v", ErrMalformed, start.Name.Local, err)
		}
		issue := assertion.issue(layer)
		if err := validateIssue(issue, limits, &totalTextBytes); err != nil {
			return Report{}, err
		}
		report.Issues = append(report.Issues, issue)
		if start.Name.Local == "failed-assert" {
			report.FailedAssertions++
		} else {
			report.SuccessfulReports++
		}
	}
	if !rootSeen {
		return Report{}, fmt.Errorf("%w: document has no root element", ErrMalformed)
	}
	return report, nil
}

type assertion struct {
	Kind        queue.ValidationIssueKind
	ID          string
	Flag        string
	Role        string
	Location    string
	Test        string
	Text        string
	Diagnostics []diagnostic
}

type diagnostic struct {
	ID   string
	Text string
}

func decodeAssertion(decoder *xml.Decoder, start xml.StartElement) (assertion, error) {
	result := assertion{
		Kind:     issueKind(start.Name.Local),
		ID:       attribute(start, "id"),
		Flag:     attribute(start, "flag"),
		Role:     attribute(start, "role"),
		Location: attribute(start, "location"),
		Test:     attribute(start, "test"),
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return assertion{}, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Space != Namespace {
				continue
			}
			switch value.Name.Local {
			case "text":
				text, err := decodeElementText(decoder, value)
				if err != nil {
					return assertion{}, err
				}
				result.Text = text
			case "diagnostic-reference":
				text, err := decodeElementText(decoder, value)
				if err != nil {
					return assertion{}, err
				}
				result.Diagnostics = append(result.Diagnostics, diagnostic{ID: attribute(value, "diagnostic"), Text: text})
			}
		case xml.EndElement:
			if value.Name == start.Name {
				return result, nil
			}
		}
	}
}

func decodeElementText(decoder *xml.Decoder, start xml.StartElement) (string, error) {
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
	return normalizeWhitespace(text.String()), nil
}

func (assertion assertion) issue(layer queue.ValidationLayer) queue.ValidationIssue {
	severity := normalizeWhitespace(assertion.Flag)
	if severity == "" {
		severity = normalizeWhitespace(assertion.Role)
	}
	diagnostics := make([]string, 0, len(assertion.Diagnostics))
	for _, reference := range assertion.Diagnostics {
		id := normalizeWhitespace(reference.ID)
		text := normalizeWhitespace(reference.Text)
		switch {
		case id != "" && text != "":
			diagnostics = append(diagnostics, id+": "+text)
		case text != "":
			diagnostics = append(diagnostics, text)
		case id != "":
			diagnostics = append(diagnostics, id)
		}
	}
	return queue.ValidationIssue{
		Kind:       assertion.Kind,
		Layer:      layer,
		Rule:       normalizeWhitespace(assertion.ID),
		Severity:   severity,
		Location:   normalizeWhitespace(assertion.Location),
		Test:       normalizeWhitespace(assertion.Test),
		Message:    normalizeWhitespace(assertion.Text),
		Diagnostic: strings.Join(diagnostics, "; "),
	}
}

func issueKind(element string) queue.ValidationIssueKind {
	if element == "successful-report" {
		return queue.ValidationIssueSuccessfulReport
	}
	return queue.ValidationIssueFailedAssert
}

func attribute(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func validateIssue(issue queue.ValidationIssue, limits Limits, total *int) error {
	fields := []struct {
		name  string
		value string
		limit int
	}{
		{"rule", issue.Rule, limits.MaxRuleBytes},
		{"severity", issue.Severity, limits.MaxSeverityBytes},
		{"location", issue.Location, limits.MaxLocationBytes},
		{"test", issue.Test, limits.MaxTestBytes},
		{"message", issue.Message, limits.MaxMessageBytes},
		{"diagnostic", issue.Diagnostic, limits.MaxDiagnosticBytes},
	}
	for _, field := range fields {
		if len(field.value) > field.limit {
			return fmt.Errorf("%w: %s exceeds %d bytes", ErrLimitExceeded, field.name, field.limit)
		}
		*total += len(field.value)
		if *total > limits.MaxTotalTextBytes {
			return fmt.Errorf("%w: normalized issue text exceeds %d bytes", ErrLimitExceeded, limits.MaxTotalTextBytes)
		}
	}
	return nil
}

func validateLimits(limits Limits) error {
	if limits.MaxInputBytes <= 0 || limits.MaxInputBytes == math.MaxInt64 || limits.MaxIssues <= 0 || limits.MaxRuleBytes <= 0 ||
		limits.MaxSeverityBytes <= 0 || limits.MaxLocationBytes <= 0 || limits.MaxTestBytes <= 0 ||
		limits.MaxMessageBytes <= 0 || limits.MaxDiagnosticBytes <= 0 || limits.MaxTotalTextBytes <= 0 {
		return fmt.Errorf("invalid SVRL limits: all limits must be positive")
	}
	return nil
}
