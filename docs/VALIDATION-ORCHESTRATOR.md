# Durable validation orchestrator state machine

**Status:** Normative state-machine contract for `TODO-VALIDATION-SPLIT-005`

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
then authorizes ordinal 1, attempt 1:

```text
pending -> stage-queued(ordinal=1, attempt=1)
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

`HandleStageResultMessage` is the durable consumer boundary. It ACKs only after
the evidence create, execution CAS, and any next-stage publication succeed.
Malformed, stale, out-of-order, plan-drifted, or unauthorized results remain
unacknowledged.

## Deliberate rollout boundaries

This issue supplies and tests the state machine without switching the existing
API or monolithic worker to it. Worker extraction and the migration barrier are
later validation-split issues.

The state transition is persisted before its request is published. An explicit
`EnsureDispatch` can safely republish the exact stored request, including its
original timestamp, using the deterministic message ID. A durable transition
outbox and automatic crash-window reconciliation belong to
`TODO-VALIDATION-SPLIT-006` (#53).

Terminal `valid`, `invalid`, and `processing-failed` are authoritative execution
outcomes. Publishing the canonical public `ValidationResult` is deliberately
left to `TODO-VALIDATION-SPLIT-009` (#56), because the current permanent result
subject is keyed only by upload ID and cannot safely represent multiple
validation lineages and generations. Until that contract is adapted, no split
stage result is exposed as a final API result.
