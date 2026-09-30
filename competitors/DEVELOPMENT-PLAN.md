# Development plan: managed e-invoicing integration

Prepared: 30 September 2026.
Repository: PaddyFrenchman/e-ubl21. Target branch: development.
Status: proposed implementation plan; no features implemented by this document.

## Goal and planning assumptions

Deliver a commercially usable service for ERP/accounting vendors: authenticated customer integration, document management, validation, notifications, routing/delivery, and payment tracking/reconciliation. Provider-backed payment execution is a separate optional milestone.

Reference documents:
- [Comparison report](COMPARISON-REPORT.md)
- [Service expansion requirements](SERVICE-EXPANSION-REQUIREMENTS.md)
- [Current OpenAPI](../server/openapi.yaml)

Plan assumptions: start with one delivery partner and a limited supported market/profile set; retain Go backend, Kubernetes deployment, existing durable queue and persistence foundations where suitable. These assumptions require confirmation before dependent implementation. Do not recreate functionality already implemented or covered by existing issues.

This is a dependency plan, not a calendar commitment. Estimate effort after implementation mapping, connector selection and representative prototypes.

## Milestones and dependencies

| Milestone | Outcome | Depends on |
|---|---|---|
| M0 | Verified implementation baseline and mapped backlog | None |
| M1 | Secure customer boundaries and accurate API contract | M0 |
| M2 | Retry-safe document lifecycle and discovery | M1 |
| M3 | Signed notifications and bulk integration | M2 |
| M4 | Invoice generation and supported-format access | M2; profile decisions |
| M5 | One operational routing/delivery connector | M2, M3; M4 if generated invoices are required |
| M6 | Payment instructions, records and reconciliation | M2, M3; supported monetary model |
| M7 | Qualified paid pilot and operational readiness | M1–M3, M5; M4/M6 if included in pilot offer |
| M8 | Optional provider-backed payment execution | M6; provider and operating-model decisions |

M4 and M6 can be developed independently after their prerequisites. Payment tracking need not wait for the first delivery connector; historical or externally delivered invoices may need reconciliation.

## M0 — Establish the actual baseline

Work:
- Review current development code, public OpenAPI, persistence schema, runtime configuration and existing GitHub issues.
- Map each REQ group from the requirements report to implemented, partial, planned or absent, with source evidence.
- Verify tenant ownership, client request idempotency, replay and Peppol support rather than assuming they are absent from implementation.
- Run an OpenAPI validator for the declared version, example/schema checks and representative API contract tests.
- Record discrepancies between code and documentation and prioritize fixes.
- Define the initial customer segment, market/profile set and delivery partner shortlist.

Deliverables: evidence-backed capability matrix, issue mapping, revised estimates and baseline contract report.
Exit gate: every planned work item has an owner, dependencies, acceptance criteria and an existing issue or a proposed issue; baseline limitations are explicit.

## M1 — Customer identity, isolation and API foundations

Work packages:
1. Model vendor organization, end-client tenant, legal entity and service principal.
2. Implement or prove tenant ownership checks at API, persistence, background job and event boundaries.
3. Define credential issuance/rotation/revocation and least-privilege permissions.
4. Correct the OpenAPI authentication model and deployment server configuration.
5. Define shared machine-readable errors, request correlation, limits and version/deprecation policy.
6. Establish per-customer quota/metering semantics separate from per-IP protection.
7. Provide isolated sandbox credentials and sample requests.

Deliverables: reviewed identity model, migration plan, permission matrix, updated OpenAPI and sandbox walkthrough.
Exit gate: direct-ID, listing and job access cannot cross tenants; revoked credentials fail; generated clients can target the actual sandbox; error codes and 429 retry guidance are documented.

Relevant requirements: REQ-IDENTITY and common API requirements.

## M2 — Retry-safe document management

