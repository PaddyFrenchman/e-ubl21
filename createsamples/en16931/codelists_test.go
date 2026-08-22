package en16931

import "testing"

// TestCodeListsNonEmptyAndWellFormed is a lightweight smoke test guarding
// against accidental empty tables or blank codes in codelists.go; it does
// not re-verify legality against the official EN16931-UBL-codes.sch (that
// was done manually when the tables were authored - see the file's header
// comment) because embedding or re-downloading the full master lists here
// would defeat the point of keeping these tables small and curated (plan
// §7) and would violate the generator's offline-operation requirement
// (.github/create-samples.md §16).
func TestCodeListsNonEmptyAndWellFormed(t *testing.T) {
	tables := map[string][]CodeListEntry{
		"Currencies":          Currencies,
		"Countries":           Countries,
		"VATCategories":       VATCategories,
		"InvoiceTypeCodes":    InvoiceTypeCodes,
		"CreditNoteTypeCodes": CreditNoteTypeCodes,
		"UnitCodes":           UnitCodes,
		"PaymentMeansCodes":   PaymentMeansCodes,
		"VATExemptionReasons": VATExemptionReasons,
	}

	for name, entries := range tables {
		if len(entries) == 0 {
			t.Errorf("%s is empty", name)
		}
		seen := map[string]bool{}
		for _, e := range entries {
			if e.Code == "" {
				t.Errorf("%s contains an entry with an empty Code", name)
			}
			if e.Description == "" {
				t.Errorf("%s entry %q has an empty Description", name, e.Code)
			}
			if seen[e.Code] {
				t.Errorf("%s contains duplicate code %q", name, e.Code)
			}
			seen[e.Code] = true
		}
	}

	// The seven VAT categories required by the plan §7 variant matrix must
	// all be present, using "K" (not "IC") for intra-community supply -
	// see the VATCategories doc comment.
	required := []string{"S", "Z", "E", "AE", "G", "K", "O"}
	present := map[string]bool{}
	for _, e := range VATCategories {
		present[e.Code] = true
	}
	for _, code := range required {
		if !present[code] {
			t.Errorf("VATCategories is missing required code %q", code)
		}
	}
}
