# Validation-split service boundaries and state ownership

**Status:** Normative architecture contract
**Applies to:** `TODO-VALIDATION-SPLIT-001` and every later validation-split change

This document freezes decision and mutation ownership for the asynchronous
validation architecture. Later work may change schemas, identifiers, subjects,
or the physical placement of resources, but it must preserve these boundaries
unless this contract is deliberately revised.

The current monolithic worker is transitional: it validates multiple stages,
updates `UploadRecord`, and publishes a final result. That implementation is not
precedent for the split architecture and is not changed by this issue.

## Capabilities and decision authority

A replicated deployment is one logical role. Replicas of that role may contend
under the same CAS and idempotency rules; they are not unrelated writers. Each
stage worker role includes its delivery/retry controller: that controller is
not a separate service role and may mutate only the matching stage's delivery,
attempt and DLQ state.

| Role | Owns | Must not do |
| --- | --- | --- |
| API / ingress reconciler | Authenticate and validate the request envelope; claim upload-request idempotency keys; allocate the upload ID; create acceptance metadata; store immutable XML; durably request validation; expose read-only status projections. | Select or dispatch stages; write execution or stage state; publish a stage request or final result; validate invoice semantics. |
| Orchestrator / transition reconciler | Create the frozen validation execution and plan; accept stage evidence; choose the first and every subsequent stage; persist transition intent; decide terminal document outcome; publish the final result. | Parse or validate invoice XML; change upload acceptance facts; manufacture stage evidence; delete content. |
| XSD worker pool | Execute exactly the requested XSD stage and publish its result; ACK/NAK its own stage-request consumer delivery. | Select another stage; write upload or execution state; finalize document validity; publish the final result; delete content. |
| EN 16931 worker pool | Execute exactly the requested EN stage and publish its result; ACK/NAK its own stage-request consumer delivery. | Select another stage; write upload or execution state; finalize document validity; publish the final result; delete content. |
| Future Peppol worker pool | Execute exactly the requested Peppol stage and publish its result; ACK/NAK its own stage-request consumer delivery. | Select another stage; write upload or execution state; finalize document validity; publish the final result; delete content. |
| Retention controller | Evaluate the frozen retention rule; delete eligible raw XML; record content unavailability. | Change validation evidence or outcomes; delete content before eligibility; overwrite content. |
| Operator / replay controller | Submit an auditable replay command or intent. | Rewrite accepted evidence or final results; bypass the orchestrator by publishing stage requests directly. |
| Infrastructure provisioner | Create and mutate stream, KV, Object Store, and consumer definitions. | Mutate business records. Runtime services verify compatible infrastructure and readiness but do not reconfigure it. |

Only the API accepts an upload and its requested profile. Only the orchestrator
resolves that request to a frozen plan, chooses stage order, and finalizes the
whole-document outcome. A stage worker decides only the outcome of its own
stage. Retention alone decides content deletion eligibility under the frozen
retention rule.

## Durable resource ownership

Sharing a physical stream or store does not grant shared mutation rights.
Publication of a message and mutation of a durable consumer's delivery/ACK
state are separate mutation domains.

