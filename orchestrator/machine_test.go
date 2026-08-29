package orchestrator_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/PaddyFrenchman/e-ubl21/queue/queuetest"
	"github.com/nats-io/nats.go/jetstream"
)

func TestMachineAdvancesFrozenPlanAndFinalizesValid(t *testing.T) {
	machine, client, ctx := newMachine(t)
	state, err := machine.Start(ctx, testIntent(t, testPlan()))
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued || state.Execution.Orchestration.CurrentStageOrdinal != 1 {
		t.Fatalf("initial orchestration state = %+v", state.Execution.Orchestration)
	}

	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	xsdResult := resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	state, err = machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued || state.Execution.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("post-XSD orchestration state = %+v", state.Execution.Orchestration)
	}
	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	if enRequest.StageOrdinal != 2 || enRequest.Attempt != 1 {
		t.Fatalf("EN request = %+v", enRequest)
	}
	enResult := resultFor(t, enRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	state, err = machine.HandleStageResult(ctx, enResult)
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusValid || state.Execution.Orchestration.FinalizedAt == "" {
		t.Fatalf("terminal orchestration state = %+v", state.Execution.Orchestration)
	}
	if len(state.Execution.Orchestration.Stages) != 2 || state.Execution.Orchestration.Stages[0].TerminalOutcome != queue.StageOutcomePassed || state.Execution.Orchestration.Stages[1].TerminalOutcome != queue.StageOutcomePassed {
		t.Fatalf("terminal stage state = %+v", state.Execution.Orchestration.Stages)
	}
	for _, result := range []queue.StageResult{xsdResult, enResult} {
		stored, err := client.GetValidationStageEvidence(ctx, result.UploadID, result.ValidationID, result.Generation, result.StageOrdinal, result.Attempt)
		if err != nil || stored.ResultID != result.ResultID {
			t.Fatalf("stored evidence = %+v, %v; want %s", stored, err, result.ResultID)
		}
	}
}

func TestMachineStopsOnStandardsInvalidWithoutQueueingLaterStage(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	result := resultFor(t, request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone)
	state, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusInvalid || len(state.Execution.Orchestration.Stages) != 1 {
		t.Fatalf("standards-invalid state = %+v", state.Execution.Orchestration)
	}
	assertNoRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

func TestMachineRetriesOperationalFailureAndFailsOnlyAfterExhaustion(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan(), 2)); err != nil {
		t.Fatal(err)
	}
	first := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	firstFailure := resultFor(t, first, queue.StageOutcomeOperationalFailure, queue.StageReasonTimeout)
	state, err := machine.HandleStageResult(ctx, firstFailure)
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusStageQueued || state.Execution.Orchestration.Stages[0].AuthorizedAttempt != 2 || state.Execution.Orchestration.Stages[0].TerminalOutcome != "" {
		t.Fatalf("retry state = %+v", state.Execution.Orchestration)
	}
	second := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	if second.Attempt != 2 || second.RequestID == first.RequestID {
		t.Fatalf("retry request = %+v, first = %+v", second, first)
	}
	secondFailure := resultFor(t, second, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure)
	state, err = machine.HandleStageResult(ctx, secondFailure)
	if err != nil {
		t.Fatal(err)
	}
	if state.Execution.Orchestration.Status != queue.ExecutionStatusProcessingFailed || state.Execution.Orchestration.Stages[0].TerminalOutcome != queue.StageOutcomeOperationalFailure || len(state.Execution.Orchestration.Stages[0].Attempts) != 2 {
		t.Fatalf("exhausted state = %+v", state.Execution.Orchestration)
	}
	assertNoRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

