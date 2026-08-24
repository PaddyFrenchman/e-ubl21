# TODO

**Project:** e-ubl21 / UBL-21 e-invoice validation  
**Updated:** 2026-08-24

## Numbering convention

Every identified project TODO receives a permanent sequential identifier:

```text
TODO-001
TODO-002
TODO-003
...
```

Identifiers are never reused. If an item is completed or cancelled, its identifier remains reserved so references in commits, documentation, issues and discussions stay stable.

The TODO identifier is **not** its implementation priority. The sections below are intentionally ordered by the recommended execution sequence while preserving permanent IDs.

## Recommended implementation order

| Execution order | TODO | Summary |
|---:|---|---|
| 1 | **TODO-001** | ✅ DONE — Make JetStream stream, subject and KV names configurable |
| 2 | **TODO-005** | ✅ DONE — Optimize upload parsing and eliminate unnecessary full-payload copies |
| 3 | **TODO-006** | Split the lightweight API document catalog from worker XSD extraction |
| 4 | **TODO-002** | Separate upload metadata/status from raw XML content |
| 5 | **TODO-004** | Optimize and bound `xmllint` validation execution |
| 6 | **TODO-003** | Bound JetStream worker prefetch and make validation concurrency explicit |
| 7 | **TODO-008** | Add HTTP resource/time-limit tuning and graceful API shutdown |
| 8 | **TODO-007** | Cache reusable JetStream/API runtime objects |

### Execution phases

```text
Phase A — foundation
  TODO-001 ✅ DONE
  TODO-005 ✅ DONE
  TODO-006

Phase B — reduce data movement
  TODO-002

Phase C — validation throughput
  TODO-004
  TODO-003

Phase D — production hardening and polish
  TODO-008
  TODO-007
```

---

## TODO-001 — Make JetStream stream, subject and KV names configurable ✅ DONE

**Status:** Completed and re-verified on 2026-08-24.

### Former state

The queue topology was partly hard-coded in `queue/subjects.go`:

- work stream: `UBL_DOCUMENTS`
- document subject prefix: `ubl.documents.`
- result stream: `UBL_VALIDATION_RESULTS`
- result subject: `ubl.validation.results`
- upload KV bucket: `UBL_UPLOADS`

Only the NATS URL and worker subject filter were configurable.

### Goal

Make the queue namespace configurable so several deployments or environments can safely share the same NATS cluster without collisions.

Prefer a single namespace/prefix setting over many unrelated settings.

Example:

```text
EUBL21_QUEUE_NAMESPACE=eubl21.prod
```

Derived subjects:

```text
eubl21.prod.documents.Invoice
eubl21.prod.documents.CreditNote
eubl21.prod.validation.results
```

Derived JetStream/KV resource names:

```text
EUBL21_PROD_DOCUMENTS
EUBL21_PROD_VALIDATION_RESULTS
EUBL21_PROD_UPLOADS
```

### Resolution

Implemented through `queue.Namespace`, with `--queue-namespace` / `EUBL21_QUEUE_NAMESPACE` configuration shared by API and worker processes.

Namespace values are validated as empty/default or dot-separated lowercase alphanumeric segments matching:

```text
^[a-z0-9]+(\.[a-z0-9]+)*$
```

This prevents distinct configured namespace strings from normalizing to the same JetStream stream/KV resource prefix. Ambiguous values such as `eubl21--prod`, `eubl21_prod`, `EUBL21.Prod`, leading/trailing dots and consecutive dots are rejected at configuration time.

The empty namespace preserves the original pre-namespace stream, subject and KV names for backwards compatibility.

### Requirements

- [x] Add a queue namespace/prefix setting to Cobra/Viper configuration.
- [x] Support `EUBL21_QUEUE_NAMESPACE`.
- [x] Remove direct dependencies on hard-coded subject prefixes from publishers and consumers.
- [x] Derive the work stream, result stream and upload KV bucket names consistently.
- [x] Ensure `EnsureInfra()` uses the derived names.
- [x] Ensure document subject generation/parsing uses the configured namespace.
- [x] Validate worker filters against the configured namespace.
- [x] Keep defaults that preserve existing local behavior.
- [x] Update `docker-compose.yml` with an explicit development namespace.
- [x] Update queue/server architecture documentation.
- [x] Add integration tests proving two namespaces can coexist on the same JetStream server without cross-consuming messages or records.
- [x] Reject namespace forms that could collide after JetStream resource-name normalization.

