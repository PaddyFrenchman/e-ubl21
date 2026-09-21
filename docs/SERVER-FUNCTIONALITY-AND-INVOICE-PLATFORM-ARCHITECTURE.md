# Server Functionality and Generic Invoice Platform Architecture

**Project:** e-ubl21  
**Status:** Architecture and product-direction document  
**Baseline:** `main` at `1da2dbb11e8fbe8842b3f305c5bdbc902f1805e7`  
**Date:** 2026-08-26

## 1. Purpose

This document describes:

1. the functionality currently implemented by the e-ubl21 server;
2. what remains to reach broader, production-grade e-invoice validation rather than only UBL 2.1 + EN 16931 validation;
3. the proposed extension of the product with a client-friendly, generic JSON invoice API;
4. how a minimal JSON invoice can be enriched with controlled defaults, rendered as compliant UBL XML, validated, returned to the client, and optionally queued for delivery;
5. a target architecture for onboarding parties, generation, validation, routing, delivery, acknowledgements, audit and future multi-tenant persistence.

The central product direction is to move from a **UBL validator** toward an **e-invoicing compliance gateway** while preserving the validator as the standards-enforcement core.

---

## 2. Current server functionality

### 2.1 Current high-level flow

```text
Client
  |
  | POST /api/v1/ubl/upload  (UBL XML)
  v
API
  |
  +--> detect UBL document root + namespace
  +--> Object Store: immutable original XML
  +--> KV: upload state / metadata
  +--> JetStream work descriptor
              |
              v
            Worker
              |
              +--> retrieve XML from Object Store
              +--> UBL 2.1 XSD validation with xmllint
              +--> for Invoice/CreditNote: EN 16931 validation with Saxon
              +--> persist structured validation stages
              +--> publish one permanent validation result
              +--> ACK / retry / DLQ as appropriate
```

The API and worker are separate execution modes of the same Go binary and communicate through NATS JetStream.

### 2.2 UBL document recognition and XSD validation

The server embeds the authoritative UBL 2.1 XSD tree. The document catalog is built by enumerating the embedded `maindoc` schemas, reading each root element and target namespace, and mapping that root to the correct XSD.

UBL 2.1 defines 65 top-level document types. The current catalog architecture is therefore not hard-coded to Invoice/CreditNote; it is derived from the UBL main-document schema set. Recognized non-billing documents such as `Order` receive their applicable UBL 2.1 XSD validation.

Only the worker extracts the complete XSD tree to disk, because `xmllint` requires filesystem paths for imported schemas. API startup uses only the lightweight in-memory catalog.

### 2.3 EN 16931 validation

`Invoice` and `CreditNote` have a mandatory two-stage validity contract:

```text
UBL 2.1 XSD
    |
    +-- fail --> standards-invalid
    |
    v
EN 16931 semantic/business-rule validation
    |
    +-- fail --> standards-invalid
    |
    v
valid
```

The EN 16931 stage uses:

- the pinned official EN 16931 UBL validation artefact, version 1.3.16;
- Saxon-HE 12.7;
- XML Resolver 6.0.17;
- a restricted Saxon configuration;
- bounded SVRL output and diagnostics;
- pinned checksums and startup verification;
- no runtime download of standards assets.

The final compliance gate proves through the public API that an Invoice or CreditNote cannot become `valid` unless both required layers have passed.

### 2.4 Durable asynchronous processing

Raw XML is stored once in JetStream Object Store. Work messages contain only compact descriptors, not the complete invoice.

The server maintains:

- immutable source content in Object Store;
- mutable upload/status metadata in KV;
- a WorkQueue stream for validation work;
- a one-result-per-upload permanent validation result stream;
- a dead-letter path for exhausted operational failures.

Validation results distinguish:

- standards invalidity;
- operational validator failure;
- final valid documents.

Infrastructure failures do not masquerade as invalid invoices.

### 2.5 Reliability and hardening

The worker currently provides:

