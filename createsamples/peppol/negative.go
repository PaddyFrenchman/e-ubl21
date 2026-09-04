package peppol

import (
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
)

// NegativeFixture is one UBL- and EN-16931-valid Invoice containing a single
// controlled Peppol defect. ExpectedRuleIDs names the assertions from the
// pinned Peppol release that the regression runner must observe.
type NegativeFixture struct {
	ID              string
	Family          string
	Mutation        string
	ExpectedRuleIDs []string
	XML             string
}

// NegativeInvoiceFixtures builds one deterministic mutation for each Peppol
// rule family required by the regression catalogue. Mutations start from the
// same EN-16931-valid Profile 01 Invoice used by the positive corpus.
func NegativeInvoiceFixtures() ([]NegativeFixture, error) {
	type mutation struct {
		id, family, description string
		rules                   []string
		mutate                  func(*en16931.InvoiceDocument)
	}
	mutations := []mutation{
		{
			id: "invalid-customization-id", family: "identifier",
			description: "replace the Peppol specification identifier with the EN 16931 base identifier",
			rules:       []string{"PEPPOL-EN16931-R004"},
			mutate: func(document *en16931.InvoiceDocument) {
				document.CustomizationID = "urn:cen.eu:en16931:2017"
			},
		},
		{
			id: "unsupported-profile-id", family: "profile",
			description: "replace the approved Peppol process identifier with an unknown process",
			rules:       []string{"PEPPOL-EN16931-R007"},
			mutate: func(document *en16931.InvoiceDocument) {
				document.ProfileID = "urn:fdc:peppol.eu:2017:poacc:billing:99:1.0"
			},
		},
		{
			id: "invalid-endpoint-scheme", family: "code-list",
			description: "replace the seller electronic-address scheme with 0037, allowed by EN 16931 but excluded from the Peppol EAS subset",
			rules:       []string{"PEPPOL-EN16931-CL008"},
			mutate: func(document *en16931.InvoiceDocument) {
				document.Seller.EndpointSchemeID = "0037"
			},
		},
		{
			id: "invalid-gln-check-digit", family: "common",
			description: "change the seller GLN check digit while retaining the GLN identifier scheme",
			rules:       []string{"PEPPOL-COMMON-R040"},
			mutate: func(document *en16931.InvoiceDocument) {
				document.Seller.EndpointID = "7300010000002"
			},
		},
		{
			id: "danish-supplier-legal-scheme", family: "country",
			description: "make the supplier Danish without the mandatory 0184 legal-registration scheme",
			rules:       []string{"DK-R-014"},
			mutate: func(document *en16931.InvoiceDocument) {
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
		catalogueEntry.mutate(&document)
		xml, err := en16931.BuildInvoiceXML(document)
		if err != nil {
			return nil, fmt.Errorf("build Peppol negative fixture %s: %w", catalogueEntry.id, err)
		}
		fixtures = append(fixtures, NegativeFixture{
			ID: catalogueEntry.id, Family: catalogueEntry.family,
			Mutation:        catalogueEntry.description,
			ExpectedRuleIDs: append([]string(nil), catalogueEntry.rules...), XML: xml,
		})
	}
	return fixtures, nil
}
