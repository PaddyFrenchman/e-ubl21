# YugabyteDB client ownership and party master-data schema

## Purpose

This document defines the first relational model for client ownership and customer/supplier master data in e-ubl21.

The design supports the third-party integration architecture described in `ideas/third-party-integration.md` and the planned migration to YugabyteDB described in `ideas/postgresql-to-yugabytedb-migration-plan.md`.

The central rule is:

> Every piece of client-owned business data belongs to exactly one e-ubl21 client, and that ownership is represented explicitly by `owner_id`, whose value is the e-ubl21 `client.id`.

This ownership model must apply consistently across YugabyteDB rows, Go durable structures, cache keys, object-storage keys, idempotency records, and asynchronous messages.

---

# Terminology

The word **client** has one precise meaning in this document:

- a **client** is a customer of e-ubl21;
- the primary key of that client is the **owner ID**;
- every business record owned by that client carries `owner_id`;
- `owner_id` is a foreign key to `client.id` when the record is stored in YugabyteDB.

A **party** is a business or person belonging to the client's accounting/master-data universe. A party can play one or more roles such as:

- customer;
- supplier;
- both customer and supplier;
- subsidiary;
- other trading/business partner roles added later.

The same party must not be duplicated merely because one source system calls it a customer and another calls it a supplier.

A **principal** is a user, service account, or other authenticated identity that may be authorized to act for one or more clients.

The existing project frequently uses `tenant` or `customer identifier` terminology. New persistence and Go APIs should converge on:

```text
OwnerID / owner_id = e-ubl21 client ID
```

Documentation may continue to describe the system as multi-tenant, but persisted ownership should use one unambiguous term.

---

# Ownership invariants

These are product invariants, not optional implementation conventions.

1. Every client-owned YugabyteDB table contains `owner_id NOT NULL`.
2. `owner_id` references `client.id`.
3. Client-owned child tables use owner-aware foreign keys, normally `(owner_id, parent_id)` to `(owner_id, id)`, so a row can never reference a parent owned by another client.
4. Every query, update and delete of client-owned data is scoped by `owner_id`.
5. The API derives the owner from the authenticated authorization context. A caller must not be allowed to escape its authorized owner scope merely by supplying another `owner_id` in a request body.
6. Ownership is immutable after record creation. Moving data between clients is an explicit copy/import operation, not an `UPDATE owner_id` operation.
7. External source identifiers are unique only inside the owning client and source-system context.
8. Cache entries are owner-scoped.
9. Object-storage paths are owner-scoped.
10. Idempotency keys are owner-scoped.
11. Durable messages carry `ownerId` whenever the message references client-owned state.
12. Workers and reconcilers verify that owner identity agrees across the upload, execution and message being processed.
13. Global reference data such as ISO country codes, currency codes, UNECE unit codes and versioned validation assets is not client-owned and therefore does not require `owner_id`.

The existing Phase 3 roadmap already requires every stored document to participate in a customer/tenant ownership boundary. This proposal generalizes that rule from stored UBL documents to every client-owned business object in the platform.

---

# High-level model

```text
                         client
                           |
              +------------+-------------+
              |                          |
       access-control              integration sources
              |                          |
              v                          v
      client access grants       integration_connection
                                         |
                                         v
                                  master_import_batch
                                         |
                                         v
                                   source_record
                                         |
                                         v
                                       party
                      +------------------+------------------+
                      |                  |                  |
                      v                  v                  v
                 party_role       party_address    party_external_id
                      |                                     |
                      |                  +------------------+------------------+
                      |                  |                  |                  |
                      v                  v                  v                  v
                customer/supplier  identifiers      tax registrations   electronic addresses

All client-owned rows carry the same owner_id.
```

---

# Client and access-control tables

## `client`

One row represents one customer of e-ubl21 and defines the ownership root for all client data.

Suggested columns:

| Column | Type | Purpose |
| --- | --- | --- |
| `id` | UUID | Primary key and owner ID used everywhere else |
| `code` | TEXT | Human-friendly stable client code |
| `legal_name` | TEXT | Client legal/business name |
| `display_name` | TEXT | UI display name |
| `status` | TEXT | `active`, `suspended`, `disabled`, etc. |
| `home_region` | TEXT | Preferred processing/data locality region |
| `data_residency_policy` | TEXT/JSONB | Future residency policy |
| `default_currency_code` | TEXT | Optional client-wide safe default |
| `default_target_profile` | TEXT | Optional default output/validation profile |
| `created_at` | TIMESTAMPTZ | Creation time |
| `updated_at` | TIMESTAMPTZ | Last administrative update |

