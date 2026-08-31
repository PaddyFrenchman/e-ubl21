# Chaos, crash, and replay integration gate

Issue #62 is enforced by a deterministic JetStream integration gate:

```bash
make test-validation-chaos
```

Before running the scenarios, the gate enumerates the compiled tests and fails
if any mapped test is absent. Renaming or removing a required test therefore
cannot silently reduce the 15-scenario contract.

The gate uses embedded, file-backed JetStream servers and controlled failure
injection. It does not depend on fixed sleeps for readiness or on probabilistic
process killing. Each simulated crash occurs at an explicit durable boundary,
and every recovery assertion checks the stored execution, immutable stage
evidence, queue state, and permanent final-result cardinality where applicable.

## Required scenario coverage

| # | Failure scenario | Executable coverage |
| ---: | --- | --- |
| 1 | Orchestrator crashes before first XSD publish | `TestReconcilerRepairsCrashAfterIntentBeforePublish` |
| 2 | XSD worker crashes during xmllint | `TestXSDWorkerShutdownCancelsValidationAndRedeliversRequest` |
| 3 | XSD result is published but request ACK is lost | `TestXSDWorkerHandleControlAndPublicationFailures` and duplicate-result convergence |
| 4 | Orchestrator crashes after XSD persistence but before EN publish | `TestReconcilerRepairsCrashAfterResultStateBeforeNextPublish` |
| 5 | Duplicate XSD results arrive | `TestConcurrentDuplicateResultsConvergeWithoutRedelivery` |
| 6 | EN worker crashes during Saxon | `TestENWorkerShutdownCancelsValidationAndRedeliversRequest` |
| 7 | EN result is published but request ACK is lost | `TestENWorkerHandleControlAndPublicationFailures` and duplicate-result convergence |
| 8 | Duplicate EN results arrive | `TestChaosGateDuplicateENResultsProduceOneFinalResult` |
| 9 | Object Store is temporarily unavailable to a stage | `TestXSDWorkerRecoversAfterTemporaryObjectStoreOutage` injects an open failure against already-durable XML, restores access, and verifies retry convergence |
| 10 | A stage-result stream is temporarily unavailable | `TestXSDWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds` and `TestENWorkerLeavesRequestUnacknowledgedUntilResultPublicationSucceeds` |
| 11 | The final-result stream is temporarily unavailable | `TestFinalResultPublicationFailureLeavesDurablePendingIntentForReconciliation` |
| 12 | Orchestrator restarts with thousands of completed, unadvanced transitions | `TestChaosGateRestartDrainsThousandsOfCompletedTransitions` |
| 13 | A worker pool scales to zero and is later restored | `TestENWorkerScalesFromZeroAndFinalizesWithoutXSDWorker` |
| 14 | A stale previous-generation request arrives after revalidation starts | `TestXSDWorkerRejectsStalePreviousGenerationAfterRevalidation` terminates the broker delivery without evidence, then completes the current generation |
| 15 | Concurrent replay commands target the same failed stage | `TestChaosGateConcurrentReplayCommandsConverge` |

The gate passes only when each terminal scenario converges to one correct
durable execution state and one permanent final result for that execution
generation. Duplicate delivery may repeat physical work, but it must not add a
second accepted attempt, advance the plan twice, or replace first evidence.
