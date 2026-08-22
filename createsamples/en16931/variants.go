// This file implements the VAT-category variant matrix described in plan
// §7: one small, targeted Invoice per VAT category rule family, each
// toggling only the fields that family's rules examine, so a failure is
// always attributable to one rule family during debugging.
//
// Every variant's extra required fields were verified directly against the
// official compiled EN16931-UBL-validation.xslt (release validation-1.3.16),
// not assumed from the plan's summary table alone - in particular:
//
//   - BR-Z-02 / BR-E-02 require the Seller VAT identifier for the Z and E
//     categories too, not only S (the plan §7 table only mentions this
//     explicitly for S).
//   - BR-S-10 / BR-Z-10 forbid a VAT exemption reason for S/Z, while
//     BR-E-10 / BR-AE-10 / BR-G-10 / BR-IC-10 / BR-O-10 require one for
//     E/AE/G/K/O.
//   - BR-O-05 requires the "Not subject to VAT" line's Invoiced item VAT
//     rate (cbc:Percent) to be entirely absent, not merely zero - see
//     build.go's buildLineNode.
//   - BR-IC-11 requires an Actual delivery date (BT-72) or Invoicing
//     period (BG-14) whenever a "K" (intra-community supply) VAT breakdown
//     entry is present; this v1 model only supports the former (see
//     InvoiceDocument.ActualDeliveryDate in model.go).
//
// The mutation-based negative corpus (plan §8, later steps) starts from
// the S variant produced here.
package en16931

// baseInvoiceSkeleton returns the document metadata, seller/buyer identity
// and payment means shared by every VAT-category variant below. Callers
// still need to set Lines/TaxBreakdown/Totals (via Finalize) and any
// VAT-category-specific fields (VAT identifiers, exemption reasons, actual
// delivery date).
func baseInvoiceSkeleton() InvoiceDocument {
	return InvoiceDocument{
		CustomizationID:      EN16931CustomizationID,
		ID:                   "INV-0001",
		IssueDate:            "2026-01-15",
		DueDate:              "2026-02-14",
		InvoiceTypeCode:      "380", // commercial invoice, see codelists.go InvoiceTypeCodes
		DocumentCurrencyCode: "EUR", // see codelists.go Currencies
		Seller: Party{
			Name:        "Acme Supplies B.V.",
			StreetName:  "Hoofdstraat 1",
			CityName:    "Amsterdam",
			PostalZone:  "1011AB",
			CountryCode: "NL", // see codelists.go Countries
		},
		Buyer: Party{
			Name:        "Beta Buyer Ltd",
			StreetName:  "High Street 2",
			CityName:    "London",
			PostalZone:  "EC1A 1AA",
			CountryCode: "GB", // see codelists.go Countries; non-EU, suits the G variant too
		},
		PaymentMeans: []PaymentMeans{
			{Code: "30", PayeeAccountID: "NL91ABNA0417164300"}, // credit transfer; BR-61 requires the account id for code 30/58
		},
	}
}

// withExemptionReason sets the VATEX reason code on every entry of
// breakdown (this generator's variants always use exactly one VAT
// category per document, so "every entry" means "the one entry").
func withExemptionReason(breakdown []TaxSubtotal, reasonCode string) []TaxSubtotal {
	for i := range breakdown {
		breakdown[i].ExemptionReasonCode = reasonCode
	}
	return breakdown
}

// NewInvoiceVariantS builds the "Standard rated" VAT-category variant
// (BR-S-*): Seller VAT identifier present, VAT rate > 0, no exemption
// reason. This is the variant proven end to end in Phase EN-2 (plan steps
// 11-12).
func NewInvoiceVariantS() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"
	doc.Seller.LegalRegistrationID = "12345678"

	lines := []Line{
		{ID: "1", Quantity: 2, UnitCode: "C62", ItemName: "Widget", NetPrice: NewMoneyFromMajorUnits(50.00), VATCategoryID: "S", VATRate: 21},
		{ID: "2", Quantity: 1, UnitCode: "C62", ItemName: "Gadget", NetPrice: NewMoneyFromMajorUnits(30.00), VATCategoryID: "S", VATRate: 21},
	}
	doc.Lines, doc.TaxBreakdown, doc.Totals = Finalize(lines)
	return doc
}

// NewInvoiceVariantZ builds the "Zero rated" VAT-category variant
// (BR-Z-*): Seller VAT identifier present (BR-Z-02), VAT rate = 0, no
// exemption reason (BR-Z-10).
func NewInvoiceVariantZ() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"

	lines := []Line{
		{ID: "1", Quantity: 1, UnitCode: "C62", ItemName: "Zero-rated book", NetPrice: NewMoneyFromMajorUnits(20.00), VATCategoryID: "Z", VATRate: 0},
	}
	doc.Lines, doc.TaxBreakdown, doc.Totals = Finalize(lines)
	return doc
}

