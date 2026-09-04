package peppol

import (
	"fmt"
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
)

// NegativeFixture is one Invoice containing a single controlled defect.
// FailureStage identifies the first validation layer that must reject it;
// Peppol-targeted fixtures remain UBL- and EN-16931-valid, while the two
// controls prove that earlier failures stop before later validation layers.
type NegativeFixture struct {
	ID                 string
	Family             string
	Mutation           string
	FailureStage       string
	ExpectedRuleIDs    []string
	ExpectedDiagnostic string
	XML                string
}

// NegativeInvoiceFixtures builds one deterministic mutation for each Peppol
// rule family required by the regression catalogue. Mutations start from the
// same EN-16931-valid Profile 01 Invoice used by the positive corpus.
func NegativeInvoiceFixtures() ([]NegativeFixture, error) {
	type mutation struct {
		id, family, description string
		rules                   []string
		mutateDocument          func(*en16931.InvoiceDocument)
		mutateXML               func(string) (string, error)
	}
	mutations := []mutation{
		{
			id: "control-xsd-missing-invoice-id", family: "stage-control",
			description: "remove the mandatory UBL Invoice identifier so validation stops at XSD",
			mutateXML: func(content string) (string, error) {
				return replaceExactlyOnce(content, "    <cbc:ID>PEPPOL-INV-P01</cbc:ID>\n", "")
			},
		},
		{
			id: "control-en-missing-customization-id", family: "stage-control",
			description: "remove the specification identifier to prove the missing case stops at EN 16931",
			rules:       []string{"BR-01"},
			mutateXML: func(content string) (string, error) {
				return replaceExactlyOnce(content, "    <cbc:CustomizationID>urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0</cbc:CustomizationID>\n", "")
			},
		},
		{
			id: "invalid-customization-id", family: "identifier",
			description: "replace the Peppol specification identifier with the EN 16931 base identifier",
			rules:       []string{"PEPPOL-EN16931-R004"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.CustomizationID = "urn:cen.eu:en16931:2017"
			},
		},
		{
			id: "missing-profile-id", family: "profile",
			description: "remove the Peppol business-process identifier",
			rules:       []string{"PEPPOL-EN16931-R001", "PEPPOL-EN16931-R007"},
			mutateXML: func(content string) (string, error) {
				profileID, err := ProfileID(Profile01)
				if err != nil {
					return "", err
				}
				return replaceExactlyOnce(content, "    <cbc:ProfileID>"+profileID+"</cbc:ProfileID>\n", "")
			},
		},
		{
			id: "unsupported-profile-id", family: "profile",
			description: "replace the approved Peppol process identifier with an unknown process",
			rules:       []string{"PEPPOL-EN16931-R007"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.ProfileID = "urn:fdc:peppol.eu:2017:poacc:billing:99:1.0"
			},
		},
		{
			id: "missing-seller-endpoint", family: "endpoint",
			description: "remove the seller electronic address while retaining an otherwise valid Peppol invoice",
			rules:       []string{"PEPPOL-EN16931-R020"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.Seller.EndpointID = ""
				document.Seller.EndpointSchemeID = ""
			},
		},
		{
			id: "invalid-endpoint-scheme", family: "endpoint",
			description: "replace the seller electronic-address scheme with 0037, allowed by EN 16931 but excluded from the Peppol EAS subset",
			rules:       []string{"PEPPOL-EN16931-CL008"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.Seller.EndpointSchemeID = "0037"
			},
		},
		{
			id: "missing-danish-party-scheme", family: "party-scheme",
			description: "remove the buyer party identifier scheme required for a domestic Danish invoice",
			rules:       []string{"DK-R-013"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				makeDomesticDanish(document)
				document.Buyer.PartyIdentificationSchemeID = ""
			},
		},
		{
			id: "restricted-invoice-type-code", family: "code-list",
			description: "use invoice type 384, which EN 16931 permits but Peppol restricts to domestic German trading partners",
			rules:       []string{"PEPPOL-EN16931-P0112"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.InvoiceTypeCode = "384"
			},
		},
		{
			id: "danish-payment-means-restriction", family: "payment",
			description: "use EN-valid payment means code 20, which is excluded for domestic Danish invoices",
			rules:       []string{"DK-R-005"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				makeDomesticDanish(document)
				document.PaymentMeans[0].Code = "20"
			},
		},
		{
			id: "multiple-document-notes", family: "cardinality",
			description: "add two document notes where Peppol permits at most one for non-German parties",
			rules:       []string{"PEPPOL-EN16931-R002"},
			mutateXML: func(content string) (string, error) {
				anchor := "    <cbc:InvoiceTypeCode>380</cbc:InvoiceTypeCode>\n"
				addition := anchor + "    <cbc:Note>First note</cbc:Note>\n    <cbc:Note>Second note</cbc:Note>\n"
				return replaceExactlyOnce(content, anchor, addition)
			},
		},
		{
			id: "inconsistent-line-currency", family: "currency",
			description: "use a different currency identifier on one line amount while retaining the document currency",
			rules:       []string{"PEPPOL-EN16931-R051"},
			mutateXML: func(content string) (string, error) {
				return replaceExactlyOnce(content,
					"        <cbc:LineExtensionAmount currencyID=\"EUR\">100.00</cbc:LineExtensionAmount>",
					"        <cbc:LineExtensionAmount currencyID=\"USD\">100.00</cbc:LineExtensionAmount>")
			},
		},
		{
			id: "line-arithmetic-outside-slack", family: "arithmetic",
			description: "increase one line net amount by 0.03 and reconcile every EN 16931 total while leaving quantity and price unchanged",
			rules:       []string{"PEPPOL-EN16931-R120"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.Lines[0].NetAmount += 3
				document.TaxBreakdown = en16931.ComputeTaxBreakdown(document.Lines)
				document.Totals = en16931.ComputeTotals(document.Lines, document.TaxBreakdown)
			},
		},
		{
			id: "control-en-unsupported-attachment-mime", family: "stage-control",
			description: "attach an embedded application/zip document to prove the shared MIME restriction stops at EN 16931",
			rules:       []string{"BR-CL-24"},
			mutateXML: func(content string) (string, error) {
				anchor := "    <cac:AccountingSupplierParty>\n"
				attachment := "    <cac:AdditionalDocumentReference>\n" +
					"        <cbc:ID>ATTACHMENT-1</cbc:ID>\n" +
					"        <cac:Attachment>\n" +
					"            <cbc:EmbeddedDocumentBinaryObject mimeCode=\"application/zip\" filename=\"evidence.zip\">UEs=</cbc:EmbeddedDocumentBinaryObject>\n" +
					"        </cac:Attachment>\n" +
					"    </cac:AdditionalDocumentReference>\n"
				return replaceExactlyOnce(content, anchor, attachment+anchor)
			},
		},
		{
			id: "invalid-gln-check-digit", family: "common",
			description: "change the seller GLN check digit while retaining the GLN identifier scheme",
			rules:       []string{"PEPPOL-COMMON-R040"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.Seller.EndpointID = "7300010000002"
			},
		},
		{
			id: "danish-supplier-legal-scheme", family: "country",
			description: "make the supplier Danish without the mandatory 0184 legal-registration scheme",
			rules:       []string{"DK-R-014"},
			mutateDocument: func(document *en16931.InvoiceDocument) {
				document.Seller.CountryCode = "DK"
			},
		},
	}

	fixtures := make([]NegativeFixture, 0, len(mutations))
	for _, catalogueEntry := range mutations {
		document, err := NewInvoice(Profile01)
		if err != nil {
			return nil, err
		}
		if catalogueEntry.mutateDocument != nil {
			catalogueEntry.mutateDocument(&document)
		}
		xml, err := en16931.BuildInvoiceXML(document)
		if err != nil {
			return nil, fmt.Errorf("build Peppol negative fixture %s: %w", catalogueEntry.id, err)
		}
		if catalogueEntry.mutateXML != nil {
			xml, err = catalogueEntry.mutateXML(xml)
			if err != nil {
				return nil, fmt.Errorf("mutate Peppol negative fixture %s: %w", catalogueEntry.id, err)
			}
		}
		fixtures = append(fixtures, NegativeFixture{
			ID: catalogueEntry.id, Family: catalogueEntry.family,
			Mutation: catalogueEntry.description, FailureStage: "peppol-bis-billing",
			ExpectedRuleIDs: append([]string(nil), catalogueEntry.rules...), XML: xml,
		})
	}
	for index := range fixtures {
		switch fixtures[index].ID {
		case "control-xsd-missing-invoice-id":
			fixtures[index].FailureStage = "ubl21-xsd"
			fixtures[index].ExpectedDiagnostic = "CommonBasicComponents-2}ID"
		case "control-en-missing-customization-id", "control-en-unsupported-attachment-mime":
			fixtures[index].FailureStage = "en16931"
		}
	}
	return fixtures, nil
}

func makeDomesticDanish(document *en16931.InvoiceDocument) {
	document.Seller.CountryCode = "DK"
	document.Seller.LegalRegistrationID = "12345678"
	document.Seller.LegalRegistrationSchemeID = "0184"
	document.Buyer.CountryCode = "DK"
	document.Buyer.LegalRegistrationID = "87654321"
	document.Buyer.LegalRegistrationSchemeID = "0184"
	document.PaymentMeans[0].Code = "1"
	document.PaymentMeans[0].PayeeAccountID = ""
}

func replaceExactlyOnce(content, old, replacement string) (string, error) {
	if count := strings.Count(content, old); count != 1 {
		return "", fmt.Errorf("mutation anchor occurs %d times, want exactly once", count)
	}
	return strings.Replace(content, old, replacement, 1), nil
}
