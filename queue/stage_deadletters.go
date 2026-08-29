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
	digest := sha256.Sum256([]byte("e-ubl21.stage-dead-letter.v1\x00" + result.ResultID))
	dead := StageDeadLetter{
		ContractVersion: StageDeadLetterContractVersion, DeadLetterID: hex.EncodeToString(digest[:16]),
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
	digest := sha256.Sum256([]byte("e-ubl21.stage-dead-letter.v1\x00" + dead.ResultID))
	if dead.DeadLetterID != hex.EncodeToString(digest[:16]) {
		return errors.New("stage dead-letter id is inconsistent")
	}
	return nil
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
