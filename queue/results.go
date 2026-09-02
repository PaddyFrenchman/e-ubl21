package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/PaddyFrenchman/e-ubl21/observability"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ValidationStatus describes the lifecycle state of an uploaded document.
type ValidationStatus string

// ValidationProfile identifies the standards pipeline selected for one
// validation execution. It is persisted so asynchronous workers cannot
// accidentally apply a different profile later.
type ValidationProfile string

const (
	// ValidationProfileAuto is a requested-profile value only. A durable
	// execution must persist the concrete profile produced by resolution.
	ValidationProfileAuto         ValidationProfile = "auto"
	ValidationProfileUBL21        ValidationProfile = "ubl21"
	ValidationProfileUBL21EN16931 ValidationProfile = "ubl21-en16931"
	// ValidationProfilePeppolBISBilling is available to frozen execution and
	// result contracts before the public HTTP selector is enabled by issue #75.
	ValidationProfilePeppolBISBilling ValidationProfile = "peppol-bis-billing"
)

// ValidValidationProfile reports whether profile is supported by durable
// execution and result contracts.
func ValidValidationProfile(profile ValidationProfile) bool {
	return profile == ValidationProfileUBL21 ||
		profile == ValidationProfileUBL21EN16931 ||
		profile == ValidationProfilePeppolBISBilling
}

// ValidPublicValidationProfile reports whether the current HTTP API may accept
// profile. Peppol remains internal until its API contract is implemented.
func ValidPublicValidationProfile(profile ValidationProfile) bool {
	return profile == ValidationProfileUBL21 || profile == ValidationProfileUBL21EN16931
}

// ValidationLayer is a stable identifier for one independently reportable
// standards-validation stage.
type ValidationLayer string

// PeppolProcessProfile is the stable code for the Billing process profile
// resolved from the document's normalized ProfileID.
type PeppolProcessProfile string

// ValidationIssueKind identifies the standards mechanism that emitted an
// issue. Kind is independent of severity: in particular, an SVRL
// successful-report is not a failed assertion even if it carries an unusual
// severity value.
type ValidationIssueKind string

const (
	ValidationLayerUBL21XSD ValidationLayer = "ubl21-xsd"
	ValidationLayerEN16931  ValidationLayer = "en16931"
	// ValidationLayerPeppolBISBilling reserves the stable stage identity and
	// queue topology for the future Peppol worker. It does not enable the
	// public Peppol validation profile or runtime validator.
	ValidationLayerPeppolBISBilling ValidationLayer = "peppol-bis-billing"

	PeppolProcessProfile01 PeppolProcessProfile = "01"
	PeppolProcessProfile02 PeppolProcessProfile = "02"

	ValidationIssueXSDReport        ValidationIssueKind = "xsd-report"
	ValidationIssueFailedAssert     ValidationIssueKind = "failed-assert"
	ValidationIssueSuccessfulReport ValidationIssueKind = "successful-report"
)

// ValidationIssue is one bounded, structured standards diagnostic. Raw XML
// and raw validator output must never be stored in this model.
type ValidationIssue struct {
	Kind       ValidationIssueKind `json:"kind"`
	Layer      ValidationLayer     `json:"layer"`
	Rule       string              `json:"rule,omitempty"`
	Severity   string              `json:"severity"`
	Location   string              `json:"location,omitempty"`
	Test       string              `json:"test,omitempty"`
	Message    string              `json:"message"`
	Diagnostic string              `json:"diagnostic,omitempty"`
}

// UnmarshalJSON upgrades the brief pre-kind XSD issue format in place. The
// layer makes xsd-report unambiguous. No equivalent inference is attempted for
// EN 16931 because severity cannot distinguish failed-assert from
// successful-report.
func (issue *ValidationIssue) UnmarshalJSON(data []byte) error {
	type wireIssue ValidationIssue
	var decoded wireIssue
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Kind == "" && decoded.Layer == ValidationLayerUBL21XSD {
		decoded.Kind = ValidationIssueXSDReport
	}
	*issue = ValidationIssue(decoded)
	return nil
}

