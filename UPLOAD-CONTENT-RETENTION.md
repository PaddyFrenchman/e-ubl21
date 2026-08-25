# Raw upload-content retention and storage operations

e-ubl21 retains lightweight upload metadata and validation results after the
original XML is reclaimed. Raw content has a separate lifecycle and a hard
shared Object Store byte quota.

## Configuration

Both `serve` and every `worker` replica must use the same values:

| CLI flag | Environment variable | Default | Meaning |
|---|---|---:|---|
| `--upload-content-retention` | `EUBL21_UPLOAD_CONTENT_RETENTION` | `720h` (30 days) | Time raw XML remains available after validation. `0` disables expiry. |
| `--upload-content-max-bytes` | `EUBL21_UPLOAD_CONTENT_MAX_BYTES` | `10737418240` (10 GiB) | Hard shared Object Store limit. `0` is unlimited. |

A non-zero retention must be at least one minute. A non-zero byte limit must
be at least 16 MiB so an otherwise valid near-10-MiB upload can fit with Object
Store metadata and chunking overhead.

The worker records `contentExpiresAt = validatedAt + retention` when it writes
`valid` or `invalid`. The API maintenance loop runs immediately at startup and
every 30 seconds. Once the deadline is due it deletes only the Object Store
object while retaining metadata with `contentExpiredAt`. The metadata marker
is CAS-committed first, so a failed write never deletes unmarked content; a
failed or interrupted object deletion is retried from the active-object scan.
Consequently:

- queued or retrying documents never expire before validation, even when a
  backlog is older than the configured retention;
- `GET /api/v1/ubl/{id}` continues returning validation metadata indefinitely;
- metadata includes `contentAvailable`, `contentExpiresAt`, and, after cleanup,
  `contentExpiredAt`;
- `GET /api/v1/ubl/{id}?includeContent=true` returns `410 Gone` after expiry;
- expiry is idempotent and safe when several API replicas sweep concurrently.

`contentAvailable` is derived from lightweight lifecycle metadata so normal
status polling never performs an Object Store lookup. It becomes false for
failed/cleanup/expired states; an unexpected storage failure can still make an
otherwise eligible object unavailable and is reported when content is fetched.

Object Store TTL is intentionally not used because it starts from object
creation and could remove an unprocessed document in a long backlog. Existing
terminal records created before this feature are enrolled by using their
`validatedAt` timestamp during the first sweep. The active retention setting
is applied to every unexpired terminal record with active raw content on each
scan. Shortening the duration can make older content immediately eligible for
expiry; extending it postpones expiry. Setting retention to `0` clears those
existing schedules without deleting content.

Sweeps enumerate the bounded active Object Store inventory and then load only
the corresponding metadata records. Indefinitely retained validation metadata
therefore does not make each retention scan grow with the service's complete
upload history.

## Capacity behavior

JetStream enforces the configured byte limit with discard-new behavior. When
the bucket is full, new raw-content writes fail rather than evicting older or
pending documents. Upload acceptance records the failure and performs the
existing partial-content cleanup flow. Operators should alert before the limit
is reached and either increase capacity or reduce retention.

The permanent per-upload result event introduced by TODO-009 is stored in a
separate stream and is not counted in the raw-content Object Store quota.

### Deleted-object metadata

JetStream Object Store deletion removes an object's payload chunks but retains
a delete-marker metadata record. Ordinary `List` calls exclude these deleted
entries, which keeps the expiry scan proportional to active raw objects, but
the tombstones remain part of the bucket's physical history and can accumulate
in very high-volume, long-running installations.

The hard Object Store byte cap still bounds payloads and tombstone metadata,
and `e_ubl21_upload_content_storage_bytes` reports their combined physical
usage. Tombstone cleanup is deliberately operator-scheduled rather than part
of every API or worker process. Preview a bounded batch first:

```bash
e-ubl21 --nats-url nats://localhost:4222 \
  --queue-namespace eubl21.prod \
  maintenance purge-content-tombstones \
  --grace-period 168h --max-items 100 --dry-run
```

Remove the `--dry-run` flag to apply the same selection. Output is one JSON
object containing `scanned`, `eligible`, `purged`, and `failed` totals plus the
namespace and effective controls. The command exits non-zero when listing
fails or any eligible item cannot be completed.

Eligibility is conservative: the delete marker must be older than the grace
period and canonical upload metadata must prove completed expiry
(`valid`/`invalid` plus `contentExpiredAt`) or terminal acceptance failure.
Active content and pending, cleanup-in-progress, dead-lettered, replayable,
missing, or unreadable records are never purged. Each marker is re-read before
its metadata subject is removed. The purge combines that subject with a broker
sequence boundary, so metadata for an object recreated after the read survives
the purge atomically. Repeated runs are idempotent, and `max-items` bounds
eligible work per invocation.

`--max-items` does not bound inventory enumeration: nats.go's Object Store
`List` API materializes the current live/deleted inventory before selection.
The configured Object Store byte cap provides an ultimate bound, but very
large installations should account for full-inventory scan cost when choosing
the schedule. Replacing `List` with lower-level paged/consumer iteration is a
future scalability improvement, not a separate cleanup implementation.

Use the exact command unchanged from cron, for example:

```cron
17 3 * * * /usr/local/bin/e-ubl21 --nats-url nats://nats:4222 --queue-namespace eubl21.prod maintenance purge-content-tombstones --grace-period 168h --max-items 500
```

The same implementation can be scheduled in Kubernetes:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e-ubl21-content-tombstones
spec:
  schedule: "17 3 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: maintenance
              image: e-ubl21:latest
              args:
                - --nats-url=nats://nats:4222
                - --queue-namespace=eubl21.prod
                - maintenance
                - purge-content-tombstones
                - --grace-period=168h
                - --max-items=500
```

## Metrics

The API `/metrics` endpoint exposes:

- `e_ubl21_upload_content_storage_bytes` — current Object Store size,
  including JetStream metadata;
- `e_ubl21_upload_content_storage_limit_bytes` — configured limit, or zero for
  unlimited;
- `e_ubl21_upload_content_expirations_total{outcome}` — scheduled,
  unscheduled, logically expired, physically reclaimed, already-missing, and
  error outcomes;
- `e_ubl21_upload_content_expired_bytes_total` — original XML payload bytes
  reclaimed, excluding Object Store metadata.

Recommended alerts include sustained storage above 80% of the configured
limit, any increase in expiry `error`, and upload failures caused by Object
Store capacity exhaustion.

## Upgrade behavior

Startup updates the existing upload-content Object Store with the configured
byte cap but does not set a broker TTL. The immediate maintenance scan will,
however, reclaim raw content for terminal records already older than the
configured retention, so operators should review the value before rollout.
Only due `valid`/`invalid` content is expired; acceptance and pending states
remain protected. Tombstone purge remains a separate explicit/scheduled
command and is not started automatically during upgrade.
