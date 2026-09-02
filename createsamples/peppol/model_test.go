package peppol_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/createsamples/en16931"
	peppolsamples "github.com/PaddyFrenchman/e-ubl21/createsamples/peppol"
	peppolprofile "github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestPeppolCandidatesAreDeterministicAndResolvable(t *testing.T) {
	for _, profile := range peppolsamples.Profiles() {
		t.Run("Invoice Profile "+string(profile), func(t *testing.T) {
			first, err := peppolsamples.NewInvoice(profile)
			if err != nil {
				t.Fatal(err)
			}
			second, err := peppolsamples.NewInvoice(profile)
			if err != nil {
				t.Fatal(err)
			}
			firstXML, err := en16931.BuildInvoiceXML(first)
			if err != nil {
				t.Fatal(err)
			}
			secondXML, err := en16931.BuildInvoiceXML(second)
			if err != nil {
				t.Fatal(err)
			}
			if firstXML != secondXML {
				t.Fatal("Invoice generation is not deterministic")
			}
			assertPeppolCandidate(t, "Invoice", profile, firstXML)
		})

		t.Run("CreditNote Profile "+string(profile), func(t *testing.T) {
			document, err := peppolsamples.NewCreditNote(profile)
			if err != nil {
				t.Fatal(err)
			}
			content, err := en16931.BuildCreditNoteXML(document)
			if err != nil {
				t.Fatal(err)
			}
			assertPeppolCandidate(t, "CreditNote", profile, content)
		})
	}
}

func TestPeppolCandidateCodeListValues(t *testing.T) {
	for name, value := range map[string]string{
		"seller GLN": peppolsamples.SellerGLN,
		"buyer GLN":  peppolsamples.BuyerGLN,
	} {
		if !validGLN(value) {
			t.Errorf("%s %q has an invalid check digit", name, value)
		}
	}
	if peppolsamples.EndpointSchemeGLN != "0088" {
		t.Fatalf("endpoint scheme = %q, want 0088", peppolsamples.EndpointSchemeGLN)
	}
	for _, name := range []string{"eas.xml", "icd.xml"} {
		content, err := os.ReadFile(filepath.Join(
			"..", "..", "validation-assets", "peppol", "3.0.21", "codelists", name,
		))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "<Id>0088</Id>") {
			t.Errorf("pinned %s does not contain scheme 0088", name)
		}
	}
}

func TestPeppolCandidateRejectsUnknownProfile(t *testing.T) {
	if _, err := peppolsamples.NewInvoice("03"); err == nil {
		t.Fatal("unsupported Invoice profile was accepted")
	}
	if _, err := peppolsamples.NewCreditNote("03"); err == nil {
		t.Fatal("unsupported CreditNote profile was accepted")
	}
}

func assertPeppolCandidate(t *testing.T, documentType string, profile peppolsamples.ProcessProfile, content string) {
	t.Helper()
	wantProfileID, err := peppolsamples.ProfileID(profile)
	if err != nil {
		t.Fatal(err)
	}
	resolution, err := peppolprofile.Resolve(documentType, queue.ValidationProfileAuto, []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.PeppolIdentifiersValid() ||
		resolution.Identifiers.ProfileID != wantProfileID ||
		string(resolution.Identifiers.ProcessProfile) != string(profile) {
		t.Fatalf("generated %s does not resolve to Profile %s: %+v", documentType, profile, resolution)
	}
	for _, required := range []string{
		`<cbc:CustomizationID>` + peppolprofile.CustomizationIDBase + `</cbc:CustomizationID>`,
		`<cbc:ProfileID>` + wantProfileID + `</cbc:ProfileID>`,
		`<cbc:EndpointID schemeID="0088">` + peppolsamples.SellerGLN + `</cbc:EndpointID>`,
		`<cbc:EndpointID schemeID="0088">` + peppolsamples.BuyerGLN + `</cbc:EndpointID>`,
		`<cac:PartyIdentification>`,
		`<cbc:ID schemeID="0088">`,
		`<cbc:BuyerReference>BUYER-REF-P` + string(profile) + `</cbc:BuyerReference>`,
	} {
		if !strings.Contains(content, required) {
			t.Errorf("generated %s Profile %s lacks %s", documentType, profile, required)
		}
	}
}

func validGLN(value string) bool {
	if len(value) != 13 {
		return false
	}
	sum := 0
	for index := len(value) - 2; index >= 0; index-- {
		digit := int(value[index] - '0')
		if digit < 0 || digit > 9 {
			return false
		}
		positionFromRight := len(value) - 2 - index
		weight := 3
		if positionFromRight%2 == 1 {
			weight = 1
		}
		sum += digit * weight
	}
	checkDigit := (10 - sum%10) % 10
	return int(value[len(value)-1]-'0') == checkDigit
}
