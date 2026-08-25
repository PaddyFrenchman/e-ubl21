package server

import (
	"slices"
	"testing"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestRequiredValidationLayersFreezeSupportedScopeAndOrder(t *testing.T) {
	tests := []struct {
		documentType string
		want         []validationLayer
	}{
		{"Invoice", []validationLayer{validationLayerUBL21XSD, validationLayerEN16931}},
		{"CreditNote", []validationLayer{validationLayerUBL21XSD, validationLayerEN16931}},
		{"Order", []validationLayer{validationLayerUBL21XSD}},
		{"", []validationLayer{validationLayerUBL21XSD}},
	}
	for _, test := range tests {
		t.Run(test.documentType, func(t *testing.T) {
			got := requiredValidationLayers(test.documentType)
			if !slices.Equal(got, test.want) {
				t.Fatalf("requiredValidationLayers(%q) = %v, want %v", test.documentType, got, test.want)
			}
			// Callers cannot mutate the package's frozen ordering.
			got[0] = "changed"
			if requiredValidationLayers(test.documentType)[0] != validationLayerUBL21XSD {
				t.Fatal("required validation layers exposed mutable contract state")
			}
		})
	}
}

func TestOverallValidationStatusContract(t *testing.T) {
	tests := []struct {
		name, documentType string
		outcomes           map[validationLayer]validationStageOutcome
		want               queue.ValidationStatus
	}{
		{"invoice all stages pass", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed, validationLayerEN16931: validationStagePassed}, queue.StatusValid},
		{"credit note all stages pass", "CreditNote", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed, validationLayerEN16931: validationStagePassed}, queue.StatusValid},
		{"other UBL needs XSD only", "Order", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed}, queue.StatusValid},
		{"XSD invalid short circuits EN", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStageStandardsInvalid}, queue.StatusInvalid},
		{"EN invalid", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed, validationLayerEN16931: validationStageStandardsInvalid}, queue.StatusInvalid},
		{"XSD operational failure", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStageOperationalFailure}, queue.StatusProcessingFailed},
		{"EN operational failure", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed, validationLayerEN16931: validationStageOperationalFailure}, queue.StatusProcessingFailed},
		{"missing mandatory EN stage", "Invoice", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: validationStagePassed}, queue.StatusProcessingFailed},
		{"unknown outcome", "Order", map[validationLayer]validationStageOutcome{validationLayerUBL21XSD: "unknown"}, queue.StatusProcessingFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := overallValidationStatus(test.documentType, test.outcomes); got != test.want {
				t.Fatalf("overallValidationStatus(%q, %v) = %q, want %q", test.documentType, test.outcomes, got, test.want)
			}
		})
	}
}

func TestTerminalValidationStagesComplete(t *testing.T) {
	passedXSD := queue.ValidationStageResult{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, ValidatedAt: "2026-08-25T12:00:00Z"}
	failedXSD := queue.ValidationStageResult{Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: false, ValidatedAt: "2026-08-25T12:00:00Z"}
	passedEN := queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: true, ValidatedAt: "2026-08-25T12:00:01Z"}
	failedEN := queue.ValidationStageResult{Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: false, ValidatedAt: "2026-08-25T12:00:01Z"}
	tests := []struct {
		name   string
		record queue.UploadRecord
		want   bool
	}{
		{"complete Invoice valid", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusValid, ValidationStages: queue.ValidationStages{passedXSD, passedEN}}, true},
		{"Invoice missing EN", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusValid, ValidationStages: queue.ValidationStages{passedXSD}}, false},
		{"Invoice XSD invalid short circuit", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusInvalid, ValidationStages: queue.ValidationStages{failedXSD}}, true},
		{"Invoice EN invalid", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusInvalid, ValidationStages: queue.ValidationStages{passedXSD, failedEN}}, true},
		{"invalid despite passing stages", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusInvalid, ValidationStages: queue.ValidationStages{passedXSD, passedEN}}, false},
		{"wrong stage order", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusValid, ValidationStages: queue.ValidationStages{passedEN, passedXSD}}, false},
		{"missing stage metadata", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusValid, ValidationStages: queue.ValidationStages{{Layer: queue.ValidationLayerUBL21XSD, Valid: true}, passedEN}}, false},
		{"stages after short circuit", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusInvalid, ValidationStages: queue.ValidationStages{failedXSD, passedEN}}, false},
		{"Order XSD valid", queue.UploadRecord{DocumentType: "Order", Status: queue.StatusValid, ValidationStages: queue.ValidationStages{passedXSD}}, true},
		{"legacy result without stages", queue.UploadRecord{DocumentType: "Invoice", Status: queue.StatusValid}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := terminalValidationStagesComplete(test.record); got != test.want {
				t.Fatalf("terminalValidationStagesComplete(%+v) = %t, want %t", test.record, got, test.want)
			}
		})
	}
}

func TestEN16931SeverityPolicy(t *testing.T) {
	tests := []struct {
		severity string
		invalid  bool
	}{
		{"fatal", true},
		{"ERROR", true},
		{" warning ", false},
		{"warn", false},
		{"", true},
		{"information", true},
		{"vendor-specific", true},
	}
	for _, test := range tests {
		t.Run(test.severity, func(t *testing.T) {
			if got := en16931SeverityInvalid(test.severity); got != test.invalid {
				t.Fatalf("en16931SeverityInvalid(%q) = %t, want %t", test.severity, got, test.invalid)
			}
		})
	}
}
