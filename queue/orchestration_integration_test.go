package queue_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
)

func TestDurableOrchestrationRetriesAndPersistsTerminalEvidence(t *testing.T) {
	client := newExecutionTestClient(t, queue.MustNamespace("queue.orchestration"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}

	plan := testFrozenPlan()
	plan.Stages = plan.Stages[:1]
	intent := testInitialIntent(t, testID(t), plan, time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC))
	intent.RetryPolicy = queue.DefaultFrozenValidationRetryPolicy()
	intent.RetryPolicy.MaxAttempts = 2
	machine := orchestrator.New(client)
	now := intent.CreatedAt
	machine.Now = func() time.Time {
		now = now.Add(time.Millisecond)
		return now
	}

	started, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	first := fetchCoverageStageRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	firstResult := coverageStageResult(t, first, queue.StageOutcomeOperationalFailure, queue.StageReasonTimeout)
	retrying, err := machine.HandleStageResult(ctx, firstResult)
	if err != nil {
		t.Fatal(err)
	}
	if retrying.Execution.Orchestration.Stages[0].AuthorizedAttempt != 2 {
		t.Fatalf("retry state = %+v", retrying.Execution.Orchestration)
	}
	second := fetchCoverageStageRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	secondResult := coverageStageResult(t, second, queue.StageOutcomePassed, queue.StageReasonNone)
	terminal, err := machine.HandleStageResult(ctx, secondResult)
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.Status != queue.ExecutionStatusValid || terminal.Execution.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("terminal state = %+v", terminal.Execution.Orchestration)
	}

	for _, result := range []queue.StageResult{firstResult, secondResult} {
		stored, err := client.GetValidationStageEvidence(ctx, result.UploadID, result.ValidationID, result.Generation, result.StageOrdinal, result.Attempt)
		if err != nil {
			t.Fatal(err)
		}
		if !queue.SemanticallyEqualStageResults(stored, result) {
			t.Fatalf("stored evidence = %+v, want %+v", stored, result)
		}
		if _, err := client.CreateValidationStageEvidence(ctx, result); err != nil {
			t.Fatalf("idempotent evidence create: %v", err)
		}
	}
	if _, err := client.GetValidationStageEvidence(ctx, started.Execution.UploadID, started.Execution.ValidationID, 0, 1, 1); err == nil {
		t.Fatal("invalid evidence identity was accepted")
	}
}

func TestExecutionRecoveryScansFindOnlyPendingOutboxWork(t *testing.T) {
	client := newExecutionTestClient(t, queue.MustNamespace("queue.recovery"))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	if got, err := client.ValidationExecutionsRequiringDispatch(ctx); err != nil || len(got) != 0 {
		t.Fatalf("empty dispatch scan = %+v, %v", got, err)
	}
	if got, err := client.ValidationExecutionsRequiringFinalResult(ctx); err != nil || len(got) != 0 {
		t.Fatalf("empty final scan = %+v, %v", got, err)
	}

	plan := testFrozenPlan()
	plan.Stages = plan.Stages[:1]
	intent := testInitialIntent(t, testID(t), plan, time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC))
	execution, err := queue.NewValidationExecution(intent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}
	pending, err := client.ValidationExecutionsRequiringDispatch(ctx)
	if err != nil || len(pending) != 1 || pending[0].Execution.Orchestration.Status != queue.ExecutionStatusPending {
		t.Fatalf("pending execution scan = %+v, %v", pending, err)
	}

	machine := orchestrator.New(client)
	machine.PublishStageRequest = func(context.Context, queue.StageRequest) error { return errors.New("broker unavailable") }
	if _, err := machine.EnsureDispatch(ctx, pending[0]); err == nil {
		t.Fatal("failed request publication unexpectedly succeeded")
	}
	queued, err := client.ValidationExecutionsRequiringDispatch(ctx)
	if err != nil || len(queued) != 1 || queued[0].Execution.Orchestration.Stages[0].DispatchStatus != queue.StageDispatchPending {
		t.Fatalf("pending request scan = %+v, %v", queued, err)
	}
	machine.PublishStageRequest = client.PublishStageRequest
	if _, err := machine.EnsureDispatch(ctx, queued[0]); err != nil {
		t.Fatal(err)
	}
	if got, err := client.ValidationExecutionsRequiringDispatch(ctx); err != nil || len(got) != 0 {
		t.Fatalf("published request remained pending: %+v, %v", got, err)
	}

	request := fetchCoverageStageRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	machine.PublishValidationResult = func(context.Context, queue.ValidationResult) error { return errors.New("result broker unavailable") }
	if _, err := machine.HandleStageResult(ctx, coverageStageResult(t, request, queue.StageOutcomePassed, queue.StageReasonNone)); err == nil {
		t.Fatal("failed final-result publication unexpectedly succeeded")
	}
	finals, err := client.ValidationExecutionsRequiringFinalResult(ctx)
	if err != nil || len(finals) != 1 || finals[0].Execution.Orchestration.FinalResultStatus != queue.FinalResultPending {
		t.Fatalf("pending final-result scan = %+v, %v", finals, err)
	}
	machine.PublishValidationResult = client.PublishResult
	if _, err := machine.EnsureFinalResult(ctx, finals[0]); err != nil {
		t.Fatal(err)
	}
	if got, err := client.ValidationExecutionsRequiringFinalResult(ctx); err != nil || len(got) != 0 {
		t.Fatalf("published final result remained pending: %+v, %v", got, err)
	}
}

