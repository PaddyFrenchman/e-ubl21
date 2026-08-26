package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

const minimalValidInvoice = `<?xml version="1.0" encoding="UTF-8"?>
<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" xmlns:ext="urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2">
    <cbc:ID>INV-1</cbc:ID>
    <cbc:IssueDate>2026-01-01</cbc:IssueDate>
    <cac:AccountingSupplierParty/>
    <cac:AccountingCustomerParty/>
    <cac:LegalMonetaryTotal>
        <cbc:PayableAmount currencyID="USD">1.00</cbc:PayableAmount>
    </cac:LegalMonetaryTotal>
    <cac:InvoiceLine>
        <cbc:ID>1</cbc:ID>
        <cbc:LineExtensionAmount currencyID="USD">1.00</cbc:LineExtensionAmount>
        <cac:Item/>
    </cac:InvoiceLine>
</Invoice>
`

type ublNegativeMutation struct {
	id                 string
	description        string
	expectedDiagnostic string
	apply              func(string) (string, error)
}

func ublNegativeMutationCatalogue() []ublNegativeMutation {
	return []ublNegativeMutation{
		{
			id:                 "invalid-datatype",
			description:        "Sets the optional top-level cbc:CopyIndicator to a value outside the XML Schema boolean datatype.",
			expectedDiagnostic: "is not a valid value of the atomic type 'xs:boolean'",
			apply: func(document string) (string, error) {
				return replaceExactlyOnce(document,
					"    <cbc:ID>INV-1</cbc:ID>\n",
					"    <cbc:ID>INV-1</cbc:ID>\n    <cbc:CopyIndicator>not-a-boolean</cbc:CopyIndicator>\n")
			},
		},
		{
			id:                 "invalid-enumeration",
			description:        "Adds an XAdES Identifier whose Qualifier is outside the UBL-imported OIDAsURI/OIDAsURN enumeration.",
			expectedDiagnostic: "[facet 'enumeration']",
			apply: func(document string) (string, error) {
				extension := `    <ext:UBLExtensions>
        <ext:UBLExtension>
            <ext:ExtensionContent>
                <xades:ObjectIdentifier xmlns:xades="http://uri.etsi.org/01903/v1.3.2#">
                    <xades:Identifier Qualifier="NOT-LEGAL">urn:example:policy</xades:Identifier>
                </xades:ObjectIdentifier>
            </ext:ExtensionContent>
        </ext:UBLExtension>
    </ext:UBLExtensions>
`
				return replaceExactlyOnce(document, "    <cbc:ID>INV-1</cbc:ID>\n", extension+"    <cbc:ID>INV-1</cbc:ID>\n")
			},
		},
		{
			id:                 "invalid-namespace",
			description:        "Moves the required top-level Invoice ID into an unrecognized namespace instead of UBL Common Basic Components.",
			expectedDiagnostic: "This element is not expected",
			apply: func(document string) (string, error) {
				return replaceExactlyOnce(document,
					"    <cbc:ID>INV-1</cbc:ID>\n",
					"    <bad:ID xmlns:bad=\"urn:e-ubl21:invalid\">INV-1</bad:ID>\n")
			},
		},
		{
			id:                 "malformed-xml",
			description:        "Removes the Invoice closing tag so the fixture is not well-formed XML.",
			expectedDiagnostic: "Premature end of data in tag Invoice",
			apply: func(document string) (string, error) {
				return replaceExactlyOnce(document, "</Invoice>\n", "")
			},
		},
		{
			id:                 "missing-required-element",
			description:        "Removes the required top-level cbc:ID from the Invoice sequence.",
			expectedDiagnostic: "CommonBasicComponents-2}ID ).",
			apply: func(document string) (string, error) {
				return replaceExactlyOnce(document, "    <cbc:ID>INV-1</cbc:ID>\n", "")
			},
		},
		{
			id:                 "wrong-element-order",
			description:        "Places required cbc:IssueDate before required cbc:ID, violating the Invoice XSD sequence.",
			expectedDiagnostic: "This element is not expected",
			apply: func(document string) (string, error) {
				return replaceExactlyOnce(document,
					"    <cbc:ID>INV-1</cbc:ID>\n    <cbc:IssueDate>2026-01-01</cbc:IssueDate>\n",
					"    <cbc:IssueDate>2026-01-01</cbc:IssueDate>\n    <cbc:ID>INV-1</cbc:ID>\n")
			},
		},
	}
}

func generateUBLNegativeCorpus(xsdDir, generatorVersion string) (generatedCorpus, error) {
	mutations := ublNegativeMutationCatalogue()
	fixtures := make([]generatedFixture, 0, len(mutations))
	for _, mutation := range mutations {
		content, err := mutation.apply(minimalValidInvoice)
		if err != nil {
			return generatedCorpus{}, fmt.Errorf("apply UBL negative mutation %s: %w", mutation.id, err)
		}
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
		fixtures = append(fixtures, generatedFixture{
			entry: fixtureManifestEntry{
				ID:           "ubl21.invoice." + mutation.id,
				DocumentType: "Invoice",
				Profile:      "ubl21",
				File:         filepath.ToSlash(filepath.Join("negative", "Invoice", mutation.id+".xml")),
				SHA256:       digest,
				Standards:    fixtureStandards{UBL: "2.1"},
				Provenance: generatedProvenance{
					Kind: "generated", Generator: "createsamples", GeneratorVersion: generatorVersion,
				},
				Mutation:       &fixtureMutation{ID: mutation.id, Description: mutation.description},
				ExpectedFinal:  "invalid",
				ExpectedStages: []fixtureExpectedStage{{Stage: "ubl21-xsd", Outcome: "standards-invalid"}},
				Tags:           []string{"invoice", mutation.id, "negative", "ubl21"},
			},
			content:            []byte(content),
			schemaPath:         filepath.Join(xsdDir, "maindoc", "UBL-Invoice-2.1.xsd"),
			expectedDiagnostic: mutation.expectedDiagnostic,
		})
	}
	return buildGeneratedCorpus(fixtures)
}

func replaceExactlyOnce(document, old, replacement string) (string, error) {
	if occurrences := strings.Count(document, old); occurrences != 1 {
		return "", fmt.Errorf("mutation target occurs %d times, want exactly once", occurrences)
	}
	return strings.Replace(document, old, replacement, 1), nil
}
