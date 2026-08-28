package queue

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

// StageContractVersion is the only StageRequest/StageResult wire version this
// build understands. A consumer must fail closed when it receives any other
// version; durable messages are never guessed or silently upgraded.
const StageContractVersion uint32 = 1

const (
	MaxStageIssues               = 256
	MaxStageDiagnosticBytes      = 8 << 10
	MaxStageIssueRuleBytes       = 256
	MaxStageIssueSeverityBytes   = 64
	MaxStageIssueLocationBytes   = 2 << 10
	MaxStageIssueTestBytes       = 16 << 10
	MaxStageIssueMessageBytes    = 8 << 10
	MaxStageIssueDiagnosticBytes = 4 << 10
	MaxStageIssueTextBytes       = 256 << 10
	maxStageVersionBytes         = 256
)

// ErrUnsupportedStageContractVersion identifies a fail-closed wire-version
// rejection. Callers may use errors.Is without parsing diagnostic text.
var ErrUnsupportedStageContractVersion = errors.New("unsupported stage contract version")

// StageOutcome is the result of one authorized logical stage attempt.
// Operational failure is retry evidence; it is not standards invalidity and
// does not itself finalize a stage or document.
type StageOutcome string

const (
	StageOutcomePassed             StageOutcome = "passed"
	StageOutcomeStandardsInvalid   StageOutcome = "standards-invalid"
	StageOutcomeOperationalFailure StageOutcome = "operational-failure"
)

// StageOperationalReason is a deliberately bounded, low-cardinality reason
// vocabulary. Human-readable detail belongs in Diagnostic, not this enum.
type StageOperationalReason string

const (
	StageReasonNone               StageOperationalReason = ""
	StageReasonTimeout            StageOperationalReason = "timeout"
	StageReasonCancelled          StageOperationalReason = "cancelled"
	StageReasonContentUnavailable StageOperationalReason = "content-unavailable"
	StageReasonRuntimeUnavailable StageOperationalReason = "runtime-unavailable"
	StageReasonRuntimeFailure     StageOperationalReason = "runtime-failure"
	StageReasonMalformedOutput    StageOperationalReason = "malformed-output"
	StageReasonResourceLimit      StageOperationalReason = "resource-limit"
	StageReasonInternalError      StageOperationalReason = "internal-error"
)

// StageRequest is the complete, immutable instruction for one authorized
// logical stage attempt. It contains references only and can never carry XML.
type StageRequest struct {
	ContractVersion uint32          `json:"contractVersion"`
	RequestID       string          `json:"requestId"`
	ValidationID    string          `json:"validationId"`
	UploadID        string          `json:"uploadId"`
	DocumentType    string          `json:"documentType"`
	Plan            string          `json:"plan"`
	PlanVersion     string          `json:"planVersion"`
	Generation      uint64          `json:"generation"`
	Stage           ValidationLayer `json:"stage"`
	StageOrdinal    uint32          `json:"stageOrdinal"`
	RulesetVersion  string          `json:"rulesetVersion"`
	Attempt         uint32          `json:"attempt"`
	CreatedAt       string          `json:"createdAt"`
}

// StageResult is immutable evidence for exactly one StageRequest attempt.
// RequestID and ResultID are deterministic and remain identical across broker
// redelivery or recomputation after a worker crash.
type StageResult struct {
	ContractVersion uint32                 `json:"contractVersion"`
	ResultID        string                 `json:"resultId"`
	RequestID       string                 `json:"requestId"`
	ValidationID    string                 `json:"validationId"`
	UploadID        string                 `json:"uploadId"`
	DocumentType    string                 `json:"documentType"`
	Plan            string                 `json:"plan"`
	PlanVersion     string                 `json:"planVersion"`
	Generation      uint64                 `json:"generation"`
	Stage           ValidationLayer        `json:"stage"`
	StageOrdinal    uint32                 `json:"stageOrdinal"`
	RulesetVersion  string                 `json:"rulesetVersion"`
	Attempt         uint32                 `json:"attempt"`
	Outcome         StageOutcome           `json:"outcome"`
	Reason          StageOperationalReason `json:"reason,omitempty"`
	Issues          []ValidationIssue      `json:"issues,omitempty"`
	Diagnostic      string                 `json:"diagnostic,omitempty"`
	StartedAt       string                 `json:"startedAt"`
	CompletedAt     string                 `json:"completedAt"`
	DurationMS      int64                  `json:"durationMs"`
}

