package orchestrator_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/orchestrator"
	"github.com/PaddyFrenchman/e-ubl21/peppol"
	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestFinalResultPublishesOnlyAfterEveryFrozenStagePasses(t *testing.T) {
	machine, client, ctx := newMachine(t)
	intent := testIntent(t, testPlan())
	content := []byte("<Invoice>retained</Invoice>")
	if err := client.PutUploadContent(ctx, intent.UploadID, content); err != nil {
		t.Fatal(err)
	}
	started, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	if _, err := machine.HandleStageResult(ctx, resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
		t.Fatal(err)
	}
	if exists, err := client.HasValidationExecutionResult(ctx, started.Execution.ValidationID, started.Execution.Generation); err != nil || exists {
		t.Fatalf("intermediate result exists = %t, %v", exists, err)
	}

	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	terminal, err := machine.HandleStageResult(ctx, resultFor(t, enRequest, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	if terminal.Execution.Orchestration.FinalResultStatus != queue.FinalResultPublished || terminal.Execution.Orchestration.FinalResultPublishedAt == "" {
		t.Fatalf("final-result outbox = %+v", terminal.Execution.Orchestration)
	}
	result, err := client.GetValidationExecutionResult(ctx, terminal.Execution.ValidationID, terminal.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if err := result.ValidateExecutionResult(); err != nil {
		t.Fatal(err)
	}
	if result.ID != intent.UploadID || result.Status != queue.StatusValid || result.ValidatedAt != terminal.Execution.Orchestration.FinalizedAt || len(result.ValidationStages) != 2 {
		t.Fatalf("canonical valid result = %+v", result)
	}
	for index, stage := range result.ValidationStages {
		if stage.Layer != terminal.Execution.Plan.Stages[index].ID || stage.Outcome != queue.StageOutcomePassed || !stage.Valid {
			t.Fatalf("canonical stage %d = %+v", index, stage)
		}
	}
	retained, err := client.GetUploadContent(ctx, intent.UploadID)
	if err != nil || !bytes.Equal(retained, content) {
		t.Fatalf("retained raw XML = %q, %v", retained, err)
	}
}

func TestEnvelopeFinalResultPublishesAfterAllFourStages(t *testing.T) {
	tests := []struct {
		name, documentType, sample, wantProfileID string
		wantProcess                               queue.PeppolProcessProfile
	}{
		{
			name: "Invoice", documentType: "Invoice",
			sample:        "../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml",
			wantProfileID: peppol.ProfileID01, wantProcess: queue.PeppolProcessProfile01,
		},
		{
			name: "CreditNote", documentType: "CreditNote",
			sample:        "../createsamples/samples-peppol/CreditNote/Peppol-CreditNote-Profile-02.xml",
			wantProfileID: peppol.ProfileID02, wantProcess: queue.PeppolProcessProfile02,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			machine, client, ctx := newMachine(t)
			payload, err := os.ReadFile(test.sample)
			if err != nil {
				t.Fatal(err)
			}
			intent := testEnvelopeIntent(t, test.documentType)
			document := testSBDHDocument(payload)
			if err := client.PutUploadContent(ctx, intent.UploadID, document); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Start(ctx, intent); err != nil {
				t.Fatal(err)
			}
			var terminal queue.VersionedValidationExecution
			for _, stage := range intent.Plan.Stages {
				request := takeRequest(t, ctx, client, stage.ID)
				terminal, err = machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
				if err != nil {
					t.Fatalf("complete %s: %v", stage.ID, err)
				}
			}
			if terminal.Execution.Orchestration.FinalResultStatus != queue.FinalResultPublished {
				t.Fatalf("final-result state = %+v", terminal.Execution.Orchestration)
			}
			result, err := client.GetValidationExecutionResult(ctx, terminal.Execution.ValidationID, terminal.Execution.Generation)
			if err != nil {
				t.Fatal(err)
			}
			peppolStage := result.ValidationStages[2]
			if result.Status != queue.StatusValid || len(result.ValidationStages) != 4 ||
				peppolStage.ProfileID != test.wantProfileID || peppolStage.ProcessProfile != test.wantProcess {
				t.Fatalf("canonical envelope result = %+v", result)
			}
			stored, err := client.GetUploadContent(ctx, intent.UploadID)
			if err != nil || !bytes.Equal(stored, document) {
				t.Fatalf("retained envelope changed: %v", err)
			}
		})
	}
}

func TestEnvelopeInvalidityPublishesOnlyAfterThreePayloadPasses(t *testing.T) {
	machine, client, ctx := newMachine(t)
	payload, err := os.ReadFile("../createsamples/samples-peppol/Invoice/Peppol-Invoice-Profile-01.xml")
	if err != nil {
		t.Fatal(err)
	}
	intent := testEnvelopeIntent(t, "Invoice")
	if err := client.PutUploadContent(ctx, intent.UploadID, testSBDHDocument(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Start(ctx, intent); err != nil {
		t.Fatal(err)
	}
	for _, stage := range intent.Plan.Stages[:3] {
		request := takeRequest(t, ctx, client, stage.ID)
		if _, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)); err != nil {
			t.Fatalf("complete %s: %v", stage.ID, err)
		}
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerPeppolEnvelope)
	terminal, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.GetValidationExecutionResult(ctx, terminal.Execution.ValidationID, terminal.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != queue.StatusInvalid || len(result.ValidationStages) != 4 ||
		!result.ValidationStages[2].Valid || result.ValidationStages[3].Valid ||
		result.ValidationStages[3].Layer != queue.ValidationLayerPeppolEnvelope {
		t.Fatalf("canonical invalid envelope result = %+v", result)
	}
}

func testEnvelopeIntent(t *testing.T, documentType string) queue.ValidationExecutionIntent {
	t.Helper()
	return queue.ValidationExecutionIntent{
		IntentID: newID(t), UploadID: newID(t), DocumentType: documentType,
		Trigger: queue.ValidationTriggerInitial, RequestedProfile: queue.ValidationProfilePeppolEnvelope,
		Plan: queue.FrozenValidationPlan{
			ID: "peppol-envelope", Version: "1",
			Stages: []queue.FrozenValidationStage{
				{ID: queue.ValidationLayerUBL21XSD, Ordinal: 1, Requirement: queue.ValidationStageRequired, RulesetVersion: "2.1"},
				{ID: queue.ValidationLayerEN16931, Ordinal: 2, Requirement: queue.ValidationStageRequired, RulesetVersion: "1.3.16"},
				{ID: queue.ValidationLayerPeppolBISBilling, Ordinal: 3, Requirement: queue.ValidationStageRequired, RulesetVersion: "3.0.21"},
				{ID: queue.ValidationLayerPeppolEnvelope, Ordinal: 4, Requirement: queue.ValidationStageRequired, RulesetVersion: "3.0.21-envelope.1"},
			},
		},
		RetryPolicy: queue.DefaultFrozenValidationRetryPolicy(),
		CreatedAt:   time.Date(2026, 8, 28, 19, 0, 0, 0, time.UTC),
	}
}

func testSBDHDocument(payload []byte) []byte {
	return []byte(fmt.Sprintf(`<StandardBusinessDocument xmlns="%s"><StandardBusinessDocumentHeader/>%s</StandardBusinessDocument>`, peppol.SBDHNamespace, payload))
}

func TestFinalResultDistinguishesStandardsAndOperationalFailure(t *testing.T) {
	for _, test := range []struct {
		name       string
		outcome    queue.StageOutcome
		reason     queue.StageOperationalReason
		wantStatus queue.ValidationStatus
	}{
		{name: "standards invalid", outcome: queue.StageOutcomeStandardsInvalid, wantStatus: queue.StatusInvalid},
		{name: "operational exhaustion", outcome: queue.StageOutcomeOperationalFailure, reason: queue.StageReasonRuntimeFailure, wantStatus: queue.StatusProcessingFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine, client, ctx := newMachine(t)
			plan := testPlan()
			plan.Stages = plan.Stages[:1]
			if _, err := machine.Start(ctx, testIntent(t, plan, 1)); err != nil {
				t.Fatal(err)
			}
			request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
			terminal, err := machine.HandleStageResult(ctx, resultFor(t, request, test.outcome, test.reason))
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.GetValidationExecutionResult(ctx, terminal.Execution.ValidationID, terminal.Execution.Generation)
			if err != nil {
				t.Fatal(err)
			}
			stage := result.ValidationStages[0]
			if result.Status != test.wantStatus || stage.Outcome != test.outcome || stage.Valid || stage.Reason != test.reason {
				t.Fatalf("terminal result = %+v", result)
			}
			if test.outcome == queue.StageOutcomeOperationalFailure && (len(stage.Issues) != 0 || result.Error == "") {
				t.Fatalf("operational failure became ambiguous standards evidence: %+v", result)
			}
		})
	}
}

func TestFinalResultPublicationFailureLeavesDurablePendingIntentForReconciliation(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	if _, err := machine.Start(ctx, testIntent(t, plan)); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	publicationFailure := errors.New("results stream unavailable")
	machine.PublishValidationResult = func(context.Context, queue.ValidationResult) error { return publicationFailure }
	pending, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if !errors.Is(err, publicationFailure) {
		t.Fatalf("terminal transition error = %v, want %v", err, publicationFailure)
	}
	if pending.Execution.Orchestration.Status != queue.ExecutionStatusValid || pending.Execution.Orchestration.FinalResultStatus != queue.FinalResultPending {
		t.Fatalf("durable pending finalization = %+v", pending.Execution.Orchestration)
	}
	if _, err := client.GetValidationStageEvidence(ctx, request.UploadID, request.ValidationID, request.Generation, request.StageOrdinal, request.Attempt); err != nil {
		t.Fatalf("terminal evidence was not persisted before publication: %v", err)
	}
	if exists, err := client.HasValidationExecutionResult(ctx, request.ValidationID, request.Generation); err != nil || exists {
		t.Fatalf("failed publication result exists = %t, %v", exists, err)
	}

	machine.PublishValidationResult = client.PublishResult
	report, err := machine.ReconcilePendingFinalResults(ctx)
	if err != nil || report.Scanned != 1 || report.Reconciled != 1 || report.Errors != 0 {
		t.Fatalf("final-result reconcile report = %+v, %v", report, err)
	}
	stored, err := client.GetValidationExecution(ctx, request.UploadID, request.ValidationID, request.Generation)
	if err != nil || stored.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("reconciled execution = %+v, %v", stored.Orchestration, err)
	}
}

func TestFinalResultReconciliationRefusesMissingPermanentStageEvidence(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	if _, err := machine.Start(ctx, testIntent(t, plan)); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	machine.PublishValidationResult = func(context.Context, queue.ValidationResult) error { return errors.New("offline") }
	if _, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)); err == nil {
		t.Fatal("offline final-result publication unexpectedly succeeded")
	}
	evidenceBucket, err := client.JS.KeyValue(ctx, client.Namespace.ValidationStageEvidenceBucketName())
	if err != nil {
		t.Fatal(err)
	}
	evidenceKey := fmt.Sprintf("%s.%s.%d.%d.%d", request.UploadID, request.ValidationID, request.Generation, request.StageOrdinal, request.Attempt)
	if err := evidenceBucket.Delete(ctx, evidenceKey); err != nil {
		t.Fatal(err)
	}

	machine.PublishValidationResult = client.PublishResult
	report, err := machine.ReconcilePendingFinalResults(ctx)
	if err == nil || report.Scanned != 1 || report.Reconciled != 0 || report.Errors != 1 {
		t.Fatalf("missing-evidence reconcile = %+v, %v", report, err)
	}
	if exists, err := client.HasValidationExecutionResult(ctx, request.ValidationID, request.Generation); err != nil || exists {
		t.Fatalf("result without permanent stage evidence exists = %t, %v", exists, err)
	}
}

