package server

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	validationassets "github.com/PaddyFrenchman/e-ubl21/validation-assets"
)

type peppolEnvelopeValidator struct{ now func() time.Time }

func newPeppolEnvelopeValidator() semanticStageValidator {
	return &peppolEnvelopeValidator{now: time.Now}
}

func (validator *peppolEnvelopeValidator) Validate(ctx context.Context, input io.Reader) (queue.ValidationStageResult, error) {
	if ctx == nil || input == nil {
		return queue.ValidationStageResult{}, errors.New("Peppol envelope validator requires context and XML input")
	}
	started := validator.now()
	document, err := io.ReadAll(io.LimitReader(input, peppol.MaxResolutionDocumentBytes+1))
	if err != nil {
		return queue.ValidationStageResult{}, fmt.Errorf("read Peppol envelope: %w", err)
	}
	if len(document) > peppol.MaxResolutionDocumentBytes {
		return queue.ValidationStageResult{}, peppol.ErrDocumentTooLarge
	}
	envelope, err := peppol.ParseEnvelope(document)
	if err != nil {
		return queue.ValidationStageResult{}, err
	}
	identity, err := envelopePayloadIdentity(envelope.Payload)
	if err != nil {
		return queue.ValidationStageResult{}, err
	}
	var issues []queue.ValidationIssue
	add := func(rule, message string) {
		issues = append(issues, queue.ValidationIssue{
			Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerPeppolEnvelope,
			Rule: rule, Severity: "fatal", Message: message,
		})
	}
	if !envelope.Header.HasSingleReceiverIdentifier() {
		add("EUBL21-PEPPOL-ENVELOPE-RECEIVER-STRUCTURE", "SBDH must contain exactly one Receiver with exactly one direct SBDH Identifier.")
	} else if envelope.Header.Receiver != identity.CustomerEndpoint {
		add("BR-SBDH-1", "SBDH Receiver/Identifier must exactly match the customer EndpointID including schemeID.")
	}
	if !envelope.Header.HasSingleSenderIdentifier() {
		add("EUBL21-PEPPOL-ENVELOPE-SENDER-STRUCTURE", "SBDH must contain exactly one Sender with exactly one direct SBDH Identifier.")
	} else if envelope.Header.Sender != identity.SupplierEndpoint {
		add("BR-SBDH-2", "SBDH Sender/Identifier must exactly match the supplier EndpointID including schemeID.")
	}
	if envelope.Header.Standard != envelope.Namespace {
		add("EUBL21-PEPPOL-ENVELOPE-DOCUMENT-STANDARD", "SBDH DocumentIdentification/Standard must match the embedded UBL namespace.")
	}
	if envelope.Header.TypeVersion != "2.1" {
		add("EUBL21-PEPPOL-ENVELOPE-DOCUMENT-VERSION", "SBDH DocumentIdentification/TypeVersion must be 2.1.")
	}
	if envelope.Header.Type != envelope.DocumentType {
		add("EUBL21-PEPPOL-ENVELOPE-DOCUMENT-TYPE", "SBDH DocumentIdentification/Type must match the embedded UBL document type.")
	}
	wantDocumentID := envelope.Namespace + "::" + envelope.DocumentType + "##" + identity.CustomizationID + "::2.1"
	if envelope.Header.DocumentID != wantDocumentID {
		add("EUBL21-PEPPOL-ENVELOPE-DOCUMENT-ID", "SBDH DOCUMENTID must match the embedded UBL namespace, type, CustomizationID, and version.")
	}
	if envelope.Header.ProcessID != identity.ProfileID {
		add("EUBL21-PEPPOL-ENVELOPE-PROCESS-ID", "SBDH PROCESSID must match the embedded UBL ProfileID.")
	}
	completed := validator.now()
	return queue.ValidationStageResult{
		Layer: queue.ValidationLayerPeppolEnvelope, Version: validationassets.PeppolEnvelope().Version,
		Valid: len(issues) == 0, Issues: issues, ValidatedAt: completed.UTC().Format(time.RFC3339Nano),
		DurationMS: completed.Sub(started).Milliseconds(),
	}, nil
}

type payloadEnvelopeIdentity struct{ CustomizationID, ProfileID, SupplierEndpoint, CustomerEndpoint string }

func envelopePayloadIdentity(payload []byte) (payloadEnvelopeIdentity, error) {
	decoder := xml.NewDecoder(bytes.NewReader(payload))
	decoder.Strict = true
	var result payloadEnvelopeIdentity
	var path []xml.Name
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return result, fmt.Errorf("parse embedded UBL identity: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			path = append(path, value.Name)
			if len(path) == 2 && value.Name.Space == "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" && (value.Name.Local == "CustomizationID" || value.Name.Local == "ProfileID") {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return result, err
				}
				path = path[:len(path)-1]
				if value.Name.Local == "CustomizationID" {
					result.CustomizationID = strings.TrimSpace(text)
				} else {
					result.ProfileID = strings.TrimSpace(text)
				}
				continue
			}
			if value.Name.Space == "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" && value.Name.Local == "EndpointID" {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return result, err
				}
				path = path[:len(path)-1]
				scheme := ""
				for _, attr := range value.Attr {
					if attr.Name.Local == "schemeID" {
						scheme = strings.TrimSpace(attr.Value)
					}
				}
				endpoint := scheme + ":" + strings.TrimSpace(text)
				for _, ancestor := range path {
					switch ancestor.Local {
					case "AccountingSupplierParty":
						result.SupplierEndpoint = endpoint
					case "AccountingCustomerParty":
						result.CustomerEndpoint = endpoint
					}
				}
				continue
			}
		case xml.EndElement:
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		}
	}
	return result, nil
}
