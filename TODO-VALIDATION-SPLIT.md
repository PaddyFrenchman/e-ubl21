# TODO-VALIDATION-SPLIT — Asynchronous validation-stage service split

**Project:** e-ubl21  
**Status:** Planned  
**Created:** 2026-08-26  
**Baseline:** current server performs UBL 2.1 XSD and EN 16931 validation inside one worker process  
**TODO namespace:** `TODO-VALIDATION-SPLIT-###`

## 1. Objective

Split the current validation worker into independently deployable and independently scalable validation services.

The immediate target is to isolate:

1. UBL 2.1 XSD validation;
2. EN 16931 validation;
3. Peppol BIS Billing validation when the Peppol roadmap is implemented.

The stages must be **asynchronous**. A validator process must never invoke the next validation stage directly and must never wait synchronously for that next stage to complete.

Instead, every validator consumes a durable stage request and publishes a durable stage result. A validation orchestrator consumes stage results, persists state, decides what stage is required next, and publishes the next stage request.

The target processing model is:

```text
                                ┌────────────────────────────┐
                                │ Validation Orchestrator    │
                                │ durable plan/state machine │
                                └─────────────┬──────────────┘
                                              │
                                              │ StageRequest: XSD
                                              v
                                   ┌──────────────────────┐
                                   │ XSD worker pool      │
                                   │ xmllint / UBL 2.1    │
                                   └──────────┬───────────┘
                                              │
                                              │ StageResult: XSD
                                              v
                                ┌────────────────────────────┐
                                │ Validation Orchestrator    │
                                └─────────────┬──────────────┘
                                              │ PASS
                                              │ StageRequest: EN16931
                                              v
                                   ┌──────────────────────┐
                                   │ EN 16931 worker pool │
                                   │ Saxon + EN XSLT      │
                                   └──────────┬───────────┘
                                              │
                                              │ StageResult: EN16931
                                              v
                                ┌────────────────────────────┐
                                │ Validation Orchestrator    │
                                └─────────────┬──────────────┘
                                              │ PASS and Peppol requested
                                              │ StageRequest: Peppol
                                              v
                                   ┌──────────────────────┐
                                   │ Peppol worker pool   │
                                   │ Saxon + Peppol rules │
                                   └──────────┬───────────┘
                                              │
                                              │ StageResult: Peppol
                                              v
                                ┌────────────────────────────┐
                                │ Validation Orchestrator    │
                                │ finalizes whole document   │
                                └────────────────────────────┘
```

The XML document remains stored once in the existing immutable Object Store. Stage queues contain only compact descriptors and stage results.

---

## 2. Why this architecture

The validation stages have very different execution costs and resource profiles.

Typical characteristics are expected to be approximately:

```text
UBL XSD       -> relatively cheap native xmllint process
EN 16931      -> comparatively expensive Java/Saxon transform
Peppol        -> another Java/Saxon/profile-specific workload
```

A monolithic validator forces all stages to scale together. If EN 16931 is the bottleneck, increasing replicas also increases idle XSD capacity and future Peppol capacity unnecessarily.

Independent worker pools allow telemetry-driven scaling such as:

```text
XSD workers       2 replicas
EN16931 workers  12 replicas
Peppol workers    4 replicas
```

The design also provides better fault isolation, queue visibility, per-stage SLOs, safer upgrades, and easier introduction of future national CIUS/profile validators.

---

## 3. Architectural principles

1. **One immutable input object.** XML stays in Object Store and is referenced by upload ID.
2. **No synchronous stage chaining.** A validator never calls another validator.
3. **Stage workers publish results only.** They do not decide the whole-document status or next stage.
4. **One orchestrator owns the validation plan and final state.** Only the orchestrator/finalizer may declare a document globally `valid`.
5. **Every queue transition is durable and idempotent.** Duplicate requests/results are expected and harmless.
6. **Standards invalidity remains distinct from operational failure.** A bad invoice is not a broken validator.
7. **Each stage has independent concurrency, prefetch, timeout, retry and DLQ policy.**
8. **Each stage has independent telemetry and can be scaled independently.**
9. **Stage output is versioned evidence.** The result records the exact schema/ruleset/runtime profile used.
10. **The architecture must support adding future stages without modifying existing validator workers.**
11. **The API remains asynchronous.** Upload returns before validation is complete.
12. **Existing first-result permanence and replay safety must be preserved.**

