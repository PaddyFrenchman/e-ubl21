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
	senderCount, receiverCount  int
	senderIDs, receiverIDs      int
}

func (header EnvelopeHeader) HasSingleSenderIdentifier() bool {
	return header.senderCount == 1 && header.senderIDs == 1
}

func (header EnvelopeHeader) HasSingleReceiverIdentifier() bool {
	return header.receiverCount == 1 && header.receiverIDs == 1
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
	result.Header.normalizeBindings()
	return result, nil
}

// ExtractEnvelopePayload returns the byte-exact Invoice or CreditNote carried
// by a complete SBDH document. Keeping this selection in the Peppol package
// gives payload-stage workers and final-result shaping one parsing contract.
func ExtractEnvelopePayload(document []byte) ([]byte, error) {
	envelope, err := ParseEnvelope(document)
	if err != nil {
		return nil, err
	}
	return envelope.Payload, nil
}

func (header *EnvelopeHeader) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	if start.Name.Space != SBDHNamespace || start.Name.Local != "StandardBusinessDocumentHeader" {
		return fmt.Errorf("unexpected header {%s}%s", start.Name.Space, start.Name.Local)
	}
	path := []xml.Name{start.Name}
	documentCount := 0
	documentIDs, processIDs := 0, 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch value := token.(type) {
		case xml.Directive:
			return errors.New("XML directives are not allowed")
		case xml.StartElement:
			path = append(path, value.Name)
			if len(path) == 2 && sbdhName(value.Name, "Sender") {
				header.senderCount++
			} else if len(path) == 2 && sbdhName(value.Name, "Receiver") {
				header.receiverCount++
			} else if len(path) == 2 && sbdhName(value.Name, "DocumentIdentification") {
				documentCount++
			}

			var destination *string
			switch {
			case len(path) == 3 && sbdhName(path[1], "Sender") && sbdhName(value.Name, "Identifier"):
				header.senderIDs++
				destination = &header.Sender
			case len(path) == 3 && sbdhName(path[1], "Receiver") && sbdhName(value.Name, "Identifier"):
				header.receiverIDs++
				destination = &header.Receiver
			case len(path) == 3 && sbdhName(path[1], "DocumentIdentification") && sbdhName(value.Name, "Standard"):
				destination = &header.Standard
			case len(path) == 3 && sbdhName(path[1], "DocumentIdentification") && sbdhName(value.Name, "TypeVersion"):
				destination = &header.TypeVersion
			case len(path) == 3 && sbdhName(path[1], "DocumentIdentification") && sbdhName(value.Name, "Type"):
				destination = &header.Type
			}
			if destination != nil {
				text, err := envelopeTextElement(decoder, value)
				if err != nil {
					return err
				}
				*destination = text
				path = path[:len(path)-1]
				continue
			}
			if len(path) == 3 && sbdhName(path[1], "BusinessScope") && sbdhName(value.Name, "Scope") {
				scopeType, identifier, err := parseEnvelopeScope(decoder, value)
				if err != nil {
					return err
				}
				switch strings.TrimSpace(scopeType) {
				case "DOCUMENTID":
					documentIDs++
					header.DocumentID = identifier
				case "PROCESSID":
					processIDs++
					header.ProcessID = identifier
				}
				path = path[:len(path)-1]
			}
		case xml.EndElement:
			if len(path) == 1 {
				if value.Name != start.Name {
					return fmt.Errorf("unexpected closing element {%s}%s", value.Name.Space, value.Name.Local)
				}
				if documentCount != 1 {
					header.Standard, header.TypeVersion, header.Type = "", "", ""
				}
				if documentIDs != 1 {
					header.DocumentID = ""
				}
				if processIDs != 1 {
					header.ProcessID = ""
				}
				return nil
			}
			path = path[:len(path)-1]
		}
	}
}

func parseEnvelopeScope(decoder *xml.Decoder, start xml.StartElement) (string, string, error) {
	depth := 1
	var scopeType, identifier string
	for depth > 0 {
		token, err := decoder.Token()
		if err != nil {
			return "", "", err
		}
		switch value := token.(type) {
		case xml.Directive:
			return "", "", errors.New("XML directives are not allowed")
		case xml.StartElement:
			depth++
			if depth == 2 && (sbdhName(value.Name, "Type") || sbdhName(value.Name, "InstanceIdentifier")) {
				text, err := envelopeTextElement(decoder, value)
				if err != nil {
					return "", "", err
				}
				depth--
				if value.Name.Local == "Type" {
					scopeType = text
				} else {
					identifier = text
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	return scopeType, identifier, nil
}

func envelopeTextElement(decoder *xml.Decoder, start xml.StartElement) (string, error) {
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.CharData:
			text.Write(value)
		case xml.Comment:
			continue
		case xml.EndElement:
			if value.Name != start.Name {
				return "", fmt.Errorf("unexpected closing element {%s}%s", value.Name.Space, value.Name.Local)
			}
			return text.String(), nil
		case xml.StartElement:
			return "", fmt.Errorf("{%s}%s must contain text only", start.Name.Space, start.Name.Local)
		case xml.Directive:
			return "", errors.New("XML directives are not allowed")
		}
	}
}

func sbdhName(name xml.Name, local string) bool {
	return name.Space == SBDHNamespace && name.Local == local
}

func (header *EnvelopeHeader) normalizeBindings() {
	header.Standard = strings.TrimSpace(header.Standard)
	header.TypeVersion = strings.TrimSpace(header.TypeVersion)
	header.Type = strings.TrimSpace(header.Type)
	header.DocumentID = strings.TrimSpace(header.DocumentID)
	header.ProcessID = strings.TrimSpace(header.ProcessID)
}
