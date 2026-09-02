package queue_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestExecutionValidationResultRequiresCompleteFrozenPlanForValid(t *testing.T) {
	result := validExecutionResult(t, 1)
	result.PlanStages = append(result.PlanStages, queue.FrozenValidationStage{
		ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16",
	})
	result.Profile = queue.ValidationProfileUBL21EN16931
	if err := result.ValidateExecutionResult(); err == nil || !strings.Contains(err.Error(), "every frozen-plan stage") {
		t.Fatalf("partial valid result error = %v", err)
	}
}

func TestExecutionValidationResultRejectsProfilePlanMismatch(t *testing.T) {
	result := validExecutionResult(t, 1)
	result.Profile = queue.ValidationProfilePeppolBISBilling
	err := result.ValidateExecutionResult()
	if err == nil || !strings.Contains(err.Error(), "requires stages") {
		t.Fatalf("Peppol result without Peppol stage error = %v, want profile-plan mismatch", err)
	}
}

func TestExecutionValidationResultLookupsFailClosed(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if found, err := client.HasValidationExecutionResult(ctx, "bad", 1); err == nil || found {
		t.Fatalf("invalid result identity = %t, %v", found, err)
	}
	validationID, _ := queue.NewID()
	if _, err := client.GetValidationExecutionResult(ctx, validationID, 1); err == nil {
		t.Fatal("missing execution result was returned")
	}

	for _, mode := range []string{"malformed", "invalid", "identity"} {
		t.Run(mode, func(t *testing.T) {
			client := newTestClient(t)
			ctx := context.Background()
			validationID, _ := queue.NewID()
			resultID, err := queue.ValidationExecutionResultID(validationID, 1)
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("{")
			if mode == "invalid" {
				payload = []byte(`{}`)
			} else if mode == "identity" {
				other := validExecutionResult(t, 1)
				payload, err = json.Marshal(other)
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := client.JS.Publish(ctx, client.Namespace.ResultSubject(resultID), payload); err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetValidationExecutionResult(ctx, validationID, 1); err == nil {
				t.Fatal("corrupt execution result was returned")
			}
		})
	}
}

func TestExecutionValidationResultsArePermanentPerGenerationAndRejectConflicts(t *testing.T) {
	client := newTestClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	first := validExecutionResult(t, 1)
	if found, err := client.HasValidationExecutionResult(ctx, first.ValidationID, first.Generation); err != nil || found {
		t.Fatalf("result existed before publication: %v, %v", found, err)
	}
	if err := client.PublishResult(ctx, first); err != nil {
		t.Fatal(err)
	}
	if found, err := client.HasValidationExecutionResult(ctx, first.ValidationID, first.Generation); err != nil || !found {
		t.Fatalf("published result not found: %v, %v", found, err)
	}
	conflict := first
	conflict.ValidatedAt = "2026-08-29T11:00:01Z"
	if err := client.PublishResult(ctx, conflict); err == nil || !strings.Contains(err.Error(), "conflicting permanent validation result") {
		t.Fatalf("conflicting permanent result error = %v", err)
	}
	stored, err := client.GetValidationExecutionResult(ctx, first.ValidationID, first.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if !queue.SemanticallyEqualValidationResults(stored, first) {
		t.Fatalf("conflict replaced first result: got %+v, want %+v", stored, first)
	}

	second := validExecutionResult(t, 2)
	second.ID = first.ID
	second.ValidationID = first.ValidationID
	second.ResultID, err = queue.ValidationExecutionResultID(second.ValidationID, second.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.PublishResult(ctx, second); err != nil {
		t.Fatal(err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 2 {
		t.Fatalf("two execution generations stored %d results, want 2", info.State.Msgs)
	}
}

func TestExecutionValidationResultRejectsMalformedTerminalEvidence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*queue.ValidationResult)
	}{
		{"upload id", func(r *queue.ValidationResult) { r.ID = "bad" }},
		{"validation id", func(r *queue.ValidationResult) { r.ValidationID = "bad" }},
		{"result id", func(r *queue.ValidationResult) { r.ResultID = "bad" }},
		{"document type", func(r *queue.ValidationResult) { r.DocumentType = "bad type" }},
		{"profile", func(r *queue.ValidationResult) { r.Profile = "bad" }},
		{"plan", func(r *queue.ValidationResult) { r.Plan = "Bad Plan" }},
		{"validated at", func(r *queue.ValidationResult) { r.ValidatedAt = "bad" }},
		{"error bound", func(r *queue.ValidationResult) { r.Error = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1) }},
		{"no stages", func(r *queue.ValidationResult) { r.ValidationStages = nil }},
		{"stage plan mismatch", func(r *queue.ValidationResult) { r.ValidationStages[0].Layer = queue.ValidationLayerEN16931 }},
		{"stage version", func(r *queue.ValidationResult) { r.ValidationStages[0].Version = "" }},
		{"stage time", func(r *queue.ValidationResult) { r.ValidationStages[0].ValidatedAt = "bad" }},
		{"stage duration", func(r *queue.ValidationResult) { r.ValidationStages[0].DurationMS = -1 }},
		{"stage diagnostic", func(r *queue.ValidationResult) {
			r.ValidationStages[0].Diagnostic = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1)
		}},
		{"passed invalid", func(r *queue.ValidationResult) { r.ValidationStages[0].Valid = false }},
		{"passed reason", func(r *queue.ValidationResult) { r.ValidationStages[0].Reason = queue.StageReasonTimeout }},
		{"outcome", func(r *queue.ValidationResult) { r.ValidationStages[0].Outcome = "other" }},
		{"valid error", func(r *queue.ValidationResult) { r.Error = "failure" }},
		{"status", func(r *queue.ValidationResult) { r.Status = queue.StatusPending }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validExecutionResult(t, 1)
			test.mutate(&result)
			if err := result.ValidateExecutionResult(); err == nil {
				t.Fatalf("malformed result was accepted: %+v", result)
			}
		})
	}
}

