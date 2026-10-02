# Peppol Route
Date: 2026-10-02
Status: proposed enhancements; provider integration is not yet claimed implemented.

## Goal and component ownership
Deliver and receive validated Peppol invoices through a certified Access Point provider. Preserve the ability to switch providers or later operate our own AP.

Use separate components:
- Invoice generator: produces business documents.
- UBL21: validates document conformity and retains versioned validation evidence.
- Router: owns delivery jobs, tenant policy, provider integration and delivery evidence.
- Provider: owns its certified network connection and contracted AP/discovery/registration services.

Provider contracts must explicitly establish responsibility for participant onboarding, SMP publication, delivery receipts, data retention, support and incident handling. Using a provider does not require us to become an AP operator, although our own contractual/security obligations remain.

## Current baseline and gaps
UBL21 implements XSD -> EN16931 -> Peppol BIS Billing validation for Invoice/CreditNote, including explicit peppol-bis-billing profile selection. The router must use that explicit profile for outgoing Peppol traffic; EN-only validity is insufficient. Validate against the release required by the destination/provider, and fail clearly if our pinned release is unsuitable.

The current inspection verifies validation code and manifests, not a completed provider integration or fresh end-to-end run. Inventory existing API/envelope capabilities before adding duplicate functionality.

## Proposed outbound flow
1. Authenticate tenant and verify authorisation to use the sender participant identifier.
2. Accept document plus recipient, document type, process identifier and client idempotency key.
3. Persist immutable document reference/hash and delivery intent.
4. Submit to UBL21 with explicit Peppol profile; await terminal results.
5. Reject standards-invalid documents with rule diagnostics. Keep operational validation failures distinct and retry only under policy.
6. Ensure provider supports destination, identifiers and required release; request discovery through its API when offered.
7. Create durable dispatch work with validated document hash and validation execution identity.
8. Send via adapter, persist provider/message IDs, and reconcile callbacks or status polling.
9. Expose final transport state and receipt evidence to the client.

Do not send if the document bytes differ from the validated hash. Preserve immutable validation evidence including ruleset versions. Decide whether later standards upgrades require revalidation of queued jobs.

## Provider abstraction
Define a transport interface for submission, status lookup, capability lookup where supported, receipt retrieval and inbound acknowledgement where required. Provider onboarding/participant administration can be a separate interface.

Normalise states and errors without erasing provider-specific evidence. Store raw events under bounded retention. Make discovery, callbacks, idempotency, receipts and inbound support explicit adapter capabilities; provider APIs are not standardised.

Choose an existing compatible AS4 engine only for a later self-operated adapter. Do not implement AS4 inside UBL21 merely to integrate with a provider's HTTP API.

## Router API and state model
Proposed endpoints:
- POST /deliveries: asynchronous submission; returns internal job ID.
- GET /deliveries/{id}: validation and transport status.
- GET /deliveries/{id}/events: ordered audit events.
- GET /deliveries/{id}/receipts: authorised receipt retrieval.
- Provider-specific authenticated callback endpoint.
- Tenant-authorised inbound document retrieval/event interface.

Proposed states: accepted, validating, validation_failed, ready, dispatching, provider_accepted, delivered, delivery_failed, delivery_unknown.
Provider acceptance is not delivery. delivered means supported transport-level receipt evidence, not business acceptance. Track any Invoice Response/business status separately; do not claim UBL21 validates those response payloads.

## Persistence and delivery guarantees
Persist tenant, sender/recipient scheme and value, document/process IDs, document hash/reference, validation execution and release identity, client idempotency key, provider IDs, attempt history, receipts and timestamps.

Use a database transaction and outbox to avoid committing a job without dispatch work. Enforce tenant-scoped idempotency: repeated key and same input returns the existing job; changed input conflicts.

After an ambiguous timeout, reconcile with provider IDs/idempotency/status before resubmitting. If the provider cannot reconcile safely, mark delivery_unknown for investigation. Do not promise exactly-once delivery across an external API.

Use bounded retries with jitter, rate limits, provider circuit breakers, dead-letter handling and controlled replay. Preserve stable logical document identity across retries.

## Inbound flow
Authenticate provider events, prevent replay, deduplicate using documented provider/message identifiers and persist bytes before acknowledging according to provider semantics. Validate received payloads through UBL21, then notify the correct tenant. Quarantine invalid documents with evidence rather than silently discarding them.

Define whether provider acknowledgements indicate receipt by our router or receipt by the ultimate business application. Registration must establish tenant ownership of receiving participant IDs.

## Security and operations
Use tenant-isolated credentials, a secrets manager, rotation and least privilege. Apply webhook signatures or mTLS as supported; an unguessable URL alone is insufficient authentication. Encrypt storage, limit payload sizes, protect XML parsing, redact sensitive logging and define retention/deletion.

Monitor validation latency, queue depth/age, provider errors, unknown outcomes and receipt delay. Correlate internal job, validation execution and provider message IDs. Provide incident, outage, reconciliation and recovery runbooks.

## Provider selection
Compare sandbox access, supported countries/documents/processes, registration/discovery services, delivery evidence, API idempotency, callback security, polling fallback, rate limits, SLA, exit/migration terms, data location and total cost. Obtain written API semantics and pricing before selecting a provider.

## Implementation work packages and acceptance criteria
1. Provider evaluation and contract: document capabilities, ownership and approved sandbox access.
2. Router contract: OpenAPI, identifier handling, tenant access, normalised states and errors reviewed.
3. Persistence/outbox: restart and crash recovery leave no lost delivery intent.
4. Validation integration: only explicit Peppol-valid, hash-matched payloads dispatch; invalid documents retain diagnostics.
5. First provider adapter: submit, correlate, poll/callback and retrieve receipts in sandbox.
6. Reliability: demonstrate rate-limit handling, duplicate events, provider outages and ambiguous timeout reconciliation without blind resend.
7. Inbound/onboarding: demonstrate tenant registration authorisation, durable reception, validation and tenant isolation if inbound is in launch scope.
8. Qualification: end-to-end sandbox exchange with an agreed receiving participant, security review, restore test and operational runbooks.

These are planning work packages, not newly created GitHub issues. Keep validation regression tests and provider sandbox tests separate. Mock adapter tests cannot establish network interoperability.

## References
- [Certification and operations plan](PEPPOL-CERTIFICATION.md)
- [Existing validation contract](PEPPOL-VALIDATION-CONTRACT.md)
- [OpenPeppol interoperability framework](https://peppol.org/learn-more/peppol-interoperability-framework/)
- [Official Access Point onboarding guide](https://peppol.org/wp-content/uploads/2024/04/how_to_set-up_a__post-award__peppol_access_point_v2.1.pdf)
