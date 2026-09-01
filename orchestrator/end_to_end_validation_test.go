package orchestrator_test

import (
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
	"github.com/nats-io/nats.go/jetstream"
)

// TestOrchestratorValidPlanCompletes validates orchestrator sequencing when both stages pass.
// This is orchestrator-level coverage, not deployment-level E2E. Stages are simulated via
// direct StageResult injection; real worker deployment and API flows are tested elsewhere.
func TestOrchestratorValidPlanCompletes(t *testing.T) {
	machine, client, ctx := newMachine(t)

	// Start validation with standard UBL21+EN16931 plan.
	intent := testIntent(t, testPlan())
	_, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// XSD stage receives request and passes.
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	if xsdRequest.Stage != queue.ValidationLayerUBL21XSD {
		t.Fatalf("expected XSD request, got %v", xsdRequest.Stage)
	}
	xsdResult := resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	state, err := machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatalf("XSD result failed: %v", err)
	}

	// EN stage receives request and passes.
	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	if enRequest.Stage != queue.ValidationLayerEN16931 {
		t.Fatalf("expected EN request, got %v", enRequest.Stage)
	}
	enResult := resultFor(t, enRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	state, err = machine.HandleStageResult(ctx, enResult)
	if err != nil {
		t.Fatalf("EN result failed: %v", err)
	}

	// Verify final result is valid.
	if state.Execution.Orchestration.Status != queue.ExecutionStatusValid {
		t.Fatalf("expected status Valid, got %v", state.Execution.Orchestration.Status)
	}
	if len(state.Execution.Orchestration.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(state.Execution.Orchestration.Stages))
	}

	// Verify no more requests exist.
	assertNoRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	assertNoRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

// TestOrchestratorXSDInvalidStopsBeforeEN validates orchestrator early-stop when XSD fails.
// EN stage must never be queued. This is orchestrator-level coverage; real worker and API
// flows are tested at deployment level.
func TestOrchestratorXSDInvalidStopsBeforeEN(t *testing.T) {
	machine, client, ctx := newMachine(t)

	intent := testIntent(t, testPlan())
	_, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// XSD stage fails with standards-invalid outcome.
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	xsdResult := resultFor(t, xsdRequest, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone)
	state, err := machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatalf("XSD result failed: %v", err)
	}

	// Verify final result is invalid.
	if state.Execution.Orchestration.Status != queue.ExecutionStatusInvalid {
		t.Fatalf("expected status Invalid, got %v", state.Execution.Orchestration.Status)
	}

	// Verify EN stage was never queued.
	assertNoRequest(t, ctx, client, queue.ValidationLayerEN16931)

	// Verify only XSD stage was executed.
	if len(state.Execution.Orchestration.Stages) != 1 {
		t.Fatalf("expected 1 stage (XSD only), got %d", len(state.Execution.Orchestration.Stages))
	}
}

// TestOrchestratorENInvalidAfterXSDPass validates orchestrator state when XSD passes but EN fails.
// This is orchestrator-level coverage; real worker and API flows are tested at deployment level.
func TestOrchestratorENInvalidAfterXSDPass(t *testing.T) {
	machine, client, ctx := newMachine(t)

	intent := testIntent(t, testPlan())
	_, err := machine.Start(ctx, intent)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// XSD passes.
	xsdRequest := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	xsdResult := resultFor(t, xsdRequest, queue.StageOutcomePassed, queue.StageReasonNone)
	_, err = machine.HandleStageResult(ctx, xsdResult)
	if err != nil {
		t.Fatalf("XSD result failed: %v", err)
	}

	// EN fails with standards-invalid.
	enRequest := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
	enResult := resultFor(t, enRequest, queue.StageOutcomeStandardsInvalid, queue.StageReasonNone)
	state, err := machine.HandleStageResult(ctx, enResult)
	if err != nil {
		t.Fatalf("EN result failed: %v", err)
	}

	// Verify final result is invalid.
	if state.Execution.Orchestration.Status != queue.ExecutionStatusInvalid {
		t.Fatalf("expected status Invalid, got %v", state.Execution.Orchestration.Status)
	}

	// Verify both stages were executed.
	if len(state.Execution.Orchestration.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(state.Execution.Orchestration.Stages))
	}
}

