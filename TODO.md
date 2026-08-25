# TODO

**Project:** e-ubl21 / UBL-21 e-invoice validation  
**Updated:** 2026-08-25

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
| 7 | **TODO-008** | ✅ DONE — Add HTTP resource/time-limit tuning and graceful API shutdown |
| 8 | **TODO-007** | ✅ DONE — Cache reusable JetStream/API runtime objects |

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
  TODO-008 ✅ DONE
  TODO-007 ✅ DONE
```

---

## TODO-001 — Make JetStream stream, subject and KV names configurable ✅ DONE

**Status:** Completed and re-verified on 2026-08-24.

### Resolution

Queue topology is derived from `queue.Namespace`, configured by `--queue-namespace` / `EUBL21_QUEUE_NAMESPACE`. The empty namespace preserves legacy resources; later versioned resources are added without mutating them. Non-empty namespaces are restricted to dot-separated lowercase alphanumeric segments, preventing different namespace strings from collapsing to the same JetStream resource names.

### Requirements

- [x] Cobra/Viper queue namespace setting.
- [x] `EUBL21_QUEUE_NAMESPACE` support.
- [x] Namespace-derived document/result subjects and stream/KV names.
- [x] `EnsureInfra()` uses derived names.
- [x] Document subject generation/parsing uses the configured namespace.
- [x] Worker filters are validated against the namespace.
- [x] Legacy resources are preserved rather than renamed or destructively migrated.
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
`concurrency + prefetch` by default, providing a broker-side upper bound on
outstanding work as well. `--worker-max-ack-pending` can raise the shared
durable consumer's fleet-wide limit for multiple replicas, but cannot be set
below one replica's local capacity.

The default validation timeout is `30s` and default `AckWait` is `60s`; startup
rejects `AckWait <= ValidationTimeout`. While a message is active, the worker
also sends `InProgress()` at half the AckWait interval, resetting the NATS
redelivery timer through XML fetch, validation, and result persistence.

Prometheus now exposes in-flight validations, the explicit Go jobs-channel
depth and capacity, the per-process NATS iterator capacity, and observed
redeliveries. The NATS iterator's contents are deliberately not included in
the jobs-channel depth metric. The end-to-end
`BenchmarkWorkerValidationThroughput` covers Object Store, xmllint, KV and
result publication for small, 1 MiB, and near-limit 9 MiB documents. A local
three-document sample at concurrency/prefetch `2/2` measured 14.33, 15.76,
and 10.15 docs/s respectively. Allocations per operation were 1.27 MB,
13.97 MB, and 102.11 MB. These end-to-end B/op figures include the embedded
broker and Object Store client path, not only the validator subprocess; they
measure cumulative allocations, not peak live heap or RSS.

### Requirements

- [x] Add configurable worker concurrency.
- [x] Add configurable pull prefetch/buffer limits.
- [x] Use `consumer.Messages()` with a bounded worker pool.
- [x] Align `MaxAckPending`, `AckWait`, pull buffering and concurrency.
- [x] Add in-flight/buffered/redelivery metrics.
- [x] Benchmark throughput and memory.

### Acceptance criteria

The architecture bounds live raw-document work to configured concurrency and
bounds descriptor buffering independently of backlog depth. The throughput
benchmark does not by itself prove peak live heap/RSS under a deep backlog;
use `BenchmarkWorkerDeepBacklogMemory` for that stress verification.

---

## TODO-008 — Add HTTP resource/time-limit tuning and graceful API shutdown ✅ DONE

**Status:** Completed and verified on 2026-08-24.

### Goal

Bound connection resource consumption and make rolling/container shutdown predictable.

### Requirements

- [x] Configure `ReadHeaderTimeout`, read/write/idle timeouts and `MaxHeaderBytes`.
- [x] Preserve legitimate large uploads within documented limits.
- [x] Add SIGINT/SIGTERM handling and `http.Server.Shutdown`.
- [x] Make deployment-specific timeout values configurable where useful.

### Resolution

The API server defaults to a 5-second header timeout, 60-second whole-request
read timeout, 60-second write timeout, 2-minute keep-alive idle timeout,
64 KiB aggregate request-header limit, and 15-second graceful-shutdown drain
deadline. Each value is configurable by CLI, config file, or `EUBL21_*`
environment variable. The 60-second read deadline applies to the complete
request: it allows normal uploads up to the independently enforced 10 MiB
limit while deliberately rejecting clients that cannot finish within that
time. Deployments serving slower WAN/VPN clients can raise the deadline.

Configuration loading rejects timeouts below one second, a whole-request read
timeout shorter than the header timeout, and header limits below 4 KiB.

`serve` now derives a signal-aware context from SIGINT/SIGTERM, stops accepting
connections with `http.Server.Shutdown`, allows in-flight handlers to finish
within the configured grace period, and force-closes the server if that period
expires. Lifecycle tests cover both successful draining and the bounded
deadline path; upload tests continue to accept bodies exactly at 10 MiB.

### Acceptance criteria

Slow/stalled clients cannot hold API resources indefinitely and API shutdown drains in-flight requests within a bounded grace period.

---

## TODO-007 — Cache reusable JetStream/API runtime objects ✅ DONE

**Status:** Completed and verified on 2026-08-24.

### Goal

Remove avoidable control-plane lookups and repeated construction from hot paths.

### Requirements

- [x] Cache JetStream KV/Object Store handles after infrastructure initialization.
- [x] Prebuild immutable JWT parser/options where practical.
- [x] Precompute/cache the served OpenAPI representation.
- [x] Benchmark before/after.

### Resolution

`queue.Client` caches the upload metadata KV and raw-content Object Store
handles returned by `EnsureInfra`; synchronized lazy fallback remains available
for callers that open an already-created deployment without initialization.
Cold fallback discovery uses double-checked exclusive locking, so concurrent
first callers coalesce into one KV lookup and one Object Store lookup.
Normal record/content operations no longer repeat JetStream control-plane
discovery. The API's OAuth validator now reuses an immutable configured JWT
parser and key function. The OpenAPI map and its serialized JSON payload are
both constructed once; `/openapi.json` writes the immutable cached bytes
without walking and encoding the map per request.

All API construction now uses lifecycle-owning `server.Application`, which
closes its shared NATS connection after graceful HTTP shutdown. The former
handler-only constructor was removed so callers cannot discard ownership.

Comparative 100-iteration benchmarks on the development host measured cached
KV handle access at 15.5 ns/op and zero allocations versus 93.7 us/op,
18.8 KiB, and 159 allocations for control-plane lookup. Cached Object Store
handle access measured 16.3 ns/op and zero allocations versus 79.3 us/op,
18.7 KiB, and 160 allocations for control-plane lookup. Cached JWT parsing measured
6.30 us/op and 49 allocations versus 7.80 us/op and 60 allocations when
rebuilding configuration. OpenAPI endpoint-ready byte access measured
3.87 ns/op with zero allocations, versus 98.2 us/op, 32.8 KiB, and 787
allocations to serialize the cached map, or 129.5 us/op, 86.7 KiB, and 1,179
allocations to rebuild and serialize it.

### Acceptance criteria

Normal upload/status/worker paths do not repeatedly discover infrastructure handles that can safely be reused.

---

## TODO-009 — Make upload acceptance recoverable across partial persistence failures ✅ DONE

**Status:** Completed and verified on 2026-08-25.

The upload path currently performs Object Store write → KV write → work-message
publish without a transaction. A KV or publish failure can leave orphaned XML
or a permanently pending record.

### Goal

Make every durably accepted upload eventually queued, and make incomplete
acceptance states observable and recoverable.

### Requirements

- [x] Define explicit acceptance/enqueue lifecycle states.
- [x] Add idempotent reconciliation for partially completed uploads.
- [x] Remove or reclaim orphan content when acceptance cannot complete.
- [x] Add failure-injection tests for every persistence boundary.

### Resolution

Upload intent is now recorded before content is written. Acceptance progresses
through `accepting` → `enqueue_pending` → `pending`; content failures become
`acceptance_failed`, or `cleanup_pending` when partial Object Store cleanup
must be retried. Once intent and content are durable, queue/status failures
return HTTP 202 with an observable recovery state instead of losing the upload.

Every API application runs an immediate and periodic reconciler. It waits for
active uploads to age past a one-minute safety window, verifies content by
Object Store metadata without loading XML, retries deterministic-ID work
publication, advances recovered records to `pending`, and reclaims
`cleanup_pending` content. Repeated reconciliation is safe and tested.
Prometheus exposes reconciliation outcomes through
`e_ubl21_upload_reconciliations_total`.

All acceptance and reconciliation transitions use JetStream KV revisions and
compare-and-swap updates. A stale reconciler therefore cannot overwrite a
worker's `valid`/`invalid` result with `pending`. Publication failures retain
their latest diagnostic in the record. Worker processing also reads the
versioned record before validation: terminal records acknowledge late
duplicate descriptors without reprocessing, including descriptors replayed
after JetStream's time-bounded duplicate window. A durable `resultPending`
marker makes result publication retryable without repeating validation.
Results themselves use a unique `<results-base>.<upload-id>` subject in a
`DiscardNewPerSubject` stream limited to one message per subject. The first
small result is retained without `MaxAge`, so a crash between result
publication and clearing `resultPending` cannot create another result after
the ordinary JetStream message-ID window expires. Result consumers filter the
per-upload subject space through `Namespace.ResultsFilterSubject()`.

Failure-injection tests cover intent persistence, content persistence,
content cleanup, enqueue-state persistence, work publication, and final status
persistence. Integration tests cover recovery, cleanup, idempotent repeated
scans, automatic startup reconciliation, and terminal missing-content state.
Additional tests force stale CAS transitions, ambiguous publish acknowledgement,
and a late duplicate descriptor with a distinct message ID.

### Acceptance criteria

Every upload with durable intent and content is eventually queued or remains
in an explicit retryable state; content that cannot be accepted is reclaimed
before the record becomes terminal.

---

## TODO-010 — Add raw upload-content retention and storage limits ✅ DONE

**Status:** Completed and verified on 2026-08-25.

JetStream Object Store content previously had no application retention policy
and could grow indefinitely.

### Requirements

- [x] Define configurable raw-content retention.
- [x] Enforce or reconcile expiry across Object Store and metadata.
- [x] Expose storage/expiry metrics and operational documentation.

### Resolution

Raw XML now has a configurable post-validation retention period (30 days by
default) and the shared Object Store has a configurable hard byte limit (10
GiB by default). Both settings are available through CLI, Viper/config files,
and `EUBL21_UPLOAD_CONTENT_*` environment variables, and Docker Compose gives
the API and worker identical explicit values.

Workers persist `contentExpiresAt` with the terminal validation record. For due
`valid`/`invalid` content, the API maintenance loop CAS-persists
`contentExpiredAt` before deletion, then retries any interrupted Object Store
cleanup; queued documents are never aged out before they can be processed.
Existing terminal records are enrolled from `validatedAt`.
Metadata remains available, ordinary status responses expose content
availability/expiry, and explicit content retrieval returns `410 Gone` after
reclamation.

Prometheus reports current/maximum Object Store bytes, expiry outcomes, and
reclaimed payload bytes. Operational configuration, capacity behavior,
multi-replica requirements, API semantics, metrics, and upgrade behavior are
documented in `UPLOAD-CONTENT-RETENTION.md`.

Object Store deletion tombstones remain subject to the same hard bucket cap.
Their long-running storage impact and the need for a separately designed,
graceful tombstone-purge policy are documented rather than hidden by the raw
payload expiry guarantee.

### Acceptance criteria

Raw XML storage is bounded by deployer policy without deleting unprocessed
work, while lightweight validation metadata remains queryable after content
expiry.

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

## TODO-013 — Add safe Object Store tombstone purge maintenance

Raw-content expiry in TODO-010 purges Object Store payload chunks but leaves
JetStream deleted-object metadata. Those tombstones are small and remain
bounded by the Object Store byte cap, but can consume meaningful capacity in a
high-volume, long-running deployment.

### Goal

Provide one safe maintenance implementation that can be invoked manually from
the CLI and scheduled unchanged through cron or a Kubernetes CronJob.

### Requirements

- Add a command such as `e-ubl21 maintenance purge-content-tombstones`.
- Respect the configured queue namespace and operate only on that namespace's
  raw-upload Object Store.
- Enumerate deleted Object Store entries and purge only tombstones older than a
  configurable grace period.
- Provide a non-destructive `--dry-run` mode.
- Bound each run with `--max-items` or an equivalent batch-size control.
- Never purge active objects or metadata belonging to pending/unprocessed
  uploads.
- Make repeated runs idempotent and safe when another maintenance run overlaps.
- Emit structured scanned/eligible/purged/failed totals and return a non-zero
  exit status when cleanup is incomplete.
- Add integration tests against the repository's supported NATS version,
  including proof that live/pending content is unaffected.
- Document manual operation and scheduling the same command through cron or a
  Kubernetes CronJob; do not create a second cleanup implementation.

### Acceptance criteria

An operator can preview and then reclaim eligible Object Store delete-marker
metadata in bounded batches without touching active upload content. The same
command is safe for periodic scheduled execution and repeated runs.

---

## Related architecture documents

See `SERVER-DATAFLOW-2026-08-24.md` for the baseline server architecture and `SERVER-DATAFLOW-TODO-002-2026-08-24.md` for the TODO-002 storage and work-queue update.