func TestMachineRejectsOutOfOrderAndPlanDrift(t *testing.T) {
	machine, client, ctx := newMachine(t)
	state, err := machine.Start(ctx, testIntent(t, testPlan()))
	if err != nil {
		t.Fatal(err)
	}
	_ = takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	unauthorized, err := queue.NewStageRequest(state.Execution, 2, 1, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.HandleStageResult(ctx, resultFor(t, unauthorized, queue.StageOutcomePassed, queue.StageReasonNone)); err == nil {
		t.Fatal("orchestrator accepted a result for a later plan stage")
	}
	current, err := client.GetValidationExecution(ctx, state.Execution.UploadID, state.Execution.ValidationID, state.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if current.Orchestration.CurrentStageOrdinal != 1 || len(current.Orchestration.Stages[0].Attempts) != 0 {
		t.Fatalf("rejected result changed durable state: %+v", current.Orchestration)
	}
}

func TestMachineRejectsFrozenStageWithoutQueueTopology(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = []queue.FrozenValidationStage{{ID: "future-stage", Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "1"}}
	intent := testIntent(t, plan)
	if _, err := machine.Start(ctx, intent); err == nil {
		t.Fatal("orchestrator persisted a plan stage without a queue topology")
	}
	executions, err := client.ValidationExecutionsForUpload(ctx, intent.UploadID)
	if err != nil {
		t.Fatal(err)
	}
	if len(executions) != 0 {
		t.Fatalf("unsupported plan created executions: %+v", executions)
	}
}

func TestStageResultConsumerAcknowledgesOnlyAfterDurableTransition(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	result := resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)
	if err := client.PublishStageResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	consumer, err := client.StageResultConsumer(ctx, queue.ValidationLayerUBL21XSD)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for message := range batch.Messages() {
		count++
		if err := machine.HandleStageResultMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("consumed %d stage results, want 1", count)
	}
	stored, err := client.GetValidationExecution(ctx, result.UploadID, result.ValidationID, result.Generation)
	if err != nil || stored.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("durable state after ACK = %+v, %v", stored.Orchestration, err)
	}
}

func TestDuplicateAcceptedResultDoesNotAdvanceTwice(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	result := resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)
	first, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	second, err := machine.HandleStageResult(ctx, result)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || len(second.Execution.Orchestration.Stages) != 2 {
		t.Fatalf("duplicate result advanced state: first=%+v second=%+v", first, second)
	}
	_ = takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

func TestConflictingDuplicateResultIsRejectedAgainstPermanentEvidence(t *testing.T) {
	machine, client, ctx := newMachine(t)
	if _, err := machine.Start(ctx, testIntent(t, testPlan())); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	accepted := resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)
	if _, err := machine.HandleStageResult(ctx, accepted); err != nil {
		t.Fatal(err)
	}
	conflicting := resultFor(t, request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone)
	if conflicting.ResultID != accepted.ResultID {
		t.Fatalf("conflicting result id = %q, want deterministic %q", conflicting.ResultID, accepted.ResultID)
	}
	if _, err := machine.HandleStageResult(ctx, conflicting); err == nil {
		t.Fatal("orchestrator accepted conflicting evidence as an identical duplicate")
	}
	stored, err := client.GetValidationStageEvidence(ctx, accepted.UploadID, accepted.ValidationID, accepted.Generation, accepted.StageOrdinal, accepted.Attempt)
	if err != nil || !reflect.DeepEqual(stored, accepted) {
		t.Fatalf("permanent evidence = %+v, %v; want %+v", stored, err, accepted)
	}
}

func TestPersistenceRejectsLatestRevisionHistoryRewrite(t *testing.T) {
	machine, client, ctx := newMachine(t)
	started, err := machine.Start(ctx, testIntent(t, testPlan(), 2))
	if err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	retrying, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomeOperationalFailure, queue.StageReasonTimeout))
	if err != nil {
		t.Fatal(err)
	}
	rewritten := retrying.Execution
	rewritten.Orchestration = started.Execution.Orchestration
	rewritten.Orchestration.UpdatedAt = retrying.Execution.Orchestration.UpdatedAt
	if _, err := client.UpdateValidationExecutionState(ctx, retrying, rewritten); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("latest revision history rewrite error = %v, want append-only rejection", err)
	}

	retryRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	terminal, err := machine.HandleStageResult(ctx, resultFor(t, retryRequest, queue.StageOutcomeOperationalFailure, queue.StageReasonRuntimeFailure))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateValidationExecutionState(ctx, terminal, terminal.Execution); err == nil || !strings.Contains(err.Error(), "no successor") {
		t.Fatalf("terminal successor error = %v, want terminal rejection", err)
	}
}