---

## 4. Target process roles

The project may continue to build one Go binary and one Docker image, but the image should expose several independent commands/roles:

```text
e-ubl21 serve

e-ubl21 orchestrator

e-ubl21 worker-xsd

e-ubl21 worker-en16931

e-ubl21 worker-peppol
```

Later stages may add roles such as:

```text
e-ubl21 worker-cius-ie

e-ubl21 worker-cius-fr

e-ubl21 worker-sbdh
```

Using one repository/image initially avoids unnecessary deployment/version fragmentation while still permitting each role to have a separate Kubernetes Deployment and replica count.

---

## 5. Validation plan model

The orchestrator must operate from a durable `ValidationPlan`, not hard-coded worker chaining.

Examples:

### UBL-only document

```text
plan: ubl21

1. ubl21-xsd
```

### EN 16931 billing document

```text
plan: ubl21-en16931

1. ubl21-xsd
2. en16931
```

### Peppol billing document

```text
plan: peppol-bis-billing

1. ubl21-xsd
2. en16931
3. peppol-bis-billing
```

Future country-specific plan:

```text
plan: peppol-bis-billing-ie

1. ubl21-xsd
2. en16931
3. peppol-bis-billing
4. ireland-cius
```

Each stage definition should contain at least:

```text
stage ID
required/optional status
applicability rule
ruleset/profile version
request subject
result subject
retry policy
stage timeout
```

The plan should be frozen for a validation execution so a ruleset/configuration change in the middle of processing cannot silently alter the required stage graph.

---

## 6. Proposed queue/event topology

Use explicit stage request and result subjects.

Suggested logical subjects:

```text
ubl.validation.request.xsd
ubl.validation.result.xsd

ubl.validation.request.en16931
ubl.validation.result.en16931

ubl.validation.request.peppol
ubl.validation.result.peppol
```

These must pass through the existing namespace abstraction in production, for example:

```text
eubl21.prod.validation.request.xsd
```

The physical implementation may use separate streams per stage or a small number of streams with subject filters. The selection must preserve independent retention, consumer configuration, queue depth telemetry and migration safety.

### StageRequest

A compact request should resemble:

```json
{
  "validationId": "...",
  "uploadId": "...",
  "documentType": "Invoice",
  "plan": "peppol-bis-billing",
  "stage": "en16931",
  "stageOrdinal": 2,
  "rulesetVersion": "1.3.16",
  "generation": 1,
  "createdAt": "2026-08-26T18:00:00Z"
}
```

It must never contain the raw XML.

### StageResult

A stage result event should resemble:

```json
{
  "validationId": "...",
  "uploadId": "...",
  "stage": "en16931",
  "stageOrdinal": 2,
  "rulesetVersion": "1.3.16",
  "outcome": "passed",
  "issues": [],
  "startedAt": "...",
  "completedAt": "...",
  "durationMs": 1432,
  "attempt": 1
}
```

Operational failure must be represented separately from standards invalidity, for example:

```json
{
  "stage": "en16931",
  "outcome": "operational-failure",
  "reason": "timeout"
}
```

Stage result payloads must retain the current diagnostic bounds and must never contain the invoice body.

---

# TODO-VALIDATION-SPLIT-001 — Freeze service boundaries and state ownership

## Goal

Define which process owns which decisions before changing runtime behavior.

## Work

- Define the API, orchestrator, XSD worker, EN worker and future Peppol worker responsibilities.
- Freeze the invariant that stage workers cannot finalize whole-document validity.
- Freeze the invariant that only the orchestrator decides the next stage.
- Freeze ownership of upload metadata, stage state and final result publication.
- Document which resources are shared and which are stage-specific.
- Document the crash boundaries between every durable write and publish.

