package en16931

import (
	"strings"
	"testing"
)

func TestAllInvoiceVariantsBuildCompleteXML(t *testing.T) {
	variants := map[string]func() InvoiceDocument{
		"S":  NewInvoiceVariantS,
		"Z":  NewInvoiceVariantZ,
		"E":  NewInvoiceVariantE,
		"AE": NewInvoiceVariantAE,
		"G":  NewInvoiceVariantG,
		"K":  NewInvoiceVariantK,
		"O":  NewInvoiceVariantO,
	}
	for category, build := range variants {
		t.Run(category, func(t *testing.T) {
			doc := build()
			xml, err := BuildInvoiceXML(doc)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"<Invoice ", "<cbc:CustomizationID>", "<cac:TaxTotal>", "<cac:InvoiceLine>"} {
				if !strings.Contains(xml, required) {
					t.Errorf("generated %s invoice lacks %s", category, required)
				}
			}
			if category == "O" && strings.Count(xml, "<cbc:Percent>") != 1 {
				t.Error("O category line must omit VAT Percent; only the tax subtotal may contain it")
			}
		})
	}
}

func TestBuildInvoiceXMLRejectsNoLinesAndCoversOptionalNodes(t *testing.T) {
	if _, err := BuildInvoiceXML(InvoiceDocument{ID: "empty"}); err == nil {
		t.Fatal("expected no-lines error")
	}
	doc := NewInvoiceVariantS()
	doc.DueDate = ""
	doc.PaymentMeans = []PaymentMeans{{Code: "1"}}
	doc.TaxBreakdown[0].ExemptionReason = "reason"
	doc.Lines[0].Quantity = 1.5
	xml, err := BuildInvoiceXML(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<cbc:TaxExemptionReason>reason</cbc:TaxExemptionReason>", ">1.5</cbc:InvoicedQuantity>"} {
		if !strings.Contains(xml, want) {
			t.Errorf("XML lacks %q", want)
		}
	}
	if got := Money(123).String(); got != "1.23" {
		t.Fatalf("Money.String() = %q", got)
	}
}
