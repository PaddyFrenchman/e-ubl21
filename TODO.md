# TODO

**Project:** e-ubl21 / UBL-21 e-invoice validation  
**Updated:** 2026-08-24

## Numbering convention

Every identified project TODO receives a permanent sequential identifier (`TODO-001`, `TODO-002`, ...). Identifiers are never reused. Completion does not change an identifier.

The TODO identifier is **not** its implementation priority.

## Recommended implementation order

| Execution order | TODO | Summary |
|---:|---|---|
| 1 | **TODO-001** | ✅ DONE — Make JetStream stream, subject and KV names configurable |
| 2 | **TODO-005** | ✅ DONE — Optimize upload parsing and eliminate unnecessary full-payload copies |
| 3 | **TODO-006** | ✅ DONE — Split the lightweight API document catalog from worker XSD extraction |
| 4 | **TODO-002** | ✅ DONE — Separate upload metadata/status from raw XML content and eliminate raw XML from work messages |
| 5 | **TODO-004** | ✅ DONE — Optimize and bound `xmllint` validation execution |
| 6 | **TODO-003** | ✅ DONE — Bound JetStream worker prefetch and make validation concurrency explicit |
| 7 | **TODO-008** | Add HTTP resource/time-limit tuning and graceful API shutdown |
| 8 | **TODO-007** | Cache reusable JetStream/API runtime objects |

### Execution phases

```text
Phase A — foundation
  TODO-001 ✅ DONE
  TODO-005 ✅ DONE
  TODO-006 ✅ DONE

Phase B — reduce data movement
  TODO-002 ✅ DONE

Phase C — validation throughput
  TODO-004 ✅ DONE
  TODO-003 ✅ DONE

Phase D — production hardening and polish
  TODO-008
  TODO-007
```

---

## TODO-001 — Make JetStream stream, subject and KV names configurable ✅ DONE

**Status:** Completed and re-verified on 2026-08-24.

### Resolution

Queue topology is derived from `queue.Namespace`, configured by `--queue-namespace` / `EUBL21_QUEUE_NAMESPACE`. The empty namespace preserves legacy names. Non-empty namespaces are restricted to dot-separated lowercase alphanumeric segments, preventing different namespace strings from collapsing to the same JetStream resource names.

### Requirements

- [x] Cobra/Viper queue namespace setting.
- [x] `EUBL21_QUEUE_NAMESPACE` support.
- [x] Namespace-derived document/result subjects and stream/KV names.
- [x] `EnsureInfra()` uses derived names.
- [x] Document subject generation/parsing uses the configured namespace.
- [x] Worker filters are validated against the namespace.
- [x] Legacy defaults are preserved.
- [x] Docker Compose has an explicit development namespace.
- [x] Architecture docs updated.
- [x] Integration tests prove namespace isolation.
- [x] Collision-prone namespace forms are rejected.

---

## TODO-005 — Optimize upload parsing and eliminate unnecessary full-payload copies ✅ DONE

**Status:** Completed and re-verified on 2026-08-24.

### Resolution

The ingress path keeps the request as `[]byte`, uses `bytes.TrimSpace`, and stops an `xml.Decoder` at the first `StartElement`. Root recognition validates both local name and UBL namespace. Oversized requests are rejected deterministically with `http.MaxBytesReader` and `413 Payload Too Large`.

Full XML well-formedness is deliberately deferred to the asynchronous worker. Following TODO-002, the raw `[]byte` body is stored once in JetStream Object Store and the queue publication contains only a compact descriptor, so upload ingress no longer creates a second durable raw-XML copy in the work stream.

### Requirements

- [x] No full-body string conversion for ingress checks.
- [x] Root-only streaming XML recognition.
- [x] Root namespace validation.
- [x] Exact upload-size enforcement.
- [x] No unnecessary full-body copies before durable storage.
- [x] API/comments/docs accurately describe root-only synchronous validation.

---

## TODO-006 — Split the lightweight API document catalog from worker XSD extraction ✅ DONE

**Status:** Completed on 2026-08-24.

`loadSchemaCatalog()` currently extracts the complete embedded UBL XSD tree to a temporary directory. The API process only needs to recognize supported document root types; only the worker needs filesystem paths for `xmllint`.

### Goal

Reduce API startup I/O, temporary-disk use and initialization work.

### Resolution

The API now builds a `documentCatalog` directly from the embedded
`maindoc/*.xsd` metadata. It retains only the root element, target namespace,
and authoritative schema filename, and performs no filesystem extraction.

The worker builds a `validationCatalog` from that same `documentCatalog` and
adds the extracted filesystem tree required by `xmllint` import resolution.
Tests prove that both views share the same authoritative mapping and that API
catalog construction leaves the temporary directory empty. Separate startup
benchmarks cover both catalog paths.

### Requirements

- [x] Build an in-memory supported-document catalog directly from embedded schema metadata for the API.
- [x] Extract the full XSD filesystem tree only in worker/validator processes that actually invoke `xmllint`.
- [x] Keep one authoritative root-element/namespace mapping shared by both paths.
- [x] Add startup benchmarks/tests to ensure both catalogs agree on supported document types.