Important constraints:

```text
PRIMARY KEY (id)
UNIQUE (code)
```

`client.id` is the value used as `owner_id` everywhere else.

## `auth_principal`

Represents an authenticated user or service identity independently of a particular client.

Suggested columns:

| Column | Type | Purpose |
| --- | --- | --- |
| `id` | UUID | Internal principal ID |
| `principal_type` | TEXT | `user`, `service`, `integration` |
| `issuer` | TEXT | OAuth/OIDC issuer |
| `subject` | TEXT | OAuth/OIDC subject or service identity |
| `display_name` | TEXT | Administrative display value |
| `email` | TEXT | Optional user email |
| `status` | TEXT | Active/disabled |
| `created_at` | TIMESTAMPTZ | Creation time |
| `updated_at` | TIMESTAMPTZ | Update time |

Suggested uniqueness:

```text
UNIQUE (issuer, subject)
```

Credentials and secrets should not be stored in this table. It identifies the authenticated principal. Secret material remains in the configured secret-management system.

## `client_role`

Defines roles owned by a client.

Suggested columns:

```text
owner_id
id
name
built_in
created_at
updated_at
```

Example roles:

```text
client-admin
invoice-submit
invoice-read
master-data-admin
integration-admin
auditor
```

Primary key:

```text
PRIMARY KEY (owner_id, id)
FOREIGN KEY (owner_id) REFERENCES client(id)
UNIQUE (owner_id, name)
```

## `client_role_permission`

Maps a client role to explicit permissions.

Suggested columns:

```text
owner_id
role_id
permission_code
```

Examples:

```text
invoice.submit
invoice.read
invoice.delete
masterdata.read
masterdata.write
integration.manage
client.access.manage
audit.read
```

Primary key:

```text
PRIMARY KEY (owner_id, role_id, permission_code)
FOREIGN KEY (owner_id, role_id) REFERENCES client_role(owner_id, id)
```

## `client_principal_role`

Grants a principal a role within one client.

Suggested columns:

```text
owner_id
principal_id
role_id
granted_at
granted_by_principal_id
expires_at
```

The important point is that authorization is always evaluated in an owner context. One authenticated user can therefore legitimately have access to Client A and Client B without the data of those clients becoming commingled.

---

# Integration and source-provenance tables

## `integration_connection`

Represents one configured source-system connection for one owner.

Suggested columns:

| Column | Purpose |
| --- | --- |
| `owner_id` | Client owner |
| `id` | Integration connection ID |
| `system` | `sage-active`, `xero`, `quickbooks-online`, `sap-b1`, `sap-s4`, `netsuite`, etc. |
| `system_version` | API/schema version |
| `name` | Administrative name |
| `external_account_id` | Source tenant/company/account identifier |
| `credentials_secret_ref` | Reference to secret storage, never raw credentials |
| `status` | Active/disabled/error |
| `last_sync_at` | Last successful synchronization |
| `created_at` | Creation time |
| `updated_at` | Update time |

Primary key:

```text
PRIMARY KEY (owner_id, id)
```

## `master_import_batch`

Represents one source synchronization or uploaded client export.

Suggested columns:

```text
owner_id
id
integration_connection_id nullable
import_method              -- api-sync, csv, json, xml, spreadsheet, manual
source_name
source_object_key nullable  -- original uploaded export in object storage
source_hash
started_at
finished_at
status
records_seen
records_imported
records_rejected
error_summary
```

This table allows an onboarding report to say exactly which import created or changed a party.

## `source_record`

Represents an immutable source-system record observed during an import/sync.

Suggested columns:

```text
owner_id
id
import_batch_id
integration_connection_id nullable
entity_type               -- customer, supplier, contact, business-partner, etc.
external_id
source_revision nullable
source_hash
raw_payload_object_key nullable
observed_at
```

This is the durable bridge between canonical data and the third-party record from which it came.

Large raw source payloads should normally be stored in object storage. YugabyteDB stores their hashes, identifiers and object references.

---

# Canonical party tables

