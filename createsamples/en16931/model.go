// Package en16931 generates EN 16931 semantically-compliant UBL Invoice and
// CreditNote samples, as a follow-on to the structural (XSD-only) generator
// in the parent createsamples package. See
// ideas/EN16931-samples-generator-plan.md for the full design.
//
// This file defines the business-term data model described in plan §5: a
// small, flat model that only encodes the fields EN 16931 actually
// constrains (plus enough structural context to render valid UBL XML around
// them), as opposed to the full UBL schema graph the structural generator
// in generate.go traverses.
package en16931

import (
	"fmt"
	"math"
)

// Money represents a monetary amount as an integer number of minor currency
// units (e.g. euro cents). Every EN 16931 monetary business term is stored
// this way - never as a raw float64 - so that the arithmetic engine in
// calculate.go always performs exact integer arithmetic and can never
// accumulate the binary floating-point rounding drift that would otherwise
// make two supposedly-equal totals compare unequal (the exact failure mode
// the MAIN doc's §3 worked example warns about, and what BR-CO-*/BR-DEC-*
// rules check for).
type Money int64

// NewMoneyFromMajorUnits converts a major-unit amount (e.g. 173.00 for
// "EUR 173.00") into Money, rounding to the nearest minor unit (2 decimal
// places, per the BR-DEC-* family). Callers that already hold minor units
// (e.g. constants) should construct Money directly instead.
func NewMoneyFromMajorUnits(major float64) Money {
	return Money(math.Round(major * 100))
}

// Float64 returns m as a major-unit float64 (e.g. Money(17300) -> 173.00).
// Intended for arithmetic in calculate.go and for handing a value to
// String; callers must not use it to accumulate further Money values
// because that would reintroduce floating-point drift - use Money's own
// integer arithmetic (addition, or NewMoneyFromMajorUnits at the point a
// rate/percentage is applied) instead.
func (m Money) Float64() float64 {
	return float64(m) / 100
}

// String renders m with exactly two decimal places (e.g. "173.00"), the
// fixed precision EN 16931 monetary business terms require.
func (m Money) String() string {
	return fmt.Sprintf("%.2f", m.Float64())
}

// Party models the subset of cac:Party fields EN 16931 constrains for a
// seller (BG-4) or buyer (BG-7): a legal name, a postal address with
// country, an optional VAT identification, and an optional legal
// registration. Not every field is required for every VAT-category variant
// (see plan §7); build.go decides which to emit based on VATCategoryID.
type Party struct {
	// Name is the party's legal/registered name (BT-27 Seller name / BT-44
	// Buyer name), rendered as cac:PartyLegalEntity/cbc:RegistrationName -
	// EN 16931's official UBL mapping uses that element for the party's
	// name, not a separate cac:PartyName/cbc:Name (which instead carries
	// the optional, EN-16931-uncovered "trading name" BT-28/BT-45).
	Name string
	// StreetName, CityName, PostalZone are the postal address (BG-5/BG-8).
	StreetName string
	CityName   string
	PostalZone string
	// CountryCode is the ISO 3166-1 alpha-2 country code of the postal
	// address (BT-40 / BT-55), required for both seller and buyer.
	CountryCode string
	// VATSchemeID is the party's VAT identifier (BT-31 seller / BT-48
	// buyer), e.g. "NL123456789B01". Empty means the party has none, which
	// is legal for a buyer but not for a seller in most VAT categories.
	VATSchemeID string
	// LegalRegistrationID is the party's legal registration identifier
	// (BT-30 Seller legal registration identifier), rendered as
	// cac:PartyLegalEntity/cbc:CompanyID. Empty means omitted.
	LegalRegistrationID string
}

// PaymentMeans models cac:PaymentMeans (BG-16): a payment means type code
// (BT-81) and an optional payee financial account identifier (BT-84).
type PaymentMeans struct {
	Code           string
	PayeeAccountID string
}

// Line models one invoice line (BG-25): quantity and unit, unit price, the
// derived line net amount, an item name, and the VAT category/rate applied
// to the line (BG-30). NetAmount is populated by calculate.go from Quantity
// and NetPrice - it is never hand-authored independently of them, per the
// golden rule in plan §6.
type Line struct {
	// ID is the line identifier (BT-126).
	ID string
	// Quantity is the invoiced quantity (BT-129); UnitCode is its unit of
	// measure code (BT-130), e.g. "C62", "H87".
	Quantity float64
	UnitCode string
	// ItemName is the item's name (BT-153).
	ItemName string
	// NetPrice is the item's net price (BT-146).
	NetPrice Money
	// NetAmount is the line's net amount (BT-131), derived as
	// round2(Quantity * NetPrice) by calculate.go.
	NetAmount Money
	// VATCategoryID is the invoiced item's VAT category code (BT-151), one
	// of S/Z/E/AE/G/IC/O (see plan §7). VATRate is its VAT rate percentage
	// (BT-152), e.g. 21.0; category-dependent rules constrain whether this
	// must be zero (Z, E, AE, G, IC, O) or positive (S).
	VATCategoryID string
	VATRate       float64
}

