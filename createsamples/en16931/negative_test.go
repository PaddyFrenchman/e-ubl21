package en16931

import "testing"

func TestNegativeInvoiceFixturesHaveStableUniqueContracts(t *testing.T) {
	fixtures, err := NegativeInvoiceFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 22 {
		t.Fatalf("negative fixture count = %d, want 22", len(fixtures))
	}
	seen := map[string]bool{}
	rules := map[string]bool{}
	for _, fixture := range fixtures {
		if fixture.ID == "" || fixture.DocumentType != "Invoice" || fixture.Mutation == "" || fixture.XML == "" || len(fixture.ExpectedRuleIDs) == 0 {
			t.Fatalf("incomplete fixture contract: %+v", fixture)
		}
		if seen[fixture.ID] {
			t.Fatalf("duplicate fixture ID %q", fixture.ID)
		}
		seen[fixture.ID] = true
		for _, rule := range fixture.ExpectedRuleIDs {
			rules[rule] = true
		}
	}
	for _, required := range []string{
		"BR-CO-15", "BR-DEC-12", "BR-CL-01", "BR-06", "BR-07", "BR-CO-26", "BR-AE-02", "BR-61",
		"BR-S-05", "BR-Z-05", "BR-E-05", "BR-AE-05", "BR-G-05", "BR-IC-05", "BR-O-05",
		"BR-E-10", "BR-AE-10", "BR-G-10", "BR-IC-10", "BR-O-10", "UBL-SR-42", "UBL-CR-673",
	} {
		if !rules[required] {
			t.Errorf("negative corpus does not target required rule %s", required)
		}
	}
}
