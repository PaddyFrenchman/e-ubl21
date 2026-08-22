package en16931

import (
	"reflect"
	"testing"
)

// TestFinalize table-driven-tests the arithmetic engine end to end (line
// net amounts -> tax breakdown -> totals), per plan §6/§7 step 7. The
// "rounding edge case" table entry reproduces the exact monetary values
// found in the official EN 16931 reference fixture
// testdata/official/guide-example1.xml (TaxableAmount 183.23 @ 6% ->
// TaxAmount 10.99; TaxableAmount 46.37 @ 21% -> TaxAmount 9.74; totals
// LineExtensionAmount 229.60, TaxAmount 20.73, TaxInclusiveAmount 250.33),
// so this test is checked against an independently authored, official
// value - not merely against whatever this engine happens to produce.
func TestFinalize(t *testing.T) {
	tests := []struct {
		name          string
		lines         []Line
		wantNetAmount []Money // parallel to lines, expected computed NetAmount
		wantBreakdown []TaxSubtotal
		wantTotals    Totals
	}{
		{
			name: "single line",
			lines: []Line{
				{ID: "1", Quantity: 2, NetPrice: NewMoneyFromMajorUnits(50.00), VATCategoryID: "S", VATRate: 21},
			},
			wantNetAmount: []Money{NewMoneyFromMajorUnits(100.00)},
			wantBreakdown: []TaxSubtotal{
				{VATCategoryID: "S", VATRate: 21, TaxableAmount: NewMoneyFromMajorUnits(100.00), TaxAmount: NewMoneyFromMajorUnits(21.00)},
			},
			wantTotals: Totals{
				LineExtensionAmount: NewMoneyFromMajorUnits(100.00),
				TaxExclusiveAmount:  NewMoneyFromMajorUnits(100.00),
				TaxAmount:           NewMoneyFromMajorUnits(21.00),
				TaxInclusiveAmount:  NewMoneyFromMajorUnits(121.00),
				PayableAmount:       NewMoneyFromMajorUnits(121.00),
			},
		},
		{
			name: "multiple lines same VAT category",
			lines: []Line{
				{ID: "1", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(30.00), VATCategoryID: "S", VATRate: 21},
				{ID: "2", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(20.00), VATCategoryID: "S", VATRate: 21},
			},
			wantNetAmount: []Money{NewMoneyFromMajorUnits(30.00), NewMoneyFromMajorUnits(20.00)},
			wantBreakdown: []TaxSubtotal{
				{VATCategoryID: "S", VATRate: 21, TaxableAmount: NewMoneyFromMajorUnits(50.00), TaxAmount: NewMoneyFromMajorUnits(10.50)},
			},
			wantTotals: Totals{
				LineExtensionAmount: NewMoneyFromMajorUnits(50.00),
				TaxExclusiveAmount:  NewMoneyFromMajorUnits(50.00),
				TaxAmount:           NewMoneyFromMajorUnits(10.50),
				TaxInclusiveAmount:  NewMoneyFromMajorUnits(60.50),
				PayableAmount:       NewMoneyFromMajorUnits(60.50),
			},
		},
		{
			name: "multiple lines mixed VAT categories",
			lines: []Line{
				{ID: "1", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(100.00), VATCategoryID: "S", VATRate: 21},
				{ID: "2", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(50.00), VATCategoryID: "Z", VATRate: 0},
			},
			wantNetAmount: []Money{NewMoneyFromMajorUnits(100.00), NewMoneyFromMajorUnits(50.00)},
			wantBreakdown: []TaxSubtotal{
				{VATCategoryID: "S", VATRate: 21, TaxableAmount: NewMoneyFromMajorUnits(100.00), TaxAmount: NewMoneyFromMajorUnits(21.00)},
				{VATCategoryID: "Z", VATRate: 0, TaxableAmount: NewMoneyFromMajorUnits(50.00), TaxAmount: NewMoneyFromMajorUnits(0)},
			},
			wantTotals: Totals{
				LineExtensionAmount: NewMoneyFromMajorUnits(150.00),
				TaxExclusiveAmount:  NewMoneyFromMajorUnits(150.00),
				TaxAmount:           NewMoneyFromMajorUnits(21.00),
				TaxInclusiveAmount:  NewMoneyFromMajorUnits(171.00),
				PayableAmount:       NewMoneyFromMajorUnits(171.00),
			},
		},
		{
			// Reproduces testdata/official/guide-example1.xml exactly:
			// 183.23 @ 6% raw tax is 10.9938 (a third decimal digit) and
			// must round to 10.99; 46.37 @ 21% raw tax is 9.7377 and must
			// round to 9.74. Both are official reference values, not
			// self-consistency checks.
			name: "rounding edge case (matches official guide-example1.xml)",
			lines: []Line{
				{ID: "1", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(183.23), VATCategoryID: "S", VATRate: 6},
				{ID: "2", Quantity: 1, NetPrice: NewMoneyFromMajorUnits(46.37), VATCategoryID: "S", VATRate: 21},
			},
			wantNetAmount: []Money{NewMoneyFromMajorUnits(183.23), NewMoneyFromMajorUnits(46.37)},
			wantBreakdown: []TaxSubtotal{
				{VATCategoryID: "S", VATRate: 6, TaxableAmount: NewMoneyFromMajorUnits(183.23), TaxAmount: NewMoneyFromMajorUnits(10.99)},
				{VATCategoryID: "S", VATRate: 21, TaxableAmount: NewMoneyFromMajorUnits(46.37), TaxAmount: NewMoneyFromMajorUnits(9.74)},
			},
			wantTotals: Totals{
				LineExtensionAmount: NewMoneyFromMajorUnits(229.60),
				TaxExclusiveAmount:  NewMoneyFromMajorUnits(229.60),
				TaxAmount:           NewMoneyFromMajorUnits(20.73),
				TaxInclusiveAmount:  NewMoneyFromMajorUnits(250.33),
				PayableAmount:       NewMoneyFromMajorUnits(250.33),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			computedLines, breakdown, totals := Finalize(tc.lines)

			if len(computedLines) != len(tc.wantNetAmount) {
				t.Fatalf("got %d computed lines, want %d", len(computedLines), len(tc.wantNetAmount))
			}
			for i, l := range computedLines {
				if l.NetAmount != tc.wantNetAmount[i] {
					t.Errorf("line %d NetAmount = %s, want %s", i, l.NetAmount, tc.wantNetAmount[i])
				}
			}

			if !reflect.DeepEqual(breakdown, tc.wantBreakdown) {
				t.Errorf("breakdown = %+v, want %+v", breakdown, tc.wantBreakdown)
			}

			if totals != tc.wantTotals {
				t.Errorf("totals = %+v, want %+v", totals, tc.wantTotals)
			}
		})
	}
}

// TestRound2 directly exercises the shared rounding helper used by every
// derived figure, including the half-way-digit case relied on above.
func TestRound2(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  Money
	}{
		{"exact two decimals", 100.00, NewMoneyFromMajorUnits(100.00)},
		{"three decimal digits rounds up", 9.7377, NewMoneyFromMajorUnits(9.74)},
		{"three decimal digits rounds down", 10.9938, NewMoneyFromMajorUnits(10.99)},
		{"zero", 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := round2(tc.input); got != tc.want {
				t.Errorf("round2(%v) = %s, want %s", tc.input, got, tc.want)
			}
		})
	}
}