func TestExecutionValidationResultAcceptsInvalidAndProcessingFailedOutcomes(t *testing.T) {
	invalid := validExecutionResult(t, 1)
	invalid.Status = queue.StatusInvalid
	invalid.ValidationStages[0].Valid = false
	invalid.ValidationStages[0].Outcome = queue.StageOutcomeStandardsInvalid
	invalid.ValidationStages[0].Diagnostic = "schema-invalid"
	if err := invalid.ValidateExecutionResult(); err != nil {
		t.Fatalf("invalid result rejected: %v", err)
	}

	failed := validExecutionResult(t, 1)
	failed.Status = queue.StatusProcessingFailed
	failed.Error = "runtime unavailable"
	failed.ValidationStages[0].Valid = false
	failed.ValidationStages[0].Outcome = queue.StageOutcomeOperationalFailure
	failed.ValidationStages[0].Reason = queue.StageReasonRuntimeUnavailable
	if err := failed.ValidateExecutionResult(); err != nil {
		t.Fatalf("processing-failed result rejected: %v", err)
	}
	failed.ValidationStages[0].Issues = []queue.ValidationIssue{{Layer: queue.ValidationLayerUBL21XSD, Severity: "error", Message: "bad"}}
	if err := failed.ValidateExecutionResult(); err == nil {
		t.Fatal("operational result with standards issues was accepted")
	}
}

func TestExecutionValidationResultRepresentsPeppolFailureIndependently(t *testing.T) {
	result := peppolInvalidExecutionResult(t)
	if err := result.ValidateExecutionResult(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded queue.ValidationResult
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	xsd, xsdOK := decoded.ValidationStages.Find(queue.ValidationLayerUBL21XSD)
	en16931, enOK := decoded.ValidationStages.Find(queue.ValidationLayerEN16931)
	peppol, peppolOK := decoded.ValidationStages.Find(queue.ValidationLayerPeppolBISBilling)
	if !xsdOK || !xsd.Valid || !enOK || !en16931.Valid || !peppolOK || peppol.Valid {
		t.Fatalf("layered Peppol result = %+v", decoded.ValidationStages)
	}
	if peppol.Version != "1.3.16" || peppol.BISVersion != "3.0.21" ||
		peppol.CustomizationID != peppolCustomizationID || peppol.ProfileID != peppolProfile01ID ||
		peppol.ProcessProfile != queue.PeppolProcessProfile01 ||
		len(peppol.Issues) != 1 || peppol.Issues[0].Layer != queue.ValidationLayerPeppolBISBilling ||
		peppol.Issues[0].Rule != "PEPPOL-EN16931-R001" {
		t.Fatalf("Peppol identity or rule evidence was flattened/lost: %+v", peppol)
	}
}

func TestExecutionValidationResultAcceptsPeppolProfile02Identity(t *testing.T) {
	result := peppolInvalidExecutionResult(t)
	result.ValidationStages[2].ProfileID = peppolProfile02ID
	result.ValidationStages[2].ProcessProfile = queue.PeppolProcessProfile02
	if err := result.ValidateExecutionResult(); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionValidationResultRejectsMalformedPeppolIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*queue.ValidationResult)
	}{
		{"missing BIS version", func(result *queue.ValidationResult) {
			result.ValidationStages[2].BISVersion = ""
		}},
		{"non-normalized BIS version", func(result *queue.ValidationResult) {
			result.ValidationStages[2].BISVersion = " 3.0.21"
		}},
		{"invalid release date", func(result *queue.ValidationResult) {
			result.ValidationStages[2].BISReleaseDate = "20 May 2026"
		}},
		{"mandatory before release", func(result *queue.ValidationResult) {
			result.ValidationStages[2].BISMandatoryFrom = "2026-05-19"
		}},
		{"non-normalized profile", func(result *queue.ValidationResult) {
			result.ValidationStages[2].ProfileID = " " + peppolProfile01ID
		}},
		{"invalid process profile", func(result *queue.ValidationResult) {
			result.ValidationStages[2].ProcessProfile = "03"
		}},
		{"process without profile ID", func(result *queue.ValidationResult) {
			result.ValidationStages[2].ProfileID = ""
		}},
		{"Peppol identity on XSD", func(result *queue.ValidationResult) {
			result.ValidationStages[0].BISVersion = "3.0.21"
		}},
		{"passed without resolved profile", func(result *queue.ValidationResult) {
			stage := &result.ValidationStages[2]
			stage.Valid = true
			stage.Outcome = queue.StageOutcomePassed
			stage.Issues = nil
			stage.ProfileID = ""
			result.Status = queue.StatusValid
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := peppolInvalidExecutionResult(t)
			test.mutate(&result)
			if err := result.ValidateExecutionResult(); err == nil {
				t.Fatalf("malformed Peppol identity was accepted: %+v", result.ValidationStages[2])
			}
		})
	}
}

