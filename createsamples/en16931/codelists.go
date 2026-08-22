// This file curates small, explicitly-sourced tables of legal code values
// for the EN 16931 code-list rules described in plan §7. Values only need
// to be legal, not exhaustive: the acceptance bar is that every code this
// package ever emits is drawn from one of these tables, never an arbitrary
// placeholder string (e.g. the structural generator's "CODE-001"), because
// EN 16931's BR-CL-* rule family fails a document whose codes are XSD-valid
// strings but not members of the mandated code list (see MAIN doc §14 and
// plan §2).
//
// Every table below was verified directly against the official rule
// source, not assumed: EN16931-UBL-codes.sch from the pinned release
// (validation-1.3.16), also cross-checked against the compiled
// EN16931-UBL-validation.xslt in validation-assets/en16931/1.3.16/. Each
// table cites the exact BR-CL-* rule and code-list standard it satisfies.
// If a future EN 16931 artefact upgrade (MAIN doc §48) changes any of
// these lists, this file must be re-verified against the new
// EN16931-UBL-codes.sch before the corpus is regenerated (plan §7).
package en16931

// CodeListEntry pairs a legal code value with a short human-readable
// description. Generated XML only ever emits Code; Description exists for
// readability when reviewing this file or generated fixtures.
type CodeListEntry struct {
	Code        string
	Description string
}

// Currencies lists a small subset of ISO 4217 alpha-3 currency codes legal
// for cbc:DocumentCurrencyCode / cbc:TaxCurrencyCode and every
// currencyID-bearing monetary amount, per BR-CL-03 (verified against the
// full accepted list in EN16931-UBL-codes.sch).
var Currencies = []CodeListEntry{
	{"EUR", "Euro"},
	{"USD", "US Dollar"},
	{"GBP", "Pound Sterling"},
}

// Countries lists a small subset of ISO 3166-1 alpha-2 country codes legal
// for cac:Country/cbc:IdentificationCode (and cac:OriginCountry), per
// BR-CL-14 / BR-CL-15.
var Countries = []CodeListEntry{
	{"NL", "Netherlands"},
	{"DE", "Germany"},
	{"GB", "United Kingdom"},
}

// VATCategories lists the UNCL 5305 subset of VAT category codes EN 16931
// recognises for cac:TaxCategory/cbc:ID and cac:ClassifiedTaxCategory/cbc:ID,
// per BR-CL-17 / BR-CL-18. Only the seven codes relevant to the VAT-category
// variant matrix in plan §7 are listed here (the full accepted set also
// includes L, M and B for Canary Islands / Ceuta & Melilla / transferred
// VAT, which this generator does not target).
//
// Important correction versus an earlier informal label: the "intra-
// community supply" category's actual code letter, as asserted by
// BR-CL-17/18 and used throughout the compiled EN16931-UBL-validation.xslt
// (see the BR-IC-01 rule's test expression, which checks
// cbc:ID = 'K'), is "K" - not "IC". "IC" is only the name of the BR-IC-*
// rule *family* in MAIN doc §13, and also appears as a prefix inside the
// unrelated VATEX exemption-reason code VATEX-EU-IC (see
// VATExemptionReasons below). Plan §7's table should be read with "K" as
// the actual cac:TaxCategory/cbc:ID value for that variant.
var VATCategories = []CodeListEntry{
	{"S", "Standard rate"},
	{"Z", "Zero rated goods"},
	{"E", "Exempt from tax"},
	{"AE", "VAT Reverse Charge"},
	{"G", "Free export item, VAT not charged"},
	{"K", "VAT exempt for EEA intra-community supply of goods and services"},
	{"O", "Services outside scope of tax"},
}

// InvoiceTypeCodes lists a subset of the UNTDID 1001 invoice-related codes
// legal for cbc:InvoiceTypeCode, per BR-CL-01.
var InvoiceTypeCodes = []CodeListEntry{
	{"380", "Commercial invoice"},
	{"384", "Corrected invoice"},
	{"389", "Self-billed invoice"},
}

// CreditNoteTypeCodes lists a subset of the UNTDID 1001 credit-note-related
// codes legal for cbc:CreditNoteTypeCode, per BR-CL-01 (the same rule
// covers both InvoiceTypeCode and CreditNoteTypeCode, with a distinct
// accepted value set per element).
var CreditNoteTypeCodes = []CodeListEntry{
	{"381", "Credit note"},
	{"396", "Self-billed credit note"},
}

// UnitCodes lists a subset of the UN/ECE Recommendation 20 (with
// Recommendation 21 extension) unit-of-measure codes legal for the
// @unitCode attribute of cbc:InvoicedQuantity, cbc:BaseQuantity and
// cbc:CreditedQuantity, per BR-CL-23.
var UnitCodes = []CodeListEntry{
	{"C62", "One (unit)"},
	{"EA", "Each"},
	{"KGM", "Kilogram"},
	{"MTR", "Metre"},
	{"LTR", "Litre"},
}

// PaymentMeansCodes lists a subset of the UNCL 4461 payment-means codes
// legal for cbc:PaymentMeans/cbc:PaymentMeansCode, per BR-CL-16.
var PaymentMeansCodes = []CodeListEntry{
	{"1", "Instrument not defined"},
	{"30", "Credit transfer"},
	{"42", "Payment to bank account"},
	{"49", "Direct debit"},
	{"58", "SEPA credit transfer"},
}

// VATExemptionReasons lists a subset of the CEF VATEX code list legal for
// cbc:TaxExemptionReasonCode, per BR-CL-22. One entry is provided per
// VAT-category variant in plan §7 that requires an exemption reason
// (E, AE, G, K/"IC", O); see the VATCategories comment above for why the
// VATEX code below is spelled "VATEX-EU-IC" even though the corresponding
// cac:TaxCategory/cbc:ID value is "K", not "IC" - that is how the official
// VATEX code list itself names the reason code, independently of the
// UNCL 5305 category letter.
var VATExemptionReasons = []CodeListEntry{
	{"VATEX-EU-132", "Exempt based on article 132 of Council Directive 2006/112/EC"},
	{"VATEX-EU-AE", "Reverse charge"},
	{"VATEX-EU-G", "Export outside the EU"},
	{"VATEX-EU-IC", "Intra-community supply"},
	{"VATEX-EU-O", "Not subject to VAT"},
}
