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
	if len(first) != 15 {
		t.Fatalf("Peppol negative fixtures = %d, want 15", len(first))
	}
	wantRules := map[string][]string{
		"control-en-unsupported-attachment-mime": {"BR-CL-24"},
		"control-xsd-missing-invoice-id":         nil,
		"danish-payment-means-restriction":       {"DK-R-005"},
		"danish-supplier-legal-scheme":           {"DK-R-014"},
		"inconsistent-line-currency":             {"PEPPOL-EN16931-R051"},
		"invalid-customization-id":               {"PEPPOL-EN16931-R004"},
		"invalid-endpoint-scheme":                {"PEPPOL-EN16931-CL008"},
		"invalid-gln-check-digit":                {"PEPPOL-COMMON-R040"},
		"line-arithmetic-outside-slack":          {"PEPPOL-EN16931-R120"},
		"missing-danish-party-scheme":            {"DK-R-013"},
		"missing-profile-id":                     {"PEPPOL-EN16931-R001", "PEPPOL-EN16931-R007"},
		"missing-seller-endpoint":                {"PEPPOL-EN16931-R020"},
		"multiple-document-notes":                {"PEPPOL-EN16931-R002"},
		"restricted-invoice-type-code":           {"PEPPOL-EN16931-P0112"},
		"unsupported-profile-id":                 {"PEPPOL-EN16931-R007"},
	}
	wantFamilies := map[string]bool{
		"arithmetic": true, "cardinality": true, "code-list": true,
		"common": true, "country": true, "currency": true, "endpoint": true,
		"identifier": true, "party-scheme": true, "payment": true, "profile": true,
		"stage-control": true,
	}
	seenIDs := map[string]bool{}
	seenFamilies := map[string]bool{}
	for _, fixture := range first {
		if fixture.ID == "" || fixture.Mutation == "" || fixture.FailureStage == "" || fixture.XML == "" {
			t.Errorf("incomplete negative fixture: %+v", fixture)
		}
		if seenIDs[fixture.ID] {
			t.Errorf("duplicate fixture ID %q", fixture.ID)
		}
		seenIDs[fixture.ID] = true
		seenFamilies[fixture.Family] = true
		wantRuleIDs, ok := wantRules[fixture.ID]
		if !ok {
			t.Errorf("unexpected fixture %q", fixture.ID)
			continue
		}
		if !reflect.DeepEqual(fixture.ExpectedRuleIDs, wantRuleIDs) {
			t.Errorf("fixture %s rule IDs = %v, want %v", fixture.ID, fixture.ExpectedRuleIDs, wantRuleIDs)
		}
		if fixture.ID != "missing-profile-id" && !strings.Contains(fixture.XML, "<cbc:ProfileID>") {
			t.Errorf("fixture %s unexpectedly lacks ProfileID", fixture.ID)
		}
		if !strings.Contains(fixture.XML, "<cbc:CustomizationID>") {
			t.Errorf("fixture %s does not retain the top-level identifier structure", fixture.ID)
		}
	}
	if !reflect.DeepEqual(seenFamilies, wantFamilies) {
		t.Fatalf("covered families = %v, want %v", seenFamilies, wantFamilies)
	}
}