- explicit concurrency and prefetch limits;
- separate overall, XSD and EN 16931 deadlines;
- bounded `AckWait` relationships and progress heartbeats;
- subprocess cancellation on worker shutdown;
- bounded xmllint diagnostics;
- bounded Saxon stderr and SVRL;
- bounded EN issue count and diagnostic fields;
- delayed retries for operational failures;
- durable `processing_failed` + DLQ transition;
- replay of retained failed content;
- duplicate-replay suppression and exactly-one permanent result behavior;
- recovery after worker interruption between XSD and EN processing.

### 2.6 HTTP API

The current public API is primarily document-oriented:

- `POST /api/v1/ubl/upload`
- `GET /api/v1/ubl/{id}`
- `GET /api/v1/ubl/{id}?includeContent=true`
- OpenAPI/Swagger endpoints
- health and metrics endpoints

Uploads currently accept XML or octet-stream content, not a simplified invoice JSON business model.

### 2.7 Authentication

The current implementation protects application routes using bearer JWT validation with an HMAC signing secret, issuer/audience checks and expiration enforcement.

This is a useful security baseline, but the long-term production target remains a full OAuth 2.0 / OpenID Connect resource-server model with asymmetric signing/JWKS, tenant identity and scopes/roles.

### 2.8 Observability and capacity

The server exposes Prometheus metrics and structured logs for:

- HTTP activity;
- upload outcomes;
- XSD and EN validation outcomes;
- validation duration;
- EN issue severity;
- bounded operational error classes;
- queue/worker behavior;
- terminal processing failures and DLQ state.

The EN capacity benchmark measures the real Object Store -> XSD -> EN -> durable result path, including semantically large invoices. The present architecture intentionally keeps one isolated Saxon JVM per semantic-validation operation and favors horizontal worker scaling.

---

## 3. What "full validation" should mean

The existing implementation now provides strong **UBL 2.1 structural validation** and **EN 16931 semantic validation for Invoice/CreditNote**. That is not yet the same as universal e-invoicing compliance.

A production e-invoicing platform should treat validation as a stack of profiles.

```text
XML well-formedness
        |
UBL 2.1 XSD
        |
EN 16931 core business rules
        |
syntax/profile rules (for example Peppol BIS Billing)
        |
country / CIUS / extension rules
        |
code-list and identifier rules
        |
receiver / network eligibility rules
        |
transport / authority acknowledgement rules
```

The selected layers depend on the transaction route, seller country, buyer country, transaction type and target network.

### 3.1 Remaining validation work

#### A. Peppol BIS Billing validation

Add a separately versioned Peppol Billing profile after EN 16931. It should not be blended into the EN stage.

Conceptually:

```text
ubl21-xsd
    -> en16931
        -> peppol-bis-billing
```

The result model already supports the idea of ordered validation stages and can be extended with new profile/layer identifiers.

#### B. Country-specific CIUS and extensions

EN 16931 permits national and sector-specific usage specifications. A document may satisfy EN 16931 and still be unacceptable for a country's mandated profile.

The validator therefore needs a profile registry capable of selecting rules such as:

```text
EU core EN 16931
Peppol BIS Billing
country CIUS
country extension
specific authority/network profile
```

These rules must be pinned, versioned and reported in validation results just like the current EN artefact.

#### C. Versioned code lists

Codes used for currencies, units, tax categories, payment means, endpoint schemes and other controlled values need an explicit versioning/update strategy. A generic generation API cannot safely default or accept values without knowing which profile-specific code list applies.

#### D. Recipient and endpoint validation

Before delivery, validation must include routing-level checks that are not properties of the XML schema itself:

- recipient identifier exists;
- identifier scheme is correct;
- recipient is reachable through the selected network;
- required document type/process is supported by the recipient;
- sender is authorized for the selected delivery route.

#### E. National clearance/reporting rules

Some jurisdictions require direct tax-authority submission, clearance, signatures, sequence rules, authority identifiers, QR codes or other post-generation processing. These should be implemented as country connectors/profile modules, not hard-coded into the generic invoice model.

#### F. Complete conformance corpus

The current tests are strong for UBL XSD and EN 16931. Broader compliance needs a versioned conformance pack per supported profile containing:

- official positive examples where redistribution permits;
- targeted negative examples;
- country/profile-specific edge cases;
- routing/identifier cases;
- regression snapshots when a ruleset is upgraded.

#### G. All-document semantic scope

