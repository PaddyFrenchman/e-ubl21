package server

import (
	"strings"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

// validationLayer identifies one standards-validation layer in the mandatory
// server pipeline. These values are stable contract identifiers intended for
// reuse by the layered result model introduced later in the EN 16931 roadmap.
type validationLayer = queue.ValidationLayer

const (
	validationLayerUBL21XSD validationLayer = queue.ValidationLayerUBL21XSD
	validationLayerEN16931  validationLayer = queue.ValidationLayerEN16931
)

// validationStageOutcome distinguishes standards non-compliance from an
// operational inability to complete validation.
type validationStageOutcome string

const (
	validationStagePassed             validationStageOutcome = "passed"
	validationStageStandardsInvalid   validationStageOutcome = "standards-invalid"
	validationStageOperationalFailure validationStageOutcome = "operational-failure"
)

var billingValidationLayers = []validationLayer{
	validationLayerUBL21XSD,
	validationLayerEN16931,
}

var structuralValidationLayers = []validationLayer{
	validationLayerUBL21XSD,
}

// requiresEN16931 reports whether the target validation contract mandates the
// EN 16931 stage for this UBL root. Peppol, national CIUS profiles, CII, and
// every non-billing UBL document are deliberately outside this scope.
func requiresEN16931(documentType string) bool {
	return documentType == "Invoice" || documentType == "CreditNote"
}

// requiredValidationLayers returns mandatory stages in execution order. XSD
// is always first; EN 16931 is mandatory only for Invoice and CreditNote.
func requiredValidationLayers(documentType string) []validationLayer {
	layers := structuralValidationLayers
	if requiresEN16931(documentType) {
		layers = billingValidationLayers
	}
	return append([]validationLayer(nil), layers...)
}

// overallValidationStatus applies the frozen terminal-status contract to the
// completed stage outcomes. A standards-invalid stage short-circuits all later
// layers and yields invalid. A missing, unknown, or operationally failed stage
// means validation could not be completed and therefore yields
// processing_failed. Only every required stage passing yields valid.
func overallValidationStatus(documentType string, outcomes map[validationLayer]validationStageOutcome) queue.ValidationStatus {
	for _, layer := range requiredValidationLayers(documentType) {
		switch outcomes[layer] {
		case validationStagePassed:
			continue
		case validationStageStandardsInvalid:
			return queue.StatusInvalid
		case validationStageOperationalFailure:
			return queue.StatusProcessingFailed
		default:
			return queue.StatusProcessingFailed
		}
	}
	return queue.StatusValid
}

// en16931SeverityInvalid applies the initial SVRL severity policy. Fatal and
// error assertions invalidate the EN 16931 stage. Warnings are retained but
// do not invalidate it. Empty and unknown flags are retained by the future
// parser and conservatively treated as invalid to prevent false positives.
func en16931SeverityInvalid(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "warning", "warn":
		return false
	case "fatal", "error":
		return true
	default:
		return true
	}
}
