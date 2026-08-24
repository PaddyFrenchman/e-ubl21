# SERVER DATAFLOW — TODO-002

**Project:** e-ubl21  
**Date:** 2026-08-24  
**Branch:** `todo-002`

This document records the TODO-002 change relative to `SERVER-DATAFLOW-2026-08-24.md`.

## 1. Updated upload path

```text
Client
  │ POST /api/v1/ubl/upload (raw XML, max 10 MiB)
  ▼
API
  │ root local-name + namespace recognition only
  │
  ├──► JetStream Object Store
  │      <namespace>_UPLOAD_CONTENT
  │      object name = upload id
  │      value = immutable raw XML
  │
  ├──► JetStream KV
  │      <namespace>_UPLOADS
  │      key = upload id
  │      value = small mutable UploadRecord metadata
  │
  └──► JetStream WorkQueue stream
         <namespace>_DOCUMENTS
         subject = <namespace>.documents.<Type>
         payload = DocumentWorkItem JSON
                   {id, documentType, receivedAt}
```

The API stores the raw XML object before publishing the work descriptor. The upload id remains the JetStream `Nats-Msg-Id` de-duplication key.

## 2. Updated worker path

```text
WorkQueue descriptor
  │
  ▼
worker
  │ decode DocumentWorkItem
  │ verify descriptor == headers == subject routing
  │
  ├──► Object Store: GetUploadContent(id)
  │         │
  │         ▼
  │      raw XML
  │
  ├──► xmllint / UBL 2.1 XSD validation
  │
  ├──► metadata KV status update
  │
  └──► validation result stream
```

Malformed or identity-inconsistent work descriptors are terminal because retrying cannot repair them. Failure to retrieve the already-stored content is treated as an operational failure and is retried with the existing delayed NAK policy.

## 3. Status polling

Default:

```http
GET /api/v1/ubl/{id}
```

returns only small status/metadata fields:

```json
{
  "id": "...",
  "document": "Invoice",
  "status": "pending",
  "receivedAt": "2026-08-24T18:06:34Z"
}
```

The Object Store is not read on this path.

Explicit content retrieval:

```http
GET /api/v1/ubl/{id}?includeContent=true
```

adds the original `content` field. Invalid boolean values return `400`.

## 4. Data-movement effect

Before TODO-002, a work message could contain the complete raw XML, up to the API's 10 MiB limit. A representative reference descriptor containing a 32-character upload id, `Invoice`, and an RFC3339 timestamp is about 102 bytes.

That reduces worst-case work-stream payload size by approximately 99.999%. The branch tests enforce that descriptor payloads remain below 256 bytes and do not contain XML.

Raw XML remains durable, but it is stored once in a blob-oriented JetStream Object Store rather than duplicated into the documents work stream.

## 5. Storage choice

JetStream Object Store replaces the temporary TODO-005 upload-content KV design. Object Store is appropriate for large immutable payloads because it chunks object content internally while preserving simple name-based access.

The metadata store remains KV because its values are small, mutable records and KV semantics fit status polling and updates.

## 6. Reliability properties

- Content is persisted before the work descriptor is published.
- The descriptor contains the same upload id as the NATS headers and message de-duplication key.
- Worker routing identity is checked across descriptor, subject and headers before dereferencing content.
- Status updates never read or rewrite the XML object.
- Redelivery reuses the same immutable object.
- Namespace isolation applies independently to metadata KV, Object Store, work stream and result stream.

## 7. Migration note

The short-lived TODO-005 implementation used a KV bucket for upload content. TODO-002 changes that logical content store to JetStream Object Store. Existing development data in the old KV bucket is not automatically copied into Object Store and should be drained or re-uploaded if preservation is required.

## 8. Verification coverage

The branch includes tests for:

- compact work descriptor encoding and no raw XML in the work message;
- per-document subject filtering;
- metadata update without content mutation;
- multi-megabyte Object Store round-trip;
- namespace isolation including raw content;
- metadata-only GET by default;
- `includeContent=true` retrieval;
- invalid `includeContent` handling;
- existing API-to-worker valid/invalid lifecycle tests using worker-side content dereferencing.

`queue/documents_benchmark_test.go` provides a Go benchmark for descriptor encoding and allocation measurement.
