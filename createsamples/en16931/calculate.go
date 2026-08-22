// This file implements the monetary/tax arithmetic engine described in
// plan §6: every EN 16931 monetary business term that must reconcile with
// another (line net amounts, tax subtotals, document totals) is derived
// here from the underlying line items, never hand-authored independently.
// This is exactly the failure mode the MAIN doc's §3 worked example warns
// about (an internally inconsistent LegalMonetaryTotal that is XSD-valid
// but fails EN 16931 arithmetic rules such as BR-CO-10/15/17).
package en16931

// round2 rounds a major-unit amount to the nearest 2 decimal places and
// returns it as Money (minor units). This is the single rounding choke
// point every function below uses, so that totals and subtotals can never
// drift apart due to inconsistent rounding - the BR-DEC-* family checks
// exactly this kind of precision.
func round2(majorUnits float64) Money {
	return NewMoneyFromMajorUnits(majorUnits)
}

// ComputeLineNetAmounts returns a copy of lines with NetAmount populated as
// round2(Quantity * NetPrice) for each line (BT-131, mirrors BR-CO-10's
// per-line contribution). It does not mutate the input slice.
func ComputeLineNetAmounts(lines []Line) []Line {
	out := make([]Line, len(lines))
	for i, l := range lines {
		l.NetAmount = round2(l.Quantity * l.NetPrice.Float64())
		out[i] = l
	}
	return out
}

// LineExtensionAmount returns the sum of every line's NetAmount (BT-106),
// mirroring BR-CO-10. Lines must already have NetAmount populated, e.g. via
// ComputeLineNetAmounts.
func LineExtensionAmount(lines []Line) Money {
	var total Money
	for _, l := range lines {
		total += l.NetAmount
	}
	return total
}

// taxGroupKey identifies one VAT category/rate combination that lines are
// grouped into a single TaxSubtotal by (BG-23).
type taxGroupKey struct {
	category string
	rate     float64
}

// ComputeTaxBreakdown groups lines by VAT category and rate into
// TaxSubtotal entries (BG-23): TaxableAmount is the sum of each group's
// line net amounts (BT-116) and TaxAmount is
// round2(TaxableAmount * rate / 100) (BT-117), per BR-CO-17. Lines must
// already have NetAmount populated, e.g. via ComputeLineNetAmounts. The
// returned slice is ordered by each category/rate combination's first
// appearance in lines, so output is deterministic.
func ComputeTaxBreakdown(lines []Line) []TaxSubtotal {
	var order []taxGroupKey
	taxable := map[taxGroupKey]Money{}
	for _, l := range lines {
		key := taxGroupKey{category: l.VATCategoryID, rate: l.VATRate}
		if _, seen := taxable[key]; !seen {
			order = append(order, key)
		}
		taxable[key] += l.NetAmount
	}

	breakdown := make([]TaxSubtotal, 0, len(order))
	for _, key := range order {
		amount := taxable[key]
		tax := round2(amount.Float64() * key.rate / 100)
		breakdown = append(breakdown, TaxSubtotal{
			VATCategoryID: key.category,
			VATRate:       key.rate,
			TaxableAmount: amount,
			TaxAmount:     tax,
		})
	}
	return breakdown
}

// ComputeTotals derives the full BG-22 totals chain from lines and their
// tax breakdown (plan §6 rules 2 and 4-7):
//
//   - LineExtensionAmount (BT-106) is the sum of line net amounts.
//   - TaxExclusiveAmount (BT-109) equals LineExtensionAmount: this v1 model
//     has no document-level allowances/charges yet (see plan §11).
//   - TaxAmount (BT-110) is the sum of every subtotal's TaxAmount.
//   - TaxInclusiveAmount (BT-112) is TaxExclusiveAmount + TaxAmount, per
//     BR-CO-15.
//   - PayableAmount (BT-115) equals TaxInclusiveAmount: this v1 model has
//     no prepaid amount or rounding amount yet (see plan §11).
//
// lines must already have NetAmount populated, e.g. via
// ComputeLineNetAmounts.
func ComputeTotals(lines []Line, breakdown []TaxSubtotal) Totals {
	lineExtension := LineExtensionAmount(lines)

	var taxAmount Money
	for _, subtotal := range breakdown {
		taxAmount += subtotal.TaxAmount
	}

	taxExclusive := lineExtension
	taxInclusive := taxExclusive + taxAmount
	payable := taxInclusive

	return Totals{
		LineExtensionAmount: lineExtension,
		TaxExclusiveAmount:  taxExclusive,
		TaxAmount:           taxAmount,
		TaxInclusiveAmount:  taxInclusive,
		PayableAmount:       payable,
	}
}

// Finalize computes NetAmount for every line, the VAT tax breakdown, and
// the document totals in one call, returning values ready to attach to an
// InvoiceDocument or CreditNoteDocument. This is the single entry point
// build.go should use so that every derived monetary figure is always
// produced by this engine and never hand-authored - the golden rule in
// plan §6.
func Finalize(lines []Line) (computedLines []Line, breakdown []TaxSubtotal, totals Totals) {
	computedLines = ComputeLineNetAmounts(lines)
	breakdown = ComputeTaxBreakdown(computedLines)
	totals = ComputeTotals(computedLines, breakdown)
	return computedLines, breakdown, totals
}
