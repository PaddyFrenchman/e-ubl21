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
		{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: "2026-08-25T12:00:00Z", DurationMS: 7},
		{
			Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: false,
			ValidatedAt: "2026-08-25T12:00:01Z", DurationMS: 31,
			Issues: []queue.ValidationIssue{{
				Kind:  queue.ValidationIssueFailedAssert,
				Layer: queue.ValidationLayerEN16931, Rule: "BR-CO-10", Severity: "error",
				Location: "/ubl:Invoice/cac:LegalMonetaryTotal", Test: "sum(cac:InvoiceLine) = cbc:LineExtensionAmount",
				Message: "Invoice line net amounts do not reconcile.", Diagnostic: "BT-106 mismatch.",
			}},
		},
	}
	record := queue.UploadRecord{
		ID: "0123456789abcdef0123456789abcdef", DocumentType: "Invoice", Status: queue.StatusInvalid,
		ReceivedAt: "2026-08-25T11:59:59Z", ValidatedAt: "2026-08-25T12:00:01Z", ValidationStages: stages,
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
	if !ok || en16931.Valid || en16931.Version != "1.3.16" || en16931.Issues[0].Rule != "BR-CO-10" {
		t.Fatalf("EN 16931 stage = %#v, present %t", en16931, ok)
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
