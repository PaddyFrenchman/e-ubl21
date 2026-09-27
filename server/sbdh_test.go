package server

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

const validSBDHEnvelope = `<StandardBusinessDocument xmlns="http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader"><StandardBusinessDocumentHeader><Sender><Identifier>0088:sender</Identifier></Sender></StandardBusinessDocumentHeader><Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"><ID>one</ID></Invoice></StandardBusinessDocument>`

func TestPeppolEnvelopePayloadProjectsEmbeddedUBL(t *testing.T) {
	payload, _, _, err := peppolEnvelopePayload([]byte(validSBDHEnvelope))
	if err != nil {
		t.Fatal(err)
	}
	local, namespace, err := rootElementOfPayload(payload)
	if err != nil {
		t.Fatalf("projected payload is not XML: %v; %s", err, payload)
	}
	if local != "Invoice" || namespace != "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" {
		t.Fatalf("projected root = {%s}%s", namespace, local)
	}
}

func TestStageInputProjectsOnlyPayloadStages(t *testing.T) {
	payload, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerEN16931, []byte(validSBDHEnvelope))
	if err != nil {
		t.Fatal(err)
	}
	local, _, err := rootElementOfPayload(payload)
	if err != nil || local != "Invoice" {
		t.Fatalf("payload stage input = %q, %v", payload, err)
	}
	envelope, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerPeppolEnvelope, []byte(validSBDHEnvelope))
	if err != nil || !bytes.Equal(envelope, []byte(validSBDHEnvelope)) {
		t.Fatalf("envelope stage input was changed: %q, %v", envelope, err)
	}
}

func TestPeppolEnvelopeFixturesProjectUBLAndRetainSBDH(t *testing.T) {
	for _, name := range []string{"valid-billing.xml", "valid-creditnote.xml", "wrong-sender.xml", "wrong-receiver.xml", "valid-self-billing.xml"} {
		t.Run(name, func(t *testing.T) {
			document, err := os.ReadFile(filepath.Join("testdata", "peppol-envelope", name))
			if err != nil {
				t.Fatal(err)
			}
			payload, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerPeppolBISBilling, document)
			if err != nil {
				t.Fatal(err)
			}
			local, namespace, err := rootElementOfPayload(payload)
			if err != nil || (local != "Invoice" && local != "CreditNote") || (namespace != "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" && namespace != "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2") {
				t.Fatalf("projected payload root = {%s}%s, err=%v", namespace, local, err)
			}
			envelope, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerPeppolEnvelope, document)
			if err != nil || !bytes.Equal(envelope, document) {
				t.Fatalf("envelope input changed: %v", err)
			}
		})
	}
}

func TestPeppolEnvelopeFixtureRejectsMissingHeader(t *testing.T) {
	document, err := os.ReadFile(filepath.Join("testdata", "peppol-envelope", "missing-header.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerUBL21XSD, document); err == nil {
		t.Fatal("missing SBDH header was accepted")
	}
}

func TestPeppolEnvelopeFixturesRejectDuplicateStructure(t *testing.T) {
	for _, name := range []string{"duplicate-header.xml", "duplicate-payload.xml"} {
		t.Run(name, func(t *testing.T) {
			document, err := os.ReadFile(filepath.Join("testdata", "peppol-envelope", name))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stageInput(string(queue.ValidationProfilePeppolEnvelope), queue.ValidationLayerUBL21XSD, document); err == nil {
				t.Fatal("duplicate SBDH structure was accepted")
			}
		})
	}
}

func TestPeppolEnvelopePayloadRejectsMissingPayload(t *testing.T) {
	_, _, _, err := peppolEnvelopePayload([]byte(`<StandardBusinessDocument xmlns="http://www.unece.org/cefact/namespaces/StandardBusinessDocumentHeader"><StandardBusinessDocumentHeader/></StandardBusinessDocument>`))
	if err == nil {
		t.Fatal("expected missing payload error")
	}
}
