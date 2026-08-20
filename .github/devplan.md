# UBL-21 Validator Development Plan

## Purpose

This plan translates the project goals in the roadmap documents into a concrete, staged implementation approach for the UBL-21 Validator project.

The project is intended to evolve in three major phases:

1. validate all UBL 2.1 document types;
2. expose the complete UBL document model through OpenAPI-based HTTP access;
3. provide full multi-tenant persistence and CRUD support.

The overall architectural intent is:

- Go is the primary orchestration and API layer;
- XML standards-heavy operations are delegated to specialist XML tooling;
- a canonical model preserves UBL semantics across XML, JSON, and persistence.

---

## 1. Architecture principles

### 1.1 Go as the service layer

The application server is written in Go and is responsible for:

- HTTP server and routing;
- authentication and authorization integration;
- document detection;
- validation orchestration;
- XML and JSON serialization;
- OpenAPI exposure;
- persistence orchestration;
- tenant isolation;
- invoking external XML tooling through controlled interfaces.

### 1.2 XML standards should not be reimplemented in Go

The roadmap documents are explicit that Go is excellent for HTTP, concurrency, and orchestration, but not a complete solution for XML standards work.

For UBL and related standards, the recommended split is:

- Go: API layer, document detection, orchestration, result normalization, concurrency, caching
- libxml2/xmllint: XSD validation
- XSLT/Schematron tooling: EN16931 and additional business-rule validation
- encoding/xml: parsing, inspection, metadata extraction, namespace handling

This keeps the project practical while still meeting the XML interoperability requirements.

### 1.3 Canonical UBL model

A canonical document model sits between XML, JSON, and persistence.

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

This is essential to avoid losing information such as:

- XML element names;
- namespaces and namespace URIs;
- prefixes where needed for deterministic round-tripping;
- attributes and attribute namespaces;
- document ordering where significant;
- repeated elements and nested structures;
- extension content.

---

## 2. Proposed phased development plan

### Phase 0 — Foundation and architecture proof

#### Goal

Establish the project skeleton and technical choices before implementing feature work.

#### Scope

- initialize the Go project layout and module structure;
- configure HTTP server, config loading, logging, and health endpoints;
- define response schemas for validation and API operations;
- add CI/CD and containerization support;
- manage the authoritative UBL 2.1 schema bundle in the repo or build pipeline;
- create a sample corpus of valid and invalid UBL instances;
- confirm the XML validation toolchain and dependency versions;
- define the security model for OAuth 2 protected endpoints.

#### Key outputs

- repo structure for service code, schema assets, and fixtures;
- Dockerfile and local run instructions;
- health endpoint and service metadata endpoint;
- validation result contract;
- list of XML engines to be used in production;
- known-good document fixtures for testing.

#### Decision gates

- the project must prove the XML toolchain works with at least one representative UBL invoice and one representative non-invoice document;
- the project must confirm type detection and namespace handling work as expected;
- the project must confirm the service can call external XML validators safely and repeatedly.

---

### Phase 1 — Complete UBL 2.1 validation service

#### Goal

Build the first production-grade server: a validator capable of accepting and validating every UBL 2.1 document type.

#### Scope

- accept a UBL document through an HTTP API;
- accept XML as the primary normative representation;
- detect the document type and namespace;
- select the matching UBL schema bundle;
- validate the document against authoritative UBL 2.1 XSD assets;
- process malformed XML, unsupported types, validation failures, and internal errors distinctly;
- return a structured validation result;
- return useful diagnostics including line, column, message, and schema context where available;
- support OAuth 2 protected validation endpoints;
- expose health and service metadata endpoints.

#### Functional requirements

The validation API must distinguish at least:

- valid document;
- malformed XML;
- unsupported or unrecognized document type;
- UBL schema validation failure;
- validator execution failure;
- internal server failure.

Where the XML validator can provide it, response payloads should surface:

- line number;
- column number;
- diagnostic message;
- associated schema component or context.

#### Implementation approach

- use Go’s XML parsing for document inspection and namespace detection;
- use libxml2/xmllint or a comparable XSD validator for authoritative validation;
- keep the validation service modular so schema execution is decoupled from HTTP code;
- isolate external process execution behind a controlled wrapper with timeouts, captured output, and safe argument handling;
- treat schema execution as a pluggable validation backend that can be replaced later.

#### Success criteria

Phase 1 is complete when the server can validate instances of every supported UBL 2.1 main document schema from the authoritative distribution.

---

### Phase 2 — Comprehensive OpenAPI access to full UBL document structure

#### Goal

Expose complete UBL document data through a public API without reducing the model to a narrow business subset.

#### Scope

- OpenAPI documentation for the public API;
- document-type-aware HTTP endpoints;
- creation/import endpoints;
- retrieval endpoints;
- XML and JSON response support;
- deterministic conversion between XML and JSON;
- access to all UBL fields represented by the document;
- explicit media type negotiation;
- preservation of XML attributes and namespaces in the reversible JSON model.

#### Key design requirement

The API must not expose only a selected business subset. It must be capable of representing the complete UBL document structure and reconstructing the original document semantics.

#### Canonical model strategy

The canonical model sits between transport and persistence and prevents information loss during conversion.

The implementation should:

- parse UBL XML into a canonical document form;
- expose it to JSON and XML serializers from the same model;
- keep enough metadata to reconstruct namespace and attribute semantics;
- support the full UBL information model rather than a simplified mapping.

