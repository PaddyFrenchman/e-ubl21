package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/nats-io/nats.go/jetstream"
)

// ErrValidationExecutionConflict identifies a lost orchestration CAS race.
var ErrValidationExecutionConflict = errors.New("validation execution CAS conflict")

// UpdateValidationExecutionState replaces only the mutable orchestration
// projection under an exact KV revision. Immutable identity and plan fields
// must agree with the version that was read.
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
		return 0, errors.New("validation execution CAS cannot change immutable identity or frozen plan")
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
			if decodeErr := json.Unmarshal(entry.Value(), &existing); decodeErr == nil && reflect.DeepEqual(existing, result) {
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