All recognized UBL main documents can receive XSD validation, but semantic/business-rule validation is currently intentionally special for Invoice and CreditNote. Any future promise of semantic compliance for `Order`, `DespatchAdvice`, `RemittanceAdvice`, etc. needs its own explicitly selected business profile.

---

## 4. Product extension: generic invoice API

### 4.1 Product goal

A client should not need to understand the complete UBL XML schema in order to issue a compliant invoice.

The product should expose a **generic business invoice model** in JSON. The caller supplies the smallest trustworthy set of business facts. The server then:

1. resolves seller/buyer profiles;
2. applies controlled defaults;
3. derives calculable values;
4. chooses the required compliance profile;
5. renders UBL XML;
6. validates the generated XML through the existing validation engine;
7. makes the validated XML available to the caller;
8. optionally queues the immutable validated document for delivery.

This is the common commercial pattern to emulate: a stable transaction API hides country/network syntax behind onboarding, enrichment, mapping, asynchronous processing and delivery status. Fonoa's public e-invoicing documentation, for example, presents optional party onboarding followed by transaction submission and asynchronous webhook/poll results. ecosio documents a flow where structured ERP data is validated/mapped to the receiver or authority format and then transmitted. The goal is to adopt the useful integration pattern, not to copy another provider's data model.

### 4.2 Two JSON representations must remain distinct

The project should support two different JSON concepts.

#### Generic Invoice JSON

A deliberately simplified business API for normal clients.

It does **not** attempt to expose every UBL XML node.

#### Canonical UBL JSON

A comprehensive, reversible representation of the full UBL document model for advanced integrations and XML <-> JSON round-tripping.

These must not be confused.

```text
Generic Invoice JSON
       |
       v
Canonical Invoice Domain Model
       |
       v
Canonical UBL Model
       |
       +--> UBL XML
       +--> full reversible UBL JSON
```

The generic model is the product abstraction. The canonical UBL model is the fidelity/standards abstraction.

---

## 5. Proposed minimal JSON interface

A first API can use a request similar to the following:

```json
{
  "invoiceNumber": "2026-00127",
  "buyer": "customer-42",
  "lines": [
    {
      "description": "Consulting services",
      "quantity": "2",
      "unitPrice": "125.00",
      "vatRate": "21"
    }
  ],
  "delivery": {
    "mode": "queue"
  }
}
```

Values such as money, quantities and tax rates should use decimal strings or exact decimal types in the API/model, never binary floating-point values.

In this example the server may obtain from registered configuration:

- seller legal name and address;
- seller VAT/tax identifiers;
- seller endpoint identifier;
- buyer legal/address/endpoint data from `customer-42`;
- document currency;
- issue date;
- invoice type code;
- unit code;
- tax category associated with the VAT rule;
- tax scheme identifier;
- payment means and seller bank details;
- EN/Peppol customization/profile identifiers;
- due-date/payment-term defaults;
- destination/network route.

The server derives:

- line extension amounts;
- VAT taxable bases;
- VAT amounts;
- invoice totals;
- payable amount.

A richer caller can override any field that the active policy permits.

---

## 6. Defaulting and enrichment policy

Defaulting must be deterministic, profile-aware and auditable. It must never become a mechanism for inventing legally significant information.

### 6.1 Sources of data

Every generated field should have a provenance category:

```text
request
seller_profile
buyer_profile
product_profile
tenant_default
compliance_profile
routing_lookup
derived
```

The transaction record should retain the provenance of important generated values so that operators can explain why a final XML field exists.

### 6.2 Good candidates for controlled defaults

Subject to the selected profile and tenant configuration:

- seller details from the authenticated tenant/company profile;
- buyer details from an explicitly referenced onboarded buyer;
- currency from tenant configuration;
- unit code from a configured line/product default;
- invoice type code for a normal invoice;
- tax scheme identifier;
- customization/profile identifiers;
- configured payment means;
- issue date, if the tenant explicitly permits server issue-date generation;
- due-date derivation from configured payment terms;
- generated document UUID/internal correlation identifier.

### 6.3 Values that should not be silently invented

Examples include:

- an unknown buyer identity;
- tax/VAT identifiers;
- a tax rate without an applicable configured tax rule;
- an endpoint/network identifier;
- legal registration identifiers;
- bank account details;
- exemptions or exemption reasons;
- amounts that cannot be deterministically calculated from submitted facts;
- country-specific legally significant statements.

If mandatory information cannot be obtained from the request, an onboarded profile or an approved rule, generation must fail with a structured `missing_data` result.

---

## 7. Party onboarding and profile registry

A competitor-style generic API needs party onboarding because otherwise every invoice request must repeat large amounts of legal/routing data.

Introduce a tenant-scoped party registry.

### 7.1 Seller profile

Typical fields:

- internal party ID;
- legal/trading names;
- postal address;
- country;
- tax/VAT IDs;
- company registration IDs;
- electronic endpoint IDs + schemes;
- default currency;
- payment accounts;
- payment terms;
- invoice-numbering policy;
- supported compliance/delivery profiles.

### 7.2 Buyer/customer profile

Typical fields:

- internal customer ID;
- legal name/address;
- VAT/tax identifiers;
- endpoint ID + scheme;
- preferred network/route;
- buyer reference / purchase-order requirements;
- required profile/CIUS if known;
- payment and language preferences.

### 7.3 API examples

```text
POST /api/v1/parties
GET  /api/v1/parties/{partyId}
PUT  /api/v1/parties/{partyId}

POST /api/v1/invoices
GET  /api/v1/invoices/{invoiceId}
GET  /api/v1/invoices/{invoiceId}/document
POST /api/v1/invoices/{invoiceId}/send
```

Party storage must be tenant-isolated from the beginning.

---

## 8. Proposed generation API behavior

### 8.1 Create invoice

```http
POST /api/v1/invoices
Content-Type: application/json
Idempotency-Key: <client supplied key>
```

The default operation should be asynchronous and return:

```json
{
  "operationId": "...",
  "invoiceId": "...",
  "status": "accepted"
}
```

The client may poll or receive a webhook when the document reaches a terminal generation/validation state.

### 8.2 Retrieve result

```http
GET /api/v1/invoices/{invoiceId}
```

Example successful response:

```json
{
  "invoiceId": "...",
  "status": "ready",
  "profile": "peppol-billing",
  "validation": {
    "status": "valid",
    "stages": [
      "ubl21-xsd",
      "en16931",
      "peppol-bis-billing"
    ]
  },
  "document": {
    "mediaType": "application/xml",
    "href": "/api/v1/invoices/.../document"
  },
  "delivery": {
    "status": "not_requested"
  }
}
```

### 8.3 Retrieve generated XML

```http
GET /api/v1/invoices/{invoiceId}/document
Accept: application/xml
```

The XML returned here must be the same immutable validated artifact used for delivery. The server must not regenerate a subtly different XML document later when sending.

### 8.4 Request delivery

Delivery can be requested either in the creation request or explicitly later:

```http
POST /api/v1/invoices/{invoiceId}/send
```

Only an immutable artifact that successfully passed every required validation layer may be queued.

---

## 9. Target architecture

```text
                       +---------------------------+
                       | OAuth2 / OIDC Provider    |
                       +-------------+-------------+
                                     |
                                     v
+-------------+        +---------------------------+
| ERP / App   |------->| Public Integration API    |
| Client      | JSON   |                           |
+-------------+        | auth + tenant + idempotency|
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       | Invoice Application       |
                       | Service                   |
                       +------+------+-------------+
                              |      |
                 +------------+      +----------------+
                 |                                    |
                 v                                    v
       +------------------+                 +--------------------+
       | Party / Customer |                 | Profile Resolver   |
       | Registry         |                 | country/network    |
       +------------------+                 +--------------------+
                 |                                    |
                 +---------------+--------------------+
                                 |
                                 v
                       +---------------------------+
                       | Enrichment / Defaults     |
                       | + calculation engine      |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       | Canonical Invoice Model   |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       | UBL Renderer / Mapper     |
                       +-------------+-------------+
                                     |
                                     v
                       +---------------------------+
                       | Validation Orchestrator   |
                       |                           |
                       | UBL 2.1 XSD               |
                       | EN 16931                  |
                       | Peppol / CIUS / country   |
                       +-------------+-------------+
                                     |
                        valid only   |
                                     v
                       +---------------------------+
                       | Immutable Artifact Store  |
                       | generated compliant XML   |
                       +-------------+-------------+
                                     |
                    +----------------+----------------+
                    |                                 |
                    v                                 v
          +------------------+              +--------------------+
          | Client retrieval |              | Outbound Queue     |
          | XML / full JSON  |              | delivery jobs      |
          +------------------+              +---------+----------+
                                                     |
                                                     v
                                           +--------------------+
                                           | Delivery Router    |
                                           +----+----------+----+
                                                |          |
                                  +-------------+          +-------------+
                                  v                                      v
                         +----------------+                     +----------------+
                         | Peppol/AP      |                     | Country / Tax  |
                         | connector      |                     | connector      |
                         +-------+--------+                     +-------+--------+
                                 |                                      |
                                 +------------------+-------------------+
                                                    |
                                                    v
                                           +--------------------+
                                           | Ack / Status /      |
                                           | Webhook / Audit     |
                                           +--------------------+
```

