# Requirements for a managed e-invoicing, routing and payment service

Date: 30 September 2026. Branch: development.
Status: proposed product requirements; not implemented capabilities.
Related: [API comparison](COMPARISON-REPORT.md), [competitor landscape](../MARKET-COMPETITORS.md).

## Objective and service boundary

Evolve e-UBL21 from asynchronous UBL validation into a managed integration service for ERP/accounting vendors: ingest or create invoices, apply explicitly selected standards, route documents, expose delivery evidence and track settlement.

The target includes three distinct payment scopes: invoice payment instructions, payment recording/reconciliation, and optional provider-backed payment execution. Each must have separate permissions, states and acceptance criteria. The product must never infer delivered from valid, or paid from payment initiated.

Start with an explicitly selected market, document/profile set, transport connector and payment provider model. Country breadth must follow qualified demand and verified connector support, not a universal compliance claim.

## Baseline

The reviewed server/openapi.yaml exposes upload and status/original-content retrieval plus service endpoints. It describes UBL, EN16931 and Peppol validation profiles and detailed stage results. It does not expose the proposed account, routing, payment, notification and broader document lifecycle APIs.

Internal persistence and queue infrastructure can be reused after verifying ownership, isolation and transaction boundaries. Existing architecture documents and issues must be checked before opening implementation work; public-contract absence does not establish total implementation absence.

## Required capability groups

### REQ-IDENTITY: customer identity and access

- Model vendor organization, its end-customer tenant, legal entity, user/service principal and connector/provider account explicitly.
- Use a documented bearer/OIDC or API-key model with provisioning, rotation, revocation and least-privilege scopes.
- Enforce tenant ownership on every object lookup, search, event and background job; never trust a submitted tenant identifier without authorization.
- Separate document management, delivery, payment viewing, payment initiation and administration permissions.
- Isolate quotas and billing by authenticated customer; preserve safe audit records.
- Supply a sandbox with synthetic data and separate credentials.

Acceptance: cross-tenant access fails consistently, including direct IDs and list filters; revoked credentials fail; background retries retain original tenant scope; permission tests cover every operation.

### REQ-DOCUMENT: intake, lifecycle and conversion

- Retain immutable original input, document hash, source metadata and caller correlation ID.
- Provide document list/filter/pagination and stable retrieval, using tenant-scoped identifiers.
- Support batch submission with per-item identity/results and defined limits.
- Specify supported XML/JSON models and profile rules. Structured invoice generation is validated before routing.
- Define editable draft versus immutable issued/sent version; corrections and credit notes must be linked without overwriting historical evidence.
- Record validation ruleset identity and freeze the plan used for each submission.
- Provide explicitly defined content/metadata retention and authorized deletion/export operations.
- Preserve currency and decimal precision; do not use binary floating point for monetary accounting.
- Document limits for attachments, payload size, batch size and diagnostic truncation.

Acceptance: ambiguous submission retries return the same operation; conflicting reuse of a key is rejected; each batch item can be independently diagnosed; list pagination is stable; generated documents pass the selected validation pipeline.

### REQ-VALIDATION: standards and capabilities

- Publish supported document types, profiles, releases, national extensions and connector prerequisites through a capability API.
- Keep XSD, semantic standards failures and operational failures distinct.
- Return structured rule/location/severity diagnostics and explicit warning/truncation indicators.
- Support authorized revalidation as a new immutable execution with lineage.
- Maintain controlled asset updates with regression evidence and reproducible version metadata.
- Do not treat Peppol payload validation as Peppol network delivery certification.

Acceptance: regression sets cover valid, invalid, warning, unsupported profile and operational failure outcomes; release upgrades preserve historical evidence and are explicitly reflected in new executions.

### REQ-ROUTING: discovery and route selection

- Represent sender/recipient identifiers, schemes, legal entities and transport capabilities.
- Resolve recipient capability using a connector or directory service; distinguish unknown, unsupported and temporarily unavailable.
- Select a permitted route using sender entitlement, recipient capabilities, country/profile constraints and explicit customer preferences.
- Record the routing decision, connector version/configuration identity and discovery freshness.
- Support a defined route policy; never silently fall back to email or another network when that changes customer expectations.
- Store connector credentials securely and separate environments.
- Prefer one qualified delivery partner connector for the first release. Direct access-point operation is a separate operating decision.

