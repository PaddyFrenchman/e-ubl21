package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestUploadAcceptsEnvelopeOnlyForExplicitEnvelopeProfile(t *testing.T) {
	handler, client := directUploadHandler(t)
	payload, err := os.ReadFile("../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := envelopePayloadIdentity(payload)
	if err != nil {
		t.Fatal(err)
	}
	document := testEnvelope(string(payload), identity, "")
	request := httptest.NewRequest(http.MethodPost, "/upload?validationProfile=peppol-envelope&documentType=Invoice", strings.NewReader(document))
	request.Header.Set("Content-Type", "application/xml")
	response := httptest.NewRecorder()
	handler.handleUpload(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var accepted struct {
		ID               string                  `json:"id"`
		RequestedProfile queue.ValidationProfile `json:"requestedProfile"`
		ResolvedProfile  queue.ValidationProfile `json:"resolvedProfile"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	if accepted.RequestedProfile != queue.ValidationProfilePeppolEnvelope || accepted.ResolvedProfile != queue.ValidationProfilePeppolEnvelope {
		t.Fatalf("profiles=%+v", accepted)
	}
	stored, err := client.GetUploadContent(context.Background(), accepted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != document {
		t.Fatal("ingress did not preserve the complete envelope")
	}

	request = httptest.NewRequest(http.MethodPost, "/upload?documentType=Invoice", strings.NewReader(document))
	request.Header.Set("Content-Type", "application/xml")
	response = httptest.NewRecorder()
	handler.handleUpload(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("implicit envelope status=%d", response.Code)
	}
}

func TestPeppolEnvelopeProfileFreezesFourthIndependentStage(t *testing.T) {
	payload, err := os.ReadFile("../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml")
	if err != nil {
		t.Fatal(err)
	}
	resolution, plan, err := ResolveFrozenValidationPlanFor("Invoice", queue.ValidationProfilePeppolEnvelope, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := []queue.ValidationLayer{queue.ValidationLayerUBL21XSD, queue.ValidationLayerEN16931, queue.ValidationLayerPeppolBISBilling, queue.ValidationLayerPeppolEnvelope}
	if resolution.ResolvedProfile != queue.ValidationProfilePeppolEnvelope || len(plan.Stages) != len(want) {
		t.Fatalf("resolution=%+v plan=%+v", resolution, plan)
	}
	for index, layer := range want {
		if plan.Stages[index].ID != layer {
			t.Fatalf("stage %d=%s want %s", index, plan.Stages[index].ID, layer)
		}
	}
}

func TestPeppolEnvelopeValidatorSeparatesEnvelopeEvidence(t *testing.T) {
	payload, err := os.ReadFile("../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := envelopePayloadIdentity(payload)
	if err != nil {
		t.Fatal(err)
	}
	valid := testEnvelope(string(payload), identity, "")
	validator := newPeppolEnvelopeValidator()
	result, err := validator.Validate(context.Background(), strings.NewReader(valid))
	if err != nil || !result.Valid || result.Layer != queue.ValidationLayerPeppolEnvelope {
		t.Fatalf("valid envelope result=%+v err=%v", result, err)
	}

	invalid := testEnvelope(string(payload), identity, "bad-sender")
	result, err = validator.Validate(context.Background(), strings.NewReader(invalid))
	if err != nil || result.Valid {
		t.Fatalf("invalid envelope result=%+v err=%v", result, err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Rule != "BR-SBDH-2" || result.Issues[0].Layer != queue.ValidationLayerPeppolEnvelope {
		t.Fatalf("wrong isolated envelope evidence: %+v", result.Issues)
	}
}

func TestPeppolEnvelopeValidatorAcceptsInvoiceAndCreditNote(t *testing.T) {
	tests := []struct{ name, sample string }{
		{"Invoice", "../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml"},
		{"CreditNote", "../createsamples/samples-peppol/CreditNote/Peppol-CreditNote-Profile-02.xml"},
	}
	validator := newPeppolEnvelopeValidator()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload, err := os.ReadFile(test.sample)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := envelopePayloadIdentity(payload)
			if err != nil {
				t.Fatal(err)
			}
			result, err := validator.Validate(context.Background(), strings.NewReader(testEnvelope(string(payload), identity, "")))
			if err != nil || !result.Valid {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}

func TestPeppolEnvelopeValidatorChecksEveryEnvelopePayloadBinding(t *testing.T) {
	payload, err := os.ReadFile("../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := envelopePayloadIdentity(payload)
	if err != nil {
		t.Fatal(err)
	}
	valid := testEnvelope(string(payload), identity, "")
	tests := []struct{ name, old, replacement, rule string }{
		{"receiver", identity.CustomerEndpoint, "0088:wrong", "BR-SBDH-1"},
		{"standard", "<Standard>urn:oasis:names:specification:ubl:schema:xsd:Invoice-2</Standard>", "<Standard>urn:wrong</Standard>", "EUBL21-PEPPOL-ENVELOPE-DOCUMENT-STANDARD"},
		{"version", "<TypeVersion>2.1</TypeVersion>", "<TypeVersion>2.0</TypeVersion>", "EUBL21-PEPPOL-ENVELOPE-DOCUMENT-VERSION"},
		{"type", "<Type>Invoice</Type>", "<Type>CreditNote</Type>", "EUBL21-PEPPOL-ENVELOPE-DOCUMENT-TYPE"},
		{"document-id", "::Invoice##", "::CreditNote##", "EUBL21-PEPPOL-ENVELOPE-DOCUMENT-ID"},
		{"process-id", "<InstanceIdentifier>" + identity.ProfileID + "</InstanceIdentifier>", "<InstanceIdentifier>urn:wrong</InstanceIdentifier>", "EUBL21-PEPPOL-ENVELOPE-PROCESS-ID"},
	}
	validator := newPeppolEnvelopeValidator()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := strings.Replace(valid, test.old, test.replacement, 1)
			result, err := validator.Validate(context.Background(), strings.NewReader(document))
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid || len(result.Issues) != 1 || result.Issues[0].Rule != test.rule {
				t.Fatalf("issues=%+v", result.Issues)
			}
		})
	}
}

func TestPeppolEnvelopeConformityCorpus(t *testing.T) {
	root := "../validation-assets/peppol-envelope/3.0.21/examples/"
	tests := []struct {
		file, rule string
		valid      bool
	}{
		{"positive/invoice.xml", "", true},
		{"negative/BR-SBDH-2-sender.xml", "BR-SBDH-2", false},
		{"negative/EUBL21-PROCESS-ID.xml", "EUBL21-PEPPOL-ENVELOPE-PROCESS-ID", false},
	}
	validator := newPeppolEnvelopeValidator()
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			content, err := os.ReadFile(root + test.file)
			if err != nil {
				t.Fatal(err)
			}
			result, err := validator.Validate(context.Background(), strings.NewReader(string(content)))
			if err != nil {
				t.Fatal(err)
			}
			if result.Valid != test.valid {
				t.Fatalf("valid=%v issues=%+v", result.Valid, result.Issues)
			}
			if !test.valid && (len(result.Issues) != 1 || result.Issues[0].Rule != test.rule) {
				t.Fatalf("issues=%+v", result.Issues)
			}
		})
	}
}

func testEnvelope(payload string, identity payloadEnvelopeIdentity, senderOverride string) string {
	documentType := "Invoice"
	namespace := "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	if strings.Contains(payload, "<CreditNote") {
		documentType = "CreditNote"
		namespace = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	}
	sender := identity.SupplierEndpoint
	if senderOverride != "" {
		sender = senderOverride
	}
	documentID := namespace + "::" + documentType + "##" + identity.CustomizationID + "::2.1"
	return fmt.Sprintf(`<StandardBusinessDocument xmlns="%s"><StandardBusinessDocumentHeader><HeaderVersion>1.0</HeaderVersion><Sender><Identifier>%s</Identifier></Sender><Receiver><Identifier>%s</Identifier></Receiver><DocumentIdentification><Standard>%s</Standard><TypeVersion>2.1</TypeVersion><Type>%s</Type></DocumentIdentification><BusinessScope><Scope><Type>DOCUMENTID</Type><InstanceIdentifier>%s</InstanceIdentifier></Scope><Scope><Type>PROCESSID</Type><InstanceIdentifier>%s</InstanceIdentifier></Scope></BusinessScope></StandardBusinessDocumentHeader>%s</StandardBusinessDocument>`, peppol.SBDHNamespace, sender, identity.CustomerEndpoint, namespace, documentType, documentID, identity.ProfileID, payload)
}