// CreditNoteLine has the same shape as Line: EN 16931 constrains a credit
// note line (BG-25 there too) identically to an invoice line, the only
// syntax difference being the UBL element name used to carry the quantity
// (cbc:CreditedQuantity rather than cbc:InvoicedQuantity), which build.go
// selects based on which document type it is rendering.
type CreditNoteLine = Line

// TaxSubtotal models one cac:TaxSubtotal entry (BG-23): the taxable amount
// and tax amount for one VAT category/rate combination, plus an optional
// VAT exemption reason required by several category-specific rule families
// (BR-E-*, BR-AE-*, BR-G-*, BR-IC-*, BR-O-*; see MAIN doc §13 and plan §7).
type TaxSubtotal struct {
	VATCategoryID string
	VATRate       float64
	// TaxableAmount is the sum of the net amounts of all lines sharing this
	// VAT category/rate (BT-116); TaxAmount is
	// round2(TaxableAmount * VATRate / 100) (BT-117), per BR-CO-17.
	TaxableAmount Money
	TaxAmount     Money
	// ExemptionReasonCode / ExemptionReason are BT-121 / BT-120: required
	// whenever VATCategoryID is E, AE, G, IC or O; left empty for S and Z.
	ExemptionReasonCode string
	ExemptionReason     string
}

// Totals models cac:LegalMonetaryTotal (BG-22): the chain of derived
// document totals from line-extension amount through to the payable
// amount. Every field here is computed by calculate.go from Lines and
// TaxBreakdown - see plan §6 for the exact derivation and the v1
// simplifications (no allowances/charges, no prepaid/rounding amount).
type Totals struct {
	// LineExtensionAmount is BT-106: sum of all line net amounts.
	LineExtensionAmount Money
	// TaxExclusiveAmount is BT-109: equals LineExtensionAmount in this v1
	// model (no document-level allowances/charges yet - see plan §11).
	TaxExclusiveAmount Money
	// TaxAmount is BT-110: sum of all TaxSubtotal.TaxAmount values.
	TaxAmount Money
	// TaxInclusiveAmount is BT-112: TaxExclusiveAmount + TaxAmount.
	TaxInclusiveAmount Money
	// PayableAmount is BT-115: equals TaxInclusiveAmount in this v1 model
	// (no prepaid amount or rounding amount yet - see plan §11).
	PayableAmount Money
}

// InvoiceDocument models the EN 16931 baseline business terms for a UBL
// Invoice (per plan §5). CustomizationID must be the fixed EN 16931
// specification identifier; DocumentCurrencyCode, InvoiceTypeCode and the
// VAT category codes used across Lines/TaxBreakdown must come from
// codelists.go, not arbitrary placeholder strings, or the document will
// fail BR-CL-* code-list rules even though it remains XSD-valid.
type InvoiceDocument struct {
	// CustomizationID is BT-24, fixed to "urn:cen.eu:en16931:2017" for
	// every document this package generates.
	CustomizationID string
	// ID is BT-1, IssueDate is BT-2, DueDate is BT-9 (optional),
	// InvoiceTypeCode is BT-3, DocumentCurrencyCode is BT-5.
	ID                   string
	IssueDate            string
	DueDate              string
	InvoiceTypeCode      string
	DocumentCurrencyCode string
	Seller               Party
	Buyer                Party
	PaymentMeans         []PaymentMeans
	Lines                []Line
	TaxBreakdown         []TaxSubtotal
	Totals               Totals
}

// CreditNoteDocument mirrors InvoiceDocument with CreditNoteTypeCode (BT-3
// for a credit note) in place of InvoiceTypeCode, and CreditNoteLine in
// place of Line (see plan §5).
type CreditNoteDocument struct {
	CustomizationID      string
	ID                   string
	IssueDate            string
	CreditNoteTypeCode   string
	DocumentCurrencyCode string
	Seller               Party
	Buyer                Party
	PaymentMeans         []PaymentMeans
	Lines                []CreditNoteLine
	TaxBreakdown         []TaxSubtotal
	Totals               Totals
}

// EN16931CustomizationID is the fixed BT-24 value (plan §2, verified against
// the official guide-example1.xml fixture in testdata/official/) that every
// document produced by this package must declare.
const EN16931CustomizationID = "urn:cen.eu:en16931:2017"