## Design decision: one `party`, not separate customer and supplier tables

Do not create independent `customer` and `supplier` master tables.

The same legal entity can be both a customer and a supplier, and source products model these concepts differently. Xero has contacts, SAP Business One has business partners, NetSuite has entity records, and enterprise systems can expose the same organization under several source roles.

Use one canonical `party` table plus `party_role`.

## `party`

Stable canonical identity for a customer, supplier or other business partner.

Suggested columns:

| Column | Purpose |
| --- | --- |
| `owner_id` | Owning e-ubl21 client |
| `id` | Canonical party ID |
| `party_type` | `organization` or `person` |
| `legal_name` | Legal registered name |
| `trading_name` | Optional trading/display name |
| `status` | Active/inactive/archived |
| `default_currency_code` | Optional party-specific default |
| `default_payment_means_code` | Optional configured payment default |
| `version` | Monotonic master-data version |
| `created_at` | Creation time |
| `updated_at` | Update time |
| `archived_at` | Soft-retirement time |

Primary key:

```text
PRIMARY KEY (owner_id, id)
FOREIGN KEY (owner_id) REFERENCES client(id)
```

A party should normally be archived rather than hard-deleted after it has participated in invoice conversion, because historical provenance must remain explainable.

## `party_role`

Records the roles a canonical party plays for the owner.

Suggested columns:

```text
owner_id
party_id
role                 -- customer, supplier, subsidiary, etc.
valid_from nullable
valid_to nullable
source_record_id nullable
created_at
```

Primary key:

```text
PRIMARY KEY (owner_id, party_id, role)
FOREIGN KEY (owner_id, party_id) REFERENCES party(owner_id, id)
```

A party that is both a customer and supplier simply has two rows.

## `party_external_id`

Maps canonical party identity to third-party identities.

Suggested columns:

```text
owner_id
id
party_id
integration_connection_id
source_entity_type
external_id
external_parent_id nullable
source_record_id nullable
active
created_at
updated_at
```

Examples:

```text
Sage customer 742
Xero ContactID 0a7e...
SAP Business Partner C10037
NetSuite entity 220
```

Suggested uniqueness:

```text
UNIQUE (owner_id, integration_connection_id, source_entity_type, external_id)
```

This uniqueness must include `owner_id`. An external ID such as `742` is not globally meaningful.

## `party_address`

Stores one or more addresses for a party.

Suggested columns:

```text
owner_id
id
party_id
address_type           -- legal, billing, delivery, postal, other
line1
line2
line3
city
region
postal_code
country_code
is_default
source_record_id nullable
valid_from nullable
valid_to nullable
created_at
updated_at
```

Address rows remain owner-scoped even though they are already linked to a party. This makes isolation explicit and allows owner-first indexing and locality decisions.

## `party_identifier`

Stores legal/business identifiers that are not tax registrations.

Suggested columns:

```text
owner_id
id
party_id
identifier_type
scheme_id
value
issuing_country nullable
is_primary
source_record_id nullable
valid_from nullable
valid_to nullable
created_at
updated_at
```

Examples include company-registration identifiers and other jurisdiction-specific legal/business IDs.

Suggested uniqueness where appropriate:

```text
UNIQUE (owner_id, scheme_id, value)
```

Do not place ERP internal IDs in this table. ERP IDs belong in `party_external_id`.

## `party_tax_registration`

Stores tax registrations without assuming EU VAT semantics.

Suggested columns:

```text
owner_id
id
party_id
jurisdiction
tax_scheme
scheme_id nullable
registration_id
country_code nullable
is_primary
source_record_id nullable
valid_from nullable
valid_to nullable
created_at
updated_at
```

Examples:

```text
EU VAT registration
UK VAT registration
GST registration
sales-tax registration
other national tax schemes
```

The canonical model must permit several registrations for the same party.

## `party_electronic_address`

Stores electronic-invoicing/network addresses independently of any one network.

Suggested columns:

```text
owner_id
id
party_id
network
scheme_id
address
is_default
source_record_id nullable
valid_from nullable
valid_to nullable
created_at
updated_at
```

Examples:

```text
Peppol participant endpoint
national e-invoicing network identifier
customer-specific EDI endpoint
future network identifiers
```

Do not model this as a single `peppol_endpoint` column on `party` because the product is intended for a global market and a party may participate in several networks.