func TestAmbiguousFinalResultPublicationReconcilesToOnePermanentResult(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	if _, err := machine.Start(ctx, testIntent(t, plan)); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	ambiguous := errors.New("publication acknowledgement lost")
	machine.PublishValidationResult = func(ctx context.Context, result queue.ValidationResult) error {
		if err := client.PublishResult(ctx, result); err != nil {
			return err
		}
		return ambiguous
	}
	pending, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone))
	if !errors.Is(err, ambiguous) || pending.Execution.Orchestration.FinalResultStatus != queue.FinalResultPending {
		t.Fatalf("ambiguous finalization = %+v, %v", pending.Execution.Orchestration, err)
	}
	if exists, err := client.HasValidationExecutionResult(ctx, request.ValidationID, request.Generation); err != nil || !exists {
		t.Fatalf("ambiguously published result exists = %t, %v", exists, err)
	}

	machine.PublishValidationResult = client.PublishResult
	if report, err := machine.ReconcilePendingFinalResults(ctx); err != nil || report.Reconciled != 1 {
		t.Fatalf("ambiguous finalization reconcile = %+v, %v", report, err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.State.Msgs != 1 {
		t.Fatalf("ambiguous publication stored %d results, want 1", info.State.Msgs)
	}
}

func TestConcurrentFinalResultReconcilersConverge(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	if _, err := machine.Start(ctx, testIntent(t, plan)); err != nil {
		t.Fatal(err)
	}
	request := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	machine.PublishValidationResult = func(context.Context, queue.ValidationResult) error { return errors.New("offline") }
	if _, err := machine.HandleStageResult(ctx, resultFor(t, request, queue.StageOutcomePassed, queue.StageReasonNone)); err == nil {
		t.Fatal("offline final-result publication unexpectedly succeeded")
	}

	const replicas = 8
	start := make(chan struct{})
	errorsSeen := make(chan error, replicas)
	var group sync.WaitGroup
	for range replicas {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			replica := orchestrator.New(client)
			replica.Now = func() time.Time { return time.Date(2026, 8, 28, 22, 0, 0, 0, time.UTC) }
			if _, err := replica.ReconcilePendingFinalResults(context.Background()); err != nil {
				errorsSeen <- err
			}
		}()
	}
	close(start)
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		t.Errorf("concurrent final-result reconciliation: %v", err)
	}
	stored, err := client.GetValidationExecution(ctx, request.UploadID, request.ValidationID, request.Generation)
	if err != nil || stored.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("concurrent finalization state = %+v, %v", stored.Orchestration, err)
	}
	stream, err := client.JS.Stream(ctx, client.Namespace.ResultsStreamName())
	if err != nil {
		t.Fatal(err)
	}
	info, err := stream.Info(ctx)
	if err != nil || info.State.Msgs != 1 {
		t.Fatalf("concurrent finalization stored %d results, %v", info.State.Msgs, err)
	}
}