| Logical resource | Authoritative writer | Readers / other mutation domain | Required concurrency contract |
| --- | --- | --- | --- |
| Raw XML Object Store, keyed by upload ID | API, create-only | Stage workers read; retention deletes | No overwrite. A repeated identical create is idempotent; conflicting content fails. Deletion is eligible only after the terminal retention deadline. |
| Upload-request idempotency record | API / ingress reconciler | API reads | Create-only binding from authenticated client scope plus `Idempotency-Key` to request fingerprint and upload ID. Same key and fingerprint returns the original acceptance; same key with a different fingerprint is a conflict. |
| Upload acceptance metadata | API / ingress reconciler | API and orchestrator read | Create plus CAS-only monotonic repair. No generic cross-role `PutUploadRecord`. |
| Validation-start intent | API publishes | Orchestrator mutates only its consumer ACK state | Deterministic message ID; duplicate delivery must start at most one logical execution. |
| Validation execution and frozen plan | Orchestrator | API reads a projection | Orchestrator replicas use CAS. Identity and plan details are defined by `TODO-VALIDATION-SPLIT-002`. |
| Transition intent / outbox | Orchestrator | Orchestrator reconciler reads | CAS state transition plus deterministic publication identity; a pending intent survives crashes. |
| Per-stage request stream | Orchestrator publishes | The matching worker pool mutates only its durable consumer ACK state | Deterministic request ID. Duplicate publication and delivery are expected. |
| Per-stage result stream | Matching stage worker role publishes | Orchestrator mutates only its durable consumer ACK state | One deterministic result ID per logical attempt. Duplicate delivery of that attempt is expected and cannot replace its evidence. |
| Immutable attempt evidence | Orchestrator, inside execution state | API reads a projection | Validate execution, generation, stage, ordinal and attempt, then CAS. The first accepted result for an attempt is permanent; later attempts append evidence rather than replacing it. |
| Authoritative terminal stage outcome | Orchestrator, inside execution state | API reads a projection | CAS-only derivation from immutable attempt evidence. `passed` and `standards-invalid` are terminal immediately; an operational failure is terminal only when retry policy is exhausted or forbids retry. Exactly one terminal outcome is recorded per stage ordinal. |
| Per-stage delivery-retry and DLQ state | Matching stage worker role, including its delivery/retry controller | Orchestrator and operator read | The worker role alone owns broker delivery count and DLQ publication for its stage, using idempotent delivery and DLQ identities. It cannot authorize another logical stage attempt. |
| Logical stage-attempt authorization | Orchestrator, inside execution state | Matching stage worker reads it from the request; operator may submit replay intent | CAS against the current plan ordinal and prior immutable attempt evidence. Replay creates a new authorized attempt and never edits history. |
| Final result stream | Orchestrator publishes | API and external consumers read | One permanent first-result slot per validation execution; deterministic publication and reconciliation. |
| Content expiry / tombstone | Retention controller | API reads | CAS monotonic state. Deleting an already absent object is success. |
| Infrastructure definitions | Infrastructure provisioner | Services verify compatibility | Declarative, versioned provisioning; no runtime-service writers. |
| Logs, metrics and traces | Each role for its own activity | Observability systems read | Non-authoritative: telemetry can never repair or override business state. |

Upload acceptance and validation execution are distinct state aggregates. API
status is a read projection across those aggregates, not an API-owned mutable
copy of execution state. Workers do not write a central `running` flag; the
orchestrator derives and persists execution transitions from durable facts.

Any future proposal with two unrelated writers to one authoritative field is
invalid unless this table is amended with all of the following:

1. disjoint mutation authority or a named CAS precondition;
2. a deterministic idempotency key;
3. conflict and stale-write behavior;
4. crash recovery and replay behavior;
5. proof that attempt evidence, terminal stage outcomes and final results remain
   immutable.

## Upload request idempotency

The target upload API accepts an `Idempotency-Key` supplied by the client. Its
scope is the authenticated client or tenant, never global. The API allocates a
candidate upload ID, then atomically creates the scoped key binding before it
creates acceptance metadata. The binding contains that upload ID and a stable
fingerprint of the invoice bytes plus every request option that affects the
validation plan. Losing a concurrent create discards its unused candidate ID
and follows the existing binding.

A retry with the same scoped key and fingerprint returns the existing upload ID
and current acceptance status, including when the first request crashed before
returning a response. Reusing the key with a different fingerprint is a
conflict and cannot modify the original upload. Concurrent claims converge by
create/CAS; the losing API replica reads and applies the existing binding.

A POST without an idempotency key is a new upload request. Retrying such a POST
after an ambiguous HTTP failure may create a second upload and execution. It
must not alter or duplicate work within the first upload. Exact HTTP status,
header validation and retention details remain part of the later wire-contract
work, but no implementation may promise retry-safe HTTP acceptance without this
binding.

## Attempt and terminal stage semantics

A stage ordinal may have one or more logical attempts. Broker redelivery caused
by a crash remains the same logical attempt; only the orchestrator may authorize
and dispatch the next attempt. Each accepted attempt result is immutable:

```text
attempt 1: operational-failure (immutable)
attempt 2: operational-failure (immutable)
attempt 3: passed              (immutable)
                                |
                                v
terminal stage outcome: passed (immutable)
```

An operational failure is evidence about an attempt, not necessarily the
terminal stage outcome. The orchestrator evaluates the frozen retry policy. It
either records and dispatches a new attempt, or records an exhausted/forbidden
operational failure as the terminal stage outcome. A `passed` or
`standards-invalid` attempt immediately supplies the terminal stage outcome.
Once present, that outcome cannot be replaced and the plan can advance or stop
exactly once.

## Crash boundaries

Every sequence below crosses independent durable operations. Recovery must work
at each boundary without relying on an in-memory transaction.