// ValidationStageResult records the outcome of one validation layer. Version
// identifies the exact schema or machine validation artefact that produced the
// result. Peppol fields retain its independent BIS and process identity.
type ValidationStageResult struct {
	Layer            ValidationLayer        `json:"layer"`
	Version          string                 `json:"version,omitempty"`
	BISVersion       string                 `json:"bisVersion,omitempty"`
	BISReleaseDate   string                 `json:"bisReleaseDate,omitempty"`
	BISMandatoryFrom string                 `json:"bisMandatoryFrom,omitempty"`
	CustomizationID  string                 `json:"customizationId,omitempty"`
	ProfileID        string                 `json:"profileId,omitempty"`
	ProcessProfile   PeppolProcessProfile   `json:"processProfile,omitempty"`
	Valid            bool                   `json:"valid"`
	Outcome          StageOutcome           `json:"outcome,omitempty"`
	Reason           StageOperationalReason `json:"reason,omitempty"`
	Issues           []ValidationIssue      `json:"issues,omitempty"`
	Diagnostic       string                 `json:"diagnostic,omitempty"`
	ValidatedAt      string                 `json:"validatedAt"`
	DurationMS       int64                  `json:"durationMs,omitempty"`
}

// ValidationStages is the ordered set of completed validation layers.
type ValidationStages []ValidationStageResult

// Find returns the completed result for layer, if present.
func (stages ValidationStages) Find(layer ValidationLayer) (ValidationStageResult, bool) {
	for _, stage := range stages {
		if stage.Layer == layer {
			return stage, true
		}
	}
	return ValidationStageResult{}, false
}

const (
	// StatusAccepting records durable upload intent while immutable content is
	// being persisted. A stale accepting record is reconciled or failed.
	StatusAccepting ValidationStatus = "accepting"
	// StatusEnqueuePending means metadata and content are durable but work
	// publication has not yet been confirmed. Reconciliation retries it.
	StatusEnqueuePending ValidationStatus = "enqueue_pending"
	// StatusCleanupPending means a failed content write may have left partial
	// Object Store state which reconciliation must reclaim.
	StatusCleanupPending ValidationStatus = "cleanup_pending"
	// StatusPending is the status a document is given the moment it is
	// queued for processing, before the worker has picked it up.
	StatusPending ValidationStatus = "pending"
	// StatusValid means every standards-validation stage required for the
	// document type completed successfully.
	StatusValid ValidationStatus = "valid"
	// StatusInvalid means a required standards-validation stage completed and
	// found the document non-compliant (see ValidationStages for details).
	StatusInvalid ValidationStatus = "invalid"
	// StatusAcceptanceFailed is a terminal ingress failure for which no work
	// will be queued. Any partially written content is reclaimed.
	StatusAcceptanceFailed ValidationStatus = "acceptance_failed"
	// StatusDeadLetterPending is a recoverable worker transition: application
	// attempts are exhausted, but DLQ publication and the canonical terminal
	// status have not both been confirmed yet.
	StatusDeadLetterPending ValidationStatus = "dead_letter_pending"
	// StatusProcessingFailed is a terminal operational failure after the
	// worker exhausted its configured delivery attempts. Operators may replay
	// it from the dead-letter stream.
	StatusProcessingFailed ValidationStatus = "processing_failed"
)

