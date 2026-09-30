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

## Issue-ready development steps

These are proposed issue specifications stored only in this file. **No GitHub issues are created.** D01–D36 are local planning identifiers, not GitHub issue numbers. Reuse an existing issue when M0 establishes that it already covers the work. Dependencies refer to these local identifiers.

Each step is a separately reviewable development outcome. M0–M8 remain the milestone grouping; optional M8 is outside the initial managed-service release.

### D01 — Verify implementation and existing backlog

- **Milestone:** M0
- **Depends on:** None
- **Scope:** Map requirements to code, schemas and existing issues; identify reuse and remaining work.
- **Acceptance criteria:** Capability matrix cites source paths and existing issue numbers; tenant, retry and Peppol behaviour are verified; unresolved items are explicit.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D02 — Qualify the baseline OpenAPI contract

- **Milestone:** M0
- **Depends on:** D01
- **Scope:** Validate declared OpenAPI versions, references, examples and runtime conformance.
- **Acceptance criteria:** Record reproducible validation commands and results; representative client generation succeeds or failures are documented; auth/runtime mismatches have proposed fixes.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D03 — Define initial service and provider scope

- **Milestone:** M0
- **Depends on:** D01
- **Scope:** Select initial customer, profiles, document types and delivery provider shortlist.
- **Acceptance criteria:** Record coverage, exclusions, sandbox access, onboarding and commercial assumptions; estimate dependent work only after evidence.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D04 — Define organization, tenant and legal-entity ownership

- **Milestone:** M1
- **Depends on:** D01
- **Scope:** Specify identifiers, ownership, service principals and migration.
- **Acceptance criteria:** Permission matrix covers every resource and job; existing records have a documented ownership migration; ambiguous ownership is rejected.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D05 — Enforce tenant isolation throughout processing

- **Milestone:** M1
- **Depends on:** D04
- **Scope:** Implement or prove API, persistence, queue and event authorization.
- **Acceptance criteria:** Cross-tenant direct-ID, list, export, job and event access fails; background processing retains tenant ownership; existing validation regression remains intact.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D06 — Implement credential lifecycle and permissions

- **Milestone:** M1
- **Depends on:** D04,D05
- **Scope:** Issue, rotate and revoke credentials with scoped access.
- **Acceptance criteria:** Revocation and insufficient scopes fail; rotation has a defined overlap policy; secrets are never logged; authorization tests cover administrative and end-client roles.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D07 — Align authentication and deployment OpenAPI

- **Milestone:** M1
- **Depends on:** D02,D06
- **Scope:** Publish actual auth flows, scopes and deployment configuration.
- **Acceptance criteria:** Remove misleading placeholder contracts; sandbox requests and generated clients authenticate correctly; migration guidance describes client impact.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D08 — Standardize errors, correlation and quotas

- **Milestone:** M1
- **Depends on:** D05,D07
- **Scope:** Add stable error codes, correlation, version policy and customer quotas.
- **Acceptance criteria:** Errors are schema-valid; customer quotas remain distinct from IP limits; 429 includes documented retry guidance; duplicate retries have defined metering.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D09 — Publish isolated integration sandbox

- **Milestone:** M1
- **Depends on:** D07,D08
- **Scope:** Provide bounded sandbox tenants, credentials and sample requests.
- **Acceptance criteria:** Two sandbox tenants are isolated; walkthrough covers upload, poll and typed errors; no production credentials or data are needed.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D10 — Define document and execution lifecycle

- **Milestone:** M2
- **Depends on:** D04,D05
- **Scope:** Separate immutable document versions from validation executions and recovery states.
- **Acceptance criteria:** State transitions and ownership are documented; new executions preserve lineage; migration preserves existing upload/status compatibility.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D11 — Implement durable client upload idempotency

- **Milestone:** M2
- **Depends on:** D08,D10
- **Scope:** Persist tenant-scoped key, request fingerprint and acceptance outcome.
- **Acceptance criteria:** Identical retries produce one accepted operation; conflicting payloads fail; crashes and concurrent retries preserve one binding; expiry and metering are specified.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D12 — Add document list and search

- **Milestone:** M2
- **Depends on:** D05,D10
- **Scope:** Expose tenant-scoped filters and stable cursor pagination.
- **Acceptance criteria:** Status, type, date and caller-reference filters work; page traversal under concurrent writes has defined semantics; invalid cursors and cross-tenant access fail.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D13 — Publish supported capabilities and ruleset provenance

- **Milestone:** M2
- **Depends on:** D02,D10
- **Scope:** Expose supported types, profiles, releases and validation limitations.
- **Acceptance criteria:** Capabilities match configured runtime; validation results identify applied releases; unsupported profiles fail predictably; examples validate.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D14 — Implement retention, export and deletion policy

