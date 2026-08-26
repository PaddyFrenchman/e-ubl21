package en16931

import (
	"fmt"
	"strings"
)

// NegativeFixture is one UBL-XSD-valid document containing a controlled EN
// 16931 defect. ExpectedRuleIDs is part of the fixture contract: consumers
// must assert the named rules, rather than merely accepting a generic invalid
// outcome.
type NegativeFixture struct {
	ID              string
	DocumentType    string
	Mutation        string
	ExpectedRuleIDs []string
	XML             string
}

// NegativeInvoiceFixtures builds the targeted semantic-negative corpus from
// known-compliant Invoice variants. Each fixture changes one business field or
// adds one prohibited UBL construct; the generated documents intentionally
// remain valid against the UBL 2.1 Invoice XSD.
func NegativeInvoiceFixtures() ([]NegativeFixture, error) {
	type modelMutation struct {
		id, description string
		rules           []string
		base            func() InvoiceDocument
		mutate          func(*InvoiceDocument)
	}
	modelMutations := []modelMutation{
		{
			id: "monetary-total-reconciliation", description: "increase TaxInclusiveAmount without changing its inputs",
			rules: []string{"BR-CO-15"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) { doc.Totals.TaxInclusiveAmount++ },
		},
		{
			id: "invalid-invoice-type-code", description: "replace InvoiceTypeCode with a value outside UNTDID 1001",
			rules: []string{"BR-CL-01"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) { doc.InvoiceTypeCode = "999" },
		},
		{
			id: "missing-seller-name", description: "empty the seller registration name",
			rules: []string{"BR-06"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) { doc.Seller.Name = "" },
		},
		{
			id: "missing-buyer-name", description: "empty the buyer registration name",
			rules: []string{"BR-07"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) { doc.Buyer.Name = "" },
		},
		{
			id: "missing-seller-identifier", description: "remove the out-of-scope seller's only identifying registration",
			rules: []string{"BR-CO-26"}, base: NewInvoiceVariantO,
			mutate: func(doc *InvoiceDocument) { doc.Seller.LegalRegistrationID = "" },
		},
		{
			id: "missing-buyer-identifier", description: "remove the reverse-charge buyer's required VAT identifier",
			rules: []string{"BR-AE-02"}, base: NewInvoiceVariantAE,
			mutate: func(doc *InvoiceDocument) { doc.Buyer.VATSchemeID = "" },
		},
		{
			id: "payment-account-required", description: "remove the account identifier from credit transfer payment means",
			rules: []string{"BR-61"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) { doc.PaymentMeans[0].PayeeAccountID = "" },
		},
		{
			id: "vat-s-rate", description: "set the standard VAT rate to zero and recompute dependent tax totals",
			rules: []string{"BR-S-05"}, base: NewInvoiceVariantS,
			mutate: func(doc *InvoiceDocument) {
				for index := range doc.Lines {
					doc.Lines[index].VATRate = 0
				}
				doc.TaxBreakdown[0].VATRate = 0
				doc.TaxBreakdown[0].TaxAmount = 0
				doc.Totals.TaxAmount = 0
				doc.Totals.TaxInclusiveAmount = doc.Totals.TaxExclusiveAmount
				doc.Totals.PayableAmount = doc.Totals.TaxExclusiveAmount
			},
		},
		{
			id: "vat-z-rate", description: "set a zero-rated line VAT rate above zero",
			rules: []string{"BR-Z-05"}, base: NewInvoiceVariantZ,
			mutate: func(doc *InvoiceDocument) { doc.Lines[0].VATRate = 1 },
		},
		{
			id: "vat-e-rate", description: "set an exempt line VAT rate above zero",
			rules: []string{"BR-E-05"}, base: NewInvoiceVariantE,
			mutate: func(doc *InvoiceDocument) { doc.Lines[0].VATRate = 1 },
		},
		{
			id: "vat-ae-rate", description: "set a reverse-charge line VAT rate above zero",
			rules: []string{"BR-AE-05"}, base: NewInvoiceVariantAE,
			mutate: func(doc *InvoiceDocument) { doc.Lines[0].VATRate = 1 },
		},
		{
			id: "vat-g-rate", description: "set an export line VAT rate above zero",
			rules: []string{"BR-G-05"}, base: NewInvoiceVariantG,
			mutate: func(doc *InvoiceDocument) { doc.Lines[0].VATRate = 1 },
		},
		{
			id: "vat-k-rate", description: "set an intra-community line VAT rate above zero",
			rules: []string{"BR-IC-05"}, base: NewInvoiceVariantK,
			mutate: func(doc *InvoiceDocument) { doc.Lines[0].VATRate = 1 },
		},
		{
			id: "exemption-e-required", description: "remove the exempt VAT reason",
			rules: []string{"BR-E-10"}, base: NewInvoiceVariantE,
			mutate: func(doc *InvoiceDocument) { doc.TaxBreakdown[0].ExemptionReasonCode = "" },
		},
		{
			id: "exemption-ae-required", description: "remove the reverse-charge VAT reason",
			rules: []string{"BR-AE-10"}, base: NewInvoiceVariantAE,
			mutate: func(doc *InvoiceDocument) { doc.TaxBreakdown[0].ExemptionReasonCode = "" },
		},
		{
			id: "exemption-g-required", description: "remove the export VAT reason",
			rules: []string{"BR-G-10"}, base: NewInvoiceVariantG,
			mutate: func(doc *InvoiceDocument) { doc.TaxBreakdown[0].ExemptionReasonCode = "" },
		},
		{
			id: "exemption-k-required", description: "remove the intra-community VAT reason",
			rules: []string{"BR-IC-10"}, base: NewInvoiceVariantK,
			mutate: func(doc *InvoiceDocument) { doc.TaxBreakdown[0].ExemptionReasonCode = "" },
		},
		{
			id: "exemption-o-required", description: "remove the out-of-scope VAT reason",
			rules: []string{"BR-O-10"}, base: NewInvoiceVariantO,
			mutate: func(doc *InvoiceDocument) { doc.TaxBreakdown[0].ExemptionReasonCode = "" },
		},
	}

	fixtures := make([]NegativeFixture, 0, len(modelMutations)+4)
	for _, mutation := range modelMutations {
		doc := mutation.base()
		mutation.mutate(&doc)
		xml, err := BuildInvoiceXML(doc)
		if err != nil {
			return nil, fmt.Errorf("build negative fixture %s: %w", mutation.id, err)
		}
		fixtures = append(fixtures, NegativeFixture{
			ID: mutation.id, DocumentType: "Invoice", Mutation: mutation.description,
			ExpectedRuleIDs: mutation.rules, XML: xml,
		})
	}

	baseline, err := BuildInvoiceXML(NewInvoiceVariantS())
	if err != nil {
		return nil, err
	}
	decimalXML, err := replaceExactlyOnce(baseline,
		`<cbc:TaxExclusiveAmount currencyID="EUR">130.00</cbc:TaxExclusiveAmount>`,
		`<cbc:TaxExclusiveAmount currencyID="EUR">130.001</cbc:TaxExclusiveAmount>`)
	if err != nil {
		return nil, fmt.Errorf("build decimal precision fixture: %w", err)
	}
	fixtures = append(fixtures, NegativeFixture{
		ID: "decimal-tax-exclusive", DocumentType: "Invoice",
		Mutation: "add a third decimal place to TaxExclusiveAmount", ExpectedRuleIDs: []string{"BR-DEC-12"}, XML: decimalXML,
	})

	outOfScope, err := BuildInvoiceXML(NewInvoiceVariantO())
	if err != nil {
		return nil, err
	}
	outOfScope, err = replaceLast(outOfScope, `<cbc:ID>O</cbc:ID>`, `<cbc:ID>O</cbc:ID>`+"\n        "+`<cbc:Percent>1.00</cbc:Percent>`)
	if err != nil {
		return nil, fmt.Errorf("build VAT O rate fixture: %w", err)
	}
	fixtures = append(fixtures, NegativeFixture{
		ID: "vat-o-rate", DocumentType: "Invoice", Mutation: "add a forbidden VAT rate to an out-of-scope line",
		ExpectedRuleIDs: []string{"BR-O-05"}, XML: outOfScope,
	})

	partyTaxScheme, err := firstElement(baseline, "cac:PartyTaxScheme")
	if err != nil {
		return nil, fmt.Errorf("build syntax restriction fixture: %w", err)
	}
	tooManyTaxSchemes, err := replaceExactlyOnce(baseline, partyTaxScheme, partyTaxScheme+"\n"+partyTaxScheme+"\n"+partyTaxScheme)
	if err != nil {
		return nil, fmt.Errorf("build syntax restriction fixture: %w", err)
	}
	fixtures = append(fixtures, NegativeFixture{
		ID: "syntax-party-tax-scheme-count", DocumentType: "Invoice",
		Mutation: "repeat the seller PartyTaxScheme beyond the EN syntax limit", ExpectedRuleIDs: []string{"UBL-SR-42"}, XML: tooManyTaxSchemes,
	})

	prohibitedReference := `  <cac:AdditionalDocumentReference>
    <cbc:ID>invoice-object</cbc:ID>
    <cbc:DocumentTypeCode>130</cbc:DocumentTypeCode>
    <cbc:DocumentDescription>must not accompany an invoice object identifier</cbc:DocumentDescription>
  </cac:AdditionalDocumentReference>
`
	prohibitedConstruct, err := replaceExactlyOnce(baseline, "  <cac:AccountingSupplierParty>", prohibitedReference+"  <cac:AccountingSupplierParty>")
	if err != nil {
		return nil, fmt.Errorf("build prohibited construct fixture: %w", err)
	}
	fixtures = append(fixtures, NegativeFixture{
		ID: "prohibited-invoice-object-description", DocumentType: "Invoice",
		Mutation: "add a description to an invoice-object document reference", ExpectedRuleIDs: []string{"UBL-CR-673"}, XML: prohibitedConstruct,
	})

	return fixtures, nil
}

func replaceExactlyOnce(input, old, replacement string) (string, error) {
	if count := strings.Count(input, old); count != 1 {
		return "", fmt.Errorf("mutation target occurs %d times, want exactly once", count)
	}
	return strings.Replace(input, old, replacement, 1), nil
}

func replaceLast(input, old, replacement string) (string, error) {
	index := strings.LastIndex(input, old)
	if index < 0 {
		return "", fmt.Errorf("mutation target is absent")
	}
	return input[:index] + replacement + input[index+len(old):], nil
}

func firstElement(input, qualifiedName string) (string, error) {
	startToken := "<" + qualifiedName + ">"
	endToken := "</" + qualifiedName + ">"
	start := strings.Index(input, startToken)
	if start < 0 {
		return "", fmt.Errorf("start element %s is absent", qualifiedName)
	}
	endRelative := strings.Index(input[start:], endToken)
	if endRelative < 0 {
		return "", fmt.Errorf("end element %s is absent", qualifiedName)
	}
	end := start + endRelative + len(endToken)
	return input[start:end], nil
}
