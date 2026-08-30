package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"
	"unicode/utf8"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const StageDeadLetterContractVersion uint32 = 1

// StageDeadLetter is the permanent operator-facing record for one exhausted
// logical stage attempt. It identifies the exact immutable StageResult that
// caused exhaustion; it never represents standards-invalid evidence.
type StageDeadLetter struct {
	ContractVersion uint32                 `json:"contractVersion"`
	DeadLetterID    string                 `json:"deadLetterId"`
	RequestID       string                 `json:"requestId"`
	ResultID        string                 `json:"resultId"`
	ValidationID    string                 `json:"validationId"`
	UploadID        string                 `json:"uploadId"`
	DocumentType    string                 `json:"documentType"`
	Generation      uint64                 `json:"generation"`
	Plan            string                 `json:"plan"`
	PlanVersion     string                 `json:"planVersion"`
	Stage           ValidationLayer        `json:"stage"`
	StageOrdinal    uint32                 `json:"stageOrdinal"`
	RulesetVersion  string                 `json:"rulesetVersion"`
	Attempt         uint32                 `json:"attempt"`
	Deliveries      uint64                 `json:"deliveries"`
	Reason          StageOperationalReason `json:"reason"`
	Diagnostic      string                 `json:"diagnostic,omitempty"`
	FailedAt        string                 `json:"failedAt"`
}

func NewStageDeadLetter(result StageResult, deliveries uint64, failedAt time.Time) (StageDeadLetter, error) {
	if err := result.Validate(); err != nil {
		return StageDeadLetter{}, fmt.Errorf("stage dead letter result: %w", err)
	}
	if result.Outcome != StageOutcomeOperationalFailure {
		return StageDeadLetter{}, errors.New("stage dead letter requires operational-failure evidence")
	}
	dead := StageDeadLetter{
		ContractVersion: StageDeadLetterContractVersion, DeadLetterID: stageDeadLetterID(result.ResultID),
		RequestID: result.RequestID, ResultID: result.ResultID, ValidationID: result.ValidationID,
		UploadID: result.UploadID, DocumentType: result.DocumentType, Generation: result.Generation,
		Plan: result.Plan, PlanVersion: result.PlanVersion, Stage: result.Stage, StageOrdinal: result.StageOrdinal,
		RulesetVersion: result.RulesetVersion, Attempt: result.Attempt, Deliveries: deliveries,
		Reason: result.Reason, Diagnostic: result.Diagnostic, FailedAt: canonicalStageTime(failedAt),
	}
	if err := dead.Validate(); err != nil {
		return StageDeadLetter{}, err
	}
	return dead, nil
}

// StageDeadLetterIDForRequest derives the permanent exhausted-attempt slot
// before a worker reruns a redelivered request.
func StageDeadLetterIDForRequest(request StageRequest) (string, error) {
	if err := request.Validate(); err != nil {
		return "", err
	}
	return stageDeadLetterID(stageMessageID("e-ubl21/stage-result", request.RequestID)), nil
}

func stageDeadLetterID(resultID string) string {
	digest := sha256.Sum256([]byte("e-ubl21.stage-dead-letter.v1\x00" + resultID))
	return hex.EncodeToString(digest[:16])
}

// StageResultForRequest reconstructs the durable operational evidence frozen
// by this dead letter. Timing is normalized to FailedAt because semantic
// duplicate comparison deliberately excludes per-execution timing.
func (dead StageDeadLetter) StageResultForRequest(request StageRequest) (StageResult, error) {
	if err := dead.Validate(); err != nil {
		return StageResult{}, err
	}
	if err := request.Validate(); err != nil {
		return StageResult{}, err
	}
	deadLetterID, err := StageDeadLetterIDForRequest(request)
	if err != nil {
		return StageResult{}, err
	}
	if dead.DeadLetterID != deadLetterID || dead.RequestID != request.RequestID || dead.ValidationID != request.ValidationID ||
		dead.UploadID != request.UploadID || dead.DocumentType != request.DocumentType || dead.Generation != request.Generation ||
		dead.Plan != request.Plan || dead.PlanVersion != request.PlanVersion || dead.Stage != request.Stage ||
		dead.StageOrdinal != request.StageOrdinal || dead.RulesetVersion != request.RulesetVersion || dead.Attempt != request.Attempt {
		return StageResult{}, errors.New("stage dead letter does not match request")
	}
	failedAt, err := time.Parse(time.RFC3339Nano, dead.FailedAt)
	if err != nil {
		return StageResult{}, fmt.Errorf("parse stage dead-letter failure time: %w", err)
	}
	return NewStageResult(request, StageOutcomeOperationalFailure, dead.Reason, nil, dead.Diagnostic, failedAt, failedAt, 0)
}