Acceptance: unsupported recipients produce actionable results; stale discovery and connector outages are covered; route decisions are reproducible from retained evidence; customers can prohibit fallback.

### REQ-DELIVERY: execution and evidence

- Create a delivery operation bound to an immutable validated document version.
- Enforce tenant-scoped request idempotency before external submission.
- Maintain durable submission state and external provider correlation IDs.
- Classify transient/permanent/ambiguous errors; apply bounded retries and reconciliation after timeouts.
- Distinguish queued, submitting, submitted, delivered, rejected, failed and outcome-unknown as appropriate to the connector. Define exactly what delivered proves.
- Store transport acknowledgements and evidence; expose authorized retrieval.
- Keep invoice business acceptance/rejection separate from transport receipt.
- Provide replay/cancellation only where the connector permits it; avoid promising recall after delivery.

Acceptance: simulated timeout after provider acceptance does not cause blind resubmission; provider callbacks are idempotent; out-of-order events converge; failed delivery never changes a valid document into standards-invalid; delivery evidence can be exported.

### REQ-EVENTS: notifications and audit

- Provide webhook create/list/update/delete and event subscription APIs.
- Sign payloads; publish signature verification and secret rotation guidance.
- Include event ID, object ID, tenant-safe correlation, type, timestamp and schema version.
- Deliver at least once with bounded retry, dead-letter inspection and authorized redelivery.
- Define event ordering and allow consumers to retrieve authoritative current state.
- Reject unsafe webhook destinations and constrain outbound requests.
- Record audit actor, action, object/version and outcome without logging sensitive invoice bodies or credentials.

Acceptance: duplicate and reordered notifications are safe; signature verification and rotation work; destinations cannot access private services; failed notification delivery is visible and recoverable.

### REQ-PAYMENT-INSTRUCTIONS: invoice payment data

- Support payment means, due dates, references, currency and authorized beneficiary details.
- Preserve invoice-specific instructions and versions.
- Validate formatting and required fields for the selected provider/market without claiming bank-account ownership verification.
- Require controlled changes to beneficiary data and retain audit history.

Acceptance: instructions survive document generation/round trip within the supported model; unsupported means fail explicitly; beneficiary changes are versioned and authorized.

### REQ-PAYMENT-TRACKING: records and reconciliation

- Record invoice-linked payments with external transaction IDs, amount, currency, effective date and source.
- Support partial payments, allocations across invoices, overpayments, credit offsets and reversals through explicit models.
- Compute outstanding balance from immutable events/allocations; do not overwrite paid totals arbitrarily.
- Define fees, foreign exchange and refunds separately.
- Support manual confirmation and provider/bank imports as distinguishable evidence sources.
- Deduplicate imported settlement events and expose reconciliation exceptions.
- Define invoice settlement state independently from delivery and payment initiation.

Acceptance: duplicate imports do not alter balances; partial/reversed payments reconcile exactly; unsupported currency allocations are rejected; manual paid markings are auditable and distinguishable from provider-confirmed settlement.

### REQ-PAYMENT-EXECUTION: optional provider-backed transfers

- Select the actual execution model, provider, countries, currencies and supported payment rails before implementation.
- Keep payer authorization/consent, provider onboarding and account linkage explicit.
- Maintain dedicated payment-intent and payment-attempt resources with idempotency and provider correlation.
- Require distinct initiation permissions, beneficiary-change controls and configurable approval thresholds.
- Represent authorization-required, authorized, submitted, pending, settled, failed, cancelled and unknown outcomes according to provider semantics.
- Validate provider callbacks and reconcile ambiguous responses against the provider before retrying.
- Never store raw banking credentials when a provider-hosted authorization mechanism can be used.
- Define reversal/refund/cancellation capabilities from the provider contract.
- Establish legal, contractual and security obligations for the chosen business model through qualified assessment before launch. This report does not assert that any specific licence or exemption applies.

Acceptance: unauthorized users cannot initiate; retries do not duplicate transfers; unknown status remains unknown until reconciled; initiation does not mark an invoice paid; settlement and reversals update allocations accurately.

## Proposed API families

Paths are design proposals, not a frozen contract.

