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
under the same CAS and idempotency rules; they are not unrelated writers.

| Role | Owns | Must not do |
| --- | --- | --- |
| API / ingress reconciler | Authenticate and validate the request envelope; allocate the upload ID; create acceptance metadata; store immutable XML; durably request validation; expose read-only status projections. | Select or dispatch stages; write execution or stage state; publish a stage request or final result; validate invoice semantics. |
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
| Upload acceptance metadata | API / ingress reconciler | API and orchestrator read | Create plus CAS-only monotonic repair. No generic cross-role `PutUploadRecord`. |
| Validation-start intent | API publishes | Orchestrator mutates only its consumer ACK state | Deterministic message ID; duplicate delivery must start at most one logical execution. |
| Validation execution and frozen plan | Orchestrator | API reads a projection | Orchestrator replicas use CAS. Identity and plan details are defined by `TODO-VALIDATION-SPLIT-002`. |
| Transition intent / outbox | Orchestrator | Orchestrator reconciler reads | CAS state transition plus deterministic publication identity; a pending intent survives crashes. |
| Per-stage request stream | Orchestrator publishes | The matching worker pool mutates only its durable consumer ACK state | Deterministic request ID. Duplicate publication and delivery are expected. |
| Per-stage result stream | Matching stage worker publishes | Orchestrator mutates only its durable consumer ACK state | Deterministic result ID. Duplicate results are expected and cannot replace accepted evidence. |
| Accepted stage evidence | Orchestrator, inside execution state | API reads a projection | Validate execution, generation, stage and ordinal, then CAS. The first accepted result is permanent. |
| Per-stage retry and DLQ state | Matching stage capability / retry controller | Operator submits replay intent; orchestrator authorizes plan re-entry | Idempotent attempt and DLQ identities; replay creates a new authorized attempt and never edits history. |
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
5. proof that first accepted evidence and final results remain immutable.

## Crash boundaries

Every sequence below crosses independent durable operations. Recovery must work
at each boundary without relying on an in-memory transaction.

| Boundary | Durable recovery contract |
| --- | --- |
| API creates acceptance metadata, then crashes before storing XML | The ingress reconciler resumes the same upload under CAS. It stores only the identical body; conflicting content fails. |
| API stores XML, then crashes before publishing validation-start | Acceptance state records the pending intent. The ingress reconciler republishes it with the deterministic ID. |
| API publishes validation-start, then crashes before responding | Client retry and broker redelivery converge on the same upload/execution; neither creates another logical run. |
| Orchestrator receives validation-start, then crashes before creating the execution | The message remains unacknowledged and redelivery safely retries creation. |
| Orchestrator creates execution/plan, then crashes before recording dispatch intent | CAS inspection finds the execution without an issued first-stage transition and records it once. |
| Orchestrator records dispatch intent, then crashes before publishing the stage request | The transition reconciler publishes the pending deterministic request. |
| Orchestrator publishes a stage request, then crashes before marking it sent or ACKing input | Reconciliation may republish; deterministic identity and worker idempotency make the duplicate harmless. |
| Worker crashes before or during content read/validation | It publishes no evidence; request redelivery retries under the stage policy. |
| Worker validates, then crashes before publishing its result | Request redelivery may recompute the stage and publishes the same logical result. |
| Worker publishes a result, then crashes before ACKing the request | It may publish again. The orchestrator accepts the first valid result only. |
| Orchestrator receives a result, then crashes before CAS acceptance | Result redelivery retries acceptance. |
| Orchestrator accepts evidence and records transition intent, then crashes before publishing the next request | The transition reconciler publishes from durable intent; the worker never performs this transition. |
| Orchestrator publishes the next request, then crashes before marking it sent or ACKing the result | Duplicate publication/redelivery cannot advance the plan twice because plan ordinal and CAS state gate acceptance. |
| Orchestrator reaches terminal state, then crashes before final-result publication | A durable finalization intent remains pending and the reconciler publishes it. |
| Orchestrator publishes the final result, then crashes before marking completion or ACKing input | Republish is harmless because the permanent first-result slot and deterministic ID reject replacement. |
| Retry/DLQ processing crashes between failure count, DLQ publication, and ACK | Stage-specific attempt and DLQ IDs reconcile exactly one logical attempt; replay is a new authorized attempt, not mutation of the old result. |
| Retention crashes after eligibility, deletion, or before tombstoning | Eligibility is recalculated; an absent object counts as deleted; the tombstone advances monotonically under CAS. |

An operational failure is stage evidence, not whole-document invalidity. The
orchestrator applies the frozen plan policy to decide whether to retry, stop as
an operational failure, or continue. A standards-invalid stage result may stop
the plan, but only the orchestrator records that terminal document outcome.

## Forbidden coupling

- A stage worker cannot invoke another worker or publish another stage request.
- A stage worker cannot update upload metadata, execution state, or final state.
- The API cannot infer, skip, reorder, or dispatch validation stages.
- The orchestrator cannot parse XML or synthesize validator findings.
- No role can overwrite raw XML, accepted stage evidence, or a final result.
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
