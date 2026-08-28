package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func testFrozenPlan() queue.FrozenValidationPlan {
	return queue.FrozenValidationPlan{
		ID: "ubl21-en16931", Version: "1",
		Stages: []queue.FrozenValidationStage{
			{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
			{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
		},
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
	return client
}

func TestValidationExecutionGenerationsPreserveHistory(t *testing.T) {
	client := newExecutionTestClient(t, queue.DefaultNamespace)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uploadID, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 8, 28, 12, 0, 0, 123, time.UTC)
	plan := testFrozenPlan()
	initial, err := queue.NewValidationExecution(uploadID, "Invoice", queue.ValidationProfileUBL21EN16931, plan, queue.ValidationTriggerInitial, created)
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

	replay, err := queue.NextValidationGeneration(initial, created.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if replay.ValidationID != initial.ValidationID || replay.Generation != 2 || replay.Trigger != queue.ValidationTriggerReplay {
		t.Fatalf("replay identity = %+v, initial = %+v", replay, initial)
	}
	if _, err := client.CreateValidationExecution(ctx, replay); err != nil {
		t.Fatal(err)
	}
	changedReplay := replay
	changedReplay.Generation = 3
	changedReplay.Plan.Stages = append([]queue.FrozenValidationStage(nil), replay.Plan.Stages...)
	changedReplay.Plan.Stages[1].RulesetVersion = "changed-during-replay"
	changedReplay.CreatedAt = created.Add(90 * time.Minute).Format(time.RFC3339Nano)
	if _, err := client.CreateValidationExecution(ctx, changedReplay); err == nil {
		t.Fatal("replay generation changed the frozen plan")
	}
	gappedReplay := replay
	gappedReplay.Generation = 4
	gappedReplay.CreatedAt = created.Add(2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := client.CreateValidationExecution(ctx, gappedReplay); err == nil {
		t.Fatal("replay generation was created without its predecessor")
	}

	revalidationPlan := testFrozenPlan()
	revalidationPlan.Version = "2"
	revalidationPlan.Stages[1].RulesetVersion = "1.3.17"
	revalidation, err := queue.NewValidationExecution(uploadID, "Invoice", queue.ValidationProfileUBL21EN16931, revalidationPlan, queue.ValidationTriggerRevalidation, created.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if revalidation.ValidationID == initial.ValidationID || revalidation.Generation != 1 {
		t.Fatalf("revalidation did not create a new lineage: %+v", revalidation)
	}
	if _, err := client.CreateValidationExecution(ctx, revalidation); err != nil {
		t.Fatal(err)
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
	uploadID, _ := queue.NewID()
	execution, err := queue.NewValidationExecution(uploadID, "Invoice", queue.ValidationProfileUBL21EN16931, testFrozenPlan(), queue.ValidationTriggerInitial, time.Now())
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

	tests := []struct {
		name   string
		mutate func(*queue.ValidationExecution)
	}{
		{"schema", func(e *queue.ValidationExecution) { e.SchemaVersion++ }},
		{"validation id", func(e *queue.ValidationExecution) { e.ValidationID = "bad" }},
		{"upload id", func(e *queue.ValidationExecution) { e.UploadID = "bad" }},
		{"document type", func(e *queue.ValidationExecution) { e.DocumentType = "Invoice Type" }},
		{"generation", func(e *queue.ValidationExecution) { e.Generation = 0 }},
		{"initial generation", func(e *queue.ValidationExecution) { e.Generation = 2 }},
		{"profile", func(e *queue.ValidationExecution) { e.RequestedProfile = "unknown" }},
		{"created at", func(e *queue.ValidationExecution) { e.CreatedAt = "yesterday" }},
		{"zero created at", func(e *queue.ValidationExecution) { e.CreatedAt = time.Time{}.Format(time.RFC3339Nano) }},
		{"plan id", func(e *queue.ValidationExecution) { e.Plan.ID = "Bad Plan" }},
		{"plan version", func(e *queue.ValidationExecution) { e.Plan.Version = "" }},
		{"no stages", func(e *queue.ValidationExecution) { e.Plan.Stages = nil }},
		{"ordinal", func(e *queue.ValidationExecution) { e.Plan.Stages[1].Ordinal = 3 }},
		{"stage id", func(e *queue.ValidationExecution) { e.Plan.Stages[1].ID = "Bad Stage" }},
		{"duplicate stage", func(e *queue.ValidationExecution) { e.Plan.Stages[1].ID = e.Plan.Stages[0].ID }},
		{"requirement", func(e *queue.ValidationExecution) { e.Plan.Stages[0].Requirement = "sometimes" }},
		{"ruleset", func(e *queue.ValidationExecution) { e.Plan.Stages[0].RulesetVersion = "" }},
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

func TestValidationExecutionLookupsRejectWrongIdentity(t *testing.T) {
	client := newExecutionTestClient(t, queue.DefaultNamespace)
	ctx := context.Background()
	uploadID, _ := queue.NewID()
	validationID, _ := queue.NewID()
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
