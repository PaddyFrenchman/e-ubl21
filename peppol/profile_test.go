package peppol_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestResolveAutoRecognizesProfile01AndProfile02(t *testing.T) {
	tests := []struct {
		name           string
		documentType   string
		profileID      string
		processProfile queue.PeppolProcessProfile
	}{
		{"Invoice Profile 01", "Invoice", peppol.ProfileID01, queue.PeppolProcessProfile01},
		{"CreditNote Profile 02", "CreditNote", peppol.ProfileID02, queue.PeppolProcessProfile02},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := billingDocument(test.documentType,
				" \n "+peppol.CustomizationIDBase+"#extension\t ",
				"\r"+test.profileID+"\n")
			resolution, err := peppol.Resolve(test.documentType, queue.ValidationProfileAuto, document)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.RequestedProfile != queue.ValidationProfileAuto ||
				resolution.ResolvedProfile != queue.ValidationProfilePeppolBISBilling ||
				resolution.Identifiers.CustomizationID != peppol.CustomizationIDBase+"#extension" ||
				resolution.Identifiers.ProfileID != test.profileID ||
				resolution.Identifiers.ProcessProfile != test.processProfile ||
				!resolution.PeppolIdentifiersValid() {
				t.Fatalf("resolution = %+v", resolution)
			}
		})
	}
}

func TestResolveAutoFallsBackWithoutPositiveCaseSensitiveMatch(t *testing.T) {
	tests := []struct {
		name            string
		customizationID string
		profileID       string
	}{
		{"missing customization", "", peppol.ProfileID01},
		{"wrong customization", "urn:cen.eu:en16931:2017", peppol.ProfileID01},
		{"wrong customization case", strings.ToUpper(peppol.CustomizationIDBase), peppol.ProfileID01},
		{"missing profile", peppol.CustomizationIDBase, ""},
		{"unsupported profile", peppol.CustomizationIDBase, "urn:peppol:unsupported"},
		{"wrong profile case", peppol.CustomizationIDBase, strings.ToUpper(peppol.ProfileID01)},
		{"Unicode whitespace is not substituted", "\u00a0" + peppol.CustomizationIDBase, peppol.ProfileID01},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution, err := peppol.Resolve("Invoice", queue.ValidationProfileAuto,
				billingDocument("Invoice", test.customizationID, test.profileID))
			if err != nil {
				t.Fatal(err)
			}
			if resolution.ResolvedProfile != queue.ValidationProfileUBL21EN16931 ||
				resolution.PeppolIdentifiersValid() || len(resolution.Failures) != 0 {
				t.Fatalf("auto fallback resolution = %+v", resolution)
			}
		})
	}
}