## Acceptance criteria

There is a written ownership table with no state mutation performed by two unrelated roles without an explicit CAS/idempotency contract.

The normative boundary, resource-ownership and crash-recovery contract is
[`docs/VALIDATION-SPLIT-OWNERSHIP.md`](docs/VALIDATION-SPLIT-OWNERSHIP.md).

---

# TODO-VALIDATION-SPLIT-002 — Introduce durable validation execution and plan IDs

## Goal

Separate an uploaded document from one particular validation execution.

## Work

- Introduce a stable `validationId` or equivalent execution identity.
- Keep `uploadId` as the immutable-content identity.
- Persist requested validation profile/plan.
- Persist the frozen ordered stage list and selected rule versions.
- Add a generation/version field for explicit replay/revalidation.
- Ensure repeated API polling addresses the same execution consistently.
- Define whether future revalidation creates a new `validationId` or increments a validation generation.

## Acceptance criteria

One upload can be associated unambiguously with one or more validation executions without overwriting historical validation evidence.

The durable identity, generation and frozen-plan contract is
[`docs/VALIDATION-EXECUTION-IDENTITY.md`](docs/VALIDATION-EXECUTION-IDENTITY.md).

---

# TODO-VALIDATION-SPLIT-003 — Define versioned StageRequest and StageResult contracts

## Goal

Create stable queue contracts shared by all validator services.

## Work

- Define versioned Go models for `StageRequest` and `StageResult`.
- Include validation/upload identity, stage ID, ordinal, plan, generation and ruleset version.
- Include timestamps and bounded diagnostics.
- Define outcome enum:
  - `passed`;
  - `standards-invalid`;
  - `operational-failure`.
- Define bounded operational reason enum.
- Define message-ID/idempotency key construction.
- Reject unknown contract versions fail-closed.
- Add serialization/backward-compatibility tests.

## Acceptance criteria

All stage processes can exchange requests/results without importing worker-internal implementation types.

The normative versioning, identity, outcome, diagnostic-bound and
compatibility contract is
[`docs/VALIDATION-STAGE-CONTRACTS.md`](docs/VALIDATION-STAGE-CONTRACTS.md).

---

# TODO-VALIDATION-SPLIT-004 — Create namespace-derived stage queue topology

## Goal

Provision durable queues/streams for independent stage processing.

## Work

- Add namespace-derived request/result subjects for XSD and EN 16931.
- Reserve Peppol subjects now or add them when the Peppol worker lands.
- Define durable consumers independently per stage.
- Define per-stage AckWait, MaxAckPending and retention.
- Keep request messages compact.
- Ensure one stage cannot accidentally consume another stage's traffic.
- Add infrastructure migration that does not destructively replace the existing document stream.
- Add namespace-isolation tests.

## Acceptance criteria

XSD and EN worker deployments can be stopped, scaled and upgraded independently while their queues retain work durably.

The normative resource naming, retention, delivery, provisioning-ownership and
migration-safety contract is
[`docs/VALIDATION-STAGE-TOPOLOGY.md`](docs/VALIDATION-STAGE-TOPOLOGY.md).

---

# TODO-VALIDATION-SPLIT-005 — Implement the validation orchestrator state machine

## Goal

Move workflow sequencing out of the validator worker.

## Work

Implement a durable orchestrator responsible for:

1. loading the validation plan;
2. publishing the first `StageRequest`;
3. consuming `StageResult` events;
4. CAS-updating stage state;
5. deciding whether processing stops or advances;
6. publishing the next `StageRequest`;
7. finalizing the whole-document result only when the plan is terminal.

Required transitions:

```text
PENDING
  -> XSD_QUEUED
  -> XSD_RUNNING/PENDING_RESULT
  -> XSD_PASSED
  -> EN_QUEUED
  -> EN_PASSED
  -> PEPPOL_QUEUED
  -> PEPPOL_PASSED
  -> VALID
```

Standards failure:

```text
stage standards-invalid
  -> INVALID
  -> no later stage queued
```

Operational failure:

```text
stage operational-failure
  -> stage retry policy
  -> retry / stage DLQ
  -> PROCESSING_FAILED only after exhaustion
```

## Acceptance criteria

No validator service contains code that directly invokes or publishes work to a later validation stage.

The normative durable state, evidence, transition, consumer-ACK and rollout
boundary contract is
[`docs/VALIDATION-ORCHESTRATOR.md`](docs/VALIDATION-ORCHESTRATOR.md).

---

# TODO-VALIDATION-SPLIT-006 — Make orchestrator transitions crash-safe and idempotent

## Goal

Prevent lost or duplicated stage transitions.

## Work

Handle crash windows such as:

```text
stage result persisted
      |
      X process crashes before next request publish
```

and:

```text
next request published
      |
      X process crashes before acknowledging result
```

Required mechanisms:

- CAS/revision-based stage-state updates;
- deterministic StageRequest message IDs;
- durable `nextStagePending`/transition intent or equivalent;
- reconciliation of completed stage + missing next request;
- duplicate StageResult tolerance;
- duplicate StageRequest tolerance;
- no duplicate final result publication.

## Acceptance criteria

Killing the orchestrator at every transition boundary cannot lose work, skip a stage, execute a later stage prematurely, or create two permanent final results.

## Implemented contract

The normative durable outbox, CAS publication marker, permanent evidence
convergence, duplicate-tolerance, and startup/periodic reconciliation behavior
is specified in
[`docs/VALIDATION-ORCHESTRATOR.md`](docs/VALIDATION-ORCHESTRATOR.md).

---

# TODO-VALIDATION-SPLIT-007 — Extract XSD validation into `worker-xsd`

**Implementation contract:** [`docs/XSD-WORKER.md`](docs/XSD-WORKER.md)

## Goal

Make UBL 2.1 XSD validation an independent service.

## Work

- Move XSD work consumption into a dedicated command/process role.
- Consume only XSD StageRequests.
- Fetch XML by upload ID from Object Store.
- Resolve the correct UBL schema from the existing validation catalog.
- Run the current hardened xmllint adapter unchanged where possible.
- Publish a durable XSD StageResult.
- ACK the XSD request only after result publication is durable.
- Preserve timeout/cancellation/resource bounds.
- Publish typed operational-failure evidence for orchestrator-authorized XSD
  retries. Stage-specific broker delivery counters, DLQ publication, and
  operator replay are owned by `TODO-VALIDATION-SPLIT-010`.
- Add worker lifecycle and redelivery tests.

## Acceptance criteria

The XSD service can run with the EN worker completely stopped and accumulate successful XSD results without losing the documents that still require EN validation.

---

# TODO-VALIDATION-SPLIT-008 — Extract EN 16931 validation into `worker-en16931`

**Implementation contract:** [`docs/EN16931-WORKER.md`](docs/EN16931-WORKER.md)

## Goal

Make EN 16931 semantic validation an independent service.

## Work

- Consume only EN 16931 StageRequests.
- Require orchestrator evidence that XSD passed before accepting work.
- Fetch XML independently from Object Store.
- Reuse the existing pinned Saxon/EN adapter and startup verification.
- Publish a durable EN StageResult.
- ACK only after result publication is durable.
- Preserve bounded SVRL/issues/diagnostics.
- Preserve cancellation and Java process termination.
- Publish typed operational-failure evidence for orchestrator-authorized EN
  retries. Stage-specific broker delivery counters, DLQ publication, and
  operator replay are owned by `TODO-VALIDATION-SPLIT-010`.
- Reject stale requests whose validation generation/plan no longer matches durable state.

## Acceptance criteria

EN workers can be independently scaled to zero and back up without XSD workers or the API blocking synchronously.

---

# TODO-VALIDATION-SPLIT-009 — Adapt final result persistence to orchestrator ownership

## Goal

Preserve the current permanent-result guarantee after stage workers stop owning finalization.

## Work

