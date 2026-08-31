# Legacy-to-split deployment migration

Issue #61 introduces a durable, namespace-scoped deployment generation barrier.
It prevents the combined `worker` runtime and the split `orchestrator`,
`worker-xsd`, and `worker-en16931` runtimes from processing the same active
work generation.

## Durable modes

The `*_DEPLOYMENT_GENERATION` JetStream KV bucket contains one CAS-protected
marker:

| Mode | API admission | Combined worker | Split runtimes |
| --- | --- | --- | --- |
| `legacy` | Publishes legacy document descriptors | allowed | refused |
| `draining` | HTTP 503 with `Retry-After` | allowed to finish retained work | refused |
| `activating` | HTTP 503 with `Retry-After` | refused | refused |
| `split` | Creates a frozen validation execution | refused | allowed |

Every transition increments the deployment generation. The only permitted
transitions are `legacy -> draining`, `draining -> legacy`,
`draining -> activating`, and `activating -> split`. Reconciliation begins
only after the CAS-protected `activating` claim and is forward-only from there.

An installation without the marker remains compatible with the legacy
combined worker. Split runtimes always fail closed until the marker exists and
is `split`.

## Preparation

Use one immutable image for the API, migration command, orchestrator, and stage
workers. Provisioning creates the split queues, execution/evidence buckets, and
an initial `legacy` deployment marker without activating the new consumers:

```bash
e-ubl21 provision --nats-url "$NATS_URL" --queue-namespace "$QUEUE_NAMESPACE"
e-ubl21 migration status --nats-url "$NATS_URL" --queue-namespace "$QUEUE_NAMESPACE"
```

Keep the existing combined workers running during preparation. Do not start
split runtimes yet; they will refuse the legacy marker.

## Stop and drain

1. Begin the barrier. The API immediately stops accepting new uploads, while
   already durable legacy descriptors remain available to combined workers:

   ```bash
   e-ubl21 migration begin-drain --nats-url "$NATS_URL" --queue-namespace "$QUEUE_NAMESPACE"
   ```

2. Wait until `streamMessages`, `consumerPending`, and `consumerAckPending` are
   zero. The consumer counters aggregate every durable consumer on the legacy
   documents stream, including workers with custom names or subject filters.
   Then scale every combined worker deployment to zero. This removes outstanding
   pull requests, so `consumerWaiting` also becomes zero.

3. Confirm `ready: true`:

   ```bash
   e-ubl21 migration status \
     --nats-url "$NATS_URL" \
     --queue-namespace "$QUEUE_NAMESPACE" \
     --legacy-consumer-name ubl-validator
   ```

Do not activate while any of the four drain counters is non-zero. Activation
checks them both before and after legacy reconciliation and aborts if the queue
becomes active in between. The named expected consumer must exist, so a typo
fails closed. Only an installation where the combined worker was never started
may explicitly add `--allow-no-legacy-consumer`; this still enumerates and
checks every consumer that does exist.

## Reconcile and activate

Run:

```bash
e-ubl21 migration activate \
  --nats-url "$NATS_URL" \
  --queue-namespace "$QUEUE_NAMESPACE" \
  --legacy-consumer-name ubl-validator
```

Activation handles records as follows:

- existing `valid`, `invalid`, `processing_failed`, and acceptance-failed
  records are preserved byte-for-byte;
- pending/accepting/enqueue-pending records with durable content are reset to
  unambiguous pending metadata and restarted from XSD under a complete frozen
  `ubl21` or `ubl21-en16931` plan;
- incomplete legacy stage evidence is never inherited or used to manufacture
  a new valid result;
- an `accepting` record without currently visible content is left byte-for-byte
  unchanged and blocks activation because its HTTP content write may still be
  in flight; rerun activation after the request or normal stale-acceptance
  reconciliation finishes;
- an enqueue-pending record without durable content becomes `acceptance_failed`;
- a pending record without content, a pending dead-letter transition, invalid
  metadata, or any infrastructure/CAS error blocks activation.

Activation first claims the forward-only `activating` mode. Only after every
candidate has a durable execution and first-stage outbox does
the marker move to `split`. Start or scale the orchestrator and stage workers
after the command succeeds. The API observes the marker for each admission and
automatically resumes using split execution submission. Status reads project
the selected execution's permanent canonical result over immutable acceptance
metadata; they never expose a terminal split result before its complete stage
evidence is durably published. Content retention uses the same projection.

## Rollback

Before the `activate` command claims reconciliation, rollback is safe and explicit:

```bash
e-ubl21 migration rollback --nats-url "$NATS_URL" --queue-namespace "$QUEUE_NAMESPACE"
```

This changes `draining` back to `legacy`; restart/scale the combined workers
and confirm API admission. It does not delete queues, records, or content.

Once `activate` reports `activating`, rollback is prohibited even if the
command later reports a repairable record or infrastructure error. Fix the
reported condition and rerun `activate`; its durable execution creation and
dispatch are idempotent. This prevents partially reconciled split work from
being handed back to a legacy worker.

After the marker reaches `split`, rollback to the monolithic worker is
forbidden. The old worker cannot interpret frozen execution state and could
produce mixed-generation results. Roll back only to a previously verified
split-capable application image, keeping the marker and durable split queues.
If split processing is impaired, scale stage workers to zero, repair the
split-capable deployment, and resume; queued work remains durable.

## Verification

After activation verify:

- `migration status` reports `mode: split`;
- the combined worker refuses to start;
- split runtimes start successfully;
- the legacy documents stream remains empty;
- every migrated pending Invoice/CreditNote has a two-stage frozen plan
  beginning at XSD;
- previously terminal upload records and their permanent results are unchanged;
- the correctness and Minikube regression gates remain semantically equivalent
  to the frozen monolithic baseline.