- **Milestone:** M2
- **Depends on:** D05,D10
- **Scope:** Preserve originals, authorize access and define expiry/deletion/audit.
- **Acceptance criteria:** Expired content returns documented status; deletion follows policy without silently destroying required evidence; unauthorized export fails; migration and recovery are documented.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D15 — Expose revalidation and controlled recovery

- **Milestone:** M2
- **Depends on:** D10,D11,D13
- **Scope:** Create new authorized executions with lineage and safe recovery semantics.
- **Acceptance criteria:** Original outcomes remain immutable; retries do not create unintended duplicate executions; unsupported recovery transitions fail; resulting events carry execution identity.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D16 — Complete validation and polling contract

- **Milestone:** M2
- **Depends on:** D02,D10,D13
- **Scope:** Add warning/truncation signals, reusable schemas, acceptance Location and backoff guidance.
- **Acceptance criteria:** Positive, invalid, warning and operational-failure examples conform; ambiguous upload responses can be recovered through idempotency; existing clients remain supported.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D17 — Implement webhook subscriptions and safe destinations

- **Milestone:** M3
- **Depends on:** D05,D08,D10
- **Scope:** Add subscription CRUD, event selection, destination validation and secret lifecycle.
- **Acceptance criteria:** Ownership checks hold; private/internal destinations and redirect/DNS bypass attempts are rejected at delivery; secrets appear only under documented access rules.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D18 — Implement signed durable event delivery

- **Milestone:** M3
- **Depends on:** D11,D17
- **Scope:** Add versioned envelopes, signing, outbox/inbox-equivalent durability and retry records.
- **Acceptance criteria:** Crash after state commit does not lose event; signatures and rotated secrets verify; duplicate/out-of-order events are identifiable; bounded retries, dead letters and authorized redelivery work.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D19 — Add batch ingestion with per-item outcomes

- **Milestone:** M3
- **Depends on:** D11,D16,D18
- **Scope:** Implement bounded batch acceptance and item correlation.
- **Acceptance criteria:** Every item has stable identity and independent result; partial failure names the item and cause; restart/retry does not duplicate accepted items; size and concurrency limits are enforced.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D20 — Implement auditable usage metering

- **Milestone:** M3
- **Depends on:** D08,D11,D18,D19
- **Scope:** Record billable usage and support customer queries.
- **Acceptance criteria:** Delivery retries do not double-charge; batch partial acceptance has defined billing; ledger reconciles against accepted operations; tenant-safe usage queries and correction audit work.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D21 — Define structured invoice and exact monetary model

- **Milestone:** M4
- **Depends on:** D03,D10
- **Scope:** Specify supported invoice/credit-note fields, currency, taxes, rounding and attachments.
- **Acceptance criteria:** Representative monetary and correction cases have exact expected values; unsupported fields and conversion fidelity are explicit; API schemas and migration implications are reviewed.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D22 — Implement drafts and immutable issuance

- **Milestone:** M4
- **Depends on:** D11,D21
- **Scope:** Add structured create/update drafts and controlled issue transitions.
- **Acceptance criteria:** Concurrent edits follow a documented policy; issued versions cannot mutate; generated XML passes selected profiles before issuance; invalid documents cannot be issued.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D23 — Implement supported conversion and format retrieval

- **Milestone:** M4
- **Depends on:** D13,D14,D22
- **Scope:** Provide qualified conversions while preserving original input.
- **Acceptance criteria:** Supported round trips have evidence; unsupported or lossy conversions are disclosed or rejected; content access respects ownership and retention; corrections retain lineage.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D24 — Qualify delivery provider and connector interface

- **Milestone:** M5
- **Depends on:** D03,D10
- **Scope:** Choose one provider and define discovery, submission, status, evidence and cancellation capabilities.
- **Acceptance criteria:** Sandbox and onboarding requirements are documented; connector failures and provider limits are mapped; unsupported capabilities are explicit; provider choice has recorded rationale.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D25 — Implement identity discovery and route policies

- **Milestone:** M5
- **Depends on:** D05,D13,D24
- **Scope:** Resolve sender/recipient capabilities and select authorized routes.
- **Acceptance criteria:** Unsupported recipient/profile routes fail clearly; tenant route configuration is isolated; fallbacks require explicit policy; discovery caching and expiry are specified.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D26 — Implement retry-safe delivery submission

- **Milestone:** M5
- **Depends on:** D11,D18,D25; D22 if generated invoices are sent
- **Scope:** Send immutable validated versions using durable intent and provider correlation.
- **Acceptance criteria:** Provider acceptance followed by timeout triggers reconciliation before resend; crash/retry tests prevent blind duplicates; invalid versions cannot send; worker concurrency respects partner quotas.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D27 — Expose delivery status, evidence and acknowledgements

- **Milestone:** M5
- **Depends on:** D26
- **Scope:** Persist callbacks, polling results, receipts and business acknowledgements.
- **Acceptance criteria:** Duplicate callbacks are safe; delivery and business acceptance remain separate; evidence identifies exact version/route; outages and delayed statuses reconcile; tenant access is enforced.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D28 — Version payment instructions and beneficiary changes