### Acceptance criteria

Starting the API service does not extract the full UBL XSD tree to disk.

---

## TODO-002 — Separate upload metadata/status from raw XML content ✅ DONE

**Status:** Completed on branch `todo-002` on 2026-08-24.

### Former state

Originally `UploadRecord` contained the complete XML document together with mutable status metadata. TODO-005 first separated content into a dedicated KV bucket, but normal status polling still fetched the XML and the JetStream work message still duplicated the entire raw document.

### Goal

Make status operations and normal polling small and cheap regardless of document size, and avoid unnecessary duplication of raw XML through the asynchronous pipeline.

### Resolution

The final storage/dataflow is:

```text
POST XML
   │
   ├──► Object Store: immutable raw XML, keyed by upload id
   │
   ├──► KV: small mutable UploadRecord metadata/status
   │
   └──► WorkQueue stream: small DocumentWorkItem JSON descriptor
                                 │
                                 ▼
                              worker
                                 │
                                 ├── fetch XML from Object Store by id
                                 ├── validate
                                 └── update metadata / publish result
```

`GET /api/v1/ubl/{id}` is now metadata-only by default. The original XML is loaded only when the caller explicitly sends `?includeContent=true`.

Raw XML is stored in a namespace-scoped JetStream **Object Store**, rather than a single-value KV record. Object Store is purpose-built for large objects and chunks them internally.

The documents work stream now carries `DocumentWorkItem` JSON containing only `id`, `documentType`, and `receivedAt`. The worker validates consistency between the descriptor, message headers, and subject before dereferencing the XML object.

For a representative Invoice work item (32-character id and RFC3339 timestamp), the JSON descriptor is about **102 bytes**. The previous design could put up to **10 MiB** of XML in the same work message. That is roughly a **99.999% reduction** in worst-case work-stream payload size. Tests enforce a descriptor ceiling of 256 bytes and assert that the queued descriptor does not contain XML. A Go benchmark (`queue/documents_benchmark_test.go`) is included for runtime measurement in CI/development environments.

### Requirements

- [x] Keep mutable upload metadata/status separate from immutable raw XML content.
- [x] Ensure status updates do not read or rewrite raw XML.
- [x] Make `GET /api/v1/ubl/{id}` return status/metadata by default.
- [x] Expose raw XML explicitly with `?includeContent=true`.
- [x] Replace raw-payload work messages with a compact content-reference/work descriptor.
- [x] Store immutable raw XML in JetStream Object Store instead of a large single KV value.
- [x] Preserve namespace isolation for the content store.
- [x] Add tests for metadata-only polling, explicit content retrieval, large Object Store payloads, descriptor size/no-XML invariant, and worker dereferencing through the existing integration tests.
- [x] Update the served OpenAPI contract and static `server/openapi.yaml`.

### Acceptance criteria

Changing upload status does not read or rewrite raw XML. Normal status polling does not load or transfer XML. Raw content is returned only on explicit request. The work stream carries only small descriptors rather than raw document bodies. The worker retrieves the immutable XML object by upload id before validation.

### Migration note

TODO-005 briefly used a KV bucket named `UBL_UPLOAD_CONTENT` (or its namespaced equivalent) for raw content. TODO-002 now uses an Object Store with the same logical bucket name. Existing content in the short-lived KV implementation is not automatically migrated to the Object Store; deployments carrying such transient development data should drain/re-upload it before switching.

---

## TODO-004 — Optimize and bound `xmllint` validation execution ✅ DONE

**Status:** Completed on 2026-08-24.

The former validator wrote every XML payload to a temporary file and launched
a new, unbounded `xmllint` process for it.

### Goal

Reduce validation overhead while preserving libxml2 validation behavior.

### Resolution

Validation now streams XML directly to `xmllint` stdin (`-`) and runs with
`--nonet`. `exec.CommandContext` enforces the configurable
`--worker-validation-timeout` / `EUBL21_WORKER_VALIDATION_TIMEOUT` deadline
(default `30s`). Timeouts are retryable worker outcomes, not invalid-document
results. A bounded writer caps combined diagnostics at 8 KiB while the process
runs and normalizes empty/whitespace output.

`BenchmarkValidateAgainstSchemaProcess` records process-per-document cost. A
three-iteration local sample on the current development machine measured
approximately `90.8 ms/op`; the reusable/native backend remains an optional
future optimization rather than a prerequisite for this TODO.

### Requirements

- [x] Feed XML to `xmllint` through stdin (`-`) instead of a per-document temporary XML file.
- [x] Use `exec.CommandContext` with configurable validation timeout.
- [x] Use `--nonet` resource isolation.
- [x] Bound/normalize diagnostics.
- [x] Benchmark process-per-document execution after the stdin change.
- [x] Evaluate a reusable/native libxml2 backend only if process startup remains material.

### Acceptance criteria

