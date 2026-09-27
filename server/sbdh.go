package server

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

const standardBusinessDocumentNamespace = "http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader"

// peppolEnvelopePayload returns the sole non-header child of an SBDH
// StandardBusinessDocument. The returned XML is deliberately re-encoded from
// tokens: stage workers need the UBL infoset, while the original bytes remain
// stored for the later envelope stage.
func peppolEnvelopePayload(document []byte) ([]byte, string, string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(document))
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, "", "", fmt.Errorf("read SBDH envelope: %w", err)
		}
		root, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if root.Name.Local != "StandardBusinessDocument" || root.Name.Space != standardBusinessDocumentNamespace {
			return nil, "", "", fmt.Errorf("root is {%s}%s, not StandardBusinessDocument", root.Name.Space, root.Name.Local)
		}
		break
	}

	var payload []byte
	headerCount := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, "", "", fmt.Errorf("read SBDH envelope children: %w", err)
		}
		switch token := token.(type) {
		case xml.EndElement:
			if token.Name.Local == "StandardBusinessDocument" && token.Name.Space == standardBusinessDocumentNamespace {
				if headerCount != 1 {
					return nil, "", "", fmt.Errorf("SBDH envelope has no StandardBusinessDocumentHeader")
				}
				if payload == nil {
					return nil, "", "", fmt.Errorf("SBDH envelope has no payload")
				}
				return payload, "", "", nil
			}
		case xml.StartElement:
			if token.Name.Local == "StandardBusinessDocumentHeader" && token.Name.Space == standardBusinessDocumentNamespace {
				headerCount++
				if headerCount > 1 || payload != nil {
					return nil, "", "", fmt.Errorf("SBDH envelope has an invalid StandardBusinessDocumentHeader position or count")
				}
				if err := decoder.Skip(); err != nil {
					return nil, "", "", fmt.Errorf("skip SBDH header: %w", err)
				}
				continue
			}
			if payload != nil {
				return nil, "", "", fmt.Errorf("SBDH envelope has more than one payload")
			}
			var encoded bytes.Buffer
			encoder := xml.NewEncoder(&encoded)
			if err := encoder.EncodeToken(token); err != nil {
				return nil, "", "", err
			}
			depth := 1
			for depth > 0 {
				child, err := decoder.Token()
				if err != nil {
					return nil, "", "", fmt.Errorf("read SBDH payload: %w", err)
				}
				switch child.(type) {
				case xml.StartElement:
					depth++
				case xml.EndElement:
					depth--
				}
				if err := encoder.EncodeToken(child); err != nil {
					return nil, "", "", err
				}
			}
			if err := encoder.Flush(); err != nil {
				return nil, "", "", err
			}
			payload = encoded.Bytes()
		}
	}
}

func stageInput(plan string, layer validationLayer, content []byte) ([]byte, error) {
	if plan != string(queue.ValidationProfilePeppolEnvelope) || layer == queue.ValidationLayerPeppolEnvelope {
		return content, nil
	}
	payload, _, _, err := peppolEnvelopePayload(content)
	if err != nil {
		return nil, err
	}
	return payload, nil
}
