package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ErrValidationExecutionConflict identifies a lost orchestration CAS race.
var ErrValidationExecutionConflict = errors.New("validation execution CAS conflict")

// UpdateValidationExecutionState replaces only the mutable orchestration
// projection under an exact KV revision. Immutable identity, plan, and retry
// policy fields must agree with the version that was read.
func (c *Client) UpdateValidationExecutionState(ctx context.Context, current VersionedValidationExecution, next ValidationExecution) (uint64, error) {
	if current.Revision == 0 {
		return 0, errors.New("validation execution CAS revision must be non-zero")
	}
	if err := current.Execution.Validate(); err != nil {
		return 0, fmt.Errorf("current validation execution: %w", err)
	}
	if err := next.Validate(); err != nil {
		return 0, fmt.Errorf("next validation execution: %w", err)
	}
	if !sameValidationExecutionIdentity(current.Execution, next) {
		return 0, errors.New("validation execution CAS cannot change immutable identity, frozen plan, or retry policy")
	}
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return 0, err
	}
	key := validationExecutionKey(next.UploadID, next.ValidationID, next.Generation)
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("load validation execution before CAS: %w", err)
	}
	if entry.Revision() != current.Revision {
		return 0, fmt.Errorf("%w: expected revision %d, found %d", ErrValidationExecutionConflict, current.Revision, entry.Revision())
	}
	var stored ValidationExecution
	if err := json.Unmarshal(entry.Value(), &stored); err != nil {
		return 0, fmt.Errorf("decode validation execution before CAS: %w", err)
	}
	if !reflect.DeepEqual(stored, current.Execution) {
		return 0, fmt.Errorf("%w: supplied current record does not match durable revision", ErrValidationExecutionConflict)
	}
	if err := validateOrchestrationTransition(current.Execution, next); err != nil {
		return 0, fmt.Errorf("validation execution CAS transition: %w", err)
	}
	payload, err := json.Marshal(next)
	if err != nil {
		return 0, fmt.Errorf("marshal validation execution state: %w", err)
	}
	revision, err := kv.Update(ctx, key, payload, current.Revision)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			return 0, fmt.Errorf("%w: %v", ErrValidationExecutionConflict, err)
		}
		return 0, fmt.Errorf("update validation execution state: %w", err)
	}
	return revision, nil
}

// validateOrchestrationTransition prevents a writer holding the latest KV
// revision from rewriting accepted history. Each update may only dispatch the
// first stage or accept exactly one result for the currently authorized
// attempt. Terminal executions have no successor.
func validateOrchestrationTransition(current, next ValidationExecution) error {
	currentUpdatedAt, _ := time.Parse(time.RFC3339Nano, current.Orchestration.UpdatedAt)
	nextUpdatedAt, _ := time.Parse(time.RFC3339Nano, next.Orchestration.UpdatedAt)
	if nextUpdatedAt.Before(currentUpdatedAt) {
		return errors.New("orchestration updatedAt cannot move backwards")
	}

	switch current.Orchestration.Status {
	case ExecutionStatusValid, ExecutionStatusInvalid, ExecutionStatusProcessingFailed:
		return errors.New("terminal orchestration state has no successor")
	case ExecutionStatusPending:
		if next.Orchestration.Status != ExecutionStatusStageQueued || len(next.Orchestration.Stages) != 1 || len(next.Orchestration.Stages[0].Attempts) != 0 ||
			next.Orchestration.Stages[0].AuthorizedAttempt != 1 || next.Orchestration.Stages[0].DispatchStatus != StageDispatchPending {
			return errors.New("pending orchestration may only dispatch the first stage attempt")
		}
		return nil
	case ExecutionStatusStageQueued:
		if isDispatchPublicationTransition(current, next) {
			return nil
		}
		return validateQueuedOrchestrationTransition(current, next)
	default:
		return fmt.Errorf("unsupported current orchestration status %q", current.Orchestration.Status)
	}
}

func isDispatchPublicationTransition(current, next ValidationExecution) bool {
	if current.Orchestration.Status != ExecutionStatusStageQueued || next.Orchestration.Status != ExecutionStatusStageQueued ||
		current.Orchestration.CurrentStageOrdinal != next.Orchestration.CurrentStageOrdinal ||
		len(current.Orchestration.Stages) != len(next.Orchestration.Stages) || len(current.Orchestration.Stages) == 0 ||
		current.Orchestration.FinalizedAt != next.Orchestration.FinalizedAt {
		return false
	}
	last := len(current.Orchestration.Stages) - 1
	for index := 0; index < last; index++ {
		if !reflect.DeepEqual(current.Orchestration.Stages[index], next.Orchestration.Stages[index]) {
			return false
		}
	}
	before := current.Orchestration.Stages[last]
	after := next.Orchestration.Stages[last]
	before.DispatchStatus = ""
	before.DispatchedAt = ""
	after.DispatchStatus = ""
	after.DispatchedAt = ""
	return reflect.DeepEqual(before, after) &&
		current.Orchestration.Stages[last].DispatchStatus == StageDispatchPending &&
		current.Orchestration.Stages[last].DispatchedAt == "" &&
		next.Orchestration.Stages[last].DispatchStatus == StageDispatchPublished &&
		next.Orchestration.Stages[last].DispatchedAt != ""
}

