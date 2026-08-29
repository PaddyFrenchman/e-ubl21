package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestConcurrentInitialCommandDeliveryConverges(t *testing.T) {
	client := newExecutionTestClient(t, queue.DefaultNamespace)
	uploadID := testID(t)
	intent := testInitialIntent(t, uploadID, testFrozenPlan(), time.Now())
	execution, err := queue.NewValidationExecution(intent)
	if err != nil {
		t.Fatal(err)
	}
	const replicas = 16
	start := make(chan struct{})
	revisions := make(chan uint64, replicas)
	errorsSeen := make(chan error, replicas)
	var group sync.WaitGroup
	for range replicas {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			revision, err := client.CreateValidationExecution(context.Background(), execution)
			if err != nil {
				errorsSeen <- err
				return
			}
			revisions <- revision
		}()
	}
	close(start)
	group.Wait()
	close(revisions)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("concurrent create: %v", err)
	}
	var wantRevision uint64
	for revision := range revisions {
		if wantRevision == 0 {
			wantRevision = revision
		} else if revision != wantRevision {
			t.Errorf("concurrent revision = %d, want %d", revision, wantRevision)
		}
	}
	executions, err := client.ValidationExecutionsForUpload(context.Background(), uploadID)
	if err != nil || len(executions) != 1 {
		t.Fatalf("concurrent initial executions = %+v, %v", executions, err)
	}
}