func TestPeppolWarningEvidenceDoesNotInvalidateStage(t *testing.T) {
	warning := queue.ValidationIssue{
		Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerPeppolBISBilling,
		Rule: "PEPPOL-WARN", Severity: "warning", Message: "Advisory warning.",
	}
	if queue.StageIssuesContainStandardsFailure([]queue.ValidationIssue{warning}) {
		t.Fatal("Peppol warning was treated as standards-invalid")
	}
	warning.Severity = "fatal"
	if !queue.StageIssuesContainStandardsFailure([]queue.ValidationIssue{warning}) {
		t.Fatal("Peppol fatal assertion was not treated as standards-invalid")
	}
}

func peppolInvalidExecutionResult(t *testing.T) queue.ValidationResult {
	t.Helper()
	result := validExecutionResult(t, 1)
	result.Profile = queue.ValidationProfilePeppolBISBilling
	result.Plan = "ubl21-en16931-peppol"
	result.DocumentType = "Invoice"
	result.Status = queue.StatusInvalid
	result.PlanStages = []queue.FrozenValidationStage{
		{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
		{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
		{ID: queue.ValidationLayerPeppolBISBilling, Ordinal: 3, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
	}
	result.ValidationStages = queue.ValidationStages{
		{
			Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true,
			Outcome: queue.StageOutcomePassed, ValidatedAt: "2026-08-29T10:59:57Z", DurationMS: 4,
		},
		{
			Layer: queue.ValidationLayerEN16931, Version: "1.3.16", Valid: true,
			Outcome: queue.StageOutcomePassed, ValidatedAt: "2026-08-29T10:59:58Z", DurationMS: 31,
		},
		{
			Layer: queue.ValidationLayerPeppolBISBilling, Version: "1.3.16",
			BISVersion: "3.0.21", BISReleaseDate: "2026-05-20", BISMandatoryFrom: "2026-08-17",
			CustomizationID: peppolCustomizationID, ProfileID: peppolProfile01ID,
			ProcessProfile: queue.PeppolProcessProfile01,
			Valid:          false, Outcome: queue.StageOutcomeStandardsInvalid,
			Issues: []queue.ValidationIssue{{
				Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerPeppolBISBilling,
				Rule: "PEPPOL-EN16931-R001", Severity: "fatal", Location: "/ubl:Invoice",
				Test: "cbc:ProfileID", Message: "Business process MUST be provided.",
				Diagnostic: "Profile 01 mismatch.",
			}},
			ValidatedAt: "2026-08-29T10:59:59Z", DurationMS: 43,
		},
	}
	return result
}

func validExecutionResult(t *testing.T, generation uint64) queue.ValidationResult {
	t.Helper()
	uploadID, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	validationID, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	resultID, err := queue.ValidationExecutionResultID(validationID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return queue.ValidationResult{
		ID: uploadID, ResultID: resultID, ValidationID: validationID, Generation: generation,
		Profile: queue.ValidationProfileUBL21, Plan: "ubl21", PlanVersion: "1",
		PlanStages: []queue.FrozenValidationStage{{
			ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1",
		}},
		DocumentType: "Order", Status: queue.StatusValid, ValidatedAt: "2026-08-29T11:00:00Z",
		ValidationStages: queue.ValidationStages{{
			Layer: queue.ValidationLayerUBL21XSD, Version: "2.1", Valid: true, Outcome: queue.StageOutcomePassed,
			ValidatedAt: "2026-08-29T10:59:59Z", DurationMS: 4,
		}},
	}
}
