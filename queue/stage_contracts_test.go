package queue_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestStageContractV1RoundTripAndDeterministicIDs(t *testing.T) {
	createdAt := time.Date(2026, 8, 28, 18, 0, 0, 123456789, time.UTC)
	execution, err := queue.NewValidationExecution(testInitialIntent(t, strings.Repeat("1", 32), testFrozenPlan(), createdAt.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, 2, 1, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := queue.NewStageRequest(execution, 2, 1, createdAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.RequestID != request.RequestID {
		t.Fatalf("redelivery request IDs = %q/%q", request.RequestID, duplicate.RequestID)
	}
	nextAttempt, err := queue.NewStageRequest(execution, 2, 2, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	if nextAttempt.RequestID == request.RequestID {
		t.Fatal("new logical attempt reused request ID")
	}

	requestJSON, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	wantRequestJSON := fmt.Sprintf(`{"contractVersion":1,"requestId":%q,"validationId":%q,"uploadId":%q,"documentType":"Invoice","plan":"ubl21-en16931","planVersion":"1","generation":1,"stage":"en16931","stageOrdinal":2,"rulesetVersion":"1.3.16","attempt":1,"createdAt":"2026-08-28T18:00:00.123456789Z"}`, request.RequestID, request.ValidationID, request.UploadID)
	if string(requestJSON) != wantRequestJSON {
		t.Fatalf("version 1 request JSON = %s, want %s", requestJSON, wantRequestJSON)
	}
	var decodedRequest queue.StageRequest
	if err := json.Unmarshal(requestJSON, &decodedRequest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedRequest, request) {
		t.Fatalf("request round trip = %+v, want %+v", decodedRequest, request)
	}
	if strings.Contains(string(requestJSON), "Invoice xmlns") {
		t.Fatalf("request contains XML: %s", requestJSON)
	}

	issues := []queue.ValidationIssue{{
		Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerEN16931,
		Rule: "BR-01", Severity: "error", Location: "/Invoice", Test: "cbc:ID",
		Message: "Invoice identifier is required", Diagnostic: "identifier missing",
	}}
	startedAt := createdAt.Add(2 * time.Second)
	completedAt := startedAt.Add(1432 * time.Millisecond)
	result, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, issues, "", startedAt, completedAt, 1432*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	duplicateResult, err := queue.NewStageResult(duplicate, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, issues, "", startedAt.Add(time.Minute), completedAt.Add(time.Minute), 1432*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if duplicateResult.ResultID != result.ResultID || duplicateResult.RequestID != result.RequestID {
		t.Fatalf("redelivery result identity changed: %+v / %+v", result, duplicateResult)
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	wantResultJSON := fmt.Sprintf(`{"contractVersion":1,"resultId":%q,"requestId":%q,"validationId":%q,"uploadId":%q,"documentType":"Invoice","plan":"ubl21-en16931","planVersion":"1","generation":1,"stage":"en16931","stageOrdinal":2,"rulesetVersion":"1.3.16","attempt":1,"outcome":"standards-invalid","issues":[{"kind":"failed-assert","layer":"en16931","rule":"BR-01","severity":"error","location":"/Invoice","test":"cbc:ID","message":"Invoice identifier is required","diagnostic":"identifier missing"}],"startedAt":"2026-08-28T18:00:02.123456789Z","completedAt":"2026-08-28T18:00:03.555456789Z","durationMs":1432}`, result.ResultID, result.RequestID, result.ValidationID, result.UploadID)
	if string(resultJSON) != wantResultJSON {
		t.Fatalf("version 1 result JSON = %s, want %s", resultJSON, wantResultJSON)
	}
	var decodedResult queue.StageResult
	if err := json.Unmarshal(resultJSON, &decodedResult); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedResult, result) {
		t.Fatalf("result round trip = %+v, want %+v", decodedResult, result)
	}
}

func TestStageResultValidationRejectsIdentityDrift(t *testing.T) {
	request := validStageRequest(t)
	result := validOperationalStageResult(t, request, queue.StageReasonTimeout)
	tests := []struct {
		name   string
		mutate func(*queue.StageResult)
	}{
		{"contract version", func(value *queue.StageResult) { value.ContractVersion++ }},
		{"result id", func(value *queue.StageResult) { value.ResultID = strings.Repeat("0", 64) }},
		{"request id", func(value *queue.StageResult) { value.RequestID = strings.Repeat("0", 64) }},
		{"validation id", func(value *queue.StageResult) { value.ValidationID = strings.Repeat("3", 32) }},
		{"plan", func(value *queue.StageResult) { value.Plan = "ubl21" }},
		{"generation", func(value *queue.StageResult) { value.Generation++ }},
		{"stage", func(value *queue.StageResult) { value.Stage = queue.ValidationLayerUBL21XSD }},
		{"ordinal", func(value *queue.StageResult) { value.StageOrdinal++ }},
		{"ruleset", func(value *queue.StageResult) { value.RulesetVersion = "next" }},
		{"attempt", func(value *queue.StageResult) { value.Attempt++ }},
		{"started at", func(value *queue.StageResult) { value.StartedAt = "invalid" }},
		{"completed at", func(value *queue.StageResult) { value.CompletedAt = "invalid" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := result
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("Validate accepted %+v", candidate)
			}
		})
	}
}

func TestStageContractV1RejectsUnknownVersionsAndFields(t *testing.T) {
	request := validStageRequest(t)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	unknownVersion := strings.Replace(string(encoded), `"contractVersion":1`, `"contractVersion":2`, 1)
	var decoded queue.StageRequest
	if err := json.Unmarshal([]byte(unknownVersion), &decoded); !errors.Is(err, queue.ErrUnsupportedStageContractVersion) {
		t.Fatalf("unknown request version error = %v", err)
	}
	missingVersion := strings.Replace(string(encoded), `"contractVersion":1,`, "", 1)
	if err := json.Unmarshal([]byte(missingVersion), &decoded); !errors.Is(err, queue.ErrUnsupportedStageContractVersion) {
		t.Fatalf("missing request version error = %v", err)
	}
	withXML := strings.TrimSuffix(string(encoded), "}") + `,"xml":"<Invoice/>"}`
	if err := json.Unmarshal([]byte(withXML), &decoded); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("request with raw XML error = %v", err)
	}
	if err := json.Unmarshal(append(encoded, []byte(` {}`)...), &decoded); err == nil {
		t.Fatal("multiple request JSON values were accepted")
	}

	result := validOperationalStageResult(t, request, queue.StageReasonTimeout)
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	unknownResultVersion := strings.Replace(string(resultJSON), `"contractVersion":1`, `"contractVersion":9`, 1)
	var decodedResult queue.StageResult
	if err := json.Unmarshal([]byte(unknownResultVersion), &decodedResult); !errors.Is(err, queue.ErrUnsupportedStageContractVersion) {
		t.Fatalf("unknown result version error = %v", err)
	}
	unknownResultField := strings.TrimSuffix(string(resultJSON), "}") + `,"workerState":"internal"}`
	if err := json.Unmarshal([]byte(unknownResultField), &decodedResult); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("result with worker-internal field error = %v", err)
	}

	standardsResult := validStandardsInvalidStageResult(t, request)
	standardsJSON, err := json.Marshal(standardsResult)
	if err != nil {
		t.Fatal(err)
	}
	nestedUnknownField := strings.Replace(string(standardsJSON), `"message":"failed"`, `"message":"failed","workerState":"internal"`, 1)
	if err := json.Unmarshal([]byte(nestedUnknownField), &decodedResult); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("result with nested worker-internal field error = %v", err)
	}
	xsdRequest := validStageRequestAtOrdinal(t, 1)
	startedAt := time.Date(2026, 8, 28, 18, 0, 1, 0, time.UTC)
	xsdIssue := queue.ValidationIssue{Kind: queue.ValidationIssueXSDReport, Layer: xsdRequest.Stage, Severity: "error", Message: "schema failed"}
	xsdResult, err := queue.NewStageResult(xsdRequest, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, []queue.ValidationIssue{xsdIssue}, "", startedAt, startedAt.Add(time.Second), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	xsdJSON, err := json.Marshal(xsdResult)
	if err != nil {
		t.Fatal(err)
	}
	legacyIssueWithoutKind := strings.Replace(string(xsdJSON), `"kind":"xsd-report",`, "", 1)
	if err := json.Unmarshal([]byte(legacyIssueWithoutKind), &decodedResult); err == nil || !strings.Contains(err.Error(), "kind is required") {
		t.Fatalf("XSD result with legacy missing-kind issue error = %v", err)
	}
}

func TestStageRequestValidationRejectsIdentityDrift(t *testing.T) {
	request := validStageRequest(t)
	tests := []struct {
		name   string
		mutate func(*queue.StageRequest)
	}{
		{"contract version", func(value *queue.StageRequest) { value.ContractVersion++ }},
		{"request id", func(value *queue.StageRequest) { value.RequestID = strings.Repeat("0", 64) }},
		{"validation id", func(value *queue.StageRequest) { value.ValidationID = "bad" }},
		{"upload id", func(value *queue.StageRequest) { value.UploadID = "bad" }},
		{"document type", func(value *queue.StageRequest) { value.DocumentType = "Invoice Type" }},
		{"plan", func(value *queue.StageRequest) { value.Plan = "Bad Plan" }},
		{"plan version", func(value *queue.StageRequest) { value.PlanVersion = "" }},
		{"generation", func(value *queue.StageRequest) { value.Generation = 0 }},
		{"stage", func(value *queue.StageRequest) { value.Stage = "Bad Stage" }},
		{"ordinal", func(value *queue.StageRequest) { value.StageOrdinal = 0 }},
		{"ruleset", func(value *queue.StageRequest) { value.RulesetVersion = "" }},
		{"attempt", func(value *queue.StageRequest) { value.Attempt = 0 }},
		{"created at", func(value *queue.StageRequest) { value.CreatedAt = "yesterday" }},
		{"non-canonical time", func(value *queue.StageRequest) { value.CreatedAt = "2026-08-28T19:00:00+01:00" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := request
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("Validate accepted %+v", candidate)
			}
			if _, err := json.Marshal(candidate); err == nil {
				t.Fatalf("Marshal accepted %+v", candidate)
			}
		})
	}
}

func TestStageResultOutcomeAndReasonSemantics(t *testing.T) {
	request := validStageRequest(t)
	operationalReasons := []queue.StageOperationalReason{
		queue.StageReasonTimeout,
		queue.StageReasonCancelled,
		queue.StageReasonContentUnavailable,
		queue.StageReasonRuntimeUnavailable,
		queue.StageReasonRuntimeFailure,
		queue.StageReasonMalformedOutput,
		queue.StageReasonResourceLimit,
		queue.StageReasonInternalError,
	}
	for _, reason := range operationalReasons {
		if result := validOperationalStageResult(t, request, reason); result.Reason != reason {
			t.Fatalf("operational reason = %q, want %q", result.Reason, reason)
		}
	}

	startedAt := time.Date(2026, 8, 28, 18, 0, 1, 0, time.UTC)
	completedAt := startedAt.Add(time.Second)
	if _, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonTimeout, nil, "", startedAt, completedAt, time.Second); err == nil {
		t.Fatal("passed result with operational reason was accepted")
	}
	for _, kind := range []queue.ValidationIssueKind{queue.ValidationIssueFailedAssert, queue.ValidationIssueXSDReport} {
		failure := queue.ValidationIssue{Kind: kind, Layer: request.Stage, Severity: "error", Message: "failed"}
		if _, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, []queue.ValidationIssue{failure}, "", startedAt, completedAt, time.Second); err == nil {
			t.Fatalf("passed result with %q evidence was accepted", kind)
		}
	}
	report := queue.ValidationIssue{Kind: queue.ValidationIssueSuccessfulReport, Layer: request.Stage, Severity: "warning", Message: "advisory"}
	if _, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, []queue.ValidationIssue{report}, "", startedAt, completedAt, time.Second); err != nil {
		t.Fatalf("passed result with successful-report evidence: %v", err)
	}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, nil, "", startedAt, completedAt, time.Second); err == nil {
		t.Fatal("standards-invalid result without evidence was accepted")
	}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeOperationalFailure, "network-ish", nil, "detail", startedAt, completedAt, time.Second); err == nil {
		t.Fatal("unbounded operational reason was accepted")
	}
	issue := queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Layer: request.Stage, Severity: "error", Message: "failed"}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure, []queue.ValidationIssue{issue}, "detail", startedAt, completedAt, time.Second); err == nil {
		t.Fatal("operational failure containing standards issues was accepted")
	}
	if _, err := queue.NewStageResult(request, "maybe", queue.StageReasonNone, nil, "", startedAt, completedAt, time.Second); err == nil {
		t.Fatal("unknown outcome was accepted")
	}
	if _, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", startedAt, completedAt, -time.Millisecond); err == nil {
		t.Fatal("negative duration was accepted")
	}
}