- Make the orchestrator the only publisher of `ValidationResult`.
- Persist ordered stage evidence before final result publication.
- Preserve one permanent result per validation execution.
- Keep `ResultPending`/reconciliation semantics or replace them with an equivalent explicit transition.
- Ensure an intermediate stage result can never appear as the final API result.
- Ensure operational failure does not create a false standards result.
- Preserve raw XML retention behavior.

## Acceptance criteria

A final `valid` result exists only when every required stage in the frozen plan has a persisted `passed` result.

---

# TODO-VALIDATION-SPLIT-010 — Implement independent stage retry and DLQ policies

## Goal

Make failures attributable and recoverable per stage.

## Work

- Define independent attempt counters for XSD and EN 16931.
- Preserve the distinction between standards-invalid and operational-failure.
- Add stage-specific DLQ subjects/streams or stage labels in a common DLQ.
- Store stage, ruleset version, attempt count and reason in DLQ entries.
- Add operator replay for a failed stage without re-uploading content.
- Decide whether replay resumes from failed stage or creates a new complete validation generation.
- Default recommendation: same-generation operational replay resumes the failed stage; explicit standards/profile revalidation creates a new validation generation.
- Prevent replay from bypassing prerequisite evidence.

## Acceptance criteria

An EN runtime outage does not consume XSD retry budget, and replaying EN failure does not rerun XSD unless policy explicitly requests a new validation generation.

---

# TODO-VALIDATION-SPLIT-011 — Add per-stage telemetry and queue-delay measurement

## Goal

Make scaling decisions measurable instead of inferred from aggregate worker CPU.

## Required metrics

At minimum:

```text
validation_stage_requests_total{stage,outcome}
validation_stage_duration_seconds{stage}
validation_stage_queue_wait_seconds{stage}
validation_stage_inflight{stage}
validation_stage_retries_total{stage,reason}
validation_stage_dlq_depth{stage}
validation_stage_queue_depth{stage}
validation_stage_redeliveries_total{stage}
```

Also expose:

```text
stage request age
stage throughput docs/sec
stage process startup/runtime errors
stage result publication latency
orchestrator transition latency
```

Rules:

- no upload ID as metric label;
- no rule ID as high-cardinality metric label;
- structured logs may include upload/validation ID for correlation;
- use a correlation/trace ID across API, orchestrator and every stage event.

## Acceptance criteria

Operators can distinguish:

```text
validator is intrinsically slow
```

from:

```text
validator pool is undersized and documents are waiting in queue
```

without inspecting application internals.

---

# TODO-VALIDATION-SPLIT-012 — Add independent Kubernetes deployment and autoscaling configuration

## Goal

Turn logical isolation into operational isolation.

## Work

Create separate deployments for:

```text
api
orchestrator
xsd-worker
en16931-worker
```

and later:

```text
peppol-worker
```

Each validator deployment must have independent:

- replica count;
- CPU/memory requests and limits;
- concurrency;
- prefetch;
- stage timeout;
- AckWait;
- startup/readiness checks;
- PodDisruptionBudget where needed;
- autoscaling policy.

Prefer scaling signals based on:

1. queue depth / oldest request age;
2. measured service time / throughput;
3. CPU/memory as secondary signals.

Document how to calculate approximate required replicas:

```text
required throughput = arrival rate
per-replica throughput = concurrency / mean stage service time
replicas ~= arrival rate / per-replica throughput * safety factor
```

## Acceptance criteria

Load on EN 16931 can increase EN replica count without changing XSD/API/orchestrator replicas.

---

# TODO-VALIDATION-SPLIT-013 — Add distributed tracing/correlation across stage events

## Goal

Keep the now-distributed pipeline debuggable.

## Work

- Carry trace/correlation metadata in StageRequest/StageResult headers or payload.
- Preserve correlation across orchestrator republishes.
- Emit spans or equivalent structured timing for:
  - API acceptance;
  - orchestrator queueing;
  - queue wait;
  - XML fetch;
  - validator execution;
  - stage result publish;
  - orchestrator transition;
  - final result publish.
- Never include invoice body or sensitive business content in trace attributes.

## Acceptance criteria