func TestReplayGenerationGetsIndependentPermanentResult(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	first, err := machine.Start(ctx, testIntent(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	firstRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	first, err = machine.HandleStageResult(ctx, resultFor(t, firstRequest, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := queue.NextValidationGeneration(first.Execution, newID(t), time.Date(2026, 8, 28, 19, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, replay); err != nil {
		t.Fatal(err)
	}
	versioned, err := client.GetValidationExecutionVersion(ctx, replay.UploadID, replay.ValidationID, replay.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.EnsureDispatch(ctx, versioned); err != nil {
		t.Fatal(err)
	}
	replayRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	second, err := machine.HandleStageResult(ctx, resultFor(t, replayRequest, queue.StageOutcomePassed, queue.StageReasonNone))
	if err != nil {
		t.Fatal(err)
	}
	firstResult, err := client.GetValidationExecutionResult(ctx, first.Execution.ValidationID, first.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	secondResult, err := client.GetValidationExecutionResult(ctx, second.Execution.ValidationID, second.Execution.Generation)
	if err != nil {
		t.Fatal(err)
	}
	if firstResult.ID != secondResult.ID || firstResult.ResultID == secondResult.ResultID || firstResult.Generation != 1 || secondResult.Generation != 2 {
		t.Fatalf("execution-scoped results = first %+v, second %+v", firstResult, secondResult)
	}
}

func TestFinalResultReconcilerClaimsTerminalExecutionWithoutOutboxMarker(t *testing.T) {
	machine, client, ctx := newMachine(t)
	plan := testPlan()
	plan.Stages = plan.Stages[:1]
	execution, err := queue.NewValidationExecution(testIntent(t, plan))
	if err != nil {
		t.Fatal(err)
	}
	requestCreatedAt := time.Date(2026, 8, 28, 19, 1, 0, 0, time.UTC)
	request, err := queue.NewStageRequest(execution, 1, 1, requestCreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	dispatchedAt := requestCreatedAt.Add(time.Millisecond)
	completedAt := dispatchedAt.Add(time.Millisecond)
	evidence, err := queue.NewStageResult(request, queue.StageOutcomePassed, queue.StageReasonNone, nil, "", dispatchedAt, completedAt, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationStageEvidence(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	execution.Orchestration = queue.ValidationOrchestrationState{
		Status: queue.ExecutionStatusValid, CurrentStageOrdinal: 1,
		Stages: []queue.ValidationStageState{{
			Stage: request.Stage, Ordinal: 1, AuthorizedAttempt: 1, RequestID: request.RequestID, RequestCreatedAt: request.CreatedAt,
			DispatchStatus: queue.StageDispatchPublished, DispatchedAt: dispatchedAt.Format(time.RFC3339Nano),
			Attempts: []queue.ValidationStageAttempt{{
				Attempt: 1, RequestID: request.RequestID, ResultID: evidence.ResultID, DispatchedAt: dispatchedAt.Format(time.RFC3339Nano), Outcome: queue.StageOutcomePassed,
			}},
			TerminalOutcome: queue.StageOutcomePassed,
		}},
		UpdatedAt: completedAt.Format(time.RFC3339Nano), FinalizedAt: completedAt.Format(time.RFC3339Nano),
	}
	if err := execution.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateValidationExecution(ctx, execution); err != nil {
		t.Fatal(err)
	}

	report, err := machine.ReconcilePendingFinalResults(ctx)
	if err != nil || report.Scanned != 1 || report.Reconciled != 1 || report.Errors != 0 {
		t.Fatalf("markerless terminal reconcile = %+v, %v", report, err)
	}
	stored, err := client.GetValidationExecution(ctx, execution.UploadID, execution.ValidationID, execution.Generation)
	if err != nil || stored.Orchestration.FinalResultStatus != queue.FinalResultPublished {
		t.Fatalf("claimed terminal execution = %+v, %v", stored.Orchestration, err)
	}
}