func TestStaleExecutionRevisionCannotOverwriteWinningTransition(t *testing.T) {
	machine, client, ctx := newMachine(t)
	started, err := machine.Start(ctx, testIntent(t, testPlan()))
	if err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	winner, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.UpdateValidationExecutionState(ctx, started, winner.Execution); !errors.Is(err, queue.ErrValidationExecutionConflict) {
		t.Fatalf("stale CAS error = %v, want %v", err, queue.ErrValidationExecutionConflict)
	}
}

func TestRedeliveredStartLoadsAdvancedExecutionWithoutResettingIt(t *testing.T) {
	machine, client, ctx := newMachine(t)
	intent := testIntent(t, testPlan())
	if _, err := machine.Start(ctx, intent); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	advanced, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	redelivered, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	if redelivered.Revision != advanced.Revision || redelivered.Execution.Orchestration.CurrentStageOrdinal != 2 {
		t.Fatalf("redelivered start reset or changed execution: advanced=%+v redelivered=%+v", advanced, redelivered)
	}
	_ = takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

func newMachine(t *testing.T) (*orchestrator.Machine, *queue.Client, context.Context) {
	t.Helper()
	server := queuetest.StartServer(t)
	client, err := queue.ConnectWithOptions(server.URL, queue.MustNamespace("orchestrator"), server.Options...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := client.ProvisionValidationExecutionInfra(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.ProvisionStageQueueInfra(ctx, queue.DefaultStageQueueInfraConfig()); err != nil {
		t.Fatal(err)
	}
	machine := orchestrator.New(client)
	clock := time.Date(2026, 8, 28, 20, 0, 0, 0, time.UTC)
	machine.Now = func() time.Time {
		clock = clock.Add(time.Millisecond)
		return clock
	}
	return machine, client, ctx
}

func testIntent(t *testing.T, plan queue.FrozenValidationPlan, maxAttempts ...uint32) queue.ValidationExecutionIntent {
	t.Helper()
	retryPolicy := queue.DefaultFrozenValidationRetryPolicy()
	if len(maxAttempts) != 0 {
		retryPolicy.MaxAttempts = maxAttempts[0]
	}
	return queue.ValidationExecutionIntent{
		IntentID: newID(t), UploadID: newID(t), DocumentType: "Invoice",
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfileUBL21EN16931,
		Plan: plan, RetryPolicy: retryPolicy, CreatedAt: time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC),
	}
}

func testPlan() queue.FrozenValidationPlan {
	return queue.FrozenValidationPlan{
		ID: "ubl21-en16931", Version: "1",
		Stages: []queue.FrozenValidationStage{
			{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
			{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
		},
	}
}

func resultFor(t *testing.T, request queue.StageRequest, outcome queue.StageOutcome, reason queue.StageOperationalReason) queue.StageResult {
	t.Helper()
	var issues []queue.ValidationIssue
	if outcome == queue.StageOutcomeStandardsInvalid {
		issues = []queue.ValidationIssue{{Kind: queue.ValidationIssueXSDReport, Layer: request.Stage, Severity: "error", Message: "not compliant"}}
	}
	result, err := queue.NewStageResult(request, outcome, reason, issues, "", time.Now().UTC(), time.Now().UTC(), time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func takeRequest(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) queue.StageRequest {
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
	count := 0
	for message := range batch.Messages() {
		count++
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
	if count != 1 {
		t.Fatalf("received %d %s requests, want 1", count, stage)
	}
	return request
}

func assertNoRequest(t *testing.T, ctx context.Context, client *queue.Client, stage queue.ValidationLayer) {
	t.Helper()
	consumer, err := client.StageRequestConsumer(ctx, stage)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for range batch.Messages() {
		count++
	}
	if err := batch.Error(); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("received %d unexpected %s requests", count, stage)
	}
}

func newID(t *testing.T) string {
	t.Helper()
	id, err := queue.NewID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}
