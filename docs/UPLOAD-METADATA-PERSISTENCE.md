# Upload metadata persistence architecture

Issue [#89](https://github.com/PaddyFrenchman/e-ubl21/issues/89) selects
**YugabyteDB/YSQL** as the future durable system of record for upload metadata
and lifecycle status. This is an architecture decision, not a JetStream-KV
migration. The current runtime continues to use the namespace-scoped
`*_UPLOADS` JetStream KV bucket until a separately reviewed implementation
delivers the schema, cutover, and data migration described here.

## Target separation

| Data class | System of record | Why |
|---|---|---|
| Upload metadata, lifecycle status, retry/dead-letter state, retention markers and tenant/reporting fields | YugabyteDB/YSQL | Transactional, queryable, durable relational state. |
| Immutable raw XML | Configured S3-compatible document store | Blob storage with independent lifecycle and capacity management. |
| Work descriptors, stage requests, delivery/retry mechanics and result events | NATS JetStream | Durable asynchronous transport and consumer coordination. |
| Validation execution/evidence and deployment-generation KV state | JetStream KV, unless separately redesigned | These are outside #89's upload-metadata scope. |

The existing YugabyteDB object-write claim table is not the upload-metadata
model; it remains a narrow fencing mechanism for guarded object writes.

## Required future relational model

The implementation must add a versioned YSQL migration for an upload table
whose immutable identity is the upload ID and whose rows include, at minimum:

- tenant/owner identity and request-idempotency key where the public contract
  supports them;
- document type, received timestamp, content object key and SHA-256;
- current validation status, terminal timestamp and safe public error
  projection;
- processing-attempt count, last failure, failed timestamp and dead-letter
  state; and
- retention/expiry marker and deletion-completion timestamp.

It must enforce upload-ID uniqueness and use a monotonic row version (or an
equivalent YSQL conditional update) for lifecycle transitions. API acceptance,
worker transitions, retry/dead-letter handling and retention updates must
remain idempotent under duplicate JetStream delivery. A terminal state must not
be overwritten by an earlier non-terminal delivery.

The database schema is deliberately not introduced by this decision. Do not
create an application table merely to mirror KV without the transition,
privacy, indexing and migration contract below.

## Cutover requirements

The implementation issue must deliver a controlled migration, not a
best-effort dual-write period:

1. Add the YSQL schema and read/write code behind an explicit cutover setting.
2. Quiesce new uploads and drain or otherwise account for in-flight work.
3. Export each upload KV record with its revision, import it transactionally,
   and verify count, identifiers, statuses, timestamps, object keys and
   checksums.
4. Reconcile imported rows against the document store; a missing object or a
   checksum mismatch is an integrity failure, not a successful migration.
5. Enable YSQL reads and writes only after verification, while retaining the
   JetStream KV bucket read-only for the agreed rollback window.
6. Prove an idempotent rerun, an interrupted-import recovery, rollback before
   final cutover, and a post-cutover backup/restore.

The import command is deliberately explicit and must be run only while API and
worker writers are quiesced:

```bash
e-ubl21 upload-metadata migrate-legacy --confirm
```

It reads the namespace-scoped legacy `*_UPLOADS` KV bucket, verifies every
corresponding configured document-store object, records its SHA-256 in YSQL,
and refuses a missing object, checksum divergence, or a non-equivalent existing
YSQL row. It emits a count of scanned/imported/skipped/verified records. A
successful rerun reports equivalent rows as skipped; do not resume legacy KV
writes after the first successful cutover.

Do not run long-lived unsynchronised KV and SQL writers. If a short dual-read
or dual-write compatibility phase is needed, its authority, conflict rule,
metrics, rollback boundary and finite end date must be specified and tested.

## Deployment and operations

Every API, worker and maintenance process participating in upload lifecycle
changes must use the same `EUBL21_DATABASE_URL` YSQL endpoint and apply the
checked-in migrations before serving traffic. Production follows the RF-3,
failure-domain-separated YugabyteDB topology in
[YugabyteDB HA architecture](YUGABYTEDB-HA-ARCHITECTURE.md); a local
single-node database is development evidence only.

Database credentials belong in a Secret or workload identity, never in Git,
ConfigMaps, command lines, or regression evidence. TLS certificate validation
is required for production YSQL endpoints. Connection sizing, timeouts and
local qualification are defined in [YugabyteDB connectivity](YUGABYTEDB-CONNECTIVITY.md).

YSQL schema migration must be performed once as a deployment job/controlled
maintenance action, not raced by every application replica. Readiness must
fail safely when the expected migration version is absent.

## Retention, backup and restore

Metadata retention outlives raw XML retention: the row remains as the audit
and reconciliation inventory after an object deletion completes. A retention
worker must record the expiry decision in YSQL before deleting the object and
record completion only after a successful delete. Deletion retries must be
safe; an absent object may be treated as deleted only after identity/checksum
rules have been applied.

Back up the YSQL metadata database, the S3-compatible object bucket and the
JetStream work plane on coordinated, tested recovery schedules. Restore them
to a compatible recovery point, reconcile metadata rows to object key and
checksum, then resume consumers. The YBC control-plane contract is documented
in [YugabyteDB backup control plane](YUGABYTEDB-YBC-CONTROL-PLANE.md); its
retention policy must cover the upload-metadata recovery-point objective before
this feature is production-enabled.

## Licensing

The selected YugabyteDB distribution is Apache-2.0 and is suitable for the
project's intended commercial deployment model. This decision covers
YugabyteDB itself, not YugabyteDB Anywhere or other separately licensed
services. The precise on-premises obligations and the component inventory are
maintained in [the licensing guide](../deploy/on-prem/licensing.md).

## Implementation acceptance

Before the migration can replace JetStream KV, it must demonstrate:

- a real YugabyteDB integration suite for all upload lifecycle and concurrent
  transition cases;
- query/index evidence for the required status, retry, retention and tenant
  reporting paths;
- migration checksums/counts, idempotent rerun and rollback/recovery evidence;
- backup/restore and object reconciliation evidence; and
- an updated deployment guide, retention runbook and licence/SBOM review.
