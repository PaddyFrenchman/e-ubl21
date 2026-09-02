package server

import (
	"errors"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestResolveFrozenValidationPlanForAutoProfiles(t *testing.T) {
	tests := []struct {
		name         string
		documentType string
		document     []byte
		wantProfile  queue.ValidationProfile
		wantStages   []queue.ValidationLayer
		wantProcess  queue.PeppolProcessProfile
	}{
		{
			name: "Profile 01 Invoice", documentType: "Invoice",
			document:    peppolBillingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01),
			wantProfile: queue.ValidationProfilePeppolBISBilling,
			wantStages: []queue.ValidationLayer{
				queue.ValidationLayerUBL21XSD,
				queue.ValidationLayerEN16931,
				queue.ValidationLayerPeppolBISBilling,
			},
			wantProcess: queue.PeppolProcessProfile01,
		},
		{
			name: "Profile 02 CreditNote", documentType: "CreditNote",
			document:    peppolBillingDocument("CreditNote", peppol.CustomizationIDBase, peppol.ProfileID02),
			wantProfile: queue.ValidationProfilePeppolBISBilling,
			wantStages: []queue.ValidationLayer{
				queue.ValidationLayerUBL21XSD,
				queue.ValidationLayerEN16931,
				queue.ValidationLayerPeppolBISBilling,
			},
			wantProcess: queue.PeppolProcessProfile02,
		},
		{
			name: "Invoice without Peppol identifiers", documentType: "Invoice",
			document:    peppolBillingDocument("Invoice", "", ""),
			wantProfile: queue.ValidationProfileUBL21EN16931,
			wantStages: []queue.ValidationLayer{
				queue.ValidationLayerUBL21XSD,
				queue.ValidationLayerEN16931,
			},
		},
		{
			name: "other UBL document", documentType: "Order",
			wantProfile: queue.ValidationProfileUBL21,
			wantStages:  []queue.ValidationLayer{queue.ValidationLayerUBL21XSD},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution, plan, err := ResolveFrozenValidationPlanFor(test.documentType, queue.ValidationProfileAuto, test.document)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.ResolvedProfile != test.wantProfile || resolution.Identifiers.ProcessProfile != test.wantProcess {
				t.Fatalf("resolution = %+v", resolution)
			}
			assertPlanLayers(t, plan, test.wantProfile, test.wantStages)
		})
	}
}

func TestResolveFrozenValidationPlanForExplicitPeppolDoesNotDowngrade(t *testing.T) {
	resolution, plan, err := ResolveFrozenValidationPlanFor(
		"Invoice",
		queue.ValidationProfilePeppolBISBilling,
		peppolBillingDocument("Invoice", "", ""),
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.ResolvedProfile != queue.ValidationProfilePeppolBISBilling ||
		len(resolution.Failures) != 2 ||
		resolution.Failures[0].Code != peppol.FailureCustomizationIDMissing ||
		resolution.Failures[1].Code != peppol.FailureProfileIDMissing {
		t.Fatalf("explicit Peppol resolution = %+v", resolution)
	}
	assertPlanLayers(t, plan, queue.ValidationProfilePeppolBISBilling, []queue.ValidationLayer{
		queue.ValidationLayerUBL21XSD,
		queue.ValidationLayerEN16931,
		queue.ValidationLayerPeppolBISBilling,
	})
}

func TestResolveFrozenValidationPlanForRejectsInapplicableProfile(t *testing.T) {
	_, _, err := ResolveFrozenValidationPlanFor("Order", queue.ValidationProfilePeppolBISBilling, nil)
	if !errors.Is(err, peppol.ErrProfileNotApplicable) {
		t.Fatalf("inapplicable Peppol plan error = %v", err)
	}
}

func assertPlanLayers(t *testing.T, plan queue.FrozenValidationPlan, profile queue.ValidationProfile, want []queue.ValidationLayer) {
	t.Helper()
	if plan.ID != string(profile) || len(plan.Stages) != len(want) {
		t.Fatalf("plan = %+v, want profile %q stages %v", plan, profile, want)
	}
	for index, layer := range want {
		if plan.Stages[index].ID != layer || plan.Stages[index].Ordinal != uint32(index+1) ||
			plan.Stages[index].Requirement != queue.ValidationStageRequired {
			t.Fatalf("plan stage %d = %+v, want %q", index, plan.Stages[index], layer)
		}
	}
}

func peppolBillingDocument(documentType, customizationID, profileID string) []byte {
	namespace := "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	if documentType == "CreditNote" {
		namespace = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	}
	var document strings.Builder
	document.WriteString("<" + documentType + ` xmlns="` + namespace +
		`" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">`)
	if customizationID != "" {
		document.WriteString("<cbc:CustomizationID>" + customizationID + "</cbc:CustomizationID>")
	}
	if profileID != "" {
		document.WriteString("<cbc:ProfileID>" + profileID + "</cbc:ProfileID>")
	}
	document.WriteString("</" + documentType + ">")
	return []byte(document.String())
}
