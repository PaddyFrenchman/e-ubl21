package queue_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestLayeredValidationModelRoundTripsAllSemanticFields(t *testing.T) {
	stages := queue.ValidationStages{
		{
			Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true,
			Outcome: queue.StageOutcomePassed, ValidatedAt: "2026-08-25T12:00:00Z", DurationMS: 7,
		},
		{
			Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: true,
			Outcome: queue.StageOutcomePassed, ValidatedAt: "2026-08-25T12:00:01Z", DurationMS: 31,
		},
		{
			Layer: queue.ValidationLayerPeppolBISBilling, Version: "1.3.16",
			BISVersion: "3.0.21", BISReleaseDate: "2026-05-20", BISMandatoryFrom: "2026-08-17",
			CustomizationID: peppolCustomizationID, ProfileID: peppolProfile01ID,
			ProcessProfile: queue.PeppolProcessProfile01,
			Valid:          false, Outcome: queue.StageOutcomeStandardsInvalid,
			ValidatedAt: "2026-08-25T12:00:02Z", DurationMS: 43,
			Issues: []queue.ValidationIssue{{
				Kind:  queue.ValidationIssueFailedAssert,
				Layer: queue.ValidationLayerPeppolBISBilling, Rule: "PEPPOL-EN16931-R001", Severity: "fatal",
				Location: "/ubl:Invoice", Test: "cbc:ProfileID",
				Message: "Business process MUST be provided.", Diagnostic: "Profile 01 mismatch.",
			}},
		},
	}
	record := queue.UploadRecord{
		ID: "0123456789abcdef0123456789abcdef", DocumentType: "Invoice", Status: queue.StatusInvalid,
		ReceivedAt: "2026-08-25T11:59:59Z", ValidatedAt: "2026-08-25T12:00:02Z", ValidationStages: stages,
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("schematron-output")) || bytes.Contains(encoded, []byte("<Invoice")) {
		t.Fatalf("layered metadata contains raw validation content: %s", encoded)
	}
	var decoded queue.UploadRecord
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.ValidationStages, stages) {
		t.Fatalf("validation stages after round trip = %#v, want %#v", decoded.ValidationStages, stages)
	}
	xsd, ok := decoded.ValidationStages.Find(queue.ValidationLayerUBL21XSD)
	if !ok || !xsd.Valid {
		t.Fatalf("XSD stage = %#v, present %t", xsd, ok)
	}
	en16931, ok := decoded.ValidationStages.Find(queue.ValidationLayerEN16931)
	if !ok || !en16931.Valid || en16931.Version != "1.3.16" {
		t.Fatalf("EN 16931 stage = %#v, present %t", en16931, ok)
	}
	peppol, ok := decoded.ValidationStages.Find(queue.ValidationLayerPeppolBISBilling)
	if !ok || peppol.Valid || peppol.BISVersion != "3.0.21" ||
		peppol.ProfileID != peppolProfile01ID || peppol.ProcessProfile != queue.PeppolProcessProfile01 ||
		len(peppol.Issues) != 1 || peppol.Issues[0].Rule != "PEPPOL-EN16931-R001" {
		t.Fatalf("Peppol stage = %#v, present %t", peppol, ok)
	}
}

const (
	peppolCustomizationID = "urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0"
	peppolProfile01ID     = "urn:fdc:peppol.eu:2017:poacc:billing:01:1.0"
	peppolProfile02ID     = "urn:peppol:bis:billing_with_response"
)

func TestPeppolProfileIsDurableButNotYetPublic(t *testing.T) {
	if !queue.ValidValidationProfile(queue.ValidationProfilePeppolBISBilling) {
		t.Fatal("durable validation contracts reject the Peppol profile")
	}
	if queue.ValidPublicValidationProfile(queue.ValidationProfilePeppolBISBilling) {
		t.Fatal("public validation API enabled Peppol before issue #75")
	}
	if queue.ValidValidationProfile(queue.ValidationProfileAuto) ||
		queue.ValidPublicValidationProfile(queue.ValidationProfileAuto) {
		t.Fatal("unresolved auto profile was accepted as a durable or public profile")
	}
}

func TestLegacyValidationPayloadWithoutStagesRemainsCompatible(t *testing.T) {
	var result queue.ValidationResult
	if err := json.Unmarshal([]byte(`{"id":"0123456789abcdef0123456789abcdef","documentType":"Order","status":"valid","validatedAt":"2026-08-25T12:00:00Z"}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusValid || len(result.ValidationStages) != 0 {
		t.Fatalf("legacy validation result = %+v", result)
	}
}

func TestLegacyXSDIssueWithoutKindIsUpgradedOnDecode(t *testing.T) {
	legacy := []byte(`{
  "id":"0123456789abcdef0123456789abcdef",
  "documentType":"Invoice",
  "status":"invalid",
  "validatedAt":"2026-08-25T12:00:00Z",
  "validationStages":[{
    "layer":"ubl21-xsd",
    "version":"2.1",
    "valid":false,
    "validatedAt":"2026-08-25T12:00:00Z",
    "issues":[{
      "layer":"ubl21-xsd",
      "severity":"error",
      "message":"schema validation failed"
    }]
  }]
}`)
	var result queue.UploadRecord
	if err := json.Unmarshal(legacy, &result); err != nil {
		t.Fatal(err)
	}
	issue := result.ValidationStages[0].Issues[0]
	if issue.Kind != queue.ValidationIssueXSDReport {
		t.Fatalf("legacy XSD issue kind = %q, want %q", issue.Kind, queue.ValidationIssueXSDReport)
	}
	reencoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(reencoded, []byte(`"kind":"xsd-report"`)) || bytes.Contains(reencoded, []byte(`"kind":""`)) {
		t.Fatalf("re-encoded legacy result does not satisfy the issue-kind contract: %s", reencoded)
	}
}

func TestMissingEN16931IssueKindIsNotGuessedFromSeverity(t *testing.T) {
	var issue queue.ValidationIssue
	if err := json.Unmarshal([]byte(`{"layer":"en16931","severity":"error","message":"report"}`), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.Kind != "" {
		t.Fatalf("missing EN 16931 issue kind was guessed as %q", issue.Kind)
	}
}