func testFrozenPlan() queue.FrozenValidationPlan {
	return queue.FrozenValidationPlan{
		ID: "ubl21-en16931", Version: "1",
		Stages: []queue.FrozenValidationStage{
			{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
			{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
		},
	}
}

func testID(t *testing.T) string {
	t.Helper()
	id, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func testInitialIntent(t *testing.T, uploadID string, plan queue.FrozenValidationPlan, createdAt time.Time) queue.ValidationExecutionIntent {
	t.Helper()
	plan.Stages = append([]queue.FrozenValidationStage(nil), plan.Stages...)
	return queue.ValidationExecutionIntent{
		IntentID: testID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan: plan, CreatedAt: createdAt,
	}
}

func newExecutionTestClient(t *testing.T, namespace queue.Namespace) *queue.Client {
	t.Helper()
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, namespace, server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	if err := client.EnsureInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionValidationExecutionInfra(context.Background()); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestValidationExecutionGenerationsPreserveHistory(t *testing.T) {
	client := newExecutionTestClient(t, queue.DefaultNamespace)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uploadID := testID(t)
	created := time.Date(2026, 8, 28, 12, 0, 0, 123, time.UTC)
	plan := testFrozenPlan()
	initialIntent := testInitialIntent(t, uploadID, plan, created)
	initial, err := queue.NewValidationExecution(initialIntent)
	if err != nil {
		t.Fatal(err)
	}
	// The constructor owns its frozen copy; caller mutation cannot change it.
	plan.Stages[1].RulesetVersion = "moving-version"
	if initial.Plan.Stages[1].RulesetVersion != "1.3.16" {
		t.Fatalf("constructor did not freeze plan: %+v", initial.Plan)
	}
	firstRevision, err := client.CreateValidationExecution(ctx, initial)
	if err != nil {
		t.Fatal(err)
	}
	if firstRevision == 0 {
		t.Fatal("initial execution revision is zero")
	}
	duplicate, err := queue.NewValidationExecution(initialIntent)
	if err != nil || !reflect.DeepEqual(duplicate, initial) {
		t.Fatalf("duplicate start intent = %+v, %v; want %+v", duplicate, err, initial)
	}
	duplicateRevision, err := client.CreateValidationExecution(ctx, duplicate)
	if err != nil || duplicateRevision != firstRevision {
		t.Fatalf("idempotent initial create revision = %d, %v; want %d", duplicateRevision, err, firstRevision)
	}
	conflictingInitialIntent := initialIntent
	conflictingInitialIntent.IntentID = testID(t)
	conflictingInitial, err := queue.NewValidationExecution(conflictingInitialIntent)
	if err != nil {
		t.Fatal(err)
	}
	if conflictingInitial.ValidationID != initial.ValidationID {
		t.Fatalf("same upload initial IDs = %q/%q; want deterministic collision", initial.ValidationID, conflictingInitial.ValidationID)
	}
	if _, err := client.CreateValidationExecution(ctx, conflictingInitial); !errors.Is(err, jetstream.ErrKeyExists) {
		t.Fatalf("second initial intent error = %v, want %v", err, jetstream.ErrKeyExists)
	}

	replayIntentID := testID(t)
	// Generation, not wall-clock order, is authoritative. Clock skew is valid.
	replay, err := queue.NextValidationGeneration(initial, replayIntentID, created.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if replay.ValidationID != initial.ValidationID || replay.Generation != 2 || replay.Trigger != queue.ValidationTriggerReplay {
		t.Fatalf("replay identity = %+v, initial = %+v", replay, initial)
	}
	if _, err := client.CreateValidationExecution(ctx, replay); err != nil {
		t.Fatal(err)
	}
	if revision, err := client.CreateValidationExecution(ctx, replay); err != nil || revision == 0 {
		t.Fatalf("idempotent replay create = %d, %v", revision, err)
	}
	reusedReplayIntent, err := queue.NextValidationGeneration(replay, replayIntentID, created.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, reusedReplayIntent); err == nil {
		t.Fatal("reused replay command created another generation")
	}
	changedReplay := replay
	changedReplay.Generation = 3
	changedReplay.CreationIntentID = testID(t)
	changedReplay.Plan.Stages = append([]queue.FrozenValidationStage(nil), replay.Plan.Stages...)
	changedReplay.Plan.Stages[1].RulesetVersion = "changed-during-replay"
	changedReplay.CreatedAt = created.Add(90 * time.Minute).Format(time.RFC3339Nano)
	if _, err := client.CreateValidationExecution(ctx, changedReplay); err == nil {
		t.Fatal("replay generation changed the frozen plan")
	}
	gappedReplay := replay
	gappedReplay.Generation = 4
	gappedReplay.CreationIntentID = testID(t)
	gappedReplay.CreatedAt = created.Add(2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := client.CreateValidationExecution(ctx, gappedReplay); err == nil {
		t.Fatal("replay generation was created without its predecessor")
	}

	revalidationPlan := testFrozenPlan()
	revalidationPlan.Version = "2"
	revalidationPlan.Stages[1].RulesetVersion = "1.3.17"
	revalidationIntent := queue.ValidationExecutionIntent{
		IntentID: testID(t), UploadID: uploadID, DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerRevalidation, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan:           revalidationPlan,
		RevalidationOf: &queue.ValidationExecutionReference{ValidationID: initial.ValidationID, Generation: initial.Generation},
		CreatedAt:      created.Add(2 * time.Hour),
	}
	revalidation, err := queue.NewValidationExecution(revalidationIntent)
	if err != nil {
		t.Fatal(err)
	}
	if revalidation.ValidationID == initial.ValidationID || revalidation.Generation != 1 {
		t.Fatalf("revalidation did not create a new lineage: %+v", revalidation)
	}
	if _, err := client.CreateValidationExecution(ctx, revalidation); err != nil {
		t.Fatal(err)
	}
	duplicateRevalidation, err := queue.NewValidationExecution(revalidationIntent)
	if err != nil || duplicateRevalidation.ValidationID != revalidation.ValidationID {
		t.Fatalf("duplicate revalidation command = %+v, %v", duplicateRevalidation, err)
	}
	if _, err := client.CreateValidationExecution(ctx, duplicateRevalidation); err != nil {
		t.Fatalf("duplicate revalidation create: %v", err)
	}
	unchangedIntent := revalidationIntent
	unchangedIntent.IntentID = testID(t)
	unchangedIntent.Plan = testFrozenPlan()
	unchanged, err := queue.NewValidationExecution(unchangedIntent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, unchanged); err == nil {
		t.Fatal("unchanged revalidation was accepted instead of requiring replay")
	}

	// Create-only persistence cannot overwrite the first generation.
	conflict := initial
	conflict.Plan.Version = "attacker-change"
	if _, err := client.CreateValidationExecution(ctx, conflict); !errors.Is(err, jetstream.ErrKeyExists) {
		t.Fatalf("duplicate generation error = %v, want %v", err, jetstream.ErrKeyExists)
	}
	stored, err := client.GetValidationExecution(ctx, uploadID, initial.ValidationID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, initial) {
		t.Fatalf("stored initial execution = %+v, want %+v", stored, initial)
	}

	executions, err := client.ValidationExecutionsForUpload(ctx, uploadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 3 {
		t.Fatalf("execution count = %d, want 3: %+v", len(executions), executions)
	}
	type executionIdentity struct {
		id         string
		generation uint64
	}
	seen := map[executionIdentity]bool{}
	for _, execution := range executions {
		seen[executionIdentity{execution.ValidationID, execution.Generation}] = true
	}
	if !seen[executionIdentity{initial.ValidationID, 1}] || !seen[executionIdentity{initial.ValidationID, 2}] || !seen[executionIdentity{revalidation.ValidationID, 1}] {
		t.Fatalf("listed execution identities = %+v", executions)
	}
}

func TestValidationExecutionRecordRoundTripAndValidation(t *testing.T) {
	uploadID := testID(t)
	execution, err := queue.NewValidationExecution(testInitialIntent(t, uploadID, testFrozenPlan(), time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	var decoded queue.ValidationExecution
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, execution) {
		t.Fatalf("round trip = %+v, want %+v", decoded, execution)
	}
	if execution.RetryPolicy != queue.DefaultFrozenValidationRetryPolicy() {
		t.Fatalf("default retry policy = %+v, want %+v", execution.RetryPolicy, queue.DefaultFrozenValidationRetryPolicy())
	}

	tests := []struct {
		name   string
		mutate func(*queue.ValidationExecution)
	}{
		{"schema", func(e *queue.ValidationExecution) { e.SchemaVersion++ }},
		{"validation id", func(e *queue.ValidationExecution) { e.ValidationID = "bad" }},
		{"creation intent id", func(e *queue.ValidationExecution) { e.CreationIntentID = "bad" }},
		{"upload id", func(e *queue.ValidationExecution) { e.UploadID = "bad" }},
		{"document type", func(e *queue.ValidationExecution) { e.DocumentType = "Invoice Type" }},
		{"generation", func(e *queue.ValidationExecution) { e.Generation = 0 }},
		{"initial generation", func(e *queue.ValidationExecution) { e.Generation = 2 }},
		{"profile", func(e *queue.ValidationExecution) { e.RequestedProfile = "unknown" }},
		{"unknown retry policy id", func(e *queue.ValidationExecution) { e.RetryPolicy.ID = "some-future-policy" }},
		{"unknown retry policy version", func(e *queue.ValidationExecution) { e.RetryPolicy.Version = "99" }},
		{"retry policy zero attempts", func(e *queue.ValidationExecution) { e.RetryPolicy.MaxAttempts = 0 }},
		{"retry policy excessive attempts", func(e *queue.ValidationExecution) { e.RetryPolicy.MaxAttempts = queue.MaxValidationStageAttempts + 1 }},
		{"created at", func(e *queue.ValidationExecution) { e.CreatedAt = "yesterday" }},
		{"zero created at", func(e *queue.ValidationExecution) { e.CreatedAt = time.Time{}.Format(time.RFC3339Nano) }},
		{"plan id", func(e *queue.ValidationExecution) { e.Plan.ID = "Bad Plan" }},
		{"plan id too long", func(e *queue.ValidationExecution) { e.Plan.ID = strings.Repeat("a", queue.MaxStagePlanIDBytes+1) }},
		{"plan version", func(e *queue.ValidationExecution) { e.Plan.Version = "" }},
		{"plan version too long", func(e *queue.ValidationExecution) { e.Plan.Version = strings.Repeat("v", queue.MaxStageVersionBytes+1) }},
		{"no stages", func(e *queue.ValidationExecution) { e.Plan.Stages = nil }},
		{"ordinal", func(e *queue.ValidationExecution) { e.Plan.Stages[1].Ordinal = 3 }},
		{"stage id", func(e *queue.ValidationExecution) { e.Plan.Stages[1].ID = "Bad Stage" }},
		{"stage id too long", func(e *queue.ValidationExecution) {
			e.Plan.Stages[1].ID = queue.ValidationLayer(strings.Repeat("a", queue.MaxStageIDBytes+1))
		}},
		{"duplicate stage", func(e *queue.ValidationExecution) { e.Plan.Stages[1].ID = e.Plan.Stages[0].ID }},
		{"requirement", func(e *queue.ValidationExecution) { e.Plan.Stages[0].Requirement = "sometimes" }},
		{"ruleset", func(e *queue.ValidationExecution) { e.Plan.Stages[0].RulesetVersion = "" }},
		{"ruleset too long", func(e *queue.ValidationExecution) {
			e.Plan.Stages[0].RulesetVersion = strings.Repeat("r", queue.MaxStageVersionBytes+1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := execution
			candidate.Plan.Stages = append([]queue.FrozenValidationStage(nil), execution.Plan.Stages...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatalf("Validate accepted %+v", candidate)
			}
		})
	}
}

func TestRevalidationRequiresExplicitPredecessor(t *testing.T) {
	intent := testInitialIntent(t, testID(t), testFrozenPlan(), time.Now())
	intent.Trigger = queue.ValidationTriggerRevalidation
	if _, err := queue.NewValidationExecution(intent); err == nil {
		t.Fatal("revalidation without predecessor was accepted")
	}
	intent.RevalidationOf = &queue.ValidationExecutionReference{ValidationID: "bad", Generation: 1}
	if _, err := queue.NewValidationExecution(intent); err == nil {
		t.Fatal("revalidation with malformed predecessor was accepted")
	}
}

func TestValidationExecutionLookupsRejectWrongIdentity(t *testing.T) {
	client := newExecutionTestClient(t, queue.DefaultNamespace)
	ctx := context.Background()
	uploadID := testID(t)
	validationID := testID(t)
	if _, err := client.GetValidationExecution(ctx, uploadID, validationID, 1); !errors.Is(err, queue.ErrValidationExecutionNotFound) {
		t.Fatalf("missing execution error = %v", err)
	}
	if _, err := client.GetValidationExecution(ctx, "bad", validationID, 1); !errors.Is(err, queue.ErrValidationExecutionNotFound) {
		t.Fatalf("invalid upload error = %v", err)
	}
	if _, err := client.ValidationExecutionsForUpload(ctx, "bad"); !errors.Is(err, queue.ErrValidationExecutionNotFound) {
		t.Fatalf("invalid list error = %v", err)
	}
	empty, err := client.ValidationExecutionsForUpload(ctx, uploadID)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty upload executions = %+v, %v", empty, err)
	}
}