func TestValidationExecutionCASRejectsInvalidAndStaleWriters(t *testing.T) {
	client := newExecutionTestClient(t, queue.MustNamespace("queue.cas"))
	ctx := context.Background()
	plan := testFrozenPlan()
	plan.Stages = plan.Stages[:1]
	execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), plan, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := client.CreateValidationExecution(ctx, execution)
	if err != nil {
		t.Fatal(err)
	}
	current := queue.VersionedValidationExecution{Execution: execution, Revision: revision}
	if _, err := client.UpdateValidationExecutionState(ctx, queue.VersionedValidationExecution{Execution: execution}, execution); err == nil {
		t.Fatal("zero CAS revision was accepted")
	}
	invalidCurrent := current
	invalidCurrent.Execution.SchemaVersion++
	if _, err := client.UpdateValidationExecutionState(ctx, invalidCurrent, execution); err == nil {
		t.Fatal("invalid current record was accepted")
	}
	invalidNext := execution
	invalidNext.SchemaVersion++
	if _, err := client.UpdateValidationExecutionState(ctx, current, invalidNext); err == nil {
		t.Fatal("invalid successor was accepted")
	}
	identityDrift := execution
	identityDrift.Plan.Version = "2"
	if _, err := client.UpdateValidationExecutionState(ctx, current, identityDrift); err == nil {
		t.Fatal("immutable identity drift was accepted")
	}
	stale := current
	stale.Revision++
	if _, err := client.UpdateValidationExecutionState(ctx, stale, execution); !errors.Is(err, queue.ErrValidationExecutionConflict) {
		t.Fatalf("stale CAS error = %v", err)
	}
	mismatched := current
	mismatched.Execution.Orchestration.InitialDispatchTrace = nil
	mismatched.Execution.Orchestration.UpdatedAt = time.Now().UTC().Add(time.Minute).Format(time.RFC3339Nano)
	if _, err := client.UpdateValidationExecutionState(ctx, mismatched, mismatched.Execution); !errors.Is(err, queue.ErrValidationExecutionConflict) {
		t.Fatalf("mismatched durable record error = %v", err)
	}
}

func TestExecutionRecoveryScansFailClosedOnCorruptDurableRecords(t *testing.T) {
	tests := []struct {
		name    string
		scan    func(context.Context, *queue.Client) error
		payload func(*testing.T) []byte
		key     string
	}{
		{"dispatch malformed JSON", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringDispatch(ctx)
			return err
		}, func(*testing.T) []byte { return []byte("{") }, "malformed"},
		{"dispatch invalid execution", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringDispatch(ctx)
			return err
		}, func(*testing.T) []byte { return []byte(`{}`) }, "invalid"},
		{"dispatch identity mismatch", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringDispatch(ctx)
			return err
		}, durableCoverageExecution, "wrong.identity.1"},
		{"final malformed JSON", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringFinalResult(ctx)
			return err
		}, func(*testing.T) []byte { return []byte("{") }, "malformed"},
		{"final invalid execution", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringFinalResult(ctx)
			return err
		}, func(*testing.T) []byte { return []byte(`{}`) }, "invalid"},
		{"final identity mismatch", func(ctx context.Context, c *queue.Client) error {
			_, err := c.ValidationExecutionsRequiringFinalResult(ctx)
			return err
		}, durableCoverageExecution, "wrong.identity.1"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newExecutionTestClient(t, queue.MustNamespace("queue.corrupt"+string(rune('a'+index))))
			ctx := context.Background()
			kv, err := client.JS.KeyValue(ctx, client.Namespace.ValidationExecutionsBucketName())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := kv.Put(ctx, test.key, test.payload(t)); err != nil {
				t.Fatal(err)
			}
			if err := test.scan(ctx, client); err == nil {
				t.Fatal("corrupt durable execution was ignored")
			}
		})
	}
}