### Acceptance criteria

A single NATS/JetStream cluster can host isolated e-ubl21 deployments such as:

```text
eubl21.dev.documents.*
eubl21.prod.documents.*
```

A worker configured for one namespace does not consume documents, update upload records, or publish result events belonging to another namespace. Namespace isolation is covered by integration tests, and malformed/collision-prone namespaces are rejected before startup.

---

## TODO-005 — Optimize upload parsing and eliminate unnecessary full-payload copies ✅ DONE

**Status:** Completed and re-verified on 2026-08-24.

### Former state

The HTTP upload path converted the body to `string` multiple times and used `xml.Unmarshal` over the full document merely to identify its root element. The old `io.LimitReader` size cap could also silently truncate an oversized payload instead of distinctly rejecting it.

### Goal

Keep the synchronous API ingress path lightweight and proportional to the minimum work required before queuing.

### Resolution

The upload path now keeps the request body as `[]byte` throughout ingress, uses `bytes.TrimSpace` for cheap checks, and uses `xml.Decoder.Token()` only until the first `xml.StartElement`. Root recognition validates both local name and namespace against the UBL 2.1 schema catalog.

The API deliberately does not claim to establish complete XML well-formedness synchronously. A syntactically valid recognized root can be accepted even if the tail is malformed; full well-formedness and XSD validation are performed asynchronously by the worker's `xmllint` run.

Upload size enforcement now uses `http.MaxBytesReader`, with a declared-size fast rejection and deterministic `413 Payload Too Large` handling for oversized bodies, including unknown/chunked lengths.

The raw XML is no longer converted to `string` or JSON-wrapped on the upload path. It is stored separately as bytes and the same body slice is passed to document publication. Any conversion to string now happens only on retrieval when producing the existing JSON response.

### Requirements

- [x] Use `bytes.TrimSpace` or streaming inspection rather than converting the full payload to `string` for empty/XML-prefix checks.
- [x] Use `xml.Decoder` and stop after the first `StartElement` to identify the root element and namespace.
- [x] Validate the expected UBL root namespace as well as the local element name.
- [x] Enforce the upload size limit exactly, rejecting oversized bodies instead of silently truncating via `io.LimitReader`.
- [x] Avoid additional full-body copies before publishing/storing the document.
- [x] Correct API/comments/documentation so synchronous root recognition is not described as full-document well-formedness validation.

### Acceptance criteria

Root-document detection does not parse the entire XML tree and does not require full-size `[]byte`→`string` copies. Oversized uploads are rejected deterministically. Tests cover early root detection, namespace mismatch, exact-limit bodies and oversized bodies.

---

## TODO-006 — Split the lightweight API document catalog from worker XSD extraction

`loadSchemaCatalog()` currently extracts the complete embedded UBL XSD tree to a temporary directory. The API process only needs to recognize supported document root types; only the worker needs filesystem paths for `xmllint`.

### Goal

Reduce API startup I/O, temporary-disk use and initialization work.

### Requirements

- Build an in-memory supported-document catalog directly from embedded schema metadata for the API.
- Extract the full XSD filesystem tree only in worker/validator processes that actually invoke `xmllint`.
- Keep one authoritative root-element/namespace mapping shared by both paths.
- Add startup benchmarks/tests to ensure both catalogs agree on supported document types.

### Acceptance criteria

Starting the API service does not extract the full UBL XSD tree to disk.

---

## TODO-002 — Separate upload metadata/status from raw XML content

### Current state

Partial progress was completed while closing TODO-005: `UploadRecord` no longer contains the raw XML, and raw content now lives in a separate namespace-scoped upload-content KV bucket. Status updates therefore no longer read or rewrite the document body.

The remaining TODO-002 work is primarily about reducing data movement beyond that split: status polling still returns the full XML by default, and the JetStream work message still carries the raw XML payload rather than a small content reference/work descriptor.

### Goal

Make status operations and normal polling small and cheap regardless of document size, and avoid unnecessary duplication of raw XML through the asynchronous pipeline.

### Preferred architecture

- [x] Keep mutable upload metadata/status separate from immutable raw XML content.
- [x] Ensure status updates do not read or rewrite raw XML.
- [ ] Make `GET /api/v1/ubl/{id}` return status/metadata by default.
- [ ] Expose raw XML through a separate endpoint or explicit `includeContent` mechanism.
- [ ] Evaluate queueing a small work item containing the upload id/document metadata instead of duplicating the full XML in the work stream, with benchmarks before changing the design.
- [ ] Evaluate whether the current KV content store should remain KV or become JetStream Object Store/blob-oriented storage for large immutable XML.