// NewStageRequest derives a request from one frozen execution generation.
// stageOrdinal is one-based and attempt starts at one. Only the orchestrator
// is authorized to call this when it dispatches an attempt.
func NewStageRequest(execution ValidationExecution, stageOrdinal, attempt uint32, createdAt time.Time) (StageRequest, error) {
	if err := execution.Validate(); err != nil {
		return StageRequest{}, fmt.Errorf("stage request execution: %w", err)
	}
	if stageOrdinal == 0 || int(stageOrdinal) > len(execution.Plan.Stages) {
		return StageRequest{}, fmt.Errorf("stage ordinal %d is outside frozen plan of %d stages", stageOrdinal, len(execution.Plan.Stages))
	}
	stage := execution.Plan.Stages[stageOrdinal-1]
	request := StageRequest{
		ContractVersion: StageContractVersion,
		ValidationID:    execution.ValidationID,
		UploadID:        execution.UploadID,
		DocumentType:    execution.DocumentType,
		Plan:            execution.Plan.ID,
		PlanVersion:     execution.Plan.Version,
		Generation:      execution.Generation,
		Stage:           stage.ID,
		StageOrdinal:    stage.Ordinal,
		RulesetVersion:  stage.RulesetVersion,
		Attempt:         attempt,
		CreatedAt:       canonicalStageTime(createdAt),
	}
	request.RequestID = request.expectedRequestID()
	if err := request.Validate(); err != nil {
		return StageRequest{}, err
	}
	return request, nil
}

// NewStageResult copies the request identity and adds bounded attempt
// evidence. duration is measured independently of wall-clock timestamps.
func NewStageResult(request StageRequest, outcome StageOutcome, reason StageOperationalReason, issues []ValidationIssue, diagnostic string, startedAt, completedAt time.Time, duration time.Duration) (StageResult, error) {
	if err := request.Validate(); err != nil {
		return StageResult{}, fmt.Errorf("stage result request: %w", err)
	}
	if duration < 0 {
		return StageResult{}, errors.New("stage result duration must not be negative")
	}
	result := StageResult{
		ContractVersion: request.ContractVersion,
		RequestID:       request.RequestID,
		ValidationID:    request.ValidationID,
		UploadID:        request.UploadID,
		DocumentType:    request.DocumentType,
		Plan:            request.Plan,
		PlanVersion:     request.PlanVersion,
		Generation:      request.Generation,
		Stage:           request.Stage,
		StageOrdinal:    request.StageOrdinal,
		RulesetVersion:  request.RulesetVersion,
		Attempt:         request.Attempt,
		Outcome:         outcome,
		Reason:          reason,
		Issues:          append([]ValidationIssue(nil), issues...),
		Diagnostic:      diagnostic,
		StartedAt:       canonicalStageTime(startedAt),
		CompletedAt:     canonicalStageTime(completedAt),
		DurationMS:      duration.Milliseconds(),
	}
	result.ResultID = result.expectedResultID()
	if err := result.Validate(); err != nil {
		return StageResult{}, err
	}
	return result, nil
}

// Validate rejects malformed, ambiguous, unbounded, or tampered requests.
func (request StageRequest) Validate() error {
	if err := validateStageContractVersion("request", request.ContractVersion); err != nil {
		return err
	}
	if err := validateStageIdentity(request.ValidationID, request.UploadID, request.DocumentType, request.Plan, request.PlanVersion, request.Generation, request.Stage, request.StageOrdinal, request.RulesetVersion, request.Attempt); err != nil {
		return fmt.Errorf("stage request: %w", err)
	}
	if _, err := parseCanonicalStageTime("createdAt", request.CreatedAt); err != nil {
		return fmt.Errorf("stage request: %w", err)
	}
	if want := request.expectedRequestID(); request.RequestID != want {
		return fmt.Errorf("stage request id %q does not match deterministic id %q", request.RequestID, want)
	}
	return nil
}

