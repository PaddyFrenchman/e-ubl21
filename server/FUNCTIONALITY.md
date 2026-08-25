# Server Functionality

This document summarizes the server functionality currently implemented on the `todo-002` branch.

## Architecture at a glance

```text
Client
  │ POST /api/v1/ubl/upload
  ▼
API
  ├── Object Store: immutable raw XML
  ├── KV: mutable upload status/metadata
  └── WorkQueue: compact DocumentWorkItem
                         │
                         ▼
                      worker
                         │
                         ├── fetch XML object by upload id
                         ├── xmllint + UBL 2.1 XSD
                         ├── update metadata KV
                         └── publish validation result
```

The API and worker are separate processes built from the same Go binary (`e-ubl21 serve` and `e-ubl21 worker`) and communicate through NATS JetStream.

## HTTP endpoints

| Method | Path | Auth | Behavior |
|---|---|---|---|
| GET | `/` | Required | Service metadata |
| GET | `/healthz` | Required | API liveness |
| GET | `/metrics` | Required | Prometheus metrics |
| GET | configured OpenAPI path | Public | Served OpenAPI 3.0.3 document |
| GET | configured Swagger path | Public | Swagger UI |
| POST | `/api/v1/ubl/upload` | Required | Accept and queue a UBL document |
| GET | `/api/v1/ubl/{id}` | Required | Status/metadata only by default |
| GET | `/api/v1/ubl/{id}?includeContent=true` | Required | Status/metadata plus original XML |

## Authentication

Protected routes require an HS256 bearer JWT. The configured HMAC secret verifies the signature. Expiration is required; configured issuer and audience are checked. Missing signing configuration fails closed. JWKS/asymmetric-key discovery and scopes/roles remain outside the current milestone.

## Queue and storage topology

All physical names are derived from `queue.Namespace`. With the legacy/default namespace the resources are:

- `UBL_DOCUMENTS`: WorkQueuePolicy stream receiving `ubl.documents.<Type>` subjects.
- `UBL_VALIDATION_RESULTS_BY_UPLOAD`: result stream receiving permanently unique
  `ubl.validation.results.<upload-id>` subjects; consumers filter on
  `ubl.validation.results.*`.
- `UBL_UPLOADS`: KV bucket containing small mutable `UploadRecord` metadata.
- `UBL_UPLOAD_CONTENT`: size-capped JetStream Object Store containing immutable
  raw XML objects keyed by upload id. Terminal content is reclaimed after the
  configured post-validation retention period.

With a configured namespace such as `eubl21.prod`, the subject/resource names are correspondingly namespaced and isolated.

### Work message format

The documents stream no longer carries raw XML. Its payload is a small JSON descriptor:

```json
{
  "id": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
  "documentType": "Invoice",
  "receivedAt": "2026-08-24T18:06:34Z"
}
```

The same upload id/type/timestamp are present in NATS headers. The upload id remains the `Nats-Msg-Id` de-duplication key.

A representative descriptor is about 102 bytes, rather than as much as 10 MiB when the work stream carried the XML itself.

## Upload processing

`POST /api/v1/ubl/upload`:

1. enforces the 10 MiB body limit exactly with `http.MaxBytesReader`;
2. accepts XML/octet-stream content types;
3. uses `xml.Decoder` only until the first root `StartElement`;
4. checks both root local name and UBL namespace against the schema catalog;
5. stores the raw XML in Object Store;
6. creates a small `pending` metadata record in KV;
7. publishes the compact work descriptor;
8. returns `202 Accepted`.

Full XML well-formedness is deliberately not established synchronously. The worker's `xmllint` run performs complete parsing and XSD validation.

## Worker processing

The worker consumes a durable pull consumer on the documents stream. For each message it:

1. decodes `DocumentWorkItem`;
2. verifies descriptor identity against the subject and NATS headers;
3. resolves the document type to its UBL 2.1 XSD;
4. retrieves the raw XML object by upload id;
5. runs `xmllint` XSD validation;
6. updates the metadata KV record to `valid` or `invalid`;
7. inserts a `ValidationResult` on its unique per-upload result subject;
8. acknowledges the work message only after state/result writes succeed.

Malformed/inconsistent work descriptors are terminal. Operational failures such as unavailable content, validator unavailability, or result/state write failures use the existing delayed NAK/retry path.

The result stream uses `DiscardNew`, `DiscardNewPerSubject`, and a
one-message-per-subject limit. Results have no `MaxAge`: retaining the first
small result for each upload is the durable uniqueness index that makes a
retry safe even after the ordinary `Nats-Msg-Id` window expires. Downstream
consumers must use `queue.Namespace.ResultsFilterSubject()`.
When upgrading an existing deployment, downstream consumers must move from the
old `UBL_VALIDATION_RESULTS` stream/exact base subject to
`UBL_VALIDATION_RESULTS_BY_UPLOAD` and that wildcard filter. `EnsureInfra`
creates the versioned stream while leaving the legacy stream, its messages, and
consumer-owned configuration untouched.

## Status retrieval

Normal polling:

```http
GET /api/v1/ubl/{id}
```

returns only metadata, for example:

```json
{
  "id": "3f9c2b1a7e4d4f0a9b1c2d3e4f5a6b7c",
  "document": "Invoice",
  "status": "valid",
  "receivedAt": "2026-08-24T18:06:34Z",
  "validatedAt": "2026-08-24T18:06:35Z"
}
```

The Object Store is not read during ordinary status polling.

To retrieve the XML explicitly:

```http
GET /api/v1/ubl/{id}?includeContent=true
```

The response then additionally includes `content`. Invalid boolean values for `includeContent` return `400 Bad Request`.

Validation metadata outlives raw XML. Terminal records carry
`contentExpiresAt`; after the API maintenance loop deletes the object they also
carry `contentExpiredAt`, and metadata-only responses report
`contentAvailable: false`. Explicit raw-content retrieval then returns
`410 Gone`. Pending work is excluded from expiry so backlog age cannot remove
an input before validation. See `UPLOAD-CONTENT-RETENTION.md` for configuration,
capacity behavior, metrics, and upgrade operations.

## UBL XSD validation

The authoritative UBL 2.1 XSD tree is embedded in the binary and extracted for `xmllint` use. The worker validates against the schema selected by document type. Schematron, EN 16931 and PEPPOL business-rule validation are not part of the server's current validation milestone.

## Rate limiting and observability

The API applies a per-client-IP token-bucket limiter and structured request logging. Prometheus metrics cover HTTP activity, upload outcomes, worker validation outcomes/duration, terminal processing failures, and dead-letter depth. The worker moves operational failures that exhaust `worker-max-deliver` to a namespace-scoped dead-letter stream and records `processing_failed` with its last error, attempt count, and failure time. Operators can safely replay retained content with `e-ubl21 dead-letter retry UPLOAD_ID`. The worker also exposes internal `/metrics` and `/healthz` endpoints on its configured metrics address.

## Current known follow-up work

The numbered project register in `../TODO.md` remains authoritative. Relevant upcoming items include:

- TODO-006: avoid extracting the full XSD tree in API processes;
- TODO-004: feed `xmllint` via stdin and add execution timeout/resource bounds;
- TODO-003: explicit worker concurrency/backpressure;
- TODO-008: HTTP server resource limits and graceful API shutdown;
- TODO-007: cache reusable JetStream/API runtime objects.

See `../SERVER-DATAFLOW-TODO-002-2026-08-24.md` for the detailed TODO-002 dataflow change.
