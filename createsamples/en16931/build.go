// This file converts a populated InvoiceDocument (see model.go) into the
// shared xmlnode.Node tree (createsamples/xmlnode), reusing that package's
// Render function for serialization instead of reimplementing XML
// rendering - see plan §4/§9/§10 (Phase EN-2, step 10).
//
// build.go is a pure structural converter: it assumes doc's monetary
// fields (Line.NetAmount, TaxBreakdown, Totals) were already derived by
// calculate.go's Finalize (or equivalent), and it performs no EN 16931
// business-rule validation itself - that is the job of the official
// Saxon + EN16931-UBL-validation.xslt pipeline the generated XML is run
// through in later steps (plan §10, Phase EN-2 steps 11-12).
package en16931

import (
	"fmt"
	"math"
	"strconv"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/xmlnode"
)

// Namespaces used by every document this package generates, matching the
// structural generator's convention in createsamples/generate.go: the
// document root is unprefixed (default namespace), while cac/cbc-qualified
// elements use their usual prefixes.
const (
	nsInvoice = "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	nsCAC     = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	nsCBC     = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
)

// leaf builds a childless element with the given namespace prefix, name
// and text value.
func leaf(prefix, name, value string) xmlnode.Node {
	return xmlnode.Node{Prefix: prefix, Name: name, Value: value}
}

// leafAttr builds a childless element like leaf, plus one XML attribute
// (e.g. currencyID on a monetary amount, unitCode on a quantity).
func leafAttr(prefix, name, value, attrKey, attrVal string) xmlnode.Node {
	return xmlnode.Node{Prefix: prefix, Name: name, Value: value, Attributes: map[string]string{attrKey: attrVal}}
}

// parent builds an element with nested children.
func parent(prefix, name string, children ...xmlnode.Node) xmlnode.Node {
	return xmlnode.Node{Prefix: prefix, Name: name, Children: children}
}

// cbcLeaf and cacParent are thin shorthands for the two namespace prefixes
// every EN 16931 UBL business term uses.
func cbcLeaf(name, value string) xmlnode.Node               { return leaf("cbc", name, value) }
func cacParent(name string, c ...xmlnode.Node) xmlnode.Node { return parent("cac", name, c...) }

// cbcAmount builds a monetary leaf element with its required currencyID
// attribute (e.g. <cbc:PayableAmount currencyID="EUR">173.00</...>).
func cbcAmount(name string, amount Money, currency string) xmlnode.Node {
	return leafAttr("cbc", name, amount.String(), "currencyID", currency)
}

// formatRate renders a VAT rate percentage with 2 decimal places (e.g.
// "21.00"), matching how the official example fixtures format cbc:Percent.
func formatRate(rate float64) string {
	return fmt.Sprintf("%.2f", rate)
}

// formatQuantity renders a quantity value, omitting a trailing ".00" for
// whole numbers (e.g. "2" rather than "2.00") while still allowing
// fractional quantities where a variant needs them.
func formatQuantity(qty float64) string {
	if qty == math.Trunc(qty) {
		return strconv.FormatFloat(qty, 'f', 0, 64)
	}
	return strconv.FormatFloat(qty, 'f', -1, 64)
}

// buildPartyNode renders one cac:AccountingSupplierParty /
// cac:AccountingCustomerParty block (BG-4 / BG-7) from a Party.
func buildPartyNode(containerName string, p Party) xmlnode.Node {
	partyChildren := []xmlnode.Node{
		cacParent("PostalAddress",
			cbcLeaf("StreetName", p.StreetName),
			cbcLeaf("CityName", p.CityName),
			cbcLeaf("PostalZone", p.PostalZone),
			cacParent("Country", cbcLeaf("IdentificationCode", p.CountryCode)),
		),
	}

	if p.VATSchemeID != "" {
		partyChildren = append(partyChildren, cacParent("PartyTaxScheme",
			cbcLeaf("CompanyID", p.VATSchemeID),
			cacParent("TaxScheme", cbcLeaf("ID", "VAT")),
		))
	}

	legalChildren := []xmlnode.Node{cbcLeaf("RegistrationName", p.Name)}
	if p.LegalRegistrationID != "" {
		legalChildren = append(legalChildren, cbcLeaf("CompanyID", p.LegalRegistrationID))
	}
	partyChildren = append(partyChildren, cacParent("PartyLegalEntity", legalChildren...))

	return cacParent(containerName, cacParent("Party", partyChildren...))
}

// buildPaymentMeansNode renders one cac:PaymentMeans block (BG-16).
func buildPaymentMeansNode(pm PaymentMeans) xmlnode.Node {
	children := []xmlnode.Node{cbcLeaf("PaymentMeansCode", pm.Code)}
	if pm.PayeeAccountID != "" {
		children = append(children, cacParent("PayeeFinancialAccount", cbcLeaf("ID", pm.PayeeAccountID)))
	}
	return cacParent("PaymentMeans", children...)
}