One upload can be followed end-to-end through all services and queue waits without relying on timestamp guessing.

---

# TODO-VALIDATION-SPLIT-014 — Migrate existing deployments safely

## Goal

Move from the monolithic worker to asynchronous stages without duplicate or mixed-generation processing.

## Work

- Define a deployment generation marker.
- Add a stop-and-drain barrier for old monolithic workers.
- Do not allow old workers and new stage workers to process the same active queue unless compatibility is explicitly proven.
- Reconcile legacy pending uploads into a frozen validation plan.
- Preserve already-final valid/invalid results unchanged.
- Decide how to handle legacy pending Invoice/CreditNote where XSD/EN partial evidence is absent or incomplete.
- Prefer re-running the complete required plan for ambiguous legacy pending state.
- Document rollback procedure.

## Acceptance criteria

Migration cannot create a document marked `valid` under the new model with missing stage evidence.

---

# TODO-VALIDATION-SPLIT-015 — Add chaos/crash/replay integration tests

## Goal

Prove distributed stage transitions are durable under failure.

## Required scenarios

1. orchestrator crashes before first XSD publish;
2. XSD worker crashes during xmllint;
3. XSD result is published but XSD request ACK is lost;
4. orchestrator crashes after XSD state persistence but before EN request publish;
5. duplicate XSD results arrive;
6. EN worker crashes during Saxon;
7. EN result publish succeeds but ACK is lost;
8. orchestrator receives the same EN result multiple times;
9. Object Store temporarily unavailable to one stage;
10. stage result stream temporarily unavailable;
11. final result stream temporarily unavailable;
12. orchestrator restart with thousands of completed-but-not-advanced transitions;
13. worker pool scaled to zero and later restored;
14. stale previous-generation stage request arrives after revalidation starts;
15. concurrent replay commands target the same failed stage.

## Acceptance criteria

Every case converges to exactly one correct durable state and exactly one permanent final result when validation is terminal.

---

# TODO-VALIDATION-SPLIT-016 — Benchmark each stage and establish scaling baselines

## Goal

Produce stage-specific capacity numbers for autoscaling and cost decisions.

## Work

Benchmark separately:

```text
XSD queue wait + execution
EN queue wait + execution
orchestrator transition cost
```

and after Peppol exists:

```text
Peppol queue wait + execution
```

For each stage capture:

- docs/sec;
- p50/p95/p99 service time;
- CPU/doc;
- memory/doc;
- process/JVM startup contribution;
- maximum safe concurrency per pod;
- effect of document byte size;
- effect of semantic node/line count;
- retry/failure overhead.

Compare process-per-document Saxon with any future persistent-JVM option before changing architecture.

## Acceptance criteria

Each deployment has a documented initial resource request, concurrency and autoscaling threshold derived from measured stage behavior.

---

# TODO-VALIDATION-SPLIT-017 — Integrate Peppol as a native asynchronous stage

## Goal

Make the Peppol roadmap use the split architecture rather than reintroducing synchronous chaining.

## Work

- Implement `worker-peppol` as a separate process role.
- Add Peppol StageRequest/StageResult subjects.
- Extend validation plans requiring Peppol.
- Queue Peppol only after durable EN `passed` evidence.
- Ensure Peppol failure cannot modify EN evidence.
- Add independent Peppol timeout/retry/DLQ/telemetry/autoscaling.
- Update `TODO-PEPPOL.md` implementation steps so the Peppol adapter plugs into this stage architecture.

## Acceptance criteria

The orchestrator can process:

```text
XSD -> queued result -> EN -> queued result -> Peppol -> queued result -> final
```

with every arrow crossing a durable asynchronous boundary.

---

# TODO-VALIDATION-SPLIT-018 — Final end-to-end distributed validation gate

## Goal

Prove the split deployment preserves the existing validation contract while adding independent scaling and fault isolation.

## Required end-to-end scenarios

### EN 16931 plan

```text
public upload
  -> XSD request
  -> XSD result PASS
  -> EN request
  -> EN result PASS
  -> final valid result
```