func durableCoverageExecution(t *testing.T) []byte {
	t.Helper()
	plan := testFrozenPlan()
	plan.Stages = plan.Stages[:1]
	execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), plan, time.Now().UTC()))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(execution)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestExecutionLookupsFailClosedOnCorruptDurableRecords(t *testing.T) {
	for index, mode := range []string{"malformed", "invalid", "identity"} {
		t.Run("get "+mode, func(t *testing.T) {
			client := newExecutionTestClient(t, queue.MustNamespace("queue.getcorrupt"+string(rune('a'+index))))
			ctx := context.Background()
			plan := testFrozenPlan()
			plan.Stages = plan.Stages[:1]
			execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), plan, time.Now().UTC()))
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("{")
			if mode == "invalid" {
				payload = []byte(`{}`)
			} else if mode == "identity" {
				other, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), plan, time.Now().UTC()))
				if err != nil {
					t.Fatal(err)
				}
				payload, err = json.Marshal(other)
				if err != nil {
					t.Fatal(err)
				}
			}
			kv, err := client.JS.KeyValue(ctx, client.Namespace.ValidationExecutionsBucketName())
			if err != nil {
				t.Fatal(err)
			}
			key := execution.UploadID + "." + execution.ValidationID + ".1"
			if _, err := kv.Put(ctx, key, payload); err != nil {
				t.Fatal(err)
			}
			if _, err := client.GetValidationExecutionVersion(ctx, execution.UploadID, execution.ValidationID, 1); err == nil {
				t.Fatal("corrupt stored execution was returned")
			}
		})

		t.Run("list "+mode, func(t *testing.T) {
			client := newExecutionTestClient(t, queue.MustNamespace("queue.listcorrupt"+string(rune('a'+index))))
			ctx := context.Background()
			plan := testFrozenPlan()
			plan.Stages = plan.Stages[:1]
			execution, err := queue.NewValidationExecution(testInitialIntent(t, testID(t), plan, time.Now().UTC()))
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("{")
			if mode == "invalid" {
				payload = []byte(`{}`)
			} else if mode == "identity" {
				payload, err = json.Marshal(execution)
				if err != nil {
					t.Fatal(err)
				}
			}
			kv, err := client.JS.KeyValue(ctx, client.Namespace.ValidationExecutionsBucketName())
			if err != nil {
				t.Fatal(err)
			}
			key := execution.UploadID + ".malformed.1"
			if _, err := kv.Put(ctx, key, payload); err != nil {
				t.Fatal(err)
			}
			if _, err := client.ValidationExecutionsForUpload(ctx, execution.UploadID); err == nil {
				t.Fatal("corrupt listed execution was returned")
			}
		})
	}
}

func fetchCoverageStageRequest(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) queue.StageRequest {
	t.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	var request queue.StageRequest
	for message := range batch.Messages() {
		if err := request.UnmarshalJSON(message.Data()); err != nil {
			t.Fatal(err)
		}
		if err := message.Ack(); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	if request.RequestID == "" {
		t.Fatalf("no %s stage request received", stage)
	}
	return request
}

func coverageStageResult(t *testing.T, request queue.StageRequest, outcome queue.StageOutcome, reason queue.StageOperationalReason) queue.StageResult {
	t.Helper()
	completedAt := time.Now().UTC()
	result, err := queue.NewStageResult(request, outcome, reason, nil, "temporary infrastructure failure", completedAt.Add(-time.Millisecond), completedAt, time.Millisecond)
	if outcome == queue.StageOutcomePassed {
		result, err = queue.NewStageResult(request, outcome, reason, nil, "", completedAt.Add(-time.Millisecond), completedAt, time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	return result
}