Work packages:
1. Freeze document/version/validation-execution identities and lifecycle state machines.
2. Add tenant-scoped client idempotency: key + request fingerprint + durable acceptance outcome.
3. Add document list/search with stable pagination and filters for type, status, date and caller reference.
4. Publish capabilities: supported document types, profiles and ruleset releases.
5. Define immutable original input, content retention, authorized export/deletion and audit semantics.
6. Expose authorized revalidation/recovery as a new execution with lineage.
7. Add clear warning/truncation indicators and reusable request/status schemas.
8. Document acceptance Location, polling/backoff and ambiguous-response recovery.

Proposed resources: documents, document versions, validation executions and capabilities. Preserve compatibility with existing upload/status operations during migration.
Exit gate: identical retries return one accepted operation; conflicting keys are rejected; restart/crash recovery preserves bindings; tenant-safe pagination and retention behaviours are contract-tested.

Relevant requirements: REQ-DOCUMENT and REQ-VALIDATION.

## M3 — Notifications and bulk workflows

Work packages:
1. Implement webhook subscription CRUD and versioned event envelopes.
2. Sign notifications, support secret rotation and define verification guidance.
3. Use durable publication and delivery records with bounded retry/dead-letter/redelivery.
4. Constrain callback destinations to prevent unsafe outbound access.
5. Add batch submission with stable per-item correlation and independent failures.
6. Publish usage events without charging twice for delivery retries.

Exit gate: duplicate and out-of-order events converge safely; callbacks are verifiable; notification outages are recoverable; each failed batch item is identifiable; callback targets cannot reach private services.

Relevant requirements: REQ-EVENTS and batch portions of REQ-DOCUMENT.

## M4 — Structured invoice creation and conversion

Work packages:
1. Select a supported structured invoice model and its relation to lossless UBL access.
2. Define exact decimal/currency handling, tax amounts, rounding, attachments and payment instructions.
3. Introduce create/update-draft and controlled issue/version transitions.
4. Generate XML and validate before issuing or sending.
5. Define supported conversion fidelity; preserve original input and disclose unsupported/lossy transformations.
6. Cover invoices, credit notes and corrections within the chosen initial scope.

Exit gate: generated documents pass selected standards; round-trip claims are proven for supported inputs; issued versions remain immutable; arithmetic and correction cases are tested.

This is a product-scope addition, not required merely to sell XML validation. Include it in the first release if selected prospects need application-data intake.
Relevant requirements: REQ-DOCUMENT, REQ-VALIDATION and REQ-PAYMENT-INSTRUCTIONS.

## M5 — Routing and delivery

Work packages:
1. Select one provider and verify sandbox, commercial terms, market/profile coverage and onboarding obligations.
2. Define a connector interface for discovery, submission, status, evidence and supported cancellation.
3. Implement sender/recipient identity and capability discovery.
4. Add explicit route policy and permitted fallback configuration.
5. Submit immutable validated versions through idempotent delivery jobs.
6. Persist side-effect intent and provider correlation; reconcile ambiguous timeouts before retries.
7. Expose delivery state, receipts/evidence and business acknowledgements separately.
8. Publish delivery events and connector operational metrics.

State distinctions: valid document, transport submitted, transport delivered, business accepted and paid are separate outcomes.
Exit gate: provider acceptance followed by timeout cannot trigger blind duplicate sends; duplicate callbacks are safe; outages and reconciliation are exercised; exported evidence identifies the exact sent version and route.

Relevant requirements: REQ-ROUTING and REQ-DELIVERY.

## M6 — Payment visibility and reconciliation

Work packages:
1. Version payment instructions and control beneficiary changes.
2. Define immutable payment records, allocations, reversals and outstanding-balance calculations.
3. Support partial payments, overpayments, refunds/credit offsets and explicit currency/fee handling.
4. Add manual record entry and one selected provider/bank import, distinguishing their evidence sources.
5. Deduplicate transaction imports; expose reconciliation exceptions and audit.
6. Publish settlement events and tenant-safe payment queries.