func TestStageResultDiagnosticBounds(t *testing.T) {
	request := validStageRequest(t)
	result := validOperationalStageResult(t, request, queue.StageReasonRuntimeFailure)
	result.Diagnostic = strings.Repeat("x", queue.MaxStageDiagnosticBytes+1)
	if err := result.Validate(); err == nil {
		t.Fatal("oversized top-level diagnostic was accepted")
	}

	startedAt := time.Date(2026, 8, 28, 18, 0, 1, 0, time.UTC)
	issue := queue.ValidationIssue{
		Kind: queue.ValidationIssueFailedAssert, Layer: request.Stage,
		Severity: "error", Message: strings.Repeat("m", queue.MaxStageIssueMessageBytes+1),
	}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, []queue.ValidationIssue{issue}, "", startedAt, startedAt.Add(time.Second), time.Second); err == nil {
		t.Fatal("oversized issue field was accepted")
	}
	issues := make([]queue.ValidationIssue, queue.MaxStageIssues+1)
	for index := range issues {
		issues[index] = queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Layer: request.Stage, Severity: "error", Message: "failed"}
	}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, issues, "", startedAt, startedAt.Add(time.Second), time.Second); err == nil {
		t.Fatal("oversized issue collection was accepted")
	}
	wrongLayer := queue.ValidationIssue{Kind: queue.ValidationIssueFailedAssert, Layer: queue.ValidationLayerUBL21XSD, Severity: "error", Message: "failed"}
	if _, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, []queue.ValidationIssue{wrongLayer}, "", startedAt, startedAt.Add(time.Second), time.Second); err == nil {
		t.Fatal("issue for another stage was accepted")
	}
}