// Validate rejects malformed, ambiguous, unbounded, or tampered result
// evidence before an orchestrator may persist it.
func (result StageResult) Validate() error {
	if err := validateStageContractVersion("result", result.ContractVersion); err != nil {
		return err
	}
	if err := validateStageIdentity(result.ValidationID, result.UploadID, result.DocumentType, result.Plan, result.PlanVersion, result.Generation, result.Stage, result.StageOrdinal, result.RulesetVersion, result.Attempt); err != nil {
		return fmt.Errorf("stage result: %w", err)
	}
	if want := stageRequestID(result.ContractVersion, result.ValidationID, result.UploadID, result.DocumentType, result.Plan, result.PlanVersion, result.Generation, result.Stage, result.StageOrdinal, result.RulesetVersion, result.Attempt); result.RequestID != want {
		return fmt.Errorf("stage result request id %q does not match deterministic id %q", result.RequestID, want)
	}
	if want := result.expectedResultID(); result.ResultID != want {
		return fmt.Errorf("stage result id %q does not match deterministic id %q", result.ResultID, want)
	}
	if _, err := parseCanonicalStageTime("startedAt", result.StartedAt); err != nil {
		return fmt.Errorf("stage result: %w", err)
	}
	if _, err := parseCanonicalStageTime("completedAt", result.CompletedAt); err != nil {
		return fmt.Errorf("stage result: %w", err)
	}
	if result.DurationMS < 0 {
		return errors.New("stage result durationMs must not be negative")
	}
	if !utf8.ValidString(result.Diagnostic) || len(result.Diagnostic) > MaxStageDiagnosticBytes {
		return fmt.Errorf("stage result diagnostic exceeds %d bytes or is not UTF-8", MaxStageDiagnosticBytes)
	}
	if err := validateStageIssues(result.Stage, result.Issues); err != nil {
		return err
	}
	switch result.Outcome {
	case StageOutcomePassed:
		if result.Reason != StageReasonNone {
			return errors.New("passed stage result cannot have an operational reason")
		}
		if hasStandardsFailure(result.Issues) {
			return errors.New("passed stage result cannot contain standards-failure evidence")
		}
	case StageOutcomeStandardsInvalid:
		if result.Reason != StageReasonNone {
			return errors.New("standards-invalid stage result cannot have an operational reason")
		}
		if !hasStandardsFailure(result.Issues) && strings.TrimSpace(result.Diagnostic) == "" {
			return errors.New("standards-invalid stage result requires bounded standards evidence")
		}
	case StageOutcomeOperationalFailure:
		if !validStageOperationalReason(result.Reason) {
			return fmt.Errorf("operational-failure stage result has invalid reason %q", result.Reason)
		}
		if len(result.Issues) != 0 {
			return errors.New("operational-failure stage result cannot contain standards issues")
		}
	default:
		return fmt.Errorf("invalid stage outcome %q", result.Outcome)
	}
	return nil
}

// MarshalJSON prevents invalid in-memory messages from entering a stream.
func (request StageRequest) MarshalJSON() ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	type wire StageRequest
	return json.Marshal(wire(request))
}

// UnmarshalJSON performs strict schema decoding and fail-closed version
// validation. New fields require a new contract version.
func (request *StageRequest) UnmarshalJSON(data []byte) error {
	if request == nil {
		return errors.New("decode stage request into nil receiver")
	}
	if err := preflightStageContractVersion("request", data); err != nil {
		return err
	}
	type wire StageRequest
	var decoded wire
	if err := strictStageJSON(data, &decoded); err != nil {
		return fmt.Errorf("decode stage request version %d: %w", StageContractVersion, err)
	}
	candidate := StageRequest(decoded)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*request = candidate
	return nil
}

