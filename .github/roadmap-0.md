# UBL-21 Validator — Roadmap 0

**Document:** `Roadmap-0.md`  
**Status:** High-level project definition  
**Scope:** UBL 2.1 validation, API access, and persistent multi-tenant document storage

## 1. Project Aim

The aim of the UBL-21 Validator project is to provide a production-oriented Go server capable of validating, exposing, converting, storing, retrieving, and managing every document type defined by UBL 2.1.

The project is intended to evolve in three major phases:

1. validation of all UBL 2.1 document types;
2. comprehensive OpenAPI-based access to the complete contents of every UBL 2.1 document type;
3. full multi-tenant database persistence and CRUD support for every UBL 2.1 document type.

The server must support both XML and JSON representations while preserving the complete UBL information model.

---

## 2. Core Architectural Principles

### 2.1 Go server

The application server is written in **Go (Golang)**.

Go is the primary implementation language for:

- HTTP server and routing;
- authentication and authorization integration;
- document-type detection;
- validation orchestration;
- XML and JSON serialization;
- OpenAPI implementation;
- persistence orchestration;
- tenant isolation;
- process execution when external tools are used.

### 2.2 Complete UBL 2.1 coverage

The target is **all UBL 2.1 document types**, not only Invoice, CreditNote, or an initial subset.

The authoritative OASIS UBL 2.1 XSD distribution is the normative schema source.

The server must identify the submitted UBL document type and apply the correct UBL 2.1 schema and processing rules.

### 2.3 Lossless XML and JSON representations

Every supported document must be retrievable in:

- XML;
- JSON.

Conversion between the two representations must preserve all information required to reconstruct the original UBL document semantics.

This includes, at minimum:

- XML element names;
- qualified names;
- namespaces;
- namespace prefixes where preservation is required by the chosen canonical model;
- XML attributes;
- attribute namespaces;
- element values;
- nested aggregate structures;
- repeated elements;
- document structure;
- ordering where ordering is significant;
- UBL extension content;
- identifiers and codes;
- datatype metadata required for faithful reconstruction.

A conventional simplified XML-to-JSON mapping is **not sufficient** if it loses namespaces, attributes, ordering, or other XML information.

The implementation must therefore define a canonical internal representation or an explicitly documented reversible JSON representation capable of deterministic XML ↔ JSON round-tripping.

### 2.4 OAuth 2

The server must support **OAuth 2.0** authentication and authorization.

The server should normally operate as an OAuth 2.0 protected resource server, accepting and validating access tokens issued by an external authorization server.

The protected API surface must include an authenticated upload endpoint that accepts an XML file payload containing a UBL 2.1 document, for example via a `POST /api/v1/ubl/upload` call with a bearer token in the `Authorization` header.

Authorization must also integrate with tenant isolation so that authenticated clients cannot access documents belonging to another customer.

### 2.5 External processes are permitted

The Go server may invoke external processes for operations where a mature external implementation is preferable to reimplementing the functionality in Go.

Typical candidates include:

- XML Schema validation;
- XML canonicalization or diagnostics;
- specialized XML transformation;
- interoperability testing.

External processes must be invoked through controlled interfaces with:

- explicit executable paths;
- bounded execution time;
- captured stdout and stderr;
- controlled environment variables;
- resource limits where appropriate;
- safe argument handling;
- no shell interpolation of untrusted document content.

---

# 3. Phase 1 — Complete UBL 2.1 Validation Server

## Objective

Provide a web server capable of validating **every document type defined by UBL 2.1**.

## Capabilities

The server must:

- accept a UBL document through an HTTP API;
- allow an authenticated user to upload an XML file containing a UBL 2.1 document through an API call;
- accept XML as the primary normative UBL representation;
- detect the UBL document type;
- select the corresponding UBL 2.1 schema;
- validate the complete document against the authoritative UBL 2.1 XSD set;
- return a structured validation result;
- report validation failures with useful diagnostic information;
- expose health and service metadata endpoints;
- support OAuth 2.0 protected validation and upload endpoints.

## Validation result

A validation response should distinguish at least:

- valid document;
- malformed XML;
- unsupported or unrecognized document type;
- UBL schema validation failure;
- validator execution failure;
- internal server failure.

Where available, schema validation errors should include:

- line;
- column;
- message;
- schema/component context.

## Completion criterion

Phase 1 is complete when the server can validate instances of every UBL 2.1 main document schema supported by the authoritative UBL 2.1 distribution.

---

# 4. Phase 2 — Comprehensive OpenAPI Access

## Objective

For every UBL 2.1 document type, provide an OpenAPI-described HTTP API that exposes the complete data contained in the document.

The API must not expose only a selected business subset. It must be capable of representing the **full UBL document structure**.

## Capabilities

The server must provide:

