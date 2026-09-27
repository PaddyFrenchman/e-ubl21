package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/svrl"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

// PeppolEnvelopeValidator executes the independently packaged SBDH rules.
type PeppolEnvelopeValidator interface {
	Validate(context.Context, io.Reader) (queue.ValidationStageResult, error)
}

type saxonPeppolEnvelopeValidator struct {
	runtime saxonRuntimeConfig
	timeout time.Duration
}

func newSaxonPeppolEnvelopeValidator(runtime saxonRuntimeConfig, timeout time.Duration) *saxonPeppolEnvelopeValidator {
	if timeout <= 0 {
		timeout = defaultPeppolValidationTimeout
	}
	return &saxonPeppolEnvelopeValidator{runtime: runtime, timeout: timeout}
}

func (v *saxonPeppolEnvelopeValidator) Validate(ctx context.Context, input io.Reader) (queue.ValidationStageResult, error) {
	if v == nil || input == nil {
		return queue.ValidationStageResult{}, fmt.Errorf("Peppol envelope validation: validator or XML reader is nil")
	}
	var document bytes.Buffer
	run, err := (&saxonSVRLRunner{runtime: v.runtime, stylesheetName: "Peppol envelope XSLT", layer: queue.ValidationLayerPeppolEnvelope, timeout: v.timeout, waitDelay: defaultEN16931ProcessWaitDelay, limits: svrl.DefaultLimits(), now: time.Now}).run(ctx, io.TeeReader(io.LimitReader(input, maxUploadBytes+1), &document))
	if err != nil {
		return queue.ValidationStageResult{}, classifyPeppolRuntimeError(err)
	}
	if document.Len() > maxUploadBytes {
		return queue.ValidationStageResult{}, fmt.Errorf("Peppol envelope validation: document exceeds %d bytes", maxUploadBytes)
	}
	issues := append(run.report.Issues, envelopeIdentifierIssues(document.Bytes())...)
	return queue.ValidationStageResult{Layer: queue.ValidationLayerPeppolEnvelope, Version: validationassets.PeppolEnvelopeVersion, Valid: !queue.StageIssuesContainStandardsFailure(issues), Issues: issues, ValidatedAt: run.completedAt.UTC().Format(time.RFC3339Nano), DurationMS: run.completedAt.Sub(run.startedAt).Milliseconds()}, nil
}

type sbdhScope struct{ Type, InstanceIdentifier, Identifier string }

func envelopeIdentifierIssues(document []byte) []queue.ValidationIssue {
	payload, local, namespace, err := peppolEnvelopePayload(document)
	if err != nil {
		return []queue.ValidationIssue{envelopeIssue("EUBL21-SBDH-HEADER", err.Error())}
	}
	local, namespace, err = rootElementOfPayload(payload)
	if err != nil {
		return []queue.ValidationIssue{envelopeIssue("EUBL21-SBDH-PAYLOAD", err.Error())}
	}
	customization, profile, scopes, sender, receiver := envelopeFields(document)
	issues := []queue.ValidationIssue{}
	if sender == "" {
		issues = append(issues, envelopeIssue("EUBL21-SBDH-SENDER", "SBDH Sender/Identifier is required"))
	}
	if receiver == "" {
		issues = append(issues, envelopeIssue("EUBL21-SBDH-RECEIVER", "SBDH Receiver/Identifier is required"))
	}
	wantDocument := namespace + "::" + local + "##" + customization + "::2.1"
	wantProcess := profile
	var documentScope, processScope *sbdhScope
	documentScopeCount, processScopeCount := 0, 0
	for i := range scopes {
		if scopes[i].Type == "DOCUMENTID" {
			documentScopeCount++
			documentScope = &scopes[i]
		}
		if scopes[i].Type == "PROCESSID" {
			processScopeCount++
			processScope = &scopes[i]
		}
	}
	if documentScopeCount != 1 || documentScope == nil || documentScope.Identifier != "busdox-docid-qns" || documentScope.InstanceIdentifier != wantDocument {
		issues = append(issues, envelopeIssue("EUBL21-SBDH-DOCUMENTID", "SBDH DOCUMENTID must identify the embedded UBL document and customization identifier"))
	}
	if processScopeCount != 1 || processScope == nil || processScope.Identifier != "cenbii-procid-ubl" || processScope.InstanceIdentifier != wantProcess {
		issues = append(issues, envelopeIssue("EUBL21-SBDH-PROCESSID", "SBDH PROCESSID must match the embedded UBL ProfileID"))
	}
	return issues
}

func envelopeIssue(rule, message string) queue.ValidationIssue {
	return queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerPeppolEnvelope, Rule: rule, Severity: "error", Message: message}
}

func envelopeFields(document []byte) (customization, profile string, scopes []sbdhScope, sender, receiver string) {
	d := xml.NewDecoder(bytes.NewReader(document))
	var stack []xml.Name
	for {
		token, err := d.Token()
		if err != nil {
			break
		}
		switch token := token.(type) {
		case xml.StartElement:
			stack = append(stack, token.Name)
			if token.Name.Local == "Scope" {
				var raw struct {
					Type               string `xml:"Type"`
					InstanceIdentifier string `xml:"InstanceIdentifier"`
					Identifier         string `xml:"Identifier"`
				}
				if d.DecodeElement(&raw, &token) == nil {
					scopes = append(scopes, sbdhScope{strings.TrimSpace(raw.Type), strings.TrimSpace(raw.InstanceIdentifier), strings.TrimSpace(raw.Identifier)})
				}
				stack = stack[:len(stack)-1]
				continue
			}
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			value := strings.TrimSpace(string(token))
			if value == "" {
				continue
			}
			current := stack[len(stack)-1].Local
			parent := ""
			if len(stack) > 1 {
				parent = stack[len(stack)-2].Local
			}
			if current == "CustomizationID" {
				customization = value
			}
			if current == "ProfileID" {
				profile = value
			}
			if current == "Identifier" && parent == "Sender" {
				sender = value
			}
			if current == "Identifier" && parent == "Receiver" {
				receiver = value
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return
}