func (dead StageDeadLetter) Validate() error {
	if dead.ContractVersion != StageDeadLetterContractVersion {
		return fmt.Errorf("stage dead-letter contract version %d is unsupported", dead.ContractVersion)
	}
	if !ValidID(dead.DeadLetterID) || !ValidID(dead.ValidationID) || !ValidID(dead.UploadID) {
		return errors.New("stage dead letter has invalid durable identity")
	}
	if dead.Generation == 0 || dead.StageOrdinal == 0 || dead.Attempt == 0 || dead.Deliveries == 0 {
		return errors.New("stage dead letter has invalid generation, ordinal, attempt, or delivery count")
	}
	if !validStageOperationalReason(dead.Reason) {
		return fmt.Errorf("stage dead letter has invalid operational reason %q", dead.Reason)
	}
	if !utf8.ValidString(dead.Diagnostic) || len(dead.Diagnostic) > MaxStageDiagnosticBytes {
		return fmt.Errorf("stage dead-letter diagnostic exceeds %d bytes or is not UTF-8", MaxStageDiagnosticBytes)
	}
	if _, err := parseCanonicalStageTime("stage dead-letter failedAt", dead.FailedAt); err != nil {
		return err
	}
	request := StageRequest{
		ContractVersion: StageContractVersion, RequestID: dead.RequestID, ValidationID: dead.ValidationID,
		UploadID: dead.UploadID, DocumentType: dead.DocumentType, Plan: dead.Plan, PlanVersion: dead.PlanVersion,
		Generation: dead.Generation, Stage: dead.Stage, StageOrdinal: dead.StageOrdinal,
		RulesetVersion: dead.RulesetVersion, Attempt: dead.Attempt, CreatedAt: dead.FailedAt,
	}
	if err := request.Validate(); err != nil {
		return fmt.Errorf("stage dead-letter request identity: %w", err)
	}
	if want := stageMessageID("e-ubl21/stage-result", dead.RequestID); dead.ResultID != want {
		return errors.New("stage dead-letter result identity is inconsistent")
	}
	if dead.DeadLetterID != stageDeadLetterID(dead.ResultID) {
		return errors.New("stage dead-letter id is inconsistent")
	}
	return nil
}

// StageDeadLetterForRequest returns the permanent exhausted-attempt record, if
// one exists, without making workers depend on JetStream not-found details.
func (c *Client) StageDeadLetterForRequest(ctx context.Context, request StageRequest) (StageDeadLetter, bool, error) {
	deadLetterID, err := StageDeadLetterIDForRequest(request)
	if err != nil {
		return StageDeadLetter{}, false, err
	}
	dead, err := c.GetStageDeadLetter(ctx, request.Stage, deadLetterID)
	if errors.Is(err, jetstream.ErrMsgNotFound) {
		return StageDeadLetter{}, false, nil
	}
	if err != nil {
		return StageDeadLetter{}, false, err
	}
	if _, err := dead.StageResultForRequest(request); err != nil {
		return StageDeadLetter{}, false, err
	}
	return dead, true, nil
}

// PublishStageDeadLetter creates the permanent per-attempt slot. Redelivery
// after an ambiguous publish accepts the first durable record even if the
// broker delivery counter has subsequently increased.
func (c *Client) PublishStageDeadLetter(ctx context.Context, dead StageDeadLetter) error {
	if err := dead.Validate(); err != nil {
		return err
	}
	names, err := c.Namespace.StageQueue(dead.Stage)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(dead)
	if err != nil {
		return fmt.Errorf("marshal stage dead letter %s: %w", dead.DeadLetterID, err)
	}
	msg := nats.NewMsg(names.DeadLetterSubjectPrefix + dead.DeadLetterID)
	msg.Data = payload
	msg.Header.Set(nats.MsgIdHdr, dead.DeadLetterID)
	if _, err := c.JS.PublishMsg(ctx, msg); err != nil {
		var apiErr *jetstream.APIError
		if !errors.As(err, &apiErr) || apiErr.ErrorCode != jsErrCodeStreamMaxMsgsPerSubject {
			return fmt.Errorf("publish stage dead letter %s: %w", dead.DeadLetterID, err)
		}
	}
	existing, err := c.GetStageDeadLetter(ctx, dead.Stage, dead.DeadLetterID)
	if err != nil {
		return err
	}
	left, right := existing, dead
	left.Deliveries, right.Deliveries = 0, 0
	left.FailedAt, right.FailedAt = "", ""
	if !reflect.DeepEqual(left, right) {
		return errors.New("conflicting permanent stage dead letter")
	}
	return nil
}

func (c *Client) GetStageDeadLetter(ctx context.Context, stage ValidationLayer, deadLetterID string) (StageDeadLetter, error) {
	if !ValidID(deadLetterID) {
		return StageDeadLetter{}, errors.New("invalid stage dead-letter id")
	}
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return StageDeadLetter{}, err
	}
	stream, err := c.JS.Stream(ctx, names.DeadLetterStream)
	if err != nil {
		return StageDeadLetter{}, err
	}
	message, err := stream.GetLastMsgForSubject(ctx, names.DeadLetterSubjectPrefix+deadLetterID)
	if err != nil {
		return StageDeadLetter{}, err
	}
	var dead StageDeadLetter
	if err := json.Unmarshal(message.Data, &dead); err != nil {
		return StageDeadLetter{}, fmt.Errorf("decode stage dead letter %s: %w", deadLetterID, err)
	}
	if err := dead.Validate(); err != nil {
		return StageDeadLetter{}, err
	}
	if dead.Stage != stage || dead.DeadLetterID != deadLetterID {
		return StageDeadLetter{}, errors.New("stored stage dead-letter identity does not match subject")
	}
	return dead, nil
}

func (c *Client) StageDeadLetterDepth(ctx context.Context, stage ValidationLayer) (uint64, error) {
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return 0, err
	}
	stream, err := c.JS.Stream(ctx, names.DeadLetterStream)
	if err != nil {
		return 0, err
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.State.Msgs, nil
}

func (c *Client) DeleteStageDeadLetter(ctx context.Context, stage ValidationLayer, deadLetterID string) error {
	names, err := c.Namespace.StageQueue(stage)
	if err != nil {
		return err
	}
	if !ValidID(deadLetterID) {
		return errors.New("invalid stage dead-letter id")
	}
	stream, err := c.JS.Stream(ctx, names.DeadLetterStream)
	if err != nil {
		return err
	}
	return stream.Purge(ctx, jetstream.WithPurgeSubject(names.DeadLetterSubjectPrefix+deadLetterID))
}