func TestResolveExplicitPeppolNeverFallsBack(t *testing.T) {
	tests := []struct {
		name      string
		document  []byte
		wantCodes []peppol.ProfileFailureCode
	}{
		{
			name:     "missing identifiers",
			document: billingDocument("Invoice", "", ""),
			wantCodes: []peppol.ProfileFailureCode{
				peppol.FailureCustomizationIDMissing,
				peppol.FailureProfileIDMissing,
			},
		},
		{
			name:     "unsupported identifiers",
			document: billingDocument("Invoice", "urn:unsupported", "urn:peppol:unsupported"),
			wantCodes: []peppol.ProfileFailureCode{
				peppol.FailureCustomizationIDUnsupported,
				peppol.FailureProfileIDUnsupported,
			},
		},
		{
			name:      "invalid customization format",
			document:  billingDocument("Invoice", peppol.CustomizationIDBase+"::invalid", peppol.ProfileID01),
			wantCodes: []peppol.ProfileFailureCode{peppol.FailureCustomizationIDUnsupported},
		},
		{
			name: "structured customization identifier",
			document: []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
			  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
			  <cbc:CustomizationID><cbc:ID>` + peppol.CustomizationIDBase + `</cbc:ID></cbc:CustomizationID>
			  <cbc:ProfileID>` + peppol.ProfileID01 + `</cbc:ProfileID>
			</Invoice>`),
			wantCodes: []peppol.ProfileFailureCode{peppol.FailureCustomizationIDInvalid},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution, err := peppol.Resolve("Invoice", queue.ValidationProfilePeppolBISBilling, test.document)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.ResolvedProfile != queue.ValidationProfilePeppolBISBilling ||
				resolution.PeppolIdentifiersValid() {
				t.Fatalf("explicit Peppol resolution downgraded or passed: %+v", resolution)
			}
			got := failureCodes(resolution.Failures)
			if strings.Join(got, ",") != joinFailureCodes(test.wantCodes) {
				t.Fatalf("failure codes = %v, want %v", got, test.wantCodes)
			}
			for _, code := range got {
				if strings.HasPrefix(code, "PEPPOL-") {
					t.Fatalf("resolver manufactured official rule ID %q", code)
				}
			}
		})
	}
}

func TestResolverProductRestrictionsNeverClaimOfficialRuleEvidence(t *testing.T) {
	tests := []struct {
		name      string
		profileID string
		wantCode  peppol.ProfileFailureCode
	}{
		{"French regulated Profile 01", "urn:peppol:france:billing:regulated", peppol.FailureProfileIDUnsupported},
		{"French non-regulated Profile 01", "urn:peppol:france:billing:non-regulated", peppol.FailureProfileIDUnsupported},
		{"missing ProfileID", "", peppol.FailureProfileIDMissing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution, err := peppol.Resolve("Invoice", queue.ValidationProfilePeppolBISBilling,
				billingDocument("Invoice", peppol.CustomizationIDBase, test.profileID))
			if err != nil {
				t.Fatal(err)
			}
			if len(resolution.Failures) != 1 || resolution.Failures[0].Code != test.wantCode {
				t.Fatalf("product applicability failures = %+v, want %q", resolution.Failures, test.wantCode)
			}
			if strings.HasPrefix(string(resolution.Failures[0].Code), "PEPPOL-") {
				t.Fatalf("resolver manufactured official rule ID %q", resolution.Failures[0].Code)
			}
		})
	}
}

func TestResolveAutoSelectsPeppolThenRetainsIdentifierFailure(t *testing.T) {
	resolution, err := peppol.Resolve("Invoice", queue.ValidationProfileAuto,
		billingDocument("Invoice", peppol.CustomizationIDBase+"::invalid", peppol.ProfileID01))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.ResolvedProfile != queue.ValidationProfilePeppolBISBilling ||
		strings.Join(failureCodes(resolution.Failures), ",") != string(peppol.FailureCustomizationIDUnsupported) {
		t.Fatalf("auto resolution did not preserve fail-closed Peppol check: %+v", resolution)
	}
}

func TestResolveExplicitUBLAndENProfilesIgnorePeppolIdentifiers(t *testing.T) {
	document := billingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01)
	for _, requested := range []queue.ValidationProfile{
		queue.ValidationProfileUBL21,
		queue.ValidationProfileUBL21EN16931,
	} {
		resolution, err := peppol.Resolve("Invoice", requested, document)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.ResolvedProfile != requested || resolution.Identifiers != (peppol.Identifiers{}) ||
			len(resolution.Failures) != 0 {
			t.Fatalf("explicit profile %q acquired Peppol applicability: %+v", requested, resolution)
		}
	}
}

func TestResolveOmittedProfileUsesAuto(t *testing.T) {
	resolution, err := peppol.Resolve("Invoice", "",
		billingDocument("Invoice", peppol.CustomizationIDBase, peppol.ProfileID01))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.RequestedProfile != queue.ValidationProfileAuto ||
		resolution.ResolvedProfile != queue.ValidationProfilePeppolBISBilling {
		t.Fatalf("omitted profile resolution = %+v", resolution)
	}
}

func TestResolveOnlyUsesUnambiguousTopLevelCBCIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		document string
	}{
		{
			"nested spoof",
			`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
			  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
			  xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2">
			  <cac:AdditionalDocumentReference>
			    <cbc:CustomizationID>` + peppol.CustomizationIDBase + `</cbc:CustomizationID>
			    <cbc:ProfileID>` + peppol.ProfileID01 + `</cbc:ProfileID>
			  </cac:AdditionalDocumentReference>
			</Invoice>`,
		},
		{
			"wrong namespace",
			`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
			  xmlns:wrong="urn:wrong">
			  <wrong:CustomizationID>` + peppol.CustomizationIDBase + `</wrong:CustomizationID>
			  <wrong:ProfileID>` + peppol.ProfileID01 + `</wrong:ProfileID>
			</Invoice>`,
		},
		{
			"duplicate identifiers",
			`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
			  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
			  <cbc:CustomizationID>` + peppol.CustomizationIDBase + `</cbc:CustomizationID>
			  <cbc:CustomizationID>` + peppol.CustomizationIDBase + `</cbc:CustomizationID>
			  <cbc:ProfileID>` + peppol.ProfileID01 + `</cbc:ProfileID>
			</Invoice>`,
		},
		{
			"structured identifier",
			`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
			  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
			  <cbc:CustomizationID><cbc:ID>` + peppol.CustomizationIDBase + `</cbc:ID></cbc:CustomizationID>
			  <cbc:ProfileID>` + peppol.ProfileID01 + `</cbc:ProfileID>
			</Invoice>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolution, err := peppol.Resolve("Invoice", queue.ValidationProfileAuto, []byte(test.document))
			if err != nil {
				t.Fatal(err)
			}
			if resolution.ResolvedProfile != queue.ValidationProfileUBL21EN16931 {
				t.Fatalf("ambiguous identifiers selected Peppol: %+v", resolution)
			}
		})
	}
}