Exit gate: balances reconcile exactly; repeated imports do not alter totals; reversals are compensating records; manually marked settlement is distinguishable from provider-confirmed settlement.

Relevant requirements: REQ-PAYMENT-INSTRUCTIONS and REQ-PAYMENT-TRACKING.

## M7 — Commercial pilot and operational qualification

Work:
- Integrate with one design partner and a bounded document/profile/route set.
- Complete contract, security, failure/recovery and provider-sandbox qualification.
- Prove backup/restore without repeating external side effects.
- Measure representative throughput, latency, provider limits and support workload.
- Set defensible availability/support targets, data retention and pricing.
- Supply onboarding guides, generated client examples and incident procedures.
- Define pilot success measures: integration effort, diagnostic usefulness, delivery evidence, reconciliation accuracy and support burden.
- Record exclusions explicitly: unqualified countries/profiles, unsupported transports and payment execution if absent.

Exit gate: evidence supports the offered scope; incident and recovery owners are assigned; usage billing is retry-safe; the customer can operate the integration without hidden manual intervention.

## M8 — Optional payment execution

Prerequisites: explicit decision to execute transfers, selected provider/rails, payer authorization model, qualified legal/contractual assessment and sufficient operating capacity.

Work:
- Add payment intent/attempt objects, provider-hosted authorization and distinct initiation scopes.
- Implement approval thresholds, beneficiary safeguards and idempotent provider submission.
- Verify callbacks and reconcile unknown/pending states.
- Implement cancellation/refund only where the provider supports it.
- Allocate payments only when authoritative settlement evidence arrives.

Exit gate: unauthorized initiation fails; retries cannot create duplicate transfers; unknown remains unknown until reconciled; reversals update allocations correctly; provider onboarding and operating requirements are satisfied.

## Cross-cutting engineering rules

- Keep validation, delivery and payment state ownership separate.
- Use durable outbox/inbox or equivalent publication guarantees.
- Model state transitions and crash/retry behaviour before implementing external side effects.
- Bound concurrency, queues, provider calls and payloads; scale within partner quotas.
- Version database migrations, API schemas, events and connector configuration.
- Maintain audit history and avoid secrets/invoice bodies in logs.
- Document backward compatibility and migration for every public change.
- Retain the existing deterministic standards regression baseline.

## Verification matrix

| Concern | Required evidence |
|---|---|
| OpenAPI quality | Version-aware validation, resolved references, schema-valid examples, representative client generation |
| Runtime conformance | Requests/responses match the published contract, including errors and auth |
| Isolation | Tenant and permission checks across API, storage, jobs and events |
| Retry safety | Crash/timeout tests at acceptance, publication and provider side-effect boundaries |
| Standards | Pinned positive/negative/warning regression sets |
| Webhooks | Signatures, rotation, duplicate/out-of-order delivery and unsafe destination rejection |
| Delivery | Provider sandbox receipts, ambiguity reconciliation and outage recovery |
| Payments | Exact arithmetic, allocations, import deduplication and reversals |
| Recovery | Backup/restore and replay preserve evidence without repeated external actions |
| Commercial readiness | Measured targets, usage accuracy, support procedures and scoped pilot outcome |

## Backlog decomposition

Create or reuse one milestone/epic per M0–M8 and small linked implementation issues for its work packages. This document does not create issues.

Each issue should contain:
- requirement IDs and affected operation/schema;
- implementation boundary and dependencies;
- migration/backward compatibility impact;
- positive, negative and crash/retry acceptance criteria;
- exact qualification command/evidence expected;
- documentation and operational handover requirements.

First actionable sequence: implementation/issue mapping, customer isolation proof, accurate authentication contract, client upload idempotency, document listing, then signed result notifications. Connector selection can proceed alongside these foundations.

## Definition of completion

Completion means the chosen release scope is implemented, contract-tested, operationally qualified and usable by a pilot customer. Matching competitor endpoint counts is not the goal. Routing and payment execution have separate release gates so a useful managed service can launch without claiming unimplemented capabilities.