// MarshalJSON prevents invalid in-memory evidence from entering a stream.
func (result StageResult) MarshalJSON() ([]byte, error) {
	if err := result.Validate(); err != nil {
		return nil, err
	}
	type wire StageResult
	return json.Marshal(wire(result))
}

// UnmarshalJSON performs strict schema decoding and fail-closed version
// validation. New fields require a new contract version.
func (result *StageResult) UnmarshalJSON(data []byte) error {
	if result == nil {
		return errors.New("decode stage result into nil receiver")
	}
	if err := preflightStageContractVersion("result", data); err != nil {
		return err
	}
	if err := preflightStageResultIssues(data); err != nil {
		return err
	}
	type wire StageResult
	var decoded wire
	if err := strictStageJSON(data, &decoded); err != nil {
		return fmt.Errorf("decode stage result version %d: %w", StageContractVersion, err)
	}
	candidate := StageResult(decoded)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*result = candidate
	return nil
}

// preflightStageResultIssues prevents the version 1 stage contract from
// inheriting ValidationIssue's legacy persistence/HTTP compatibility decoder.
// In particular, kind is mandatory and nested unknown fields fail closed.
func preflightStageResultIssues(data []byte) error {
	var envelope struct {
		Issues []json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode stage result issue envelope: %w", err)
	}
	type stageIssueWire ValidationIssue
	for index, raw := range envelope.Issues {
		var issue stageIssueWire
		if err := strictStageJSON(raw, &issue); err != nil {
			return fmt.Errorf("decode stage result issue %d: %w", index, err)
		}
		if issue.Kind == "" {
			return fmt.Errorf("decode stage result issue %d: kind is required by contract version %d", index, StageContractVersion)
		}
	}
	return nil
}

func validateStageContractVersion(kind string, version uint32) error {
	if version != StageContractVersion {
		return fmt.Errorf("%w: %s version %d", ErrUnsupportedStageContractVersion, kind, version)
	}
	return nil
}