func validateQueuedOrchestrationTransition(current, next ValidationExecution) error {
	currentStages := current.Orchestration.Stages
	nextStages := next.Orchestration.Stages
	if len(nextStages) < len(currentStages) || len(nextStages) > len(currentStages)+1 {
		return errors.New("queued orchestration successor must preserve stages and add at most the next plan stage")
	}
	for index := 0; index < len(currentStages)-1; index++ {
		if !reflect.DeepEqual(currentStages[index], nextStages[index]) {
			return fmt.Errorf("completed orchestration stage %d is immutable", index+1)
		}
	}

	stageIndex := len(currentStages) - 1
	currentStage := currentStages[stageIndex]
	nextStage := nextStages[stageIndex]
	if currentStage.Stage != nextStage.Stage || currentStage.Ordinal != nextStage.Ordinal {
		return errors.New("current orchestration stage identity is immutable")
	}
	if len(nextStage.Attempts) != len(currentStage.Attempts)+1 {
		return errors.New("accepted stage attempts must be append-only, one attempt per transition")
	}
	for index := range currentStage.Attempts {
		if currentStage.Attempts[index] != nextStage.Attempts[index] {
			return errors.New("accepted stage attempts must be append-only, one attempt per transition")
		}
	}
	accepted := nextStage.Attempts[len(nextStage.Attempts)-1]
	if accepted.Attempt != currentStage.AuthorizedAttempt || accepted.RequestID != currentStage.RequestID {
		return errors.New("successor did not accept the currently authorized stage attempt")
	}
	if currentStage.DispatchStatus == StageDispatchPending {
		if accepted.DispatchedAt == "" {
			return errors.New("accepted stage result must prove its request was published")
		}
	} else if accepted.DispatchedAt != currentStage.DispatchedAt {
		return errors.New("accepted stage result cannot rewrite published dispatch evidence")
	}

	if len(nextStages) == len(currentStages)+1 {
		if next.Orchestration.Status != ExecutionStatusStageQueued || accepted.Outcome != StageOutcomePassed ||
			nextStage.TerminalOutcome != StageOutcomePassed || nextStage.AuthorizedAttempt != currentStage.AuthorizedAttempt ||
			nextStage.RequestID != currentStage.RequestID || nextStage.RequestCreatedAt != currentStage.RequestCreatedAt {
			return errors.New("only a passing attempt may dispatch the next frozen-plan stage")
		}
		newStage := nextStages[len(nextStages)-1]
		if newStage.AuthorizedAttempt != 1 || len(newStage.Attempts) != 0 || newStage.TerminalOutcome != "" || newStage.DispatchStatus != StageDispatchPending {
			return errors.New("newly dispatched plan stage must begin at attempt 1 without evidence")
		}
		return nil
	}

	switch next.Orchestration.Status {
	case ExecutionStatusStageQueued:
		if accepted.Outcome != StageOutcomeOperationalFailure || accepted.Attempt >= current.RetryPolicy.MaxAttempts ||
			nextStage.TerminalOutcome != "" || nextStage.AuthorizedAttempt != currentStage.AuthorizedAttempt+1 ||
			nextStage.DispatchStatus != StageDispatchPending || nextStage.DispatchedAt != "" {
			return errors.New("queued stage may continue only with an authorized retry after operational failure")
		}
		return nil
	case ExecutionStatusValid:
		if accepted.Outcome != StageOutcomePassed || nextStage.TerminalOutcome != StageOutcomePassed {
			return errors.New("valid execution must succeed from a passing final-stage attempt")
		}
	case ExecutionStatusInvalid:
		if accepted.Outcome != StageOutcomeStandardsInvalid || nextStage.TerminalOutcome != StageOutcomeStandardsInvalid {
			return errors.New("invalid execution must succeed from standards-invalid evidence")
		}
	case ExecutionStatusProcessingFailed:
		if accepted.Outcome != StageOutcomeOperationalFailure || accepted.Attempt < current.RetryPolicy.MaxAttempts || nextStage.TerminalOutcome != StageOutcomeOperationalFailure {
			return errors.New("processing-failed execution requires exhausted operational attempts")
		}
	default:
		return fmt.Errorf("queued orchestration cannot transition to status %q", next.Orchestration.Status)
	}
	if nextStage.AuthorizedAttempt != currentStage.AuthorizedAttempt || nextStage.RequestID != currentStage.RequestID || nextStage.RequestCreatedAt != currentStage.RequestCreatedAt {
		return errors.New("terminal stage transition cannot rewrite its authorized request")
	}
	return nil
}

