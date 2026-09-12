package peppol

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const SBDHNamespace = "http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader"

var ErrMalformedEnvelope = errors.New("malformed Peppol SBDH envelope")

// Envelope is the bounded information needed to route and validate one
// StandardBusinessDocument. Payload retains the exact submitted bytes of the
// embedded Invoice or CreditNote so payload validators see no rewritten XML.
type Envelope struct {
	DocumentType string
	Namespace    string
	Payload      []byte
	Header       EnvelopeHeader
}

type EnvelopeHeader struct {
	Sender, Receiver            string
	Standard, TypeVersion, Type string
	DocumentID, ProcessID       string
}

// ParseEnvelope accepts exactly one SBDH root, header, and immediate embedded
// UBL Invoice or CreditNote. It rejects directives and ambiguous payloads.
func ParseEnvelope(document []byte) (Envelope, error) {
	if len(document) > MaxResolutionDocumentBytes {
		return Envelope{}, fmt.Errorf("%w: maximum is %d bytes", ErrDocumentTooLarge, MaxResolutionDocumentBytes)
	}
	decoder := xml.NewDecoder(bytes.NewReader(document))
	decoder.Strict = true
	var result Envelope
	rootSeen, headerSeen := false, false
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Envelope{}, fmt.Errorf("%w: %v", ErrMalformedEnvelope, err)
		}
		switch value := token.(type) {
		case xml.Directive:
			return Envelope{}, fmt.Errorf("%w: XML directives are not allowed", ErrMalformedEnvelope)
		case xml.StartElement:
			if !rootSeen {
				rootSeen = true
				if value.Name.Space != SBDHNamespace || value.Name.Local != "StandardBusinessDocument" {
					return Envelope{}, fmt.Errorf("%w: root {%s}%s is not StandardBusinessDocument", ErrMalformedEnvelope, value.Name.Space, value.Name.Local)
				}
				depth = 1
				continue
			}
			if depth == 0 {
				return Envelope{}, fmt.Errorf("%w: multiple root elements", ErrMalformedEnvelope)
			}
			if depth == 1 && value.Name.Space == SBDHNamespace && value.Name.Local == "StandardBusinessDocumentHeader" {
				if headerSeen {
					return Envelope{}, fmt.Errorf("%w: duplicate StandardBusinessDocumentHeader", ErrMalformedEnvelope)
				}
				headerSeen = true
				if err := decoder.DecodeElement(&result.Header, &value); err != nil {
					return Envelope{}, fmt.Errorf("%w: header: %v", ErrMalformedEnvelope, err)
				}
				continue
			}
			if depth == 1 && ((value.Name.Local == "Invoice" && value.Name.Space == invoiceNamespace) ||
				(value.Name.Local == "CreditNote" && value.Name.Space == creditNoteNamespace)) {
				if len(result.Payload) != 0 {
					return Envelope{}, fmt.Errorf("%w: multiple embedded UBL payloads", ErrMalformedEnvelope)
				}
				endOfStart := int(decoder.InputOffset())
				start := bytes.LastIndexByte(document[:endOfStart], '<')
				if start < 0 {
					return Envelope{}, fmt.Errorf("%w: cannot locate embedded payload", ErrMalformedEnvelope)
				}
				if err := decoder.Skip(); err != nil {
					return Envelope{}, fmt.Errorf("%w: payload: %v", ErrMalformedEnvelope, err)
				}
				result.DocumentType, result.Namespace = value.Name.Local, value.Name.Space
				result.Payload = append([]byte(nil), document[start:int(decoder.InputOffset())]...)
				continue
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return Envelope{}, fmt.Errorf("%w: unexpected closing element", ErrMalformedEnvelope)
			}
		}
	}
	if !rootSeen || !headerSeen || len(result.Payload) == 0 {
		return Envelope{}, fmt.Errorf("%w: one header and one embedded Invoice or CreditNote are required", ErrMalformedEnvelope)
	}
	result.Header.normalize()
	return result, nil
}

func (header *EnvelopeHeader) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var raw struct {
		Senders []struct {
			Identifier string `xml:"Identifier"`
		} `xml:"Sender"`
		Receivers []struct {
			Identifier string `xml:"Identifier"`
		} `xml:"Receiver"`
		Documents []struct {
			Standard    string `xml:"Standard"`
			TypeVersion string `xml:"TypeVersion"`
			Type        string `xml:"Type"`
		} `xml:"DocumentIdentification"`
		Scopes []struct {
			Type               string `xml:"Type"`
			InstanceIdentifier string `xml:"InstanceIdentifier"`
		} `xml:"BusinessScope>Scope"`
	}
	if err := decoder.DecodeElement(&raw, &start); err != nil {
		return err
	}
	if len(raw.Senders) == 1 {
		header.Sender = raw.Senders[0].Identifier
	}
	if len(raw.Receivers) == 1 {
		header.Receiver = raw.Receivers[0].Identifier
	}
	if len(raw.Documents) == 1 {
		header.Standard, header.TypeVersion, header.Type = raw.Documents[0].Standard, raw.Documents[0].TypeVersion, raw.Documents[0].Type
	}
	documentIDs, processIDs := 0, 0
	for _, scope := range raw.Scopes {
		switch strings.TrimSpace(scope.Type) {
		case "DOCUMENTID":
			documentIDs++
			header.DocumentID = scope.InstanceIdentifier
		case "PROCESSID":
			processIDs++
			header.ProcessID = scope.InstanceIdentifier
		}
	}
	if documentIDs != 1 {
		header.DocumentID = ""
	}
	if processIDs != 1 {
		header.ProcessID = ""
	}
	return nil
}

func (header *EnvelopeHeader) normalize() {
	header.Sender = strings.TrimSpace(header.Sender)
	header.Receiver = strings.TrimSpace(header.Receiver)
	header.Standard = strings.TrimSpace(header.Standard)
	header.TypeVersion = strings.TrimSpace(header.TypeVersion)
	header.Type = strings.TrimSpace(header.Type)
	header.DocumentID = strings.TrimSpace(header.DocumentID)
	header.ProcessID = strings.TrimSpace(header.ProcessID)
}