| Area | Proposed resources / operations |
|---|---|
| Customer administration | tenants, legal entities, service credentials, scopes |
| Capabilities | document types, validation profiles/releases, connector capabilities |
| Documents | create/upload, batch submission, list/search, version/status/content, authorized deletion/export |
| Validation executions | create/revalidate, result and diagnostic retrieval |
| Recipient discovery | identifier lookup and recipient transport capabilities |
| Deliveries | submit, status, evidence, eligible retry/cancel |
| Webhooks and events | subscriptions, secrets, delivery attempts, redelivery |
| Payment records | record/list/reverse, allocations and reconciliation exceptions |
| Payment intents | optional authorize/initiate/status/cancel via selected provider |
| Usage | document/delivery counts, quotas and customer-visible billing usage |

All operations must have stable IDs, explicit authentication, typed errors, examples and versioned schemas. Publish real deployment server URLs. Document idempotency keys, conflict responses, pagination, polling guidance, rate limits and Retry-After semantics.

## Reliability and data architecture

- Separate validation, delivery and payment worker responsibilities with durable versioned contracts.
- Use explicit state machines and transactional outbox/inbox or equivalent durable publication guarantees.
- Persist external side-effect intent before execution; retain idempotency/provider evidence through reconciliation.
- Define ownership of each mutable state and prevent competing workers from overwriting newer state.
- Bound queues, concurrency and downstream calls; apply deadlines, backpressure and admission control.
- Scale within provider quotas; Kubernetes scaling must not multiply unsafe external calls.
- Verify backup/restore, migrations and retention across documents, evidence, payment records and connector metadata.
- Keep financial ledger history append-only where needed; corrections use compensating entries.
- Restrict access to beneficiary details and evidence; encrypt transport and stored secrets; minimize retained sensitive data.

Acceptance: crash/replay tests cover every external side-effect boundary; restored state does not re-execute completed deliveries/transfers; audit and reconciliation remain consistent after recovery.

## Operations and commercial launch requirements

- Define supported use cases and exclusions in a service description.
- Establish measured latency/throughput objectives, availability target, support hours and escalation ownership.
- Publish incident communication, version/deprecation and standards-update policies.
- Agree data retention, hosting location, deletion and export behaviour; obtain required customer/provider agreements.
- Meter usage through idempotent business events so retries do not create duplicate charges.
- Price onboarding, validation, delivery, storage and premium support separately where costs differ.
- Produce sandbox walkthroughs, sample integrations, error troubleshooting and customer onboarding guides.
- Define provider outage procedures and fallback permissions; test incident response.
- Monitor stage/connector latency, queue lag, delivery failures, reconciliation exceptions and notification backlog using bounded metric dimensions.

Numeric targets are deliberately unset until representative measurements, provider limits and customer requirements establish defensible commitments.

## Implementation and qualification sequence

| Stage | Deliverable | Release gate |
|---|---|---|
| A | Managed validation integration layer | Isolation, idempotency, typed errors, list/search, notifications, real authentication configuration |
| B | One routing/delivery connector | Recipient discovery, validation-before-send, durable idempotent delivery, evidence, outage/reconciliation tests |
| C | Invoice payment instructions and tracking | Precise balances, partial/reversed payment allocations, audit and import deduplication |
| D | Optional provider-backed execution | Chosen operating model, onboarding/authorization, initiation safeguards and full provider reconciliation |
| E | Additional countries/formats/connectors | Demand evidence plus connector-specific standards and lifecycle acceptance tests |

Structured invoice generation and batch APIs can be developed alongside these stages when selected prospects require them.

## Decisions needed before detailed implementation issues

1. First customer segment and first supported country/profile combination.
2. First delivery partner versus direct access-point operation.
3. XML-only intake versus supported structured invoice generation.
4. Payment tracking only for the initial release, or provider execution as an explicit later milestone.
5. Tenant relationship between a software vendor and its end clients.
6. Data retention, deployment model, support capacity and measurable service commitments.

These are planning decisions, not blockers to completing this report. No implementation or regulatory approval is claimed. Existing project issues should be mapped to these requirements before creating additional work.

## Definition of readiness

The service is ready for a narrowly scoped paid pilot when its supported operations are documented and contract-tested, tenant isolation and idempotency are proven, one selected delivery path has reproducible evidence and recovery behaviour, payment scope is explicit, and support/data handling commitments match operational capability.

It is not ready merely because its OpenAPI lists the same number of endpoints as competitors.