// CreateValidationStageEvidence persists one complete immutable StageResult.
// Recreating byte-for-byte equivalent evidence is idempotent; conflicting
// evidence for the same deterministic attempt identity is rejected.
func (c *Client) CreateValidationStageEvidence(ctx context.Context, result StageResult) (uint64, error) {
	if err := result.Validate(); err != nil {
		return 0, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return 0, fmt.Errorf("marshal stage evidence %s: %w", result.ResultID, err)
	}
	kv, err := c.validationEvidenceBucket(ctx)
	if err != nil {
		return 0, err
	}
	key := validationStageEvidenceKey(result)
	revision, err := kv.Create(ctx, key, payload)
	if err == nil {
		return revision, nil
	}
	if errors.Is(err, jetstream.ErrKeyExists) {
		entry, getErr := kv.Get(ctx, key)
		if getErr == nil {
			var existing StageResult
			if decodeErr := json.Unmarshal(entry.Value(), &existing); decodeErr == nil && SemanticallyEqualStageResults(existing, result) {
				return entry.Revision(), nil
			}
		}
	}
	return 0, fmt.Errorf("create stage evidence %s: %w", result.ResultID, err)
}

// GetValidationStageEvidence retrieves one exact immutable attempt result.
func (c *Client) GetValidationStageEvidence(ctx context.Context, uploadID, validationID string, generation uint64, ordinal, attempt uint32) (StageResult, error) {
	if !ValidID(uploadID) || !ValidID(validationID) || generation == 0 || ordinal == 0 || attempt == 0 {
		return StageResult{}, errors.New("invalid validation stage evidence identity")
	}
	kv, err := c.validationEvidenceBucket(ctx)
	if err != nil {
		return StageResult{}, err
	}
	key := validationStageEvidenceKeyParts(uploadID, validationID, generation, ordinal, attempt)
	entry, err := kv.Get(ctx, key)
	if err != nil {
		return StageResult{}, fmt.Errorf("get validation stage evidence %s: %w", key, err)
	}
	var result StageResult
	if err := json.Unmarshal(entry.Value(), &result); err != nil {
		return StageResult{}, fmt.Errorf("decode validation stage evidence %s: %w", key, err)
	}
	if result.UploadID != uploadID || result.ValidationID != validationID || result.Generation != generation || result.StageOrdinal != ordinal || result.Attempt != attempt {
		return StageResult{}, errors.New("stored validation stage evidence identity does not match key")
	}
	return result, nil
}

func sameValidationExecutionIdentity(left, right ValidationExecution) bool {
	left.Orchestration = ValidationOrchestrationState{}
	right.Orchestration = ValidationOrchestrationState{}
	return reflect.DeepEqual(left, right)
}

func validationStageEvidenceKey(result StageResult) string {
	return validationStageEvidenceKeyParts(result.UploadID, result.ValidationID, result.Generation, result.StageOrdinal, result.Attempt)
}

func validationStageEvidenceKeyParts(uploadID, validationID string, generation uint64, ordinal, attempt uint32) string {
	return uploadID + "." + validationID + "." + strconv.FormatUint(generation, 10) + "." + strconv.FormatUint(uint64(ordinal), 10) + "." + strconv.FormatUint(uint64(attempt), 10)
}

func (c *Client) validationEvidenceBucket(ctx context.Context) (jetstream.KeyValue, error) {
	c.handlesMu.RLock()
	cached := c.cachedValidationEvidenceKV
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	c.handlesMu.Lock()
	defer c.handlesMu.Unlock()
	if c.cachedValidationEvidenceKV != nil {
		return c.cachedValidationEvidenceKV, nil
	}
	bucketName := c.Namespace.ValidationStageEvidenceBucketName()
	kv, err := c.JS.KeyValue(ctx, bucketName)
	if err != nil {
		return nil, fmt.Errorf("open %s KV bucket: %w", bucketName, err)
	}
	stream, err := c.JS.Stream(ctx, "KV_"+bucketName)
	if err != nil {
		return nil, fmt.Errorf("inspect %s KV stream: %w", bucketName, err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect %s KV stream configuration: %w", bucketName, err)
	}
	config := info.Config
	if config.Storage != jetstream.FileStorage || config.Retention != jetstream.LimitsPolicy || config.MaxMsgsPerSubject != 1 || config.MaxAge != 0 ||
		(config.MaxMsgSize > 0 && config.MaxMsgSize < validationEvidenceMaxValueSize) {
		return nil, fmt.Errorf("%s KV bucket has incompatible configuration", bucketName)
	}
	c.cachedValidationEvidenceKV = kv
	return kv, nil
}