// buildTaxSubtotalNode renders one cac:TaxSubtotal block (BG-23) from a
// TaxSubtotal, including its optional VAT exemption reason.
func buildTaxSubtotalNode(s TaxSubtotal, currency string) xmlnode.Node {
	categoryChildren := []xmlnode.Node{
		cbcLeaf("ID", s.VATCategoryID),
		cbcLeaf("Percent", formatRate(s.VATRate)),
	}
	if s.ExemptionReasonCode != "" {
		categoryChildren = append(categoryChildren, cbcLeaf("TaxExemptionReasonCode", s.ExemptionReasonCode))
	}
	if s.ExemptionReason != "" {
		categoryChildren = append(categoryChildren, cbcLeaf("TaxExemptionReason", s.ExemptionReason))
	}
	categoryChildren = append(categoryChildren, cacParent("TaxScheme", cbcLeaf("ID", "VAT")))

	return cacParent("TaxSubtotal",
		cbcAmount("TaxableAmount", s.TaxableAmount, currency),
		cbcAmount("TaxAmount", s.TaxAmount, currency),
		cacParent("TaxCategory", categoryChildren...),
	)
}

// buildLineNode renders one invoice/credit-note line (BG-25). lineElement
// is "InvoiceLine" or "CreditNoteLine"; quantityElement is
// "InvoicedQuantity" or "CreditedQuantity" - the only structural
// difference EN 16931 makes between the two document types' lines (see
// CreditNoteLine's doc comment in model.go).
func buildLineNode(lineElement, quantityElement string, l Line, currency string) xmlnode.Node {
	return cacParent(lineElement,
		cbcLeaf("ID", l.ID),
		leafAttr("cbc", quantityElement, formatQuantity(l.Quantity), "unitCode", l.UnitCode),
		cbcAmount("LineExtensionAmount", l.NetAmount, currency),
		cacParent("Item",
			cbcLeaf("Name", l.ItemName),
			cacParent("ClassifiedTaxCategory",
				cbcLeaf("ID", l.VATCategoryID),
				cbcLeaf("Percent", formatRate(l.VATRate)),
				cacParent("TaxScheme", cbcLeaf("ID", "VAT")),
			),
		),
		cacParent("Price", cbcAmount("PriceAmount", l.NetPrice, currency)),
	)
}

// BuildInvoiceXML converts a populated InvoiceDocument into a complete,
// serialized UBL Invoice XML document. doc must already have its monetary
// fields derived (e.g. via Finalize) and its codes drawn from
// codelists.go - this function performs no such computation or validation
// itself.
func BuildInvoiceXML(doc InvoiceDocument) (string, error) {
	if len(doc.Lines) == 0 {
		return "", fmt.Errorf("invoice %q has no lines to render", doc.ID)
	}

	children := []xmlnode.Node{
		cbcLeaf("CustomizationID", doc.CustomizationID),
		cbcLeaf("ID", doc.ID),
		cbcLeaf("IssueDate", doc.IssueDate),
	}
	if doc.DueDate != "" {
		children = append(children, cbcLeaf("DueDate", doc.DueDate))
	}
	children = append(children,
		cbcLeaf("InvoiceTypeCode", doc.InvoiceTypeCode),
		cbcLeaf("DocumentCurrencyCode", doc.DocumentCurrencyCode),
		buildPartyNode("AccountingSupplierParty", doc.Seller),
		buildPartyNode("AccountingCustomerParty", doc.Buyer),
	)

	for _, pm := range doc.PaymentMeans {
		children = append(children, buildPaymentMeansNode(pm))
	}

	taxTotalChildren := make([]xmlnode.Node, 0, len(doc.TaxBreakdown)+1)
	taxTotalChildren = append(taxTotalChildren, cbcAmount("TaxAmount", doc.Totals.TaxAmount, doc.DocumentCurrencyCode))
	for _, s := range doc.TaxBreakdown {
		taxTotalChildren = append(taxTotalChildren, buildTaxSubtotalNode(s, doc.DocumentCurrencyCode))
	}
	children = append(children, cacParent("TaxTotal", taxTotalChildren...))

	children = append(children, cacParent("LegalMonetaryTotal",
		cbcAmount("LineExtensionAmount", doc.Totals.LineExtensionAmount, doc.DocumentCurrencyCode),
		cbcAmount("TaxExclusiveAmount", doc.Totals.TaxExclusiveAmount, doc.DocumentCurrencyCode),
		cbcAmount("TaxInclusiveAmount", doc.Totals.TaxInclusiveAmount, doc.DocumentCurrencyCode),
		cbcAmount("PayableAmount", doc.Totals.PayableAmount, doc.DocumentCurrencyCode),
	))

	for _, l := range doc.Lines {
		children = append(children, buildLineNode("InvoiceLine", "InvoicedQuantity", l, doc.DocumentCurrencyCode))
	}

	root := xmlnode.Node{
		Name: "Invoice",
		Attributes: map[string]string{
			"xmlns":     nsInvoice,
			"xmlns:cac": nsCAC,
			"xmlns:cbc": nsCBC,
		},
		Children: children,
	}

	return "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" + xmlnode.Render(root, 0) + "\n", nil
}