### Acceptance criteria

Changing an upload status must not read or rewrite the raw XML payload. Polling validation status must transfer only small metadata unless the caller explicitly asks for document content. Any change from raw-payload queue messages to reference-based work messages must be justified by benchmark/reliability data.

---

## TODO-004 — Optimize and bound `xmllint` validation execution

The current validator writes every queued XML payload to a temporary file and launches a new `xmllint` process for it. This adds filesystem I/O and process startup overhead, and execution currently has no per-document timeout.

### Goal

Reduce validation overhead while preserving the proven libxml2 validation behavior.

### Requirements

- Feed the XML document to `xmllint` through stdin (`-`) instead of creating a temporary XML file for each validation.
- Use `exec.CommandContext` with a configurable validation timeout.
- Consider `--nonet`/equivalent resource isolation where compatible with the embedded UBL schemas.
- Capture and normalize validation diagnostics without unbounded output.
- Benchmark process-per-document `xmllint` after the stdin change.
- Only if process startup remains a material bottleneck, evaluate a reusable/native libxml2 validation backend with precompiled schemas.

### Acceptance criteria

The normal validation path performs no per-document temporary-file write, cannot run indefinitely, and has benchmark data documenting validation throughput and latency.

---

## TODO-003 — Bound JetStream worker prefetch and make validation concurrency explicit

The worker currently calls `consumer.Consume(worker.handle)` without pull-buffer options. The NATS Go client defaults `PullMaxMessages` to 500 messages. With uploads allowed up to 10 MiB, an unconstrained message-count buffer can create excessive memory pressure.

### Goal

Apply explicit backpressure and make worker throughput/memory usage predictable.

### Requirements

- Add configurable worker concurrency.
- Add configurable pull prefetch/buffer limits, preferably byte-based for large variable-sized XML payloads.
- Evaluate `consumer.Messages()` with a bounded worker pool/semaphore for work-queue processing.
- Ensure `MaxAckPending`, `AckWait`, pull buffering and worker concurrency are coherent.
- Add metrics for in-flight validations, buffered work and redelivery.
- Benchmark throughput and memory with small, medium and near-limit XML documents.

### Acceptance criteria

Worker memory usage must remain bounded under a backlog of maximum-size uploads, and configured concurrency must directly control the number of simultaneous validations.

---

## TODO-008 — Add HTTP resource/time-limit tuning and graceful API shutdown

The API `http.Server` currently has no explicit read-header, read, write or idle timeouts and does not perform graceful shutdown. Slow or stalled connections can therefore retain resources longer than necessary.

### Goal

Bound connection resource consumption and make rolling/container shutdown predictable.

### Requirements

- Configure sensible `ReadHeaderTimeout`, request/read limits, `WriteTimeout`, `IdleTimeout` and `MaxHeaderBytes` values.
- Ensure large but legitimate UBL uploads remain supported within documented limits.
- Add SIGINT/SIGTERM handling and `http.Server.Shutdown` for the API, matching the worker's graceful lifecycle philosophy.
- Make operational timeout values configurable where deployment-specific tuning is useful.

### Acceptance criteria

Slow/stalled clients cannot hold API resources indefinitely, and API shutdown drains in-flight requests within a bounded grace period.

---

## TODO-007 — Cache reusable JetStream/API runtime objects

Several frequently used operations recreate or look up reusable state, including opening the upload KV bucket for each record operation. Other immutable objects such as JWT parser configuration and the OpenAPI JSON document can also be prepared once.

### Goal

Remove avoidable control-plane lookups and repeated construction from request/worker hot paths.

### Requirements

- Cache the JetStream KV/Object Store handles in the queue client after infrastructure initialization.
- Prebuild immutable JWT parser/options where practical.
- Precompute/cache the served OpenAPI representation rather than reconstructing the map for each request.
- Benchmark before and after so this remains a measured micro-optimization rather than speculative complexity.

### Acceptance criteria

Normal upload, status and worker paths do not perform repeated infrastructure-handle discovery when the underlying handle can safely be reused.

---

## Related architecture document

See `SERVER-DATAFLOW-2026-08-24.md` for the current server dataflow, queue topology, architecture overview and gap analysis.