## `party_contact`

Stores human/business contacts where required by the source or target profile.

Suggested columns:

```text
owner_id
id
party_id
name
title
role
email
phone
mobile
is_default
source_record_id nullable
created_at
updated_at
```

## `party_payment_account`

Optional but recommended because supplier/master-data enrichment can require payee account information.

Suggested columns:

```text
owner_id
id
party_id
account_scheme
account_id
account_name
bank_identifier nullable
bank_name nullable
currency_code nullable
is_default
source_record_id nullable
valid_from nullable
valid_to nullable
created_at
updated_at
```

Sensitive financial values should use the project's encryption-at-rest and secret/data-protection policy. They must never be used as telemetry labels.

## `party_commercial_profile`

Stores owner-specific defaults and commercial semantics that are useful during invoice enrichment but do not belong to universal party identity.

Suggested columns:

```text
owner_id
party_id
default_currency_code nullable
default_payment_means_code nullable
default_payment_terms_code nullable
default_delivery_address_id nullable
default_buyer_reference nullable
settings_json nullable
updated_at
```

This table is deliberately separate from source mappings. It says what the owner has configured for this relationship, not how a Sage/Xero/SAP code should be interpreted.

---

# Provenance and audit tables

## `party_attribute_provenance`

Scalar canonical values such as `party.legal_name` can be changed by imports or manual administration. Their source must remain explainable.

Suggested columns:

```text
owner_id
id
party_id
attribute_path          -- legal_name, trading_name, etc.
source_kind             -- source-record, manual, rule, system
source_record_id nullable
rule_id nullable
actor_principal_id nullable
value_hash
recorded_at
```

Repeating values such as addresses, tax registrations and electronic addresses already have their own row-level source references. `party_attribute_provenance` is principally for scalar values on the canonical party record and explicit overrides.

## `master_data_audit_event`

Append-only administrative/change audit.

Suggested columns:

```text
owner_id
id
entity_type
entity_id
action
actor_principal_id nullable
source_record_id nullable
previous_version nullable
new_version nullable
change_json
occurred_at
```

This table supports questions such as:

```text
Who changed this endpoint?
Which import changed this VAT/tax registration?
What was the party record before the change?
Which master-data version was used during invoice conversion?
```

Audit rows are client-owned and therefore carry `owner_id` too.

---

# Owner-aware foreign-key pattern

For client-owned relational data, prefer owner-aware composite keys and foreign keys.

Conceptually:

```sql
CREATE TABLE party (
    owner_id UUID NOT NULL,
    id       UUID NOT NULL,
    legal_name TEXT NOT NULL,
    ...,
    PRIMARY KEY (owner_id, id),
    FOREIGN KEY (owner_id) REFERENCES client(id)
);

CREATE TABLE party_address (
    owner_id UUID NOT NULL,
    id       UUID NOT NULL,
    party_id UUID NOT NULL,
    ...,
    PRIMARY KEY (owner_id, id),
    FOREIGN KEY (owner_id, party_id)
        REFERENCES party(owner_id, id)
);
```

The composite foreign key is important. A simple `party_id` foreign key combined with an unrelated `owner_id` column would still permit an accidental cross-client reference if application code contained a bug.

Database constraints should make that class of bug impossible.

---

# Indexing rules

Most indexes over client-owned business data should begin with `owner_id` unless a deliberately global operational query requires otherwise.

Examples:

```text
(owner_id, legal_name)
(owner_id, status)
(owner_id, party_id, role)
(owner_id, integration_connection_id, source_entity_type, external_id)
(owner_id, scheme_id, value)
(owner_id, tax_scheme, registration_id)
(owner_id, network, scheme_id, address)
(owner_id, import_batch_id)
```

This gives three benefits:

1. queries naturally enforce owner scope;
2. YugabyteDB can distribute/localize data primarily by owner;
3. an accidental unscoped query is easier to identify in code review and testing.

Searches that need case-insensitive names or fuzzy matching can add dedicated indexes later after real query patterns are measured.

---

# YugabyteDB locality

`owner_id` should be the logical sharding/locality anchor for client-owned data.

The `client` row records the client's preferred `home_region` and future residency policy.

Conceptually:

```text
client A owner_id -> Europe placement
client B owner_id -> North America placement
client C owner_id -> Asia-Pacific placement
```