func TestResolveBoundsIdentifiersAndDocuments(t *testing.T) {
	oversized := strings.Repeat("x", peppol.MaxIdentifierBytes+1)
	resolution, err := peppol.Resolve("Invoice", queue.ValidationProfilePeppolBISBilling,
		billingDocument("Invoice", oversized, oversized))
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Identifiers.CustomizationID != "" || resolution.Identifiers.ProfileID != "" ||
		strings.Join(failureCodes(resolution.Failures), ",") !=
			string(peppol.FailureCustomizationIDInvalid)+","+string(peppol.FailureProfileIDInvalid) {
		t.Fatalf("oversized identifiers were retained or accepted: %+v", resolution)
	}

	_, err = peppol.Resolve("Invoice", queue.ValidationProfileAuto,
		make([]byte, peppol.MaxResolutionDocumentBytes+1))
	if !errors.Is(err, peppol.ErrDocumentTooLarge) {
		t.Fatalf("oversized document error = %v", err)
	}
}

func TestResolveBoundsNormalizedRatherThanRawWhitespace(t *testing.T) {
	padding := strings.Repeat(" ", peppol.MaxIdentifierBytes+1)
	resolution, err := peppol.Resolve("Invoice", queue.ValidationProfileAuto,
		billingDocument("Invoice", padding+peppol.CustomizationIDBase+padding, padding+peppol.ProfileID01+padding))
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.PeppolIdentifiersValid() {
		t.Fatalf("XML whitespace around bounded identifiers prevented resolution: %+v", resolution)
	}
}

func TestResolveRejectsMalformedOrMismatchedXML(t *testing.T) {
	tests := []struct {
		name     string
		document []byte
	}{
		{"malformed", []byte(`<Invoice`)},
		{"wrong root", billingDocument("CreditNote", peppol.CustomizationIDBase, peppol.ProfileID01)},
		{"multiple roots", []byte(`<Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/><Invoice/>`)},
		{"directive", []byte(`<!DOCTYPE Invoice><Invoice xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"/>`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := peppol.Resolve("Invoice", queue.ValidationProfileAuto, test.document)
			if !errors.Is(err, peppol.ErrMalformedDocument) {
				t.Fatalf("malformed document error = %v", err)
			}
		})
	}
}

func TestResolveProfileApplicability(t *testing.T) {
	for _, requested := range []queue.ValidationProfile{"", queue.ValidationProfileAuto, queue.ValidationProfileUBL21} {
		resolution, err := peppol.Resolve("Order", requested, nil)
		if err != nil {
			t.Fatal(err)
		}
		if resolution.ResolvedProfile != queue.ValidationProfileUBL21 {
			t.Fatalf("Order resolution = %+v", resolution)
		}
	}
	for _, requested := range []queue.ValidationProfile{
		queue.ValidationProfileUBL21EN16931,
		queue.ValidationProfilePeppolBISBilling,
	} {
		if _, err := peppol.Resolve("Order", requested, nil); !errors.Is(err, peppol.ErrProfileNotApplicable) {
			t.Fatalf("inapplicable profile %q error = %v", requested, err)
		}
	}
	if _, err := peppol.Resolve("Invoice", "unknown", nil); !errors.Is(err, peppol.ErrUnsupportedProfile) {
		t.Fatalf("unknown profile error = %v", err)
	}
}

func billingDocument(documentType, customizationID, profileID string) []byte {
	namespace := "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
	if documentType == "CreditNote" {
		namespace = "urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2"
	}
	var content strings.Builder
	fmtWrite(&content, `<%s xmlns="%s" xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">`, documentType, namespace)
	if customizationID != "" {
		fmtWrite(&content, "<cbc:CustomizationID>%s</cbc:CustomizationID>", customizationID)
	}
	if profileID != "" {
		fmtWrite(&content, "<cbc:ProfileID>%s</cbc:ProfileID>", profileID)
	}
	fmtWrite(&content, "</%s>", documentType)
	return []byte(content.String())
}

func fmtWrite(builder *strings.Builder, format string, values ...any) {
	if _, err := fmt.Fprintf(builder, format, values...); err != nil {
		panic(err)
	}
}

func failureCodes(failures []peppol.ProfileFailure) []string {
	codes := make([]string, len(failures))
	for index, failure := range failures {
		codes[index] = string(failure.Code)
	}
	return codes
}

func joinFailureCodes(codes []peppol.ProfileFailureCode) string {
	values := make([]string, len(codes))
	for index, code := range codes {
		values[index] = string(code)
	}
	return strings.Join(values, ",")
}