// NewInvoiceVariantE builds the "Exempt from VAT" VAT-category variant
// (BR-E-*): Seller VAT identifier present (BR-E-02), VAT rate = 0, and a
// VAT exemption reason required (BR-E-10).
func NewInvoiceVariantE() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"

	lines := []Line{
		{ID: "1", Quantity: 1, UnitCode: "C62", ItemName: "Exempt medical service", NetPrice: NewMoneyFromMajorUnits(100.00), VATCategoryID: "E", VATRate: 0},
	}
	computedLines, breakdown, totals := Finalize(lines)
	doc.Lines = computedLines
	doc.TaxBreakdown = withExemptionReason(breakdown, "VATEX-EU-132")
	doc.Totals = totals
	return doc
}

// NewInvoiceVariantAE builds the "VAT Reverse Charge" VAT-category variant
// (BR-AE-*): Seller VAT identifier and Buyer VAT identifier both present
// (BR-AE-02), VAT rate = 0, and a VAT exemption reason required
// (BR-AE-10).
func NewInvoiceVariantAE() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"
	doc.Buyer.VATSchemeID = "GB123456789"

	lines := []Line{
		{ID: "1", Quantity: 1, UnitCode: "C62", ItemName: "Consulting services", NetPrice: NewMoneyFromMajorUnits(500.00), VATCategoryID: "AE", VATRate: 0},
	}
	computedLines, breakdown, totals := Finalize(lines)
	doc.Lines = computedLines
	doc.TaxBreakdown = withExemptionReason(breakdown, "VATEX-EU-AE")
	doc.Totals = totals
	return doc
}

// NewInvoiceVariantG builds the "Export outside the EU" VAT-category
// variant (BR-G-*): Seller VAT identifier present (BR-G-02; no Buyer VAT
// identifier required, unlike K), VAT rate = 0, and a VAT exemption reason
// required (BR-G-10). The base skeleton's Buyer is already outside the EU
// (GB), which is the natural scenario for this category.
func NewInvoiceVariantG() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"

	lines := []Line{
		{ID: "1", Quantity: 3, UnitCode: "C62", ItemName: "Export goods", NetPrice: NewMoneyFromMajorUnits(75.00), VATCategoryID: "G", VATRate: 0},
	}
	computedLines, breakdown, totals := Finalize(lines)
	doc.Lines = computedLines
	doc.TaxBreakdown = withExemptionReason(breakdown, "VATEX-EU-G")
	doc.Totals = totals
	return doc
}

// NewInvoiceVariantK builds the "Intra-community supply" VAT-category
// variant (BR-IC-*; the actual cac:TaxCategory/cbc:ID value is "K", not
// "IC" - see codelists.go's VATCategories comment): Seller and Buyer VAT
// identifiers both present and both EU (BR-IC-02), VAT rate = 0, a VAT
// exemption reason required (BR-IC-10), an Actual delivery date required
// (BR-IC-11), and a Deliver-to country code required (BR-IC-12). The base
// skeleton's Buyer is replaced with a German party so both parties are
// genuinely within the EU.
func NewInvoiceVariantK() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.VATSchemeID = "NL123456789B01"
	doc.Buyer = Party{
		Name:        "Gamma GmbH",
		StreetName:  "Unter den Linden 1",
		CityName:    "Berlin",
		PostalZone:  "10117",
		CountryCode: "DE",
		VATSchemeID: "DE123456789",
	}
	doc.ActualDeliveryDate = "2026-01-20"
	doc.DeliveryCountryCode = "DE"

	lines := []Line{
		{ID: "1", Quantity: 10, UnitCode: "C62", ItemName: "Intra-EU goods", NetPrice: NewMoneyFromMajorUnits(40.00), VATCategoryID: "K", VATRate: 0},
	}
	computedLines, breakdown, totals := Finalize(lines)
	doc.Lines = computedLines
	doc.TaxBreakdown = withExemptionReason(breakdown, "VATEX-EU-IC")
	doc.Totals = totals
	return doc
}

// NewInvoiceVariantO builds the "Not subject to VAT" VAT-category variant
// (BR-O-*): neither the Seller nor the Buyer may have a VAT identifier at
// all (BR-O-02 - the base skeleton already leaves both empty), the line's
// Invoiced item VAT rate must be entirely absent rather than zero
// (BR-O-05, handled in build.go's buildLineNode), and a VAT exemption
// reason is required (BR-O-10). Since the Seller cannot carry a VAT
// identifier here, it needs a legal registration identifier instead so
// BR-CO-26 (a buyer must be able to identify the supplier by *some*
// identifier) is still satisfied.
func NewInvoiceVariantO() InvoiceDocument {
	doc := baseInvoiceSkeleton()
	doc.Seller.LegalRegistrationID = "12345678"

	lines := []Line{
		{ID: "1", Quantity: 1, UnitCode: "C62", ItemName: "Out of scope supply", NetPrice: NewMoneyFromMajorUnits(60.00), VATCategoryID: "O", VATRate: 0},
	}
	computedLines, breakdown, totals := Finalize(lines)
	doc.Lines = computedLines
	doc.TaxBreakdown = withExemptionReason(breakdown, "VATEX-EU-O")
	doc.Totals = totals
	return doc
}