// TestOrchestratorENOutageAccumulatesWhileXSDDrains validates orchestrator behavior when EN queue builds.
// This test simulates EN outage by not consuming; it proves JetStream persistence of StageRequests
// but does NOT test actual worker shutdown/restart. Real Kubernetes scale-to-zero is tested at
// deployment level.
func TestOrchestratorENOutageAccumulatesWhileXSDDrains(t *testing.T) {
	machine, client, ctx := newMachine(t)
	const outageCount = 5

	// Start outageCount validations. Each will pass XSD but queue in EN.
	executions := make([]queue.VersionedValidationExecution, 0, outageCount)
	for i := 0; i < outageCount; i++ {
		intent := testIntent(t, testPlan())
		versioned, err := machine.Start(ctx, intent)
		if err != nil {
			t.Fatalf("Start %d failed: %v", i, err)
		}
		executions = append(executions, versioned)
	}

	// All XSD requests should be available immediately.
	xsdRequests := make([]queue.StageRequest, 0, outageCount)
	for i := 0; i < outageCount; i++ {
		xsdReq := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
		xsdRequests = append(xsdRequests, xsdReq)
		xsdResult := resultFor(t, xsdReq, queue.StageOutcomePassed, queue.StageReasonNone)
		_, err := machine.HandleStageResult(ctx, xsdResult)
		if err != nil {
			t.Fatalf("XSD result %d failed: %v", i, err)
		}
	}

	// Now EN stage is "down" (simulated by not reading EN requests for a time).
	// Verify that EN queue has accumulated outageCount requests.
	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatalf("Failed to get EN consumer: %v", err)
	}

	enRequests := make([]queue.StageRequest, 0, outageCount)
	for i := 0; i < outageCount; i++ {
		batch, err := consumer.Fetch(1, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			t.Fatalf("Fetch EN request %d failed: %v", i, err)
		}
		for msg := range batch.Messages() {
			var req queue.StageRequest
			if err := req.UnmarshalJSON(msg.Data()); err != nil {
				t.Fatalf("Unmarshal request %d failed: %v", i, err)
			}
			enRequests = append(enRequests, req)
			if err := msg.Ack(); err != nil {
				t.Fatalf("Ack request %d failed: %v", i, err)
			}
		}
	}

	if len(enRequests) != outageCount {
		t.Fatalf("expected %d EN requests accumulated, got %d", outageCount, len(enRequests))
	}

	// EN stage recovers and processes all accumulated requests.
	for i, enReq := range enRequests {
		enResult := resultFor(t, enReq, queue.StageOutcomePassed, queue.StageReasonNone)
		_, err := machine.HandleStageResult(ctx, enResult)
		if err != nil {
			t.Fatalf("EN result %d failed: %v", i, err)
		}
	}

	// Verify all executions completed successfully.
	for i, versioned := range executions {
		stored, err := client.GetValidationExecution(ctx, versioned.Execution.UploadID, versioned.Execution.ValidationID, versioned.Execution.Generation)
		if err != nil {
			t.Fatalf("GetValidationExecution %d failed: %v", i, err)
		}
		if stored.Orchestration.Status != queue.ExecutionStatusValid {
			t.Fatalf("execution %d expected status Valid, got %v", i, stored.Orchestration.Status)
		}
	}
}

// TestOrchestratorScaleENToZeroAccumulatesThenDrains validates orchestrator behavior when EN queue builds and drains.
// This test simulates scale-to-zero by not consuming EN queue; it proves queue durability and recovery sequencing
// but does NOT test actual Kubernetes replica scaling or worker lifecycle. Real scale-to-zero is tested at
// deployment level.
func TestOrchestratorScaleENToZeroAccumulatesThenDrains(t *testing.T) {
	machine, client, ctx := newMachine(t)
	const scaledCount = 3

	// Start scaledCount validations with EN scaled to zero (simulated by EN not consuming).
	for i := 0; i < scaledCount; i++ {
		intent := testIntent(t, testPlan())
		_, err := machine.Start(ctx, intent)
		if err != nil {
			t.Fatalf("Start %d failed: %v", i, err)
		}
	}

	// Process all XSD requests (workers can run at scale).
	for i := 0; i < scaledCount; i++ {
		xsdReq := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
		xsdResult := resultFor(t, xsdReq, queue.StageOutcomePassed, queue.StageReasonNone)
		_, err := machine.HandleStageResult(ctx, xsdResult)
		if err != nil {
			t.Fatalf("XSD result %d failed: %v", i, err)
		}
	}

	// Fetch accumulated EN requests (EN was scaled to zero, so they remain in queue).
	consumer, err := client.StageRequestConsumer(ctx, queue.ValidationLayerEN16931)
	if err != nil {
		t.Fatalf("Failed to get EN consumer: %v", err)
	}

	batch, err := consumer.Fetch(scaledCount, jetstream.FetchMaxWait(2*time.Second))
	if err != nil {
		t.Fatalf("Fetch accumulated EN requests failed: %v", err)
	}

	enReqs := make([]queue.StageRequest, 0, scaledCount)
	for msg := range batch.Messages() {
		var req queue.StageRequest
		if err := req.UnmarshalJSON(msg.Data()); err != nil {
			t.Fatalf("Unmarshal EN request failed: %v", err)
		}
		enReqs = append(enReqs, req)
		if err := msg.Ack(); err != nil {
			t.Fatalf("Ack EN request failed: %v", err)
		}
	}

	if len(enReqs) != scaledCount {
		t.Fatalf("expected %d accumulated EN requests, got %d", scaledCount, len(enReqs))
	}

	// Scale EN back up (restore workers) and drain accumulated requests.
	for i, enReq := range enReqs {
		enResult := resultFor(t, enReq, queue.StageOutcomePassed, queue.StageReasonNone)
		_, err := machine.HandleStageResult(ctx, enResult)
		if err != nil {
			t.Fatalf("EN result %d failed: %v", i, err)
		}
	}

	// Verify all work was completed.
	assertNoRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
	assertNoRequest(t, ctx, client, queue.ValidationLayerEN16931)
}

