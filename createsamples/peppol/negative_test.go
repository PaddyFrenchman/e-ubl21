package peppol_test

import (
	"reflect"
	"strings"
	"testing"

	peppolsamples "github.com/PaddyFrenchman/e-ubl21/createsamples/peppol"
)

func TestNegativeInvoiceFixturesCoverRequiredRuleFamiliesDeterministically(t *testing.T) {
	first, err := peppolsamples.NegativeInvoiceFixtures()
	if err != nil {
		t.Fatal(err)
	}
	second, err := peppolsamples.NegativeInvoiceFixtures()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Peppol negative catalogue changed between consecutive generations")
	}
	if len(first) != 5 {
		t.Fatalf("Peppol negative fixtures = %d, want 5", len(first))
	}
	wantFamilies := map[string]string{
		"identifier": "PEPPOL-EN16931-R004",
		"profile":    "PEPPOL-EN16931-R007",
		"code-list":  "PEPPOL-EN16931-CL008",
		"common":     "PEPPOL-COMMON-R040",
		"country":    "DK-R-014",
	}
	seen := map[string]bool{}
	for _, fixture := range first {
		if fixture.ID == "" || fixture.Mutation == "" || fixture.XML == "" {
			t.Errorf("incomplete negative fixture: %+v", fixture)
		}
		if seen[fixture.Family] {
			t.Errorf("rule family %q has more than one catalogue entry", fixture.Family)
		}
		seen[fixture.Family] = true
		wantRule, ok := wantFamilies[fixture.Family]
		if !ok {
			t.Errorf("unexpected rule family %q", fixture.Family)
			continue
		}
		if !reflect.DeepEqual(fixture.ExpectedRuleIDs, []string{wantRule}) {
			t.Errorf("fixture %s rule IDs = %v, want [%s]", fixture.ID, fixture.ExpectedRuleIDs, wantRule)
		}
		if !strings.Contains(fixture.XML, "<cbc:ProfileID>") || !strings.Contains(fixture.XML, "<cbc:CustomizationID>") {
			t.Errorf("fixture %s does not retain the top-level identifier structure", fixture.ID)
		}
	}
	if len(seen) != len(wantFamilies) {
		t.Fatalf("covered families = %v, want %v", seen, wantFamilies)
	}
}
