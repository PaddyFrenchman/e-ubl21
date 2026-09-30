# e-UBL21 API comparison and commercial gap assessment

Date: 30 September 2026. Target branch: development.
Scope: static comparison of OpenAPI contracts; no authenticated calls or performance benchmarks.

## Where we stand

e-UBL21 currently exposes a focused asynchronous XML validation API. Storecove and B2Brouter expose much broader invoicing services. That breadth is a product-scope difference, not proof that their validation is more accurate or that their OpenAPI contracts are more compliant.

For a managed validation service, the immediate gaps are customer integration and service management: notifications, list/search, externally documented retry/idempotency behaviour, usable authentication configuration, and customer isolation/authorization. For a broader e-invoicing gateway, routing, delivery evidence, structured invoice creation and payment tracking become significant additional work.

Patrick's direction in this discussion: add routing and payment to the target product. This report incorporates that direction as proposed roadmap scope, not implemented functionality or an authorization to execute payments.

## Sources and reproducibility

- [Current e-UBL21 contract](../server/openapi.yaml), fetched from development. Observed blob SHA: 11532cf506908d6aa641551140cfe48aeda9d6b3.
- [Server functionality](../server/FUNCTIONALITY.md), used to clarify JWT authentication and operational behaviour.
- [Storecove snapshot](storecove-openapi.json).
- [B2Brouter snapshot](b2brouter-openapi-2026-06-26.yaml).
- Snapshot source URLs and SHA-256 checksums: [README](README.md).
- e-UBL21's README contains older scope statements alongside newer Peppol material; this report uses the current OpenAPI contract for exposed API capabilities rather than treating all README roadmap statements as current.

An absent operation below means absent from the reviewed public contract. It does not prove that no internal capability or deployment-specific behaviour exists.

## Contract inventory

| Measure | e-UBL21 | Storecove | B2Brouter |
|---|---|---|---|
| Declared specification | OpenAPI 3.0.3 | Swagger/OpenAPI 2.0 | OpenAPI 3.0.1 |
| Path entries | 5 | 31 | 60 |
| HTTP operations | 5 | 38 | 89 |
| Business-facing operations | Upload XML; retrieve status/content | Broad invoicing/document exchange | Broad invoice/account workflows |
| Authentication described | OAuth2 client credentials, with placeholder issuer | Authorization header API-key scheme | X-B2B-API-Key scheme |
| Server configuration in snapshot | Placeholder api.example.com | Concrete host/base path | Staging and production URLs |
| Missing operationId | 0 | 2 | 0 |
| Broken internal references found by traversal | 0 | 0 | 0 |

Counts include service operations and deprecated competitor operations. They are not a quality score.

### Verification limits

All three files parse as JSON/YAML. A recursive traversal resolved every internal reference. Operation inventories and selected schemas were inspected. No external references were found by that traversal.

A complete standards validator was unavailable locally; installation was attempted but the package index timed out. Therefore this report does not certify formal OpenAPI validity, generated-client compatibility, example-schema validity or live API conformance. A newer OpenAPI version alone is not evidence of better compliance.

The two missing Storecove operation IDs are PUT operations at:
- /api/v2/legal_entities/{legal_entity_id}/c5/iras/email/cancel
- /api/v2/legal_entities/{legal_entity_id}/c5/iras/redirect/cancel

operationId is optional in the standard; missing IDs are a tooling consistency concern, not automatically an OpenAPI violation.

## Direct validation comparison