The normal validation path performs no per-document temporary-file write, cannot run indefinitely, and has benchmark data documenting validation throughput and latency.

---

## TODO-003 — Bound JetStream worker prefetch and make validation concurrency explicit ✅ DONE

**Status:** Completed on 2026-08-24.

The former worker used `consumer.Consume(worker.handle)` without explicit
pull-buffer limits or validation concurrency.

### Goal

Apply explicit backpressure and make worker throughput/memory usage predictable.

### Resolution

The worker now uses `consumer.Messages()` with a bounded worker pool.
`--worker-concurrency` controls active validations and `--worker-prefetch`
caps the local waiting channel. JetStream `MaxAckPending` is set to
`concurrency + prefetch`, providing a broker-side upper bound on outstanding
work as well.

The default validation timeout is `30s` and default `AckWait` is `60s`; startup
rejects `AckWait <= ValidationTimeout`. While a message is active, the worker
also sends `InProgress()` at half the AckWait interval, resetting the NATS
redelivery timer through XML fetch, validation, and result persistence.

Prometheus now exposes in-flight validations, buffered messages, configured
buffer capacity, and observed redeliveries. The end-to-end
`BenchmarkWorkerValidationThroughput` covers Object Store, xmllint, KV and
result publication; a two-document local sample at concurrency/prefetch `2/2`
measured approximately `18.1 docs/s`, `1.47 MB/op`, and `5,140 allocs/op`.

### Requirements

- [x] Add configurable worker concurrency.
- [x] Add configurable pull prefetch/buffer limits.
- [x] Use `consumer.Messages()` with a bounded worker pool.
- [x] Align `MaxAckPending`, `AckWait`, pull buffering and concurrency.
- [x] Add in-flight/buffered/redelivery metrics.
- [x] Benchmark throughput and memory.

### Acceptance criteria

Worker memory usage remains bounded under backlog and configured concurrency directly controls simultaneous validations.

---

## TODO-008 — Add HTTP resource/time-limit tuning and graceful API shutdown

### Goal

Bound connection resource consumption and make rolling/container shutdown predictable.

### Requirements

- Configure `ReadHeaderTimeout`, read/write/idle timeouts and `MaxHeaderBytes`.
- Preserve legitimate large uploads within documented limits.
- Add SIGINT/SIGTERM handling and `http.Server.Shutdown`.
- Make deployment-specific timeout values configurable where useful.

### Acceptance criteria

Slow/stalled clients cannot hold API resources indefinitely and API shutdown drains in-flight requests within a bounded grace period.

---

## TODO-007 — Cache reusable JetStream/API runtime objects

### Goal

Remove avoidable control-plane lookups and repeated construction from hot paths.

### Requirements

- Cache JetStream KV/Object Store handles after infrastructure initialization.
- Prebuild immutable JWT parser/options where practical.
- Precompute/cache the served OpenAPI representation.
- Benchmark before/after.

### Acceptance criteria

Normal upload/status/worker paths do not repeatedly discover infrastructure handles that can safely be reused.

---

## TODO-009 — Make upload acceptance recoverable across partial persistence failures

The upload path currently performs Object Store write → KV write → work-message
publish without a transaction. A KV or publish failure can leave orphaned XML
or a permanently pending record.

### Goal

Make every durably accepted upload eventually queued, and make incomplete
acceptance states observable and recoverable.

### Requirements

- Define explicit acceptance/enqueue lifecycle states.
- Add idempotent reconciliation for partially completed uploads.
- Remove or reclaim orphan content when acceptance cannot complete.
- Add failure-injection tests for every persistence boundary.

---

## TODO-010 — Add raw upload-content retention and storage limits

JetStream Object Store content currently has no application retention policy
and can grow indefinitely.

### Requirements

- Define configurable raw-content retention.
- Enforce or reconcile expiry across Object Store and metadata.
- Expose storage/expiry metrics and operational documentation.

---

## TODO-011 — Add terminal processing failure and dead-letter handling

The worker stops receiving a message after `MaxDeliver`, but repeated
operational failures do not currently update the record to a terminal state or
publish to an explicit dead-letter flow.

### Requirements

- Define `processing_failed`/dead-letter state and last-error/attempt metadata.
- Capture messages that exhaust delivery attempts.
- Add DLQ depth and terminal-failure metrics.
- Support safe operator retry/replay.

---

## TODO-012 — Stream Object Store content directly into validation

The worker currently materializes the complete Object Store object as a
`[]byte` before passing it to xmllint. The 10 MiB ingress cap bounds this, but
Object Store and xmllint both support streaming.

### Goal

Pipe immutable upload content directly from Object Store into xmllint stdin,
preserving timeout, diagnostic, and retry behavior while eliminating the full
worker-side XML buffer.

---

## Related architecture documents

See `SERVER-DATAFLOW-2026-08-24.md` for the baseline server architecture and `SERVER-DATAFLOW-TODO-002-2026-08-24.md` for the TODO-002 storage and work-queue update.