- OpenAPI documentation for the public HTTP API;
- document-type-aware endpoints;
- creation/import endpoints;
- retrieval endpoints;
- XML response support;
- JSON response support;
- deterministic conversion between XML and JSON;
- access to all UBL fields represented by the document;
- preservation of XML attributes and namespaces in the reversible JSON model;
- explicit media types and content negotiation.

The API should use standard HTTP content negotiation where practical, including:

- `application/xml`;
- `application/json`.

## Canonical document model

A canonical representation must sit between transport formats and persistence.

Conceptually:

```text
UBL XML
   │
   ▼
Canonical UBL Document Model
   │
   ├──► XML
   │
   ├──► JSON
   │
   └──► Persistence Model
```

This canonical representation is essential to prevent XML-specific information from evaporating during JSON conversion.

## OpenAPI strategy

OpenAPI definitions may be:

- generated from the UBL schema/model;
- generated from Go models;
- built from reusable shared UBL component schemas;
- produced using a hybrid generation process.

The architecture must avoid maintaining dozens of independently duplicated copies of common UBL component definitions.

## Completion criterion

Phase 2 is complete when every supported UBL 2.1 document can be accessed comprehensively through the API and retrieved as either XML or JSON without loss of information required for reversible conversion.

---

# 5. Phase 3 — Full Multi-Tenant Database Support

## Objective

Provide complete persistent database support for every UBL 2.1 document type.

The persistence layer must support:

- insertion;
- retrieval;
- update;
- deletion;
- search/indexing where appropriate;
- transactions;
- tenant isolation.

## Multi-tenancy

Every stored document must belong to a customer/tenant identified by a **customer identifier**.

The customer identifier must participate in every persistence access path.

No document may be retrieved, updated, or deleted without applying the tenant boundary.

The architecture should assume a logical ownership model similar to:

```text
customer_id + document_type + document_id
```

The exact database key structure may evolve, but tenant isolation is a fundamental invariant rather than an optional feature.

## Persistence requirements

The database layer must support:

- all UBL 2.1 document types;
- complete document fidelity;
- transactional CRUD operations;
- XML retrieval;
- JSON retrieval;
- canonical representation reconstruction;
- customer-scoped queries;
- schema evolution;
- indexing of common identifiers and business fields;
- auditing hooks for security-sensitive mutations.

The storage design may combine:

- normalized relational columns for high-value/queryable fields;
- structured JSON storage;
- canonical XML storage;
- generated relational projections.

The final design must not sacrifice lossless reconstruction merely to obtain relational convenience.

## Completion criterion

Phase 3 is complete when every supported UBL 2.1 document type can be inserted, retrieved, updated, and deleted under enforced tenant isolation, and the stored document can be reconstructed and returned in both XML and JSON formats.

---

# 6. External Tools and Infrastructure

This section identifies the baseline external dependencies expected by the project.

## 6.1 Required at build/development time

### Go toolchain

**Required.**

Used to build and test the server.

Target version should be pinned by `go.mod`.

### OASIS UBL 2.1 XSD distribution

**Required.**

The authoritative UBL 2.1 XSD schemas are the normative source for document validation and schema/model generation.

The project should obtain them from the official OASIS UBL 2.1 distribution using the project's download scripts rather than maintaining unofficial modified copies.

### `wget`

**Required by the current XSD download workflow.**

Used by the project scripts to download the authoritative UBL 2.1 schema set.

It is a development/deployment provisioning dependency, not necessarily a server runtime dependency.

---

## 6.2 Required at runtime

### `xmllint` / libxml2

**Required by the baseline validation architecture.**

`xmllint`, provided by **libxml2**, is the baseline external XSD validation engine.

Typical responsibility:

```text
Go server
    │
    ├── detect UBL document type
    │
    ├── select UBL XSD
    │
    └── execute xmllint/libxml2
             │
             └── XSD validation
```

The Go application remains responsible for process control, timeout handling, error parsing, security boundaries, and HTTP response construction.

A future native or embedded validator may replace this dependency only if it provides equivalent UBL 2.1/XSD compatibility.

### YugabyteDB / YSQL

**Required for Phase 3.**

YugabyteDB / YSQL is the baseline relational database platform.

It is suitable for:

- transactional persistence;
- tenant-scoped relational data;
- indexing;
- JSON/JSONB where useful;
- XML or canonical serialized payload storage;
- concurrency;
- mature operational tooling.

The exact physical persistence model remains a Phase 3 design decision.

### OAuth 2.0 / OpenID Connect authorization server

**Required for secured production deployment unless the project later chooses to implement an authorization server itself.**

The UBL server is expected to act primarily as a protected resource server.

Compatible providers may include products such as:

- Keycloak;
- Auth0;
- Microsoft Entra ID;
- Okta;
- another standards-compliant OAuth 2.0 / OpenID Connect provider.

