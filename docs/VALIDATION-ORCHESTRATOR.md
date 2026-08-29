# Durable validation orchestrator state machine

**Status:** Normative state-machine, durable-dispatch, and final-result contract
for `TODO-VALIDATION-SPLIT-005`, `TODO-VALIDATION-SPLIT-006`, and
`TODO-VALIDATION-SPLIT-009`

The `orchestrator.Machine` is the only component that selects a stage from a
frozen `ValidationExecution.Plan`, authorizes logical attempts, accepts stage
evidence, advances the plan, and records a terminal execution outcome. It does
not parse XML or manufacture validator findings.

## Durable model

Each validation execution retains immutable identity and plan fields while its
compact `orchestration` projection is updated under the exact JetStream KV
revision returned by `GetValidationExecutionVersion`. A stale writer receives
`ErrValidationExecutionConflict`; it cannot overwrite the winning transition.

The compact projection records:

- lifecycle status and current frozen-plan ordinal;
- the deterministic request ID, original request timestamp, and authorized
  attempt for each reached stage;
- a `pending` or `published` durable outbox marker and the broker-acknowledged
  publication time for the current authorized request;
- immutable result IDs and outcome classifications for accepted attempts;
- exactly one terminal stage outcome where applicable;
- update and terminal-finalization timestamps.

Complete `StageResult` payloads are create-only entries in the namespace-derived
`*_VALIDATION_STAGE_EVIDENCE` KV bucket. The execution state references those
entries by deterministic result ID. Separating full bounded diagnostics from
the projection keeps the CAS record within 64 KiB while allowing an individual
valid result to approach the queue contract's 2 MiB ceiling.

## Transitions

Starting an execution creates or idempotently loads the deterministic lineage,
then authorizes ordinal 1, attempt 1 with a pending dispatch intent:

```text
pending -> stage-queued(ordinal=1, attempt=1, dispatch=pending)
        -> publish deterministic StageRequest
        -> stage-queued(ordinal=1, attempt=1, dispatch=published)
```

`stage-queued` deliberately covers queued, worker-running, and pending-result
time. Workers own their consumer delivery state and cannot write a central
`running` flag, so the orchestrator advances this projection only when durable
result evidence arrives.

Only a result matching the exact upload, validation ID, generation, plan,
plan version, stage, ordinal, ruleset, authorized attempt, and deterministic
request ID is accepted.

```text
passed + later frozen stage
  -> terminal passed for current ordinal
  -> stage-queued(next ordinal, attempt=1)

passed + final frozen stage
  -> valid

standards-invalid
  -> invalid
  -> no later request

operational-failure + retry available
  -> immutable attempt evidence
  -> same ordinal, next authorized attempt

operational-failure + attempts exhausted
  -> processing-failed
  -> no later request
```

The default logical-attempt limit is three and is bounded to at most 32. Broker
redelivery remains the same attempt; only the orchestrator increments the
logical attempt number. Duplicate delivery of already accepted identical
evidence is a no-op and cannot advance the plan twice.

## Crash-safe dispatch and reconciliation

Request authorization and request publication are intentionally separate
durable steps. The execution CAS first records the exact request as `pending`.
Only then may an orchestrator publish it. After JetStream acknowledges the
publish, a second CAS records `published` and `dispatchedAt`.

This ordering closes both unavoidable crash windows:

```text
pending intent persisted -> crash -> reconciler publishes request

request published -> crash before published marker -> reconciler republishes
the same deterministic request ID -> broker or worker duplicate tolerance
```

`ReconcilePendingDispatches` scans all execution generations both for plain
`pending` records stranded before first-stage authorization and for queued
requests with pending outbox intents. `RunDispatchReconciler` performs an
immediate startup scan and periodic scans until shutdown. Multiple orchestrator
replicas may reconcile at the same time: the deterministic `Nats-Msg-Id`
suppresses ordinary near-term duplicate publishes and execution CAS converges
the durable marker on one winner.

If a StageResult arrives while its request still says `pending`, the result is
itself durable proof that publication occurred. Accepting the result records
the attempt's publication evidence while atomically creating either the next
pending attempt/stage intent or one terminal execution state. Duplicate
requests may therefore repeat work, but their deterministic StageResult is
accepted once; conflicting evidence is rejected.

The broker's message-ID window is only an optimization. A delayed duplicate
request may be executed again after that window expires. Its result retains the
same deterministic result ID, and application-level evidence comparison
excludes only independently observed start/completion timestamps and duration.
Semantically equivalent results therefore converge on the first permanent
evidence value and one CAS transition. Different outcomes, reasons, findings,
or diagnostics conflict and fail closed. This keeps stage workers independent
of orchestrator-owned state while making duplicate request delivery harmless
to durable validation state.

`HandleStageResultMessage` is the durable consumer boundary. It ACKs only after
the evidence create, execution CAS, and any next-stage publication succeed.
Malformed, stale, out-of-order, plan-drifted, or unauthorized results remain
unacknowledged.

## Final-result publication and reconciliation

Terminal transitions persist a deterministic execution-scoped final-result
outbox as `pending` in the same CAS that records `valid`, `invalid`, or
`processing-failed`. The orchestrator then reloads every accepted terminal
stage result from immutable evidence, validates it against the frozen plan,
publishes the canonical result, reads the permanent slot back for equality, and
CAS-marks the outbox `published`.

`ReconcilePendingFinalResults` repairs both pending outboxes and terminal
records created before the outbox field existed. One deterministic result ID
per validation ID/generation gives replay generations independent permanent
slots while preserving first-result permanence. See
[`ORCHESTRATOR-FINAL-RESULTS.md`](ORCHESTRATOR-FINAL-RESULTS.md).

Activating the split roles in place of the existing combined worker remains the
stop-and-drain migration owned by `TODO-VALIDATION-SPLIT-014` (#61).