// TestOrchestratorPreservesSemanticEquivalence validates orchestrator state transitions for synthetic outcome combinations.
// This is orchestrator-level coverage of a small hand-written outcome matrix.
// Real semantic equivalence against the frozen regression corpus is tested at deployment level with actual
// validators and the issue #44 corpus baseline.
func TestOrchestratorPreservesSemanticEquivalence(t *testing.T) {
	machine, client, ctx := newMachine(t)

	// Test cases: (name, xsdOutcome, enOutcome, expectedFinalStatus)
	testCases := []struct {
		name           string
		xsdOutcome     queue.StageOutcome
		enOutcome      queue.StageOutcome
		expectedStatus queue.ValidationExecutionStatus
	}{
		{
			name:           "valid-both-pass",
			xsdOutcome:     queue.StageOutcomePassed,
			enOutcome:      queue.StageOutcomePassed,
			expectedStatus: queue.ExecutionStatusValid,
		},
		{
			name:           "xsd-invalid",
			xsdOutcome:     queue.StageOutcomeStandardsInvalid,
			enOutcome:      queue.StageOutcomePassed,
			expectedStatus: queue.ExecutionStatusInvalid,
		},
		{
			name:           "en-invalid-after-xsd-pass",
			xsdOutcome:     queue.StageOutcomePassed,
			enOutcome:      queue.StageOutcomeStandardsInvalid,
			expectedStatus: queue.ExecutionStatusInvalid,
		},
		{
			name:           "xsd-operational-failure",
			xsdOutcome:     queue.StageOutcomeOperationalFailure,
			enOutcome:      queue.StageOutcomePassed,
			expectedStatus: queue.ExecutionStatusStageQueued,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			intent := testIntent(t, testPlan())
			_, err := machine.Start(ctx, intent)
			if err != nil {
				t.Fatalf("Start failed: %v", err)
			}

			// Process XSD stage.
			xsdReq := takeRequest(t, ctx, client, queue.ValidationLayerUBL21XSD)
			var xsdReason queue.StageOperationalReason
			if tc.xsdOutcome == queue.StageOutcomeOperationalFailure {
				xsdReason = queue.StageReasonRuntimeFailure
			}
			xsdResult := resultFor(t, xsdReq, tc.xsdOutcome, xsdReason)
			state, err := machine.HandleStageResult(ctx, xsdResult)
			if err != nil {
				t.Fatalf("XSD result failed: %v", err)
			}

			// If XSD passed, process EN stage.
			if tc.xsdOutcome == queue.StageOutcomePassed {
				enReq := takeRequest(t, ctx, client, queue.ValidationLayerEN16931)
				var enReason queue.StageOperationalReason
				if tc.enOutcome == queue.StageOutcomeOperationalFailure {
					enReason = queue.StageReasonRuntimeUnavailable
				}
				enResult := resultFor(t, enReq, tc.enOutcome, enReason)
				state, err = machine.HandleStageResult(ctx, enResult)
				if err != nil {
					t.Fatalf("EN result failed: %v", err)
				}
			}

			// Verify expected final status.
			if state.Execution.Orchestration.Status != tc.expectedStatus {
				t.Fatalf("expected status %v, got %v", tc.expectedStatus, state.Execution.Orchestration.Status)
			}
		})
	}
}