| Boundary | Durable recovery contract |
| --- | --- |
| API claims an idempotency key, then crashes before creating acceptance metadata | The scoped binding retains the allocated upload ID. An identical keyed client retry resumes that upload under CAS; the server cannot invent missing request bytes. An abandoned incomplete binding expires under a defined retention policy. |
| API creates acceptance metadata, then crashes before storing XML | An identical keyed retry stores the body for the same upload. A reconciler may advance only if the XML is already durable; otherwise it leaves or expires the incomplete acceptance and never starts validation. Conflicting content fails. |
| API stores XML, then crashes before publishing validation-start | Acceptance state records the pending intent. The ingress reconciler republishes it with the deterministic ID. |
| API publishes validation-start, then crashes before responding | With the same scoped idempotency key and fingerprint, client retry and broker redelivery converge on the same upload/execution. Without a key, a retry is a new upload, while the original continues exactly once. |
| Orchestrator receives validation-start, then crashes before creating the execution | The message remains unacknowledged and redelivery safely retries creation. |
| Orchestrator creates execution/plan, then crashes before recording dispatch intent | CAS inspection finds the execution without an issued first-stage transition and records it once. |
| Orchestrator records dispatch intent, then crashes before publishing the stage request | The transition reconciler publishes the pending deterministic request. |
| Orchestrator publishes a stage request, then crashes before marking it sent or ACKing input | Reconciliation may republish; deterministic identity and permanent semantic evidence convergence make the duplicate harmless even after the broker deduplication window. |
| Worker crashes before or during content read/validation | It publishes no evidence; request redelivery retries under the stage policy. |
| Worker validates, then crashes before publishing its result | Request redelivery may recompute the stage and publishes the same logical result. |
| Worker publishes an attempt result, then crashes before ACKing the request | It may publish again with the same attempt-result ID. The orchestrator accepts at most one immutable result for that attempt. |
| Orchestrator receives an attempt result, then crashes before CAS acceptance | Result redelivery retries acceptance for that same attempt. |
| Orchestrator accepts a retryable operational failure, then crashes before authorizing the next attempt | Immutable attempt evidence remains without terminal stage outcome. CAS reconciliation evaluates policy and authorizes at most one next attempt. |
| Orchestrator accepts evidence and records transition intent, then crashes before publishing the next request | The transition reconciler publishes from durable intent; the worker never performs this transition. |
| Orchestrator publishes the next request, then crashes before marking it sent or ACKing the result | Duplicate publication/redelivery cannot advance the plan twice because plan ordinal and CAS state gate acceptance. |
| Orchestrator reaches terminal state, then crashes before final-result publication | A durable finalization intent remains pending and the reconciler publishes it. |
| Orchestrator publishes the final result, then crashes before marking completion or ACKing input | Republish is harmless because the permanent first-result slot and deterministic ID reject replacement. |
| Retry/DLQ processing crashes between failure count, DLQ publication, and ACK | Stage-specific attempt and DLQ IDs reconcile exactly one logical attempt; replay is a new authorized attempt, not mutation of the old result. |
| Retention crashes after eligibility, deletion, or before tombstoning | Eligibility is recalculated; an absent object counts as deleted; the tombstone advances monotonically under CAS. |

An operational failure is immutable attempt evidence, not whole-document
invalidity. Only an exhausted or non-retryable operational failure becomes the
terminal stage outcome. A standards-invalid terminal stage outcome may stop the
plan, but only the orchestrator records that terminal document outcome.

## Forbidden coupling

- A stage worker cannot invoke another worker or publish another stage request.
- A stage worker cannot update upload metadata, execution state, or final state.
- The API cannot infer, skip, reorder, or dispatch validation stages.
- The orchestrator cannot parse XML or synthesize validator findings.
- No role can overwrite raw XML, immutable attempt evidence, a terminal stage
  outcome, or a final result.
- Operators cannot replay by editing records or directly injecting stage work.
- Runtime services cannot silently mutate broker infrastructure definitions.

## Deliberately deferred details

This contract fixes ownership, not the wire implementation. The following remain
for their roadmap issues:

- execution, generation, plan and stage identities: `TODO-VALIDATION-SPLIT-002`;
- request/result schemas and deterministic message IDs: `TODO-VALIDATION-SPLIT-003`;
- exact stream, KV, Object Store and consumer topology: `TODO-VALIDATION-SPLIT-004`.

Those choices must implement the ownership and crash contracts above. A review
of any later split change must identify every authoritative write, its sole
logical owner, its CAS/idempotency key, and its recovery path before approval.
