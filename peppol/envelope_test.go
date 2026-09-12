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

func TestParseEnvelopeHeaderIsNamespaceAwareAndPreservesIdentifierText(t *testing.T) {
	payload := `<Invoice xmlns="` + invoiceNamespace + `"/>`
	document := `<StandardBusinessDocument xmlns="` + SBDHNamespace + `" xmlns:foreign="urn:foreign"><StandardBusinessDocumentHeader>` +
		`<Sender><Identifier> 0088:sender </Identifier></Sender>` +
		`<foreign:Sender><foreign:Identifier>0088:foreign</foreign:Identifier></foreign:Sender>` +
		`<Receiver><foreign:Identifier>0088:receiver</foreign:Identifier></Receiver>` +
		`<DocumentIdentification><foreign:Standard>` + invoiceNamespace + `</foreign:Standard><TypeVersion>2.1</TypeVersion><Type>Invoice</Type></DocumentIdentification>` +
		`<BusinessScope><Scope><foreign:Type>DOCUMENTID</foreign:Type><foreign:InstanceIdentifier>foreign</foreign:InstanceIdentifier></Scope></BusinessScope>` +
		`</StandardBusinessDocumentHeader>` + payload + `</StandardBusinessDocument>`

	envelope, err := ParseEnvelope([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Header.Sender != " 0088:sender " {
		t.Fatalf("sender = %q", envelope.Header.Sender)
	}
	if !envelope.Header.HasSingleSenderIdentifier() {
		t.Fatal("foreign Sender affected SBDH sender cardinality")
	}
	if envelope.Header.HasSingleReceiverIdentifier() || envelope.Header.Receiver != "" {
		t.Fatalf("foreign receiver Identifier populated header: %+v", envelope.Header)
	}
	if envelope.Header.Standard != "" || envelope.Header.DocumentID != "" {
		t.Fatalf("foreign binding elements populated header: %+v", envelope.Header)
	}
}
