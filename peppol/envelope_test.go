package peppol

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseEnvelopePreservesEmbeddedPayload(t *testing.T) {
	payload := `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"><ID>one</ID></Invoice>`
	document := `<StandardBusinessDocument xmlns="` + SBDHNamespace + `"><StandardBusinessDocumentHeader><Sender><Identifier>A</Identifier></Sender><Receiver><Identifier>B</Identifier></Receiver></StandardBusinessDocumentHeader>` + payload + `</StandardBusinessDocument>`
	envelope, err := ParseEnvelope([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if envelope.DocumentType != "Invoice" || !bytes.Equal(envelope.Payload, []byte(payload)) {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestParseEnvelopeRejectsAmbiguousPayload(t *testing.T) {
	payload := `<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`
	_, err := ParseEnvelope([]byte(`<StandardBusinessDocument xmlns="` + SBDHNamespace + `"><StandardBusinessDocumentHeader/>` + payload + payload + `</StandardBusinessDocument>`))
	if err == nil || !strings.Contains(err.Error(), "multiple embedded") {
		t.Fatalf("expected ambiguous payload error, got %v", err)
	}
}

func TestExtractEnvelopePayloadUsesParseEnvelopeContract(t *testing.T) {
	payload := `<Invoice xmlns="` + invoiceNamespace + `"/>`
	document := `<StandardBusinessDocument xmlns="` + SBDHNamespace + `"><StandardBusinessDocumentHeader/>` + payload + `</StandardBusinessDocument>`

	got, err := ExtractEnvelopePayload([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("payload = %q, want %q", got, payload)
	}
}