- **Milestone:** M6
- **Depends on:** D10,D21
- **Scope:** Define instructions and controlled beneficiary-change history.
- **Acceptance criteria:** Exact payment terms and beneficiary versions are retained; unauthorized changes fail; invoice/payment links reference the applicable version; no payment execution is implied.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D29 — Implement payment records, allocations and balances

- **Milestone:** M6
- **Depends on:** D05,D10,D21,D28
- **Scope:** Add immutable records, allocations, compensating reversals and queries.
- **Acceptance criteria:** Partial/overpayments, credit offsets and reversals yield exact balances; currencies and fees have explicit policy; history cannot be silently overwritten; cross-tenant allocation fails.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D30 — Implement transaction import and reconciliation

- **Milestone:** M6
- **Depends on:** D18,D29
- **Scope:** Add manual entry and one selected provider/bank import with evidence and exceptions.
- **Acceptance criteria:** Repeated imports do not change totals; ambiguous matches enter exception workflow; manual and provider-confirmed settlement remain distinct; settlement/reversal events are durable.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D31 — Qualify failures, security and restore

- **Milestone:** M7
- **Depends on:** D09,D16,D18,D20,D27; D23,D30 if included
- **Scope:** Run scoped integration, security, crash/recovery and backup/restore qualification.
- **Acceptance criteria:** Evidence includes exact commands/configuration; restores and replays do not repeat external sends; tenant boundaries and provider outage recovery hold; unavailable environments are explicit blockers.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D32 — Measure capacity and establish operating procedures

- **Milestone:** M7
- **Depends on:** D20,D27,D31
- **Scope:** Measure service/provider limits and define alerts, support, retention and pricing.
- **Acceptance criteria:** Targets cite measurements; runbooks have owners; secrets/document bodies are excluded from logs; alerts cover lag, dead letters, ambiguity and reconciliation; usage supports offered pricing.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D33 — Deliver and assess a scoped paid pilot

- **Milestone:** M7
- **Depends on:** D31,D32
- **Scope:** Provide onboarding, client examples and a bounded design-partner integration.
- **Acceptance criteria:** Customer completes integration with documented support; diagnostic, delivery and optional reconciliation outcomes meet agreed measures; exclusions and unresolved risks are recorded before release.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D34 — Define optional payment execution provider model

- **Milestone:** M8
- **Depends on:** D30
- **Scope:** Select provider/rails, payer authorization, operating obligations and initiation permissions.
- **Acceptance criteria:** Explicit product decision and qualified legal/contractual assessment exist; supported flows and exclusions are recorded; initiation and tracking permissions are separate.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D35 — Implement authorized retry-safe payment initiation

- **Milestone:** M8
- **Depends on:** D34
- **Scope:** Add payment intents/attempts, provider-hosted authorization, approvals and safeguards.
- **Acceptance criteria:** Unauthorized or unapproved attempts fail; retries do not create duplicate transfers; beneficiary changes trigger defined safeguards; ambiguous outcomes remain pending reconciliation.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### D36 — Reconcile execution settlement and supported reversals

- **Milestone:** M8
- **Depends on:** D30,D35
- **Scope:** Verify callbacks and reconcile payment execution; support cancellation/refund only where available.
- **Acceptance criteria:** Authoritative settlement drives allocation; duplicate/out-of-order callbacks are safe; unknown outcomes are not treated as failure/success; supported reversals adjust balances through compensating records.
- **Delivery evidence:** Link affected source, schema and documentation; record reproducible qualification commands and results relevant to this step. For design-only steps, provide a reviewed decision record and examples. State environment blockers explicitly.
- **Compatibility and migration:** Record impact on existing clients, stored data and operational configuration; include migration/rollback guidance where applicable.

### Sequencing and issue preparation

Start with D01–D03, then D04–D09. Build D10–D16 before D17–D20. D21–D23, connector qualification D24, and payment design can progress once their stated prerequisites are satisfied. Complete D25–D27 for routing/delivery and D28–D30 for payment visibility. D31–D33 qualify the chosen pilot scope. D34–D36 require a separate decision to execute payments.

Before promoting a draft into GitHub, attach the exact requirement IDs from [SERVICE-EXPANSION-REQUIREMENTS.md](SERVICE-EXPANSION-REQUIREMENTS.md), affected operations and schema names, existing-issue links, owner and evidence-based estimate. Split a step further if implementation mapping shows it cannot be reviewed as a coherent change. Do not duplicate existing work or imply that a proposed endpoint already exists.

## Definition of completion

Completion means the chosen release scope is implemented, contract-tested, operationally qualified and usable by a pilot customer. Matching competitor endpoint counts is not the goal. Routing and payment execution have separate release gates so a useful managed service can launch without claiming unimplemented capabilities.