No single vendor is mandated by this roadmap.

---

## 6.3 Optional development and interoperability tools

### `xmlstarlet`

Optional.

Useful for:

- XML inspection;
- XPath diagnostics;
- namespace debugging;
- test tooling.

It is not the normative XML↔JSON conversion engine.

### `jq`

Optional.

Useful for JSON inspection and automated interoperability tests.

### OpenAPI generation tooling

Optional but expected to be evaluated.

Possible Go ecosystem tools include generators and validators for OpenAPI documents and Go server/client bindings.

The selected tooling must not become the source of truth for UBL semantics. UBL 2.1 schemas and the canonical document model remain authoritative.

---

# 7. XML ↔ JSON Conversion Policy

No generic external XML-to-JSON utility is designated as a required runtime dependency.

The reason is architectural: most generic converters use lossy conventions that may discard or alter attributes, namespaces, qualified names, ordering, mixed content, or extension structures.

The normative conversion layer should therefore be implemented or controlled by the Go server using an explicit reversible mapping.

External conversion processes may be used for validation, experimentation, compatibility testing, or specialized transformations, but they must not silently redefine the canonical UBL information model.

A mandatory test for this layer is round-trip equivalence:

```text
XML₀
  │
  ▼
JSON
  │
  ▼
XML₁
```

`XML₁` must be semantically equivalent to `XML₀` according to the project's documented equivalence rules, including preservation of all UBL data, XML attributes, and namespace information.

The reverse direction must also be supported:

```text
JSON₀
  │
  ▼
XML
  │
  ▼
JSON₁
```

The project must define automated conformance tests for these round trips.

---

# 8. Security and Tenant Isolation

Security is cross-cutting across all phases.

The server must:

- use OAuth 2.0 access tokens for protected endpoints;
- validate issuer, audience, expiry, and required scopes/claims;
- derive or authorize the effective customer identifier;
- enforce customer isolation in the service and persistence layers;
- reject attempts to address another tenant's resources;
- avoid trusting a caller-supplied `customer_id` without authorization;
- protect external process execution from command injection;
- enforce request-size limits;
- use bounded validation/conversion execution times;
- avoid leaking sensitive document contents through logs or diagnostics.

Tenant isolation must be designed into the data model from the beginning even though full persistent storage is introduced in Phase 3.

---

# 9. High-Level Target Architecture

```text
                    ┌────────────────────────────┐
                    │ OAuth2 / OIDC Provider     │
                    └──────────────┬─────────────┘
                                   │
                                   ▼
┌─────────────┐          ┌────────────────────────────┐
│ API Client  │─────────►│ Go UBL-21 Server           │
└─────────────┘          │                            │
                         │ Authentication             │
                         │ Tenant authorization       │
                         │ Document-type detection    │
                         │ Canonical UBL model        │
                         │ OpenAPI endpoints          │
                         │ XML / JSON serialization   │
                         │ Persistence orchestration  │
                         └───────┬──────────┬─────────┘
                                 │          │
                          Phase 1│          │Phase 3
                                 ▼          ▼
                         ┌────────────┐  ┌──────────────┐
                         │ xmllint /  │  │ YugabyteDB   │
                         │ libxml2    │  │ multi-tenant │
                         └──────┬─────┘  └──────────────┘
                                │
                                ▼
                         ┌──────────────┐
                         │ UBL 2.1 XSDs │
                         │ from OASIS   │
                         └──────────────┘
```

---

# 10. Definition of Project Success

The project reaches its intended end state when:

1. every UBL 2.1 document type can be validated by the server;
2. every document type is exposed comprehensively through an OpenAPI-described interface;
3. every document can be represented and retrieved in both XML and JSON;
4. XML attributes, namespaces, qualified names, and document data survive XML↔JSON round trips;
5. every document type supports database insertion, retrieval, update, and deletion;
6. all persistent operations are isolated by customer identifier;
7. protected endpoints are secured through OAuth 2.0;
8. the system can safely use external processes where they provide better validation or conversion capabilities;
9. the implementation remains based on authoritative UBL 2.1 schemas rather than partial hand-maintained document definitions.

---

# 11. Phase Summary

| Phase | Goal | Principal Deliverable |
|---|---|---|
| **Phase 1** | Validate all UBL 2.1 document types | Go validation web server with complete UBL 2.1 XSD coverage |
| **Phase 2** | Expose complete UBL document contents through OpenAPI | Comprehensive APIs plus lossless XML/JSON representation and conversion |
| **Phase 3** | Persist and manage all UBL document types | Multi-tenant YugabyteDB/YSQL CRUD layer keyed by customer identity |

This roadmap defines the destination and major architectural boundaries. Detailed implementation plans, package structures, API route definitions, data models, and milestone breakdowns belong in subsequent roadmap and architecture documents.