// ValidationResult is the canonical terminal validation payload. ResultID,
// ValidationID, and Generation identify orchestrator-owned execution results;
// omitted execution fields retain decode compatibility with legacy
// upload-scoped worker results during migration.
type ValidationResult struct {
	ID               string                  `json:"id"`
	ResultID         string                  `json:"resultId,omitempty"`
	ValidationID     string                  `json:"validationId,omitempty"`
	Generation       uint64                  `json:"generation,omitempty"`
	Profile          ValidationProfile       `json:"profile,omitempty"`
	Plan             string                  `json:"plan,omitempty"`
	PlanVersion      string                  `json:"planVersion,omitempty"`
	PlanStages       []FrozenValidationStage `json:"planStages,omitempty"`
	DocumentType     string                  `json:"documentType"`
	Status           ValidationStatus        `json:"status"`
	Error            string                  `json:"error,omitempty"`
	ValidatedAt      string                  `json:"validatedAt"`
	ValidationStages ValidationStages        `json:"validationStages,omitempty"`
}

// JSErrCode 10077 is the server's stable "maximum messages per subject
// exceeded" response. nats.go does not currently export a named constant for
// it, so keep the protocol value local and covered by the integration test.
const jsErrCodeStreamMaxMsgsPerSubject jetstream.ErrorCode = 10077

// PublishResult inserts result into this client's output results stream.
// Legacy payloads retain one permanent upload-scoped slot. Execution payloads
// use ResultID for one permanent slot per validation lineage generation and
// read the accepted value back so conflicting duplicates fail closed even
// inside the Nats-Msg-Id deduplication window.
func (c *Client) PublishResult(ctx context.Context, result ValidationResult) error {
	if !ValidID(result.ID) {
		return fmt.Errorf("invalid validation result upload id %q", result.ID)
	}
	resultToken := result.ID
	messageID := result.ID + ":validation-result"
	if result.ResultID != "" || result.ValidationID != "" || result.Generation != 0 {
		if err := result.ValidateExecutionResult(); err != nil {
			return err
		}
		resultToken = result.ResultID
		messageID = result.ResultID
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal validation result for %s: %w", result.ID, err)
	}
	msg := nats.NewMsg(c.Namespace.ResultSubject(resultToken))
	msg.Data = payload
	msg.Header.Set(HeaderUploadID, result.ID)
	msg.Header.Set(HeaderDocumentType, result.DocumentType)
	msg.Header.Set(nats.MsgIdHdr, messageID)
	if result.ValidationID != "" {
		msg.Header.Set(HeaderValidationID, result.ValidationID)
		msg.Header.Set(HeaderGeneration, fmt.Sprintf("%d", result.Generation))
	}
	observability.InjectNATS(ctx, msg.Header)

	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		var apiErr *jetstream.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode != jsErrCodeStreamMaxMsgsPerSubject {
			return fmt.Errorf("publish validation result for %s: %w", result.ID, err)
		}
	}
	if result.ValidationID != "" {
		existing, err := c.GetValidationExecutionResult(ctx, result.ValidationID, result.Generation)
		if err != nil {
			return fmt.Errorf("inspect permanent validation result after publication: %w", err)
		}
		if !SemanticallyEqualValidationResults(existing, result) {
			return fmt.Errorf("conflicting permanent validation result for %s", result.ResultID)
		}
	}
	return nil
}

// HasValidationResult reports whether the permanent first-result slot for an
// upload is already occupied. Post-deployment-barrier reconciliation uses this
// to distinguish an unpublished legacy terminal record from an ambiguous
// publish acknowledgement that must preserve the result already accepted by
// JetStream. It is not an atomic mixed-version migration claim.
func (c *Client) HasValidationResult(ctx context.Context, id string) (bool, error) {
	if !ValidID(id) {
		return false, fmt.Errorf("invalid validation result upload id %q", id)
	}
	stream, err := c.JS.Stream(ctx, c.Namespace.ResultsStreamName())
	if err != nil {
		return false, fmt.Errorf("open validation results stream: %w", err)
	}
	if _, err := stream.GetLastMsgForSubject(ctx, c.Namespace.ResultSubject(id)); err != nil {
		if errors.Is(err, jetstream.ErrMsgNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("inspect validation result for %s: %w", id, err)
	}
	return true, nil
}
