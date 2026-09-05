package queue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ValidationExecutionSchemaVersion is the current durable-record schema.
// It is independent of the versioned StageRequest/StageResult wire contract.
const ValidationExecutionSchemaVersion uint32 = 2

const (
	DefaultValidationRetryPolicyID             = "bounded-attempts"
	DefaultValidationRetryPolicyVersion        = "1"
	DefaultValidationMaxAttempts        uint32 = 3
	MaxValidationStageAttempts          uint32 = 32
)

var validationPlanTokenPattern = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
var validationDocumentTypePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]{0,127}$`)

// ValidationExecutionTrigger records why a durable execution lineage or
// generation was created.
type ValidationExecutionTrigger string

const (
	ValidationTriggerInitial      ValidationExecutionTrigger = "initial"
	ValidationTriggerReplay       ValidationExecutionTrigger = "replay"
	ValidationTriggerRevalidation ValidationExecutionTrigger = "revalidation"
)

// ValidationStageRequirement records plan-construction intent. Once a stage is
// included in a frozen execution plan, its result is authoritative regardless
// of this value; optional stages must be omitted by plan construction when
// they should not affect the execution outcome.
type ValidationStageRequirement string

const (
	ValidationStageRequired ValidationStageRequirement = "required"
	ValidationStageOptional ValidationStageRequirement = "optional"
)

// FrozenValidationStage is one immutable entry in an execution's ordered
// plan. RulesetVersion identifies the exact standards artefact selected when
// the generation was created.
type FrozenValidationStage struct {
	ID             ValidationLayer            `json:"id"`
	Ordinal        uint32                     `json:"ordinal"`
	Requirement    ValidationStageRequirement `json:"requirement"`
	RulesetVersion string                     `json:"rulesetVersion"`
}

// FrozenValidationPlan is the complete ordered standards plan selected for a
// validation generation. Version identifies the plan definition; stage rule
// versions identify its immutable validation assets.
type FrozenValidationPlan struct {
	ID      string                  `json:"id"`
	Version string                  `json:"version"`
	Stages  []FrozenValidationStage `json:"stages"`
}

// FrozenValidationRetryPolicy makes operational-failure exhaustion a durable
// execution decision rather than a process-local orchestrator setting.
type FrozenValidationRetryPolicy struct {
	ID                   string `json:"id"`
	Version              string `json:"version"`
	MaxAttempts          uint32 `json:"maxAttempts"`
	XSDMaxAttempts       uint32 `json:"xsdMaxAttempts,omitempty"`
	EN16931MaxAttempts   uint32 `json:"en16931MaxAttempts,omitempty"`
	PeppolBISMaxAttempts uint32 `json:"peppolBisMaxAttempts,omitempty"`
}

// MaxAttemptsFor returns the frozen operational-attempt budget for one stage.
// A zero stage override inherits MaxAttempts, preserving older execution
// records while allowing every deployed stage to be tuned independently.
func (policy FrozenValidationRetryPolicy) MaxAttemptsFor(stage ValidationLayer) uint32 {
	switch stage {
	case ValidationLayerUBL21XSD:
		if policy.XSDMaxAttempts != 0 {
			return policy.XSDMaxAttempts
		}
	case ValidationLayerEN16931:
		if policy.EN16931MaxAttempts != 0 {
			return policy.EN16931MaxAttempts
		}
	case ValidationLayerPeppolBISBilling:
		if policy.PeppolBISMaxAttempts != 0 {
			return policy.PeppolBISMaxAttempts
		}
	}
	return policy.MaxAttempts
}

// DefaultFrozenValidationRetryPolicy returns the immutable policy selected
// when an execution intent does not explicitly provide one.
func DefaultFrozenValidationRetryPolicy() FrozenValidationRetryPolicy {
	return FrozenValidationRetryPolicy{
		ID: DefaultValidationRetryPolicyID, Version: DefaultValidationRetryPolicyVersion,
		MaxAttempts: DefaultValidationMaxAttempts,
	}
}

// ValidationExecutionReference identifies an existing generation without
// relying on a mutable "latest" alias.
type ValidationExecutionReference struct {
	ValidationID string `json:"validationId"`
	Generation   uint64 `json:"generation"`
}

// ValidationExecutionIntent is the immutable command context from which a
// generation-1 identity is derived. IntentID is the durable start or
// revalidation command ID and must be stable across broker redelivery.
type ValidationExecutionIntent struct {
	IntentID         string
	UploadID         string
	DocumentType     string
	Trigger          ValidationExecutionTrigger
	RequestedProfile ValidationProfile
	Plan             FrozenValidationPlan
	RetryPolicy      FrozenValidationRetryPolicy
	RevalidationOf   *ValidationExecutionReference
	CreatedAt        time.Time
}

// ValidationExecution identifies one validation lineage and one immutable
// generation within it. UploadID identifies the independently retained XML.
//
// A stage retry is an attempt inside this generation and does not alter this
// record. Operator replay appends a generation under the same ValidationID.
// Revalidation against a changed plan or ruleset creates another ValidationID
// at generation 1, preserving every earlier record.
type ValidationExecution struct {
	SchemaVersion    uint32                        `json:"schemaVersion"`
	ValidationID     string                        `json:"validationId"`
	CreationIntentID string                        `json:"creationIntentId"`
	UploadID         string                        `json:"uploadId"`
	DocumentType     string                        `json:"documentType"`
	Generation       uint64                        `json:"generation"`
	Trigger          ValidationExecutionTrigger    `json:"trigger"`
	RequestedProfile ValidationProfile             `json:"requestedProfile"`
	Plan             FrozenValidationPlan          `json:"plan"`
	RetryPolicy      FrozenValidationRetryPolicy   `json:"retryPolicy"`
	RevalidationOf   *ValidationExecutionReference `json:"revalidationOf,omitempty"`
	ReplayOf         *ValidationExecutionReference `json:"replayOf,omitempty"`
	CreatedAt        string                        `json:"createdAt"`
	Orchestration    ValidationOrchestrationState  `json:"orchestration"`
}

// NewValidationExecution creates generation 1 from a durable command. Initial
// validation IDs are deterministic per upload, so even conflicting duplicate
// start commands collide. Revalidation IDs are deterministic per command, so
// redelivery converges while distinct commands remain distinct lineages.
func NewValidationExecution(intent ValidationExecutionIntent) (ValidationExecution, error) {
	if intent.Trigger != ValidationTriggerInitial && intent.Trigger != ValidationTriggerRevalidation {
		return ValidationExecution{}, fmt.Errorf("new validation execution trigger %q must be %q or %q", intent.Trigger, ValidationTriggerInitial, ValidationTriggerRevalidation)
	}
	if !ValidID(intent.IntentID) {
		return ValidationExecution{}, fmt.Errorf("invalid validation creation intent id %q", intent.IntentID)
	}
	validationID := deterministicValidationID(intent.Trigger, intent.UploadID, intent.IntentID)
	retryPolicy := intent.RetryPolicy
	if retryPolicy == (FrozenValidationRetryPolicy{}) {
		retryPolicy = DefaultFrozenValidationRetryPolicy()
	}
	execution := ValidationExecution{
		SchemaVersion: ValidationExecutionSchemaVersion, ValidationID: validationID, CreationIntentID: intent.IntentID,
		UploadID: intent.UploadID, DocumentType: intent.DocumentType, Generation: 1, Trigger: intent.Trigger,
		RequestedProfile: intent.RequestedProfile, Plan: cloneFrozenValidationPlan(intent.Plan), RetryPolicy: retryPolicy,
		RevalidationOf: cloneValidationExecutionReference(intent.RevalidationOf),
		ReplayOf:       nil,
		CreatedAt:      intent.CreatedAt.UTC().Format(time.RFC3339Nano),
		Orchestration:  newPendingOrchestrationState(intent.CreatedAt),
	}
	if err := execution.Validate(); err != nil {
		return ValidationExecution{}, err
	}
	return execution, nil
}

// NextValidationGeneration appends an explicit replay generation to one
// validation lineage. Identity, upload, requested profile and frozen plan are
// copied exactly; a changed plan is a revalidation and must use a new ID.
func NextValidationGeneration(previous ValidationExecution, replayIntentID string, createdAt time.Time) (ValidationExecution, error) {
	if err := previous.Validate(); err != nil {
		return ValidationExecution{}, fmt.Errorf("previous validation execution: %w", err)
	}
	if previous.Generation == ^uint64(0) {
		return ValidationExecution{}, errors.New("validation generation overflow")
	}
	if !ValidID(replayIntentID) {
		return ValidationExecution{}, fmt.Errorf("invalid replay intent id %q", replayIntentID)
	}
	next := previous
	next.Generation++
	next.Trigger = ValidationTriggerReplay
	next.CreationIntentID = replayIntentID
	next.RevalidationOf = nil
	next.ReplayOf = &ValidationExecutionReference{ValidationID: previous.ValidationID, Generation: previous.Generation}
	next.Plan = cloneFrozenValidationPlan(previous.Plan)
	next.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	next.Orchestration = newPendingOrchestrationState(createdAt)
	return next, nil
}

// NextFailedStageReplayGeneration appends an operational replay that begins
// at the exhausted stage. Earlier passing stages are represented by exact
// immutable evidence references, so replay cannot bypass or rerun them.
func NextFailedStageReplayGeneration(previous ValidationExecution, dead StageDeadLetter, createdAt time.Time) (ValidationExecution, error) {
	if err := previous.Validate(); err != nil {
		return ValidationExecution{}, fmt.Errorf("previous validation execution: %w", err)
	}
	if err := dead.Validate(); err != nil {
		return ValidationExecution{}, err
	}
	state := previous.Orchestration
	if state.Status != ExecutionStatusProcessingFailed || state.FinalResultStatus != FinalResultPublished {
		return ValidationExecution{}, errors.New("failed-stage replay requires a finalized processing-failed generation")
	}
	failedIndex := int(state.CurrentStageOrdinal - 1)
	if failedIndex < 0 || failedIndex >= len(state.Stages) {
		return ValidationExecution{}, errors.New("processing-failed generation has no failed stage")
	}
	failed := state.Stages[failedIndex]
	if failed.InheritedEvidence != nil || len(failed.Attempts) == 0 {
		return ValidationExecution{}, errors.New("processing-failed generation has no local terminal attempt")
	}
	accepted := failed.Attempts[len(failed.Attempts)-1]
	if dead.UploadID != previous.UploadID || dead.ValidationID != previous.ValidationID || dead.Generation != previous.Generation ||
		dead.Stage != failed.Stage || dead.StageOrdinal != failed.Ordinal || dead.Attempt != accepted.Attempt ||
		dead.RequestID != accepted.RequestID || dead.ResultID != accepted.ResultID || dead.Reason != accepted.Reason {
		return ValidationExecution{}, errors.New("stage dead letter does not match the failed execution")
	}
	next, err := NextValidationGeneration(previous, dead.DeadLetterID, createdAt)
	if err != nil {
		return ValidationExecution{}, err
	}
	request, err := NewStageRequest(next, failed.Ordinal, 1, createdAt)
	if err != nil {
		return ValidationExecution{}, err
	}
	stages := make([]ValidationStageState, 0, failedIndex+1)
	for index := 0; index < failedIndex; index++ {
		prior := state.Stages[index]
		if prior.TerminalOutcome != StageOutcomePassed {
			return ValidationExecution{}, fmt.Errorf("failed-stage replay prerequisite %d did not pass", index+1)
		}
		var reference ValidationStageEvidenceReference
		if prior.InheritedEvidence != nil {
			reference = *prior.InheritedEvidence
		} else {
			if len(prior.Attempts) == 0 {
				return ValidationExecution{}, fmt.Errorf("failed-stage replay prerequisite %d has no evidence", index+1)
			}
			priorAccepted := prior.Attempts[len(prior.Attempts)-1]
			reference = ValidationStageEvidenceReference{
				ValidationID: previous.ValidationID, Generation: previous.Generation, StageOrdinal: prior.Ordinal,
				Attempt: priorAccepted.Attempt, RequestID: priorAccepted.RequestID, ResultID: priorAccepted.ResultID,
			}
		}
		stages = append(stages, ValidationStageState{
			Stage: prior.Stage, Ordinal: prior.Ordinal, TerminalOutcome: StageOutcomePassed, InheritedEvidence: &reference,
		})
	}
	stages = append(stages, ValidationStageState{
		Stage: request.Stage, Ordinal: request.StageOrdinal, AuthorizedAttempt: 1,
		RequestID: request.RequestID, RequestCreatedAt: request.CreatedAt, DispatchStatus: StageDispatchPending,
	})
	next.Orchestration = ValidationOrchestrationState{
		Status: ExecutionStatusStageQueued, CurrentStageOrdinal: failed.Ordinal, Stages: stages,
		UpdatedAt: canonicalStageTime(createdAt),
	}
	if err := next.Validate(); err != nil {
		return ValidationExecution{}, err
	}
	return next, nil
}

// Validate rejects malformed or ambiguous execution records before they can
// become durable orchestration state.
func (execution ValidationExecution) Validate() error {
	if execution.SchemaVersion != ValidationExecutionSchemaVersion {
		return fmt.Errorf("validation execution schema version %d is unsupported", execution.SchemaVersion)
	}
	if !ValidID(execution.ValidationID) {
		return fmt.Errorf("invalid validation id %q", execution.ValidationID)
	}
	if !ValidID(execution.CreationIntentID) {
		return fmt.Errorf("invalid validation creation intent id %q", execution.CreationIntentID)
	}
	if !ValidID(execution.UploadID) {
		return fmt.Errorf("invalid validation execution upload id %q", execution.UploadID)
	}
	if !validationDocumentTypePattern.MatchString(execution.DocumentType) {
		return fmt.Errorf("invalid validation execution document type %q", execution.DocumentType)
	}
	if execution.Generation == 0 {
		return errors.New("validation generation must be at least 1")
	}
	switch execution.Trigger {
	case ValidationTriggerInitial:
		if execution.Generation != 1 {
			return fmt.Errorf("validation trigger %q requires generation 1", execution.Trigger)
		}
		if execution.RevalidationOf != nil || execution.ReplayOf != nil {
			return errors.New("initial validation cannot reference a prior execution")
		}
		if want := deterministicValidationID(ValidationTriggerInitial, execution.UploadID, execution.CreationIntentID); execution.ValidationID != want {
			return fmt.Errorf("initial validation id %q does not match deterministic id %q", execution.ValidationID, want)
		}
	case ValidationTriggerRevalidation:
		if execution.Generation != 1 {
			return fmt.Errorf("validation trigger %q requires generation 1", execution.Trigger)
		}
		if err := execution.RevalidationOf.validate(); err != nil {
			return fmt.Errorf("revalidation predecessor: %w", err)
		}
		if execution.ReplayOf != nil {
			return errors.New("revalidation cannot reference a replay predecessor")
		}
		if want := deterministicValidationID(ValidationTriggerRevalidation, execution.UploadID, execution.CreationIntentID); execution.ValidationID != want {
			return fmt.Errorf("revalidation id %q does not match deterministic id %q", execution.ValidationID, want)
		}
	case ValidationTriggerReplay:
		if execution.Generation < 2 {
			return errors.New("validation replay requires generation 2 or later")
		}
		if execution.RevalidationOf != nil {
			return errors.New("replay generation cannot change revalidation predecessor")
		}
		if err := execution.ReplayOf.validate(); err != nil {
			return fmt.Errorf("replay predecessor: %w", err)
		}
		if execution.ReplayOf.ValidationID != execution.ValidationID || execution.ReplayOf.Generation+1 != execution.Generation {
			return errors.New("replay predecessor must be the immediately preceding generation")
		}
	default:
		return fmt.Errorf("invalid validation execution trigger %q", execution.Trigger)
	}
	if !ValidValidationProfile(execution.RequestedProfile) {
		return fmt.Errorf("invalid requested validation profile %q", execution.RequestedProfile)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, execution.CreatedAt)
	if err != nil {
		return fmt.Errorf("invalid validation execution createdAt %q: %w", execution.CreatedAt, err)
	}
	if createdAt.IsZero() {
		return errors.New("validation execution createdAt must not be zero")
	}
	if err := execution.Plan.validate(); err != nil {
		return err
	}
	if err := validateProfilePlan(execution.RequestedProfile, execution.Plan); err != nil {
		return err
	}
	if err := execution.RetryPolicy.validate(); err != nil {
		return err
	}
	if err := execution.Orchestration.validate(execution); err != nil {
		return fmt.Errorf("validation orchestration state: %w", err)
	}
	return nil
}

func (policy FrozenValidationRetryPolicy) validate() error {
	if policy.ID != DefaultValidationRetryPolicyID {
		return fmt.Errorf("unsupported validation retry policy %q", policy.ID)
	}
	if policy.Version != DefaultValidationRetryPolicyVersion {
		return fmt.Errorf("unsupported validation retry policy %q version %q", policy.ID, policy.Version)
	}
	if policy.MaxAttempts == 0 || policy.MaxAttempts > MaxValidationStageAttempts {
		return fmt.Errorf("validation retry policy max attempts must be between 1 and %d", MaxValidationStageAttempts)
	}
	for stage, attempts := range map[ValidationLayer]uint32{
		ValidationLayerUBL21XSD:         policy.XSDMaxAttempts,
		ValidationLayerEN16931:          policy.EN16931MaxAttempts,
		ValidationLayerPeppolBISBilling: policy.PeppolBISMaxAttempts,
	} {
		if attempts > MaxValidationStageAttempts {
			return fmt.Errorf("validation retry policy %s max attempts must be between 1 and %d when set", stage, MaxValidationStageAttempts)
		}
	}
	return nil
}

func (plan FrozenValidationPlan) validate() error {
	if len(plan.ID) > MaxStagePlanIDBytes {
		return fmt.Errorf("validation plan id exceeds %d bytes", MaxStagePlanIDBytes)
	}
	if !validationPlanTokenPattern.MatchString(plan.ID) {
		return fmt.Errorf("invalid validation plan id %q", plan.ID)
	}
	if err := validateStageVersion("validation plan version", plan.Version); err != nil {
		return err
	}
	if len(plan.Stages) == 0 {
		return errors.New("validation plan must contain at least one stage")
	}
	seen := make(map[ValidationLayer]struct{}, len(plan.Stages))
	for index, stage := range plan.Stages {
		wantOrdinal := uint32(index + 1)
		if stage.Ordinal != wantOrdinal {
			return fmt.Errorf("validation plan stage %d has ordinal %d, want %d", index, stage.Ordinal, wantOrdinal)
		}
		if len(stage.ID) > MaxStageIDBytes {
			return fmt.Errorf("validation plan stage %d id exceeds %d bytes", index, MaxStageIDBytes)
		}
		if !validationPlanTokenPattern.MatchString(string(stage.ID)) {
			return fmt.Errorf("validation plan stage %d has invalid id %q", index, stage.ID)
		}
		if _, duplicate := seen[stage.ID]; duplicate {
			return fmt.Errorf("validation plan stage id %q is duplicated", stage.ID)
		}
		seen[stage.ID] = struct{}{}
		if stage.Requirement != ValidationStageRequired && stage.Requirement != ValidationStageOptional {
			return fmt.Errorf("validation plan stage %q has invalid requirement %q", stage.ID, stage.Requirement)
		}
		if err := validateStageVersion("validation plan stage "+string(stage.ID)+" ruleset version", stage.RulesetVersion); err != nil {
			return err
		}
	}
	return nil
}

func validateProfilePlan(profile ValidationProfile, plan FrozenValidationPlan) error {
	var expected []ValidationLayer
	switch profile {
	case ValidationProfileUBL21:
		expected = []ValidationLayer{ValidationLayerUBL21XSD}
	case ValidationProfileUBL21EN16931:
		expected = []ValidationLayer{ValidationLayerUBL21XSD, ValidationLayerEN16931}
	case ValidationProfilePeppolBISBilling:
		expected = []ValidationLayer{
			ValidationLayerUBL21XSD,
			ValidationLayerEN16931,
			ValidationLayerPeppolBISBilling,
		}
	default:
		return fmt.Errorf("invalid validation profile %q", profile)
	}
	if len(plan.Stages) != len(expected) {
		return fmt.Errorf("validation profile %q requires stages %v", profile, expected)
	}
	for index, layer := range expected {
		stage := plan.Stages[index]
		if stage.ID != layer {
			return fmt.Errorf("validation profile %q requires stage %d to be layer %q", profile, index, layer)
		}
	}
	return nil
}

func (reference *ValidationExecutionReference) validate() error {
	if reference == nil {
		return errors.New("reference is required")
	}
	if !ValidID(reference.ValidationID) || reference.Generation == 0 {
		return fmt.Errorf("invalid validation execution reference %q generation %d", reference.ValidationID, reference.Generation)
	}
	return nil
}

// VersionedValidationExecution pairs a record with its JetStream KV revision.
// The initial model is create-only; the revision is reserved for later
// orchestrator CAS state without weakening frozen identity and plan fields.
type VersionedValidationExecution struct {
	Execution ValidationExecution
	Revision  uint64
}

// ErrValidationExecutionNotFound is returned when an execution generation is
// absent or any supplied identity is malformed.
var ErrValidationExecutionNotFound = errors.New("validation execution not found")

// CreateValidationExecution durably creates one immutable execution
// generation. It never overwrites a prior generation.
func (c *Client) CreateValidationExecution(ctx context.Context, execution ValidationExecution) (uint64, error) {
	if err := execution.Validate(); err != nil {
		return 0, err
	}
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return 0, err
	}
	if execution.Generation > 1 {
		for generation := uint64(1); generation < execution.Generation; generation++ {
			entry, getErr := kv.Get(ctx, validationExecutionKey(execution.UploadID, execution.ValidationID, generation))
			if getErr != nil {
				if errors.Is(getErr, jetstream.ErrKeyNotFound) {
					return 0, fmt.Errorf("create validation execution %s generation %d: generation %d is missing", execution.ValidationID, execution.Generation, generation)
				}
				return 0, fmt.Errorf("inspect replay intent history for %s generation %d: %w", execution.ValidationID, generation, getErr)
			}
			var prior ValidationExecution
			if err := json.Unmarshal(entry.Value(), &prior); err != nil {
				return 0, fmt.Errorf("decode prior validation execution %s generation %d: %w", execution.ValidationID, generation, err)
			}
			if prior.CreationIntentID == execution.CreationIntentID {
				return 0, fmt.Errorf("create validation execution %s generation %d: replay intent %s was already used by generation %d", execution.ValidationID, execution.Generation, execution.CreationIntentID, generation)
			}
		}
		previousEntry, getErr := kv.Get(ctx, validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation-1))
		if getErr != nil {
			if errors.Is(getErr, jetstream.ErrKeyNotFound) {
				return 0, fmt.Errorf("create validation execution %s generation %d: previous generation is missing", execution.ValidationID, execution.Generation)
			}
			return 0, fmt.Errorf("load previous validation execution %s generation %d: %w", execution.ValidationID, execution.Generation-1, getErr)
		}
		var previous ValidationExecution
		if err := json.Unmarshal(previousEntry.Value(), &previous); err != nil {
			return 0, fmt.Errorf("decode previous validation execution %s generation %d: %w", execution.ValidationID, execution.Generation-1, err)
		}
		if err := previous.Validate(); err != nil {
			return 0, fmt.Errorf("validate previous execution %s generation %d: %w", execution.ValidationID, execution.Generation-1, err)
		}
		if previous.UploadID != execution.UploadID || previous.ValidationID != execution.ValidationID || previous.DocumentType != execution.DocumentType ||
			previous.RequestedProfile != execution.RequestedProfile || !reflect.DeepEqual(previous.Plan, execution.Plan) || previous.RetryPolicy != execution.RetryPolicy {
			return 0, fmt.Errorf("create validation execution %s generation %d: replay lineage changed immutable identity, profile, plan, or retry policy", execution.ValidationID, execution.Generation)
		}
		if execution.ReplayOf == nil || execution.ReplayOf.ValidationID != previous.ValidationID || execution.ReplayOf.Generation != previous.Generation {
			return 0, fmt.Errorf("create validation execution %s generation %d: replay predecessor does not match generation %d", execution.ValidationID, execution.Generation, previous.Generation)
		}
		for index, stage := range execution.Orchestration.Stages {
			if stage.InheritedEvidence == nil {
				continue
			}
			reference := stage.InheritedEvidence
			evidence, evidenceErr := c.GetValidationStageEvidence(ctx, execution.UploadID, reference.ValidationID, reference.Generation, reference.StageOrdinal, reference.Attempt)
			if evidenceErr != nil {
				return 0, fmt.Errorf("create validation execution %s generation %d: load inherited prerequisite %d: %w", execution.ValidationID, execution.Generation, index+1, evidenceErr)
			}
			planStage := execution.Plan.Stages[index]
			if evidence.UploadID != execution.UploadID || evidence.ValidationID != reference.ValidationID || evidence.Generation != reference.Generation ||
				evidence.StageOrdinal != stage.Ordinal || evidence.Stage != stage.Stage || evidence.RulesetVersion != planStage.RulesetVersion ||
				evidence.Attempt != reference.Attempt || evidence.RequestID != reference.RequestID || evidence.ResultID != reference.ResultID ||
				evidence.Outcome != StageOutcomePassed {
				return 0, fmt.Errorf("create validation execution %s generation %d: inherited prerequisite %d does not match permanent passing evidence", execution.ValidationID, execution.Generation, index+1)
			}
		}
	} else if execution.Trigger == ValidationTriggerRevalidation {
		reference := execution.RevalidationOf
		previousEntry, getErr := kv.Get(ctx, validationExecutionKey(execution.UploadID, reference.ValidationID, reference.Generation))
		if getErr != nil {
			if errors.Is(getErr, jetstream.ErrKeyNotFound) {
				return 0, fmt.Errorf("create revalidation %s: referenced execution is missing", execution.ValidationID)
			}
			return 0, fmt.Errorf("load revalidation predecessor %s generation %d: %w", reference.ValidationID, reference.Generation, getErr)
		}
		var previous ValidationExecution
		if err := json.Unmarshal(previousEntry.Value(), &previous); err != nil {
			return 0, fmt.Errorf("decode revalidation predecessor %s generation %d: %w", reference.ValidationID, reference.Generation, err)
		}
		if err := previous.Validate(); err != nil {
			return 0, fmt.Errorf("validate revalidation predecessor %s generation %d: %w", reference.ValidationID, reference.Generation, err)
		}
		if previous.UploadID != execution.UploadID || previous.DocumentType != execution.DocumentType || previous.ValidationID != reference.ValidationID || previous.Generation != reference.Generation {
			return 0, fmt.Errorf("create revalidation %s: predecessor identity or document type does not match", execution.ValidationID)
		}
		if previous.RequestedProfile == execution.RequestedProfile && reflect.DeepEqual(previous.Plan, execution.Plan) && previous.RetryPolicy == execution.RetryPolicy {
			return 0, fmt.Errorf("create revalidation %s: requested profile, frozen plan, and retry policy are unchanged; use replay", execution.ValidationID)
		}
	}
	encoded, err := json.Marshal(execution)
	if err != nil {
		return 0, fmt.Errorf("marshal validation execution %s generation %d: %w", execution.ValidationID, execution.Generation, err)
	}
	revision, err := kv.Create(ctx, validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation), encoded)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyExists) {
			entry, getErr := kv.Get(ctx, validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation))
			if getErr == nil {
				var existing ValidationExecution
				if decodeErr := json.Unmarshal(entry.Value(), &existing); decodeErr == nil && sameValidationExecutionCreationIntent(existing, execution) {
					return entry.Revision(), nil
				}
			}
		}
		return 0, fmt.Errorf("create validation execution %s generation %d: %w", execution.ValidationID, execution.Generation, err)
	}
	return revision, nil
}

// sameValidationExecutionCreationIntent recognizes command redelivery after a
// concurrent first writer has already frozen CreatedAt and orchestration
// state. The stable intent and every other immutable field must match; only
// the first writer's actual command time and subsequent mutable state win.
func sameValidationExecutionCreationIntent(left, right ValidationExecution) bool {
	left.CreatedAt = ""
	right.CreatedAt = ""
	left.Orchestration = ValidationOrchestrationState{}
	right.Orchestration = ValidationOrchestrationState{}
	return reflect.DeepEqual(left, right)
}

// GetValidationExecution retrieves one exact immutable generation.
func (c *Client) GetValidationExecution(ctx context.Context, uploadID, validationID string, generation uint64) (ValidationExecution, error) {
	versioned, err := c.GetValidationExecutionVersion(ctx, uploadID, validationID, generation)
	return versioned.Execution, err
}

// GetValidationExecutionVersion retrieves one generation and its KV revision.
func (c *Client) GetValidationExecutionVersion(ctx context.Context, uploadID, validationID string, generation uint64) (VersionedValidationExecution, error) {
	if !ValidID(uploadID) || !ValidID(validationID) || generation == 0 {
		return VersionedValidationExecution{}, ErrValidationExecutionNotFound
	}
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return VersionedValidationExecution{}, err
	}
	entry, err := kv.Get(ctx, validationExecutionKey(uploadID, validationID, generation))
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return VersionedValidationExecution{}, ErrValidationExecutionNotFound
		}
		return VersionedValidationExecution{}, fmt.Errorf("get validation execution %s generation %d: %w", validationID, generation, err)
	}
	var execution ValidationExecution
	if err := json.Unmarshal(entry.Value(), &execution); err != nil {
		return VersionedValidationExecution{}, fmt.Errorf("decode validation execution %s generation %d: %w", validationID, generation, err)
	}
	if err := execution.Validate(); err != nil {
		return VersionedValidationExecution{}, fmt.Errorf("validate stored execution %s generation %d: %w", validationID, generation, err)
	}
	if execution.UploadID != uploadID || execution.ValidationID != validationID || execution.Generation != generation {
		return VersionedValidationExecution{}, fmt.Errorf("stored validation execution identity does not match key")
	}
	return VersionedValidationExecution{Execution: execution, Revision: entry.Revision()}, nil
}

// ValidationExecutionsForUpload lists every durable lineage/generation for an
// upload in stable validation-ID/generation order. Polling can therefore keep
// addressing an explicit pair rather than guessing a mutable "latest" record.
func (c *Client) ValidationExecutionsForUpload(ctx context.Context, uploadID string) ([]ValidationExecution, error) {
	if !ValidID(uploadID) {
		return nil, ErrValidationExecutionNotFound
	}
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return nil, err
	}
	lister, err := kv.ListKeysFiltered(ctx, uploadID+".>")
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("list validation executions for upload %s: %w", uploadID, err)
	}
	defer lister.Stop()
	executions := make([]ValidationExecution, 0)
	for key := range lister.Keys() {
		entry, getErr := kv.Get(ctx, key)
		if getErr != nil {
			return nil, fmt.Errorf("get validation execution key %s: %w", key, getErr)
		}
		var execution ValidationExecution
		if err := json.Unmarshal(entry.Value(), &execution); err != nil {
			return nil, fmt.Errorf("decode validation execution key %s: %w", key, err)
		}
		if err := execution.Validate(); err != nil {
			return nil, fmt.Errorf("validate stored execution key %s: %w", key, err)
		}
		if key != validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation) || execution.UploadID != uploadID {
			return nil, fmt.Errorf("stored validation execution identity does not match key %s", key)
		}
		executions = append(executions, execution)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list validation executions for upload %s: %w", uploadID, err)
	}
	sort.Slice(executions, func(i, j int) bool {
		if executions[i].ValidationID == executions[j].ValidationID {
			return executions[i].Generation < executions[j].Generation
		}
		return executions[i].ValidationID < executions[j].ValidationID
	})
	return executions, nil
}

// ProjectUploadRecord overlays the active split execution's permanent result
// onto API-owned acceptance metadata without mutating either aggregate. Legacy
// terminal records remain authoritative and unchanged.
func (c *Client) ProjectUploadRecord(ctx context.Context, record UploadRecord) (UploadRecord, error) {
	if record.Status == StatusValid || record.Status == StatusInvalid || record.Status == StatusProcessingFailed || record.Status == StatusAcceptanceFailed {
		return record, nil
	}
	deployment, err := c.DeploymentGeneration(ctx)
	if errors.Is(err, ErrDeploymentGenerationNotProvisioned) {
		return record, nil
	}
	if err != nil {
		return UploadRecord{}, fmt.Errorf("load deployment generation for upload projection: %w", err)
	}
	if deployment.State.Mode != DeploymentModeSplit {
		return record, nil
	}
	executions, err := c.ValidationExecutionsForUpload(ctx, record.ID)
	if err != nil {
		return UploadRecord{}, err
	}
	lineageID := ""
	for index := range executions {
		execution := executions[index]
		if execution.Trigger != ValidationTriggerInitial || execution.CreationIntentID != record.ID {
			continue
		}
		if lineageID != "" && lineageID != execution.ValidationID {
			return UploadRecord{}, errors.New("upload has conflicting initial validation lineages")
		}
		lineageID = execution.ValidationID
	}
	if lineageID == "" {
		return record, nil
	}
	var current *ValidationExecution
	for index := range executions {
		execution := &executions[index]
		// Replay generations have distinct creation-intent IDs but retain their
		// initial validation ID. Future explicit revalidation has a different
		// lineage and needs its own API selection contract rather than silently
		// changing this view.
		if execution.ValidationID != lineageID {
			continue
		}
		if current == nil || execution.Generation > current.Generation {
			current = execution
		}
	}
	if current == nil {
		return record, nil
	}
	record.Profile = current.RequestedProfile
	record.ResolvedProfile = current.RequestedProfile
	if record.RequestedProfile == "" {
		// Records accepted before requested/resolved profile metadata existed
		// retain their original concrete profile semantics.
		record.RequestedProfile = current.RequestedProfile
	}
	if current.Orchestration.FinalResultStatus != FinalResultPublished {
		return record, nil
	}
	result, err := c.GetValidationExecutionResult(ctx, current.ValidationID, current.Generation)
	if err != nil {
		return UploadRecord{}, fmt.Errorf("load canonical result for upload projection: %w", err)
	}
	if result.ID != record.ID || result.DocumentType != record.DocumentType || result.Profile != current.RequestedProfile {
		return UploadRecord{}, errors.New("canonical result does not match upload acceptance metadata")
	}
	record.Status = result.Status
	record.ValidatedAt = result.ValidatedAt
	record.ValidationStages = append(ValidationStages(nil), result.ValidationStages...)
	record.Error = result.Error
	if result.Status == StatusProcessingFailed {
		record.LastError = result.Error
		record.ProcessingFailedAt = result.ValidatedAt
	}
	return record, nil
}

// ValidationExecutionsRequiringDispatch lists every execution that either has
// not authorized its first stage yet or has a durable pending outbox intent.
// The caller may advance and publish each execution independently; CAS state
// and deterministic request IDs make concurrent scans harmless.
func (c *Client) ValidationExecutionsRequiringDispatch(ctx context.Context) ([]VersionedValidationExecution, error) {
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("list validation executions pending dispatch: %w", err)
	}
	pending := make([]VersionedValidationExecution, 0)
	for _, key := range keys {
		entry, getErr := kv.Get(ctx, key)
		if getErr != nil {
			return nil, fmt.Errorf("get validation execution key %s for dispatch reconciliation: %w", key, getErr)
		}
		var execution ValidationExecution
		if err := json.Unmarshal(entry.Value(), &execution); err != nil {
			return nil, fmt.Errorf("decode validation execution key %s for dispatch reconciliation: %w", key, err)
		}
		if err := execution.Validate(); err != nil {
			return nil, fmt.Errorf("validate stored execution key %s for dispatch reconciliation: %w", key, err)
		}
		if key != validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation) {
			return nil, fmt.Errorf("stored validation execution identity does not match key %s", key)
		}
		if execution.Orchestration.Status == ExecutionStatusPending {
			pending = append(pending, VersionedValidationExecution{Execution: execution, Revision: entry.Revision()})
			continue
		}
		if execution.Orchestration.Status == ExecutionStatusStageQueued {
			stage := execution.Orchestration.Stages[execution.Orchestration.CurrentStageOrdinal-1]
			if stage.DispatchStatus == StageDispatchPending {
				pending = append(pending, VersionedValidationExecution{Execution: execution, Revision: entry.Revision()})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list validation executions pending dispatch: %w", err)
	}
	sort.Slice(pending, func(i, j int) bool {
		left, right := pending[i].Execution, pending[j].Execution
		if left.UploadID != right.UploadID {
			return left.UploadID < right.UploadID
		}
		if left.ValidationID != right.ValidationID {
			return left.ValidationID < right.ValidationID
		}
		return left.Generation < right.Generation
	})
	return pending, nil
}

// ValidationExecutionsRequiringFinalResult lists terminal executions whose
// permanent result outbox is absent (pre-#56 compatibility) or pending.
func (c *Client) ValidationExecutionsRequiringFinalResult(ctx context.Context) ([]VersionedValidationExecution, error) {
	kv, err := c.validationExecutionsBucket(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := kv.Keys(ctx)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("list validation executions pending final result: %w", err)
	}
	pending := make([]VersionedValidationExecution, 0)
	for _, key := range keys {
		entry, getErr := kv.Get(ctx, key)
		if getErr != nil {
			return nil, fmt.Errorf("get validation execution key %s for final-result reconciliation: %w", key, getErr)
		}
		var execution ValidationExecution
		if err := json.Unmarshal(entry.Value(), &execution); err != nil {
			return nil, fmt.Errorf("decode validation execution key %s for final-result reconciliation: %w", key, err)
		}
		if err := execution.Validate(); err != nil {
			return nil, fmt.Errorf("validate stored execution key %s for final-result reconciliation: %w", key, err)
		}
		if key != validationExecutionKey(execution.UploadID, execution.ValidationID, execution.Generation) {
			return nil, fmt.Errorf("stored validation execution identity does not match key %s", key)
		}
		terminal := execution.Orchestration.Status == ExecutionStatusValid || execution.Orchestration.Status == ExecutionStatusInvalid || execution.Orchestration.Status == ExecutionStatusProcessingFailed
		if terminal && execution.Orchestration.FinalResultStatus != FinalResultPublished {
			pending = append(pending, VersionedValidationExecution{Execution: execution, Revision: entry.Revision()})
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("list validation executions pending final result: %w", err)
	}
	sort.Slice(pending, func(i, j int) bool {
		left, right := pending[i].Execution, pending[j].Execution
		if left.UploadID != right.UploadID {
			return left.UploadID < right.UploadID
		}
		if left.ValidationID != right.ValidationID {
			return left.ValidationID < right.ValidationID
		}
		return left.Generation < right.Generation
	})
	return pending, nil
}

func validationExecutionKey(uploadID, validationID string, generation uint64) string {
	return uploadID + "." + validationID + "." + strconv.FormatUint(generation, 10)
}

func cloneFrozenValidationPlan(plan FrozenValidationPlan) FrozenValidationPlan {
	cloned := plan
	cloned.Stages = append([]FrozenValidationStage(nil), plan.Stages...)
	return cloned
}

func cloneValidationExecutionReference(reference *ValidationExecutionReference) *ValidationExecutionReference {
	if reference == nil {
		return nil
	}
	cloned := *reference
	return &cloned
}

func deterministicValidationID(trigger ValidationExecutionTrigger, uploadID, intentID string) string {
	commandComponent := intentID
	if trigger == ValidationTriggerInitial {
		// One immutable upload has exactly one initial lineage. IntentID remains
		// in the record so a conflicting second start command is rejected rather
		// than silently treated as the original command.
		commandComponent = "initial"
	}
	digest := sha256.Sum256([]byte("e-ubl21.validation-id.v1\x00" + string(trigger) + "\x00" + uploadID + "\x00" + commandComponent))
	return hex.EncodeToString(digest[:16])
}

func (c *Client) validationExecutionsBucket(ctx context.Context) (jetstream.KeyValue, error) {
	c.handlesMu.RLock()
	cached := c.cachedValidationExecutionsKV
	c.handlesMu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	c.handlesMu.Lock()
	defer c.handlesMu.Unlock()
	if c.cachedValidationExecutionsKV != nil {
		return c.cachedValidationExecutionsKV, nil
	}
	bucketName := c.Namespace.ValidationExecutionsBucketName()
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
		(config.MaxMsgSize > 0 && config.MaxMsgSize < validationExecutionMaxValueSize) {
		return nil, fmt.Errorf("%s KV bucket has incompatible configuration", bucketName)
	}
	c.cachedValidationExecutionsKV = kv
	return kv, nil
}