#### OpenAPI strategy

The OpenAPI definition will be developed specification-first using a YAML file, rather than being generated only from Go structs or later reverse-engineered from the running service.

This means:

- the REST contract is defined in an OpenAPI YAML file first;
- the Go implementation is then built to satisfy that contract;
- shared UBL component schemas are authored and reused in the specification, rather than duplicated across many hand-maintained fragments;
- the service is validated against the specification as part of CI using the Vacuum OpenAPI validation tool;
- every time `openapi.yaml` is updated, Vacuum must be run before merge or release;
- the implemented API target is 100% compliant with the specification, with any deviations explicitly documented and justified.

This approach is preferred because it keeps the external contract explicit, reviewable, and stable before implementation begins.

OpenAPI may later be generated from:

- the specification YAML;
- UBL schema metadata;
- Go model definitions;
- shared UBL component schemas;
- a hybrid generation workflow.

The project should avoid duplicated or drift-prone definitions for common UBL components.

#### Implementation approach

- start with XML inspection and lightweight typed extraction;
- evaluate XSD-to-Go generators such as xgen against real UBL schemas before committing to generated code;
- use generated Go structures only when they help with XML/JSON conversion and API generation;
- keep shared UBL component types centralized rather than generated per document.

#### Success criteria

Phase 2 is complete when every supported UBL 2.1 document can be accessed comprehensively through the API and retrieved as either XML or JSON without loss of information required for reversible conversion.

---

### Phase 3 — Full multi-tenant database support and CRUD

#### Goal

Persist all supported UBL document types in a multi-tenant datastore and expose secure CRUD operations.

#### Scope

- database-backed storage for UBL documents;
- multi-tenant isolation and access control;
- CRUD endpoints for all supported document types;
- retrieval by tenant and identifier;
- search and filtering for stored UBL content;
- persistence of XML and JSON forms alongside canonical model metadata;
- migration tooling and database change management;
- tenant-aware authorization rules;
- operational hardening and observability.

#### Critical requirement

Authentication and authorization must integrate with tenant isolation so that no tenant can access another tenant’s documents.

#### Design points

- documents should be stored in a canonical internal form while remaining retrievable as XML or JSON;
- the persistence layer must not silently discard XML semantics;
- every storage operation should be validated against tenant context and authorization checks;
- database design should support large, schema-heavy UBL documents and repeated structures.

#### Success criteria

Phase 3 is complete when the system can store, retrieve, and manage UBL documents across tenants without cross-tenant leakage and while preserving the semantics required for reversible XML/JSON conversion.

---

## 3. Recommended execution order

1. Foundation and toolchain validation
2. Validation service MVP
3. Schema coverage and diagnostics hardening
4. Canonical model definition
5. XML/JSON round-tripping validation
6. OpenAPI API layer
7. Database persistence and tenant isolation
8. Production hardening and operational readiness

This ordering is deliberate: validation is the highest-value and highest-risk milestone, and it creates the stable contract needed for the later API and persistence work.

---

## 4. Recommended technical stack

### Core server

- Go
- net/http or a routing library appropriate to the team
- structured logging
- config management
- metrics and tracing

### XML validation and inspection

- Go standard library: encoding/xml
- libxml2/xmllint for XSD validation
- XSLT/Schematron tooling for business-rule validation, especially EN16931-related checks

### OpenAPI / API exposure

- OpenAPI generation or manual definitions
- shared schema models for common UBL components
- explicit content negotiation between application/xml and application/json

### Persistence

- a relational database with tenant-aware schema separation or explicit tenant foreign keys
- migration tooling
- secure storage and access patterns

---

## 5. Key risks and mitigation

### 5.1 Namespace and prefix loss

Mitigation:

- use canonical internal representation;
- preserve namespace metadata explicitly;
- validate round-trip behavior with real UBL fixtures.

### 5.2 Overengineering the model too early

Mitigation:

- start with document detection and validation;
- defer full generated model commitment until schema import behavior is understood;
- test candidate code generators against real UBL XSDs before standardizing on one.

### 5.3 Insight loss during XML-to-JSON conversion

Mitigation:

- do not accept naive JSON mappings;
- define a reversible representation and test round-trips.

### 5.4 Cross-tenant data leakage

Mitigation:

- enforce tenant checks in all access layers;
- test authorization boundaries and storage transactions;
- review data retrieval paths, not just authentication.

---

## 6. Immediate next steps

1. Set up the Go project skeleton and basic HTTP health endpoints.
2. Import the authoritative UBL schema bundle and a representative document corpus.
3. Validate the toolchain with xmllint or equivalent against an invoice sample.
4. Implement document-type detection using Go XML parsing.
5. Define the first validation result model and HTTP contract.
6. Begin testing XSD-to-Go generation candidates only after validation is stable.
7. Once validation is proven, define the canonical internal model and begin OpenAPI/API work.
8. Only then start the database and multi-tenant persistence phase.

---

## 7. Summary

This project should be implemented in a disciplined progression:

- first, validate every UBL 2.1 document,
- then expose the full document model in a reversible XML/JSON API,
- then persist and manage it safely in a multi-tenant system.

The roadmap documents make the key architectural direction clear: Go is the orchestration and API layer, while domain-specific XML standards engines handle the validation and transformation work that Go alone does not do well.