---

## 10. Component responsibilities

### 10.1 Public Integration API

Responsible for:

- authentication and tenant context;
- JSON schema validation;
- idempotency key handling;
- request correlation;
- creating generation operations;
- exposing status/artifact/delivery APIs;
- webhook registration and management.

It should not contain UBL rendering rules.

### 10.2 Canonical Invoice Domain Model

This becomes the stable product contract between client-facing JSON and standards-specific renderers.

It should model business concepts, not XML syntax:

- supplier/customer;
- invoice identity/dates;
- line items;
- quantities and prices;
- taxes;
- allowances/charges;
- payment information;
- references;
- totals;
- attachments/references where supported.

Version it independently, for example `invoice.v1`.

### 10.3 Enrichment/defaulting engine

Consumes:

```text
request + tenant profile + party profile + product/tax config + compliance profile
```

and produces a complete canonical invoice plus a field-provenance record.

The engine should use declarative rules where practical rather than scattering defaults across renderer code.

### 10.4 Calculation engine

All monetary calculations should use exact decimal arithmetic and explicit rounding rules.

It is responsible for deriving and verifying:

- line net amounts;
- allowances/charges;
- taxable amounts;
- VAT amounts;
- tax subtotals;
- invoice totals;
- payable amount.

The renderer should consume calculated values, not recalculate them independently.

### 10.5 Profile resolver

Selects the validation/rendering stack using facts such as:

- supplier country;
- buyer country;
- B2B/B2G/B2C;
- destination/network;
- buyer endpoint;
- requested process/profile;
- tenant configuration.

Example resolution:

```text
seller=IE
buyer=IE
network=PEPPOL
       |
       v
UBL 2.1
+ EN 16931
+ Peppol BIS Billing
+ applicable Irish profile/rules
```

### 10.6 UBL renderer

Maps the canonical invoice to a deterministic UBL XML representation.

Generated XML must never be treated as compliant merely because the renderer produced it. It must pass the same independent validation pipeline used for uploaded third-party XML.

### 10.7 Validation orchestrator

Reuse and extend the existing production validator.

Generation must call the same validation contracts and pinned artefacts as uploaded XML. This avoids a dangerous architecture in which generated documents are trusted while uploaded documents are validated.

### 10.8 Artifact store

Store at least:

- original client JSON;
- normalized/enriched canonical invoice snapshot;
- immutable generated XML;
- validation result;
- optional authority/network receipts and acknowledgements.

The generated XML should be content-addressed or checksum-recorded so that the system can prove the sent artifact is exactly the validated artifact.

### 10.9 Outbound queue

Add a delivery stream separate from the validation work stream.

A delivery descriptor should remain small:

```json
{
  "deliveryId": "...",
  "invoiceId": "...",
  "artifactId": "...",
  "tenantId": "...",
  "route": "peppol",
  "recipient": "...",
  "createdAt": "..."
}
```

Do not put the full XML in the queue message. The delivery worker retrieves the immutable validated artifact from storage, following the same data-movement principle already used by the validation worker.

### 10.10 Delivery router and connectors

Define a stable connector interface such as:

```text
ResolveDestination(...)
ValidateRoute(...)
Send(...)
QueryStatus(...)
```

Initial implementation should favor integration with an existing Peppol Access Point/provider rather than implementing the complete Peppol transport/security stack inside the core server. A native Access Point can remain a later strategic choice.

Country clearance/reporting integrations should implement the same connector boundary.

### 10.11 Delivery state machine

Generation/validation status and delivery status should be related but separate.

Suggested lifecycle:

```text
accepted
  -> enriching
  -> rendering
  -> validating
      -> generation_failed
      -> invalid
      -> ready
           -> delivery_queued
           -> sending
           -> delivered
           -> accepted_by_network_or_authority
           -> rejected_by_network_or_authority
           -> delivery_failed
```

A compliant document may still fail delivery because of routing, network or recipient problems. Do not collapse those into `invalid`.

---

## 11. Data and persistence architecture

The current Object Store + KV + JetStream design is strong for asynchronous document processing and should be retained for artifacts and work queues.

The generic API introduces durable business configuration that is better suited to YugabyteDB/YSQL:

- tenants;
- users/authorization mappings;
- seller profiles;
- buyer/customer profiles;
- tax/product defaults;
- invoice canonical state;
- idempotency keys;
- delivery attempts;
- webhook subscriptions;
- audit records;
- searchable business identifiers.

Recommended split:

```text
YugabyteDB/YSQL
  -> business metadata, tenant data, canonical records, routing config

JetStream Object Store
  -> immutable/raw/generated XML and large artefacts

JetStream streams
  -> generation, validation, delivery and event work

JetStream KV
  -> lightweight operational state where appropriate
```

Every persistent access path must include/enforce tenant identity.

---

## 12. API idempotency and exactly-once business effect

Invoice creation and sending require idempotency beyond transport-level message de-duplication.

Use a client-provided `Idempotency-Key` scoped to tenant + operation. Persist the request fingerprint and resulting operation/invoice ID.

Repeated requests with the same key and same body should return the original operation. The same key with a materially different body should fail.

Delivery should have its own durable delivery ID and claim, because a network timeout can leave the system uncertain whether the remote side accepted the document.

The design objective is not impossible distributed "exactly once transport"; it is **exactly one durable local business outcome plus idempotent/reconcilable remote delivery**.

---

## 13. Webhooks and client integration

A generic commercial API should not require high-frequency polling.

Support signed webhooks for events such as:

```text
invoice.ready
invoice.invalid
invoice.generation_failed
delivery.queued
delivery.sent
delivery.accepted
delivery.rejected
delivery.failed
```

Webhook delivery itself needs retries, signatures, event IDs and replay protection.

Polling endpoints should remain available as a fallback.

---

## 14. Recommended implementation sequence

Do not assign permanent project TODO numbers from this document; add them to the authoritative TODO register when implementation is scheduled.

### Work package A: Generic invoice model

- define `invoice.v1` JSON schema;
- exact decimal/money model;
- define required vs defaultable fields;
- define field provenance;
- publish OpenAPI examples.

### Work package B: Party onboarding

- tenant/seller profile;
- buyer/customer profile;
- endpoint identifiers;
- payment defaults;
- tax/product defaults;
- tenant isolation.

### Work package C: Calculation and enrichment

- exact monetary calculation engine;
- defaulting policy;
- missing-data diagnostics;
- deterministic canonical invoice snapshot.

### Work package D: UBL generation

- canonical invoice -> UBL Invoice;
- CreditNote generation next;
- deterministic renderer;
- generated-artifact checksum;
- generator fixture corpus.

### Work package E: Generation + existing validation integration

- generated XML enters existing XSD + EN pipeline;
- never publish `ready` unless all required layers pass;
- return/download the exact validated XML;
- preserve structured validation stages.

This is the first useful product milestone: **JSON in -> compliant validated UBL XML out**.

### Work package F: Profile framework

- profile resolver;
- Peppol BIS Billing validation;
- versioned code lists;
- country/CIUS plug-ins;
- profile-specific rendering/defaulting.

### Work package G: Outbound delivery