### XSD-invalid Invoice

```text
public upload
  -> XSD request
  -> XSD result standards-invalid
  -> final invalid
  -> EN request count = 0
```

### EN-invalid Invoice

```text
public upload
  -> XSD PASS
  -> EN standards-invalid
  -> final invalid
```

### EN outage

```text
XSD continues draining
EN request queue accumulates
API/orchestrator/XSD remain healthy
EN workers restart/scale up
queue drains
```

### Peppol plan, once available

```text
XSD PASS
  -> EN PASS
  -> Peppol PASS
  -> final valid
```

### Scale-to-zero

Scale EN worker replicas to zero, submit valid invoices, prove XSD completes and EN work remains durable, then restore EN replicas and prove completion.

## Final acceptance criteria

The split architecture is complete when all of the following are true:

> Each validation stage runs in an independently deployable process, consumes only its own durable StageRequests, publishes only durable StageResults, and has no synchronous dependency on the next stage. The orchestrator alone advances the validation plan and finalizes document validity. A failure, restart, scale-to-zero event, or bottleneck in one stage cannot lose work, skip required validation, create duplicate final results, or require unrelated validation stages to scale with it.

---

## 7. Recommended implementation order

```text
SPLIT-001  service boundaries / ownership
    |
SPLIT-002  validation execution + plan identity
    |
SPLIT-003  StageRequest / StageResult contracts
    |
SPLIT-004  queue topology
    |
SPLIT-005  orchestrator state machine
    |
SPLIT-006  crash-safe transitions
    |
SPLIT-007  XSD worker extraction
    |
SPLIT-008  EN worker extraction
    |
SPLIT-009  final-result ownership
    |
SPLIT-010  stage retry / DLQ / replay
    |
SPLIT-011  telemetry
    |
SPLIT-013  tracing
    |
SPLIT-012  independent deployments / autoscaling
    |
SPLIT-014  migration
    |
SPLIT-015  chaos / replay gate
    |
SPLIT-016  capacity baseline
    |
SPLIT-017  Peppol asynchronous stage
    |
SPLIT-018  final distributed E2E gate
```

`TODO-PEPPOL` should depend on this roadmap at the point where the Peppol validator is integrated into the production pipeline. Peppol rule/asset work can proceed independently, but production Peppol orchestration should use `worker-peppol` + queued StageResult rather than extending the current monolithic worker.

---

## 8. Target deployment architecture

```text
                              ┌───────────────────────┐
                              │        API            │
                              │ upload / status       │
                              └───────────┬───────────┘
                                          │
                                          v
                              ┌───────────────────────┐
                              │   Object Store / KV   │
                              └───────────┬───────────┘
                                          │
                                          v
                              ┌───────────────────────┐
                              │    Orchestrator       │
                              │ validation plan       │
                              └───────────┬───────────┘
                                          │
               ┌──────────────────────────┼──────────────────────────┐
               │                          │                          │
               v                          v                          v
      XSD request queue          EN16931 request queue      Peppol request queue
               │                          │                          │
               v                          v                          v
        ┌─────────────┐            ┌─────────────┐            ┌─────────────┐
        │ XSD workers │            │ EN workers  │            │ Peppol      │
        │ N replicas  │            │ N replicas  │            │ workers     │
        └──────┬──────┘            └──────┬──────┘            └──────┬──────┘
               │                          │                          │
               v                          v                          v
       XSD result queue           EN16931 result queue       Peppol result queue
               │                          │                          │
               └──────────────────────────┼──────────────────────────┘
                                          │
                                          v
                              ┌───────────────────────┐
                              │    Orchestrator       │
                              │ CAS state / advance   │
                              └───────────┬───────────┘
                                          │
                                          v
                              ┌───────────────────────┐
                              │ Permanent final       │
                              │ validation result     │
                              └───────────────────────┘
```

This is the intended foundation for later generic JSON invoice generation and outbound delivery: generated XML enters the same validation-plan orchestrator, and only a fully successful terminal validation result may authorize a later delivery workflow.