func validStageRequest(t *testing.T) queue.StageRequest {
	return validStageRequestAtOrdinal(t, 2)
}

func validStageRequestAtOrdinal(t *testing.T, ordinal uint32) queue.StageRequest {
	t.Helper()
	createdAt := time.Date(2026, 8, 28, 18, 0, 0, 0, time.UTC)
	execution, err := queue.NewValidationExecution(testInitialIntent(t, strings.Repeat("2", 32), testFrozenPlan(), createdAt.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	request, err := queue.NewStageRequest(execution, ordinal, 1, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func validOperationalStageResult(t *testing.T, request queue.StageRequest, reason queue.StageOperationalReason) queue.StageResult {
	t.Helper()
	startedAt := time.Date(2026, 8, 28, 18, 0, 1, 0, time.UTC)
	result, err := queue.NewStageResult(request, queue.StageOutcomeOperationalFailure, reason, nil, "bounded worker detail", startedAt, startedAt.Add(time.Second), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func validStandardsInvalidStageResult(t *testing.T, request queue.StageRequest) queue.StageResult {
	t.Helper()
	startedAt := time.Date(2026, 8, 28, 18, 0, 1, 0, time.UTC)
	issues := []queue.ValidationIssue{{
		Kind: queue.ValidationIssueFailedAssert, Layer: request.Stage,
		Severity: "error", Message: "failed",
	}}
	result, err := queue.NewStageResult(request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone, issues, "", startedAt, startedAt.Add(time.Second), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