func preflightStageContractVersion(kind string, data []byte) error {
	var envelope struct {
		ContractVersion uint32 `json:"contractVersion"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return fmt.Errorf("decode stage %s envelope: %w", kind, err)
	}
	return validateStageContractVersion(kind, envelope.ContractVersion)
}

func strictStageJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateStageIdentity(validationID, uploadID, documentType, plan, planVersion string, generation uint64, stage ValidationLayer, ordinal uint32, rulesetVersion string, attempt uint32) error {
	if !ValidID(validationID) {
		return fmt.Errorf("invalid validation id %q", validationID)
	}
	if !ValidID(uploadID) {
		return fmt.Errorf("invalid upload id %q", uploadID)
	}
	if !validationDocumentTypePattern.MatchString(documentType) {
		return fmt.Errorf("invalid document type %q", documentType)
	}
	if !validationPlanTokenPattern.MatchString(plan) {
		return fmt.Errorf("invalid plan %q", plan)
	}
	if err := validateStageVersion("plan version", planVersion); err != nil {
		return err
	}
	if generation == 0 {
		return errors.New("generation must be at least 1")
	}
	if !validationPlanTokenPattern.MatchString(string(stage)) {
		return fmt.Errorf("invalid stage %q", stage)
	}
	if ordinal == 0 {
		return errors.New("stage ordinal must be at least 1")
	}
	if err := validateStageVersion("ruleset version", rulesetVersion); err != nil {
		return err
	}
	if attempt == 0 {
		return errors.New("stage attempt must be at least 1")
	}
	return nil
}

func validateStageVersion(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if !utf8.ValidString(value) || len(value) > maxStageVersionBytes {
		return fmt.Errorf("%s exceeds %d bytes or is not UTF-8", name, maxStageVersionBytes)
	}
	return nil
}

func validateStageIssues(stage ValidationLayer, issues []ValidationIssue) error {
	if len(issues) > MaxStageIssues {
		return fmt.Errorf("stage result has %d issues, maximum is %d", len(issues), MaxStageIssues)
	}
	total := 0
	for index, issue := range issues {
		if issue.Layer != stage {
			return fmt.Errorf("stage result issue %d layer %q does not match stage %q", index, issue.Layer, stage)
		}
		switch issue.Kind {
		case ValidationIssueXSDReport, ValidationIssueFailedAssert, ValidationIssueSuccessfulReport:
		default:
			return fmt.Errorf("stage result issue %d has invalid kind %q", index, issue.Kind)
		}
		fields := []struct {
			name     string
			value    string
			limit    int
			required bool
		}{
			{"rule", issue.Rule, MaxStageIssueRuleBytes, false},
			{"severity", issue.Severity, MaxStageIssueSeverityBytes, true},
			{"location", issue.Location, MaxStageIssueLocationBytes, false},
			{"test", issue.Test, MaxStageIssueTestBytes, false},
			{"message", issue.Message, MaxStageIssueMessageBytes, true},
			{"diagnostic", issue.Diagnostic, MaxStageIssueDiagnosticBytes, false},
		}
		for _, field := range fields {
			if field.required && strings.TrimSpace(field.value) == "" {
				return fmt.Errorf("stage result issue %d %s is required", index, field.name)
			}
			if !utf8.ValidString(field.value) || len(field.value) > field.limit {
				return fmt.Errorf("stage result issue %d %s exceeds %d bytes or is not UTF-8", index, field.name, field.limit)
			}
			total += len(field.value)
		}
		if total > MaxStageIssueTextBytes {
			return fmt.Errorf("stage result issue text exceeds %d bytes", MaxStageIssueTextBytes)
		}
	}
	return nil
}

func hasStandardsFailure(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Kind == ValidationIssueXSDReport || issue.Kind == ValidationIssueFailedAssert {
			return true
		}
	}
	return false
}

func validStageOperationalReason(reason StageOperationalReason) bool {
	switch reason {
	case StageReasonTimeout, StageReasonCancelled, StageReasonContentUnavailable,
		StageReasonRuntimeUnavailable, StageReasonRuntimeFailure,
		StageReasonMalformedOutput, StageReasonResourceLimit, StageReasonInternalError:
		return true
	default:
		return false
	}
}

func canonicalStageTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseCanonicalStageTime(name, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.IsZero() {
		return time.Time{}, fmt.Errorf("%s %q is not a non-zero RFC3339 timestamp", name, value)
	}
	if value != parsed.UTC().Format(time.RFC3339Nano) {
		return time.Time{}, fmt.Errorf("%s %q is not canonical UTC", name, value)
	}
	return parsed, nil
}

func (request StageRequest) expectedRequestID() string {
	return stageRequestID(request.ContractVersion, request.ValidationID, request.UploadID, request.DocumentType, request.Plan, request.PlanVersion, request.Generation, request.Stage, request.StageOrdinal, request.RulesetVersion, request.Attempt)
}

func (result StageResult) expectedResultID() string {
	return stageMessageID("e-ubl21/stage-result", result.RequestID)
}

func stageRequestID(version uint32, validationID, uploadID, documentType, plan, planVersion string, generation uint64, stage ValidationLayer, ordinal uint32, rulesetVersion string, attempt uint32) string {
	return stageMessageID("e-ubl21/stage-request",
		fmt.Sprint(version), validationID, uploadID, documentType, plan, planVersion,
		fmt.Sprint(generation), string(stage), fmt.Sprint(ordinal), rulesetVersion, fmt.Sprint(attempt))
}

// stageMessageID length-prefixes every component, avoiding delimiter
// ambiguity, and emits the complete lowercase SHA-256 digest for Nats-Msg-Id.
func stageMessageID(domain string, parts ...string) string {
	hash := sha256.New()
	writeStageMessageIDPart(hash, domain)
	for _, part := range parts {
		writeStageMessageIDPart(hash, part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type stageMessageIDWriter interface {
	Write([]byte) (int, error)
}

func writeStageMessageIDPart(writer stageMessageIDWriter, value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.Write(size[:])
	_, _ = writer.Write([]byte(value))
}