| Capability | e-UBL21 contract | Competitor evidence | Assessment |
|---|---|---|---|
| Submit a raw document for validation | POST /api/v1/ubl/upload, asynchronous 202 | B2Brouter POST /documents/validate returns a validation result with 200 | e-UBL21 requires polling; a bounded synchronous evaluation mode is an optional convenience |
| Validation selection | Explicit auto, UBL, EN16931 and Peppol profiles | B2Brouter describes multiple invoice formats/profiles | e-UBL21 is narrower in input formats; supported-profile discovery would help clients |
| Detailed diagnostics | Layer, rule, severity, location, test, message, diagnostic | B2Brouter exposes XSD/Schematron errors and warnings | Both describe actionable diagnostics; no accuracy ranking established |
| Version/provenance in results | Stage version; Peppol BIS release and dates | Competitor validation results identify document/rule context | e-UBL21 exposes useful reproducibility metadata; do not claim an exclusive advantage |
| Operational failure separation | processing_failed and stage operational-failure outcomes | Different competitor result models | Useful explicit contract; validate actual behaviour separately |
| Warning presentation | Severity available inside issues; no top-level warning summary | B2Brouter explicitly models valid_with_warnings | UX gap: make warnings discoverable without requiring clients to invent aggregation rules |

B2Brouter's standalone validation endpoint describes UBL and additional formats such as CII, XRechnung, Factur-X and national variants. This corrects the earlier incomplete view that its closest validation operation was only GET /invoices/{id}/validate. Its advertised format support is not independently tested here.

## Functional gaps relevant to selling a managed service

| Gap in e-UBL21 public contract | Why a customer cares | Evidence / qualification | Suggested priority |
|---|---|---|---|
| Completion notifications/webhook management | Avoid repeated polling and simplify integration | B2Brouter exposes /web_hooks CRUD; Storecove exposes webhook-instance retrieval/deletion | High |
| Document listing, filters and pagination | Reconcile and recover work without retaining every upload ID locally | B2Brouter exposes account-scoped invoice lists and pagination | High |
| Documented client request idempotency | Retry ambiguous uploads without duplicate operations or charges | No upload Idempotency-Key parameter in e-UBL21 contract; internal queue deduplication is a different guarantee | High |
| Customer/tenant authorization contract | Isolate vendor end clients and support least privilege | e-UBL21 JWT authentication alone does not demonstrate object ownership or tenant isolation | High before shared commercial deployment |
| Structured transport/API error codes | Branch safely on errors and support customers | e-UBL21 ErrorResponse has only error:string; B2Brouter defines code/message/param/type | High |
| Bulk ingestion and per-item status | Reduce integration overhead for invoice batches | B2Brouter has asynchronous invoice import batches; not equivalent to a validation-only batch | Medium |
| Capability/profile catalogue | Discover supported documents, versions and profiles programmatically | e-UBL21 documentType is an unconstrained string and no catalogue operation is exposed | Medium |
| Customer-visible replay/revalidation | Recover failed processing through the API | Operational CLI replay is documented; no public retry/revalidation operation is exposed | Medium |
| Structured invoice creation and conversion | Integrate with application data without constructing XML | Competitor invoice/document models exist; e-UBL21 accepts raw XML and returns original XML | High for gateway positioning; optional for validation service |
| Routing, participant discovery, delivery and receipts | Complete the outbound invoice lifecycle | Storecove discovery/submission/evidence and B2Brouter transport/send/directory operations | High for gateway positioning |
| Payment recording and reconciliation | Track settlement and outstanding invoices | B2Brouter exposes payments CRUD and account payment lists | Medium/high for invoice lifecycle product |
| Executing payment transfers | Initiate and monitor movement of funds | Not established by the reviewed payments CRUD endpoints | Separate discovery decision |

Public-contract absence should be cross-checked against implementation and existing issues before creating duplicate work. Search found idempotency design documentation, but no corresponding parameter in the current OpenAPI contract; implementation status remains unconfirmed.

## OpenAPI contract improvements for e-UBL21

1. **Make authentication accurate and deployable.** The contract declares an OAuth2 client-credentials token endpoint at issuer.example.com. Current functionality documentation describes HS256 JWT verification, not a supplied authorization server. If customers bring externally issued JWTs, document that arrangement and configure a real issuer; if no OAuth acquisition flow is available, describe HTTP bearer/JWT instead. Empty OAuth scopes are valid syntax but provide no documented permission model.

2. **Replace the placeholder server URL for customer-facing deployments.** Use environment-specific generated configuration or a suitable relative server URL. Do not publish api.example.com as the effective integration target.