- outbound JetStream stream;
- delivery state machine;
- connector interface;
- Peppol/provider connector;
- routing lookup;
- acknowledgements;
- delivery retries/reconciliation;
- webhooks.

### Work package H: Full multi-tenant persistence and operational API

- PostgreSQL schema;
- canonical invoice storage;
- party/customer CRUD;
- search;
- audit;
- OAuth2/OIDC/JWKS;
- scopes/roles;
- tenant enforcement.

---

## 15. Suggested first commercial API milestone

The strongest next milestone is deliberately narrower than a full global e-invoicing platform:

> **A tenant-configured API that accepts a minimal JSON Invoice, enriches it from registered seller/buyer defaults, generates UBL 2.1, proves EN 16931 compliance using the existing validator, and returns the exact validated XML.**

Delivering this first has several advantages:

- it turns the existing validator into a document-production product;
- it gives ERP/application clients a much simpler integration than raw UBL;
- it reuses the strongest existing code rather than replacing it;
- it establishes the canonical/defaulting architecture needed for later Peppol and country connectors;
- it cleanly separates document compliance from network delivery.

After that milestone, add Peppol/profile validation and an outbound connector so the API becomes:

```text
minimal JSON
    -> enrich/default
    -> generate UBL
    -> validate
    -> return XML
    -> queue
    -> route
    -> transmit
    -> acknowledge
    -> webhook/status
```

---

## 16. Architectural invariants

The following rules should remain non-negotiable as the product expands:

1. **Generated does not mean valid.** Every generated XML artifact must pass the independent validation pipeline.
2. **Sent equals validated.** Delivery uses the exact immutable artifact that was validated.
3. **Standards profiles are versioned.** EN, Peppol, CIUS, code lists and country rules are pinned and reported.
4. **Defaults are explainable.** Important generated fields retain provenance.
5. **Legal facts are not invented.** Missing mandatory data fails generation unless an authorized profile supplies it.
6. **Money uses exact arithmetic.** No floating-point financial calculations.
7. **Validation and delivery are separate state machines.** A valid invoice can still fail delivery.
8. **Queues carry references, not large documents.** Reuse the current Object Store + descriptor pattern.
9. **Tenant isolation is structural.** It is enforced in identity, storage, profile lookup, idempotency and delivery.
10. **Transport adapters do not redefine the invoice model.** Country/network peculiarities belong behind profile/connector interfaces.
11. **Public JSON remains stable.** Changes in UBL/Peppol syntax should normally be absorbed by renderers/profiles instead of breaking every ERP integration.
12. **Every remote send is reconcilable.** Ambiguous network outcomes must be queryable/retryable without creating uncontrolled duplicate business effects.

---

## 17. External design references

The target generic API pattern is informed by publicly documented commercial e-invoicing integration approaches, without making their private/internal canonical models a dependency:

- Fonoa e-Invoicing quickstart: https://docs.fonoa.com/reference/quickstart
- Fonoa transaction submission: https://docs.fonoa.com/reference/submit-a-transaction
- Fonoa e-Invoicing overview: https://docs.fonoa.com/reference/about-fonoa-e-invoicing
- ecosio e-invoicing overview: https://connect.ecosio.com/docs/e-invoicing-overview
- ecosio Ireland automated e-invoicing flow: https://connect.ecosio.com/docs/compliance-for-ireland

Repository standards/architecture references include:

- `server/FUNCTIONALITY.md`
- `TODO-EN-16931.md`
- `docs/EN16931-VALIDATION-CONTRACT.md`
- `docs/EN16931-COMPLIANCE-GATE.md`
- `docs/EN16931-DEPLOYMENT.md`
- `docs/EN16931-CAPACITY.md`
- `.github/roadmap-0.md`
- `docs/UBL-2.1.md`

---

## 18. Target product statement

The long-term server should be describable as follows:

> e-ubl21 provides a stable JSON integration API for creating, validating and delivering electronic invoices. Clients submit business invoice data rather than standards-specific XML. The service enriches the transaction from onboarded party/configuration data, deterministically generates the required UBL document, validates it through the applicable UBL/EN/profile/country rules, exposes the exact compliant artifact, and optionally routes that immutable artifact to the recipient or tax authority through an asynchronous, observable and replay-safe delivery pipeline.