The first schema migration does not need to hard-wire one permanent physical partitioning strategy. It does need to make owner identity available in every row so future YugabyteDB geo-partitioning can be introduced without redesigning the business schema.

Do not duplicate `home_region` into every table unless the selected YugabyteDB placement strategy later requires it. `owner_id` is authoritative ownership; locality metadata belongs to `client` unless deliberately denormalized for a proven physical-placement requirement.

---

# Ownership in Go structures

The owner concept must not exist only in the database. Any independently persisted, addressed or transmitted client-owned aggregate must carry it.

Use a common semantic field:

```go
type OwnerID string
```

or an equivalent strongly validated type.

New canonical structures should use:

```go
OwnerID string `json:"ownerId"`
```

instead of introducing additional `TenantID`, `CustomerID` or `ClientID` synonyms.

At minimum the current validation architecture will need owner identity added to the client-owned top-level structures that cross durable boundaries, including:

```text
UploadRecord
ValidationExecutionIntent
ValidationExecution
StageRequest
StageResult
ValidationResult
replay/revalidation commands
dead-letter records
content-expiry/tombstone records
third-party SourceProvenance
source/import snapshots
future canonical billing-document persistence
```

Nested value objects do not all need their own `OwnerID` when they can exist only inside an already-owned aggregate. For example, every `ValidationIssue` inside a `StageResult` does not need another owner field. The top-level durable/wire aggregate does.

The current `UploadRecord`, `ValidationExecution`, `StageRequest` and `StageResult` structures do not yet carry this owner field, so ownership must be introduced as an explicit versioned contract change rather than assumed implicitly.

---

# Existing storage that must become owner-aware

## Upload metadata

Current upload metadata is keyed by upload ID. The future YugabyteDB representation must include `owner_id`, and lookup semantics become:

```text
owner_id + upload_id
```

The API must never perform a client-visible `GET /api/v1/ubl/{id}` using only the upload ID after multi-tenancy is introduced.

## Upload request idempotency

The validation ownership contract already states that idempotency is scoped to the authenticated client/tenant.

The durable key therefore becomes conceptually:

```text
owner_id + idempotency_key
```

not a globally unique idempotency key.

## Raw XML/object storage

Current JetStream Object Store entries are keyed only by upload ID. Any future object-storage layout must encode ownership explicitly.

Recommended logical path:

```text
clients/{owner_id}/uploads/{upload_id}/source.xml
```

Equivalent owner-prefixed keys should be used for generated XML, third-party source snapshots and import files.

Ownership must also be stored in metadata, not inferred only from the string path.

## Validation executions

Validation executions and all subsequent stage messages derive from an upload. The execution must copy and freeze the upload's `owner_id` at creation.

A stage request/result is invalid if its owner does not agree with the referenced execution/upload.

This gives an identity chain:

```text
client.id
   |
   +--> UploadRecord.OwnerID
           |
           +--> ValidationExecution.OwnerID
                   |
                   +--> StageRequest.OwnerID
                   |       |
                   |       +--> StageResult.OwnerID
                   |
                   +--> ValidationResult.OwnerID
```

The owner is copied, not re-resolved from an external caller at each stage.

## NATS / JetStream subjects and message bodies

Message bodies must carry `ownerId`.

It is not necessary to encode owner IDs into every NATS subject if that would create uncontrolled subject cardinality or complicate stream topology. The authoritative ownership field is the validated message/record field.

## Valkey

Every client-owned Valkey key is prefixed/scoped by owner.

Examples:

```text
party:{owner_id}:{party_id}
party-external:{owner_id}:{system}:{external_id}
party-tax:{owner_id}:{scheme}:{registration_id}
integration:{owner_id}:{connection_id}
```

Valkey is disposable. A cache miss falls back to YugabyteDB, and flushing Valkey must never change authorization or correctness.

## Metrics

Owner IDs must not become Prometheus metric labels. They are unbounded/high-cardinality identifiers. Existing telemetry policy already avoids tenant identifiers as metric labels, and the owner model must preserve that rule.

Logs and audit records may carry owner context where operationally required, subject to privacy/security policy.

---

# Authorization flow

The intended request flow is:

```text
OAuth/OIDC token
      |
      v
authenticated principal
      |
      v
client_principal_role
      |
      v
resolved authorized owner_id + permissions
      |
      v
owner-scoped request context
      |
      +--> YugabyteDB queries
      +--> Valkey keys
      +--> object-storage keys
      +--> NATS messages
```

The application should establish an owner-scoped request context once authentication/authorization succeeds.

Repository/service functions that operate on client-owned data should accept owner explicitly. Prefer APIs shaped like:

```go
GetParty(ctx, ownerID, partyID)
GetUpload(ctx, ownerID, uploadID)
GetExternalParty(ctx, ownerID, integrationID, externalID)
```

rather than APIs that accept only the object ID and hope authorization is applied elsewhere.

---

# Import and merge behavior

An import from Sage/Xero/SAP/etc. should follow this sequence:

```text
authenticated owner
      |
      v
integration_connection(owner_id)
      |
      v
master_import_batch(owner_id)
      |
      v
source_record(owner_id)
      |
      v
lookup party_external_id within same owner
      |
      +--> found: update/enrich canonical party under version/audit rules
      |
      `--> not found: create canonical party or produce match candidate
```

Automatic cross-client matching is forbidden. Even if two clients both trade with the same legal company, each client owns its own canonical party record and its own enrichment/mapping choices.

A future global directory could exist as a separate reference service, but it must not collapse client ownership of master data.

---

# Deletion and historical correctness

Hard deletion of master data should be rare.

If a supplier is removed from Sage after an invoice was already converted, the historical invoice must remain explainable.

Therefore:

- parties normally become `inactive` or `archived`;
- external IDs may become inactive without destroying history;
- import batches and source-record hashes remain available according to retention policy;
- audit history is append-only;
- generated/resolved billing documents retain the party values/provenance used at conversion time rather than re-reading today's master data when historical output is retrieved.

Master data is current enrichment state. It is not a mutable substitute for historical invoice evidence.

---

# Tables deliberately outside this document

The following YugabyteDB domains are related but should have separate detailed schemas:

```text
products/items
product external IDs
tax-code mappings
unit mappings
payment-method mappings
semantic-resolution rules
billing-document persistence
upload/validation execution persistence
mapping-rule version history
```

All of them must follow the same ownership invariant whenever the data belongs to one e-ubl21 client:

```text
owner_id -> client.id
```

Global standards/reference tables are the exception.

---

# Minimum first implementation

The minimum useful party/master-data release should include:

```text
client
auth_principal
client_role
client_role_permission
client_principal_role
integration_connection
master_import_batch
source_record
party
party_role
party_external_id
party_address
party_identifier
party_tax_registration
party_electronic_address
party_contact
party_attribute_provenance
master_data_audit_event
```

`party_payment_account` and `party_commercial_profile` can be included immediately if the first Sage/Xero integration requires them, otherwise they can follow without changing the ownership model.

---

# Required tests for the ownership model

Ownership must be tested as a security boundary.

At minimum:

- Client A cannot read Client B's party by knowing its ID.
- Client A cannot update or delete Client B's party.
- A child row cannot reference a parent row under another owner.
- The same third-party external ID can legitimately exist for different owners.
- Within one owner/source/entity type, duplicate external IDs are rejected.
- An idempotency key can be reused by two different owners without collision.
- The same upload ID is never resolved outside its owner context.
- StageRequest/StageResult owner mismatch is rejected.
- Object-storage access is owner-scoped.
- Valkey keys cannot collide across owners.
- Access-grant tests cover user and service principals.
- Revoking a principal's client role removes future access without modifying historical audit identity.
- Import/synchronization never merges records between clients.
- Regression tests continue to pass with an explicit test owner added to all existing fixtures.

These tests belong alongside the database migration and future multi-tenant regression topology, not only in UI/API tests.

---

# Implementation consequence

Introducing `owner_id` is a cross-cutting architectural change and should be done before large amounts of new persistence code are written.

The existing validation system currently identifies durable records primarily by upload/execution IDs. The correct long-term model is:

```text
owner_id + object identity
```

for every client-owned aggregate.

That gives e-ubl21 one ownership model across:

```text
access control
third-party master data
invoice imports
canonical billing documents
UBL persistence
validation metadata
object storage
cache
asynchronous processing
audit/provenance
```

This should become a platform invariant before the third-party integration subsystem begins storing production client data.