3. **Improve asynchronous job navigation.** Add a documented acceptance Location header and polling guidance. Define terminal states, whether clients should use backoff, and how to find status after ambiguous acceptance. B2Brouter's reviewed contract documents Location/Retry-After on some operations.

4. **Document rate-limit behaviour at the 429 response.** e-UBL21 already documents Retry-After on upload 503, but not on its reusable RateLimited response. Document retry timing and customer quota semantics where implemented. A per-IP deployment limiter is not a complete per-tenant commercial quota.

5. **Create reusable named upload/status schemas.** e-UBL21's upload and retrieval bodies are inline; naming them improves generated SDK types and shared contract tests. Both inline and named schemas are legitimate OpenAPI.

6. **Tighten machine-readable boundaries.** Express agreed string/array limits and identifier formats where meaningful. Severity is currently a free string. Freeze the allowed vocabulary if runtime semantics support it; do not over-constrain future rule identifiers.

7. **Expose warning/truncation semantics clearly.** Diagnostic output is operationally bounded, but clients need to know if findings were truncated and whether a valid result contains warnings. Treat this as a contract design proposal, not an observed runtime defect.

8. **Test examples and implementation against the contract.** Validate successful and failed responses, profiles, content expiry, rate limits and authentication. Examples describe behaviour but do not prove it.

e-UBL21 already supplies useful examples, required fields, closed diagnostic schemas, profile enums and explicit content-retention semantics. Avoid assuming every larger competitor contract is better: Storecove's snapshot has inconsistent operationId coverage and no example/example(s) fields found during traversal.

## Routing and payment: proposed product boundaries

### Routing and delivery

Keep document validation separate from delivery state. Validation success must not imply successful delivery.

A first routing service would need:
- sender/recipient identities and discovery;
- connector selection and country/profile constraints;
- validation-before-send rules;
- idempotent submission and delivery jobs;
- delivery status, acknowledgements, evidence and failure classification;
- signed event notifications and controlled retries.

Proposed operation families: participant/discovery, delivery submission, delivery status/evidence and webhook subscriptions. These are proposed additions, not existing e-UBL21 endpoints. A partner connector can be evaluated before operating an access point directly.

### Payment

Distinguish three scopes:
1. **Instructions:** bank details, payment means, due dates and references attached to an invoice.
2. **Tracking/reconciliation:** payments recorded against invoices, partial payments, refunds/adjustments, settlement status and remaining balance.
3. **Execution:** initiating transfers through a payment provider and tracking provider outcomes.

For an initial competitive expansion, prioritize instructions and tracking/reconciliation. The reviewed B2Brouter payment resources are evidence of lifecycle management, not sufficient evidence of bank-transfer execution. Storecove's invoice payment structures likewise do not establish transfer execution.

If execution is desired, evaluate a provider integration and define its operating/commercial requirements separately. This report does not establish regulatory obligations for a specific proposed execution model.

## Recommended sequence

| Phase | Result | Acceptance focus |
|---|---|---|
| 1: managed validation offer | Reliable customer integration | Accurate authentication, isolation evidence, typed errors, idempotency, notifications, list/search |
| 2: invoicing gateway | Validated invoices can be routed and tracked | Partner delivery connector, recipient discovery, delivery evidence, stable lifecycle and audit |
| 3: payment visibility | Invoice balances and settlement can be reconciled | Instructions, partial payment records, reconciliation and events |
| 4: optional payment execution | Provider-backed transfer initiation | Separate business case, provider scope and execution safeguards |

Batch validation, structured invoice creation and additional formats should be ordered using actual prospect requirements rather than endpoint-count parity.

## Decision

e-UBL21 has a substantive validation contract but is not yet API-feature-equivalent to a broad invoicing gateway. The most useful near-term investment is a commercially usable integration layer around that validation core. Routing is a clear expansion for gateway positioning; payment tracking is a separate valuable lifecycle addition. Payment execution is not justified merely by competitor payments endpoints.

Next qualification work: complete formal spec validation, check implementation/contract consistency, validate tenant ownership and upload idempotency, then evaluate the first delivery connector and prospect demand for payment tracking. No implementation changes or new GitHub issues were made as part of this comparison.
