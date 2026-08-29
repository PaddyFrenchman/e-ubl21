# Independent UBL 2.1 XSD worker

**Status:** Implemented process contract for `TODO-VALIDATION-SPLIT-007`

`e-ubl21 worker-xsd` is the independently deployable UBL 2.1 XSD stage
worker. It opens only the durable `ubl21-xsd` request consumer, reads the
immutable XML object identified by `uploadId`, resolves the requested document
type through the embedded UBL 2.1 catalog, and runs the existing hardened
`xmllint` streaming adapter.

The worker does not choose a validation plan, authorize another attempt or
stage, publish another `StageRequest`, mutate upload/execution state, or publish
a final document result. Those remain orchestrator responsibilities.

## Durable processing contract

For one valid XSD `StageRequest`, the worker performs these operations in
order:

1. Decode the fail-closed V1 request and confirm its stage is `ubl21-xsd`.
2. Check the requested ruleset against the embedded UBL 2.1 schema version.
3. Stream the upload from the shared Object Store into `xmllint` under one
   attempt deadline.
4. Publish a durable deterministic `StageResult` for that same logical attempt.
5. ACK the request only after JetStream confirms the result publication.

A crash before step 4 leaves no evidence and causes broker redelivery. A crash
after step 4 but before step 5 may recompute and republish the deterministic
result; the orchestrator's immutable first-evidence rule makes that duplicate
harmless beyond JetStream's finite message-ID deduplication window.

Standards violations produce `standards-invalid` with a bounded XSD issue.
Timeouts, cancellation, missing/truncated content, unavailable `xmllint`, and
runtime failures produce `operational-failure` with a typed reason and bounded
diagnostic. Content read errors take precedence over any partial schema
diagnostics, so infrastructure failure cannot become an invoice verdict.

Publication failure NAKs the request for delivery retry. An operational
`StageResult` is durable attempt evidence; the orchestrator applies the frozen
XSD attempt budget and either authorizes the next XSD attempt or records
`processing-failed` when exhausted. The stage-specific DLQ stream, delivery
counter persistence, and operator replay command are deliberately introduced
by `TODO-VALIDATION-SPLIT-010` (#57), which owns those shared policies. This
worker does not invent a competing replay contract.

Malformed or physically misrouted messages are terminated because they cannot
be valid authorized XSD attempts. Their broker-level quarantine/audit record is
also part of #57.

## Startup and shutdown

Infrastructure is provisioner-owned. On startup the worker performs read-only
compatibility checks for the XSD request stream and worker consumer, the result
stream it publishes to, and the upload Object Store. It deliberately does not
open or verify the orchestrator-owned XSD result consumer, so that consumer's
delivery tuning cannot affect worker readiness. The worker loads the embedded
schema catalog and verifies `xmllint` before it begins pulling work. It fails
readiness rather than creating or modifying broker resources.

Active and locally prefetched deliveries receive progress acknowledgements.
The worker reads the actual provisioned request consumer's `MaxAckPending` and
`AckWait` at startup. Concurrency plus prefetch must fit that `MaxAckPending`,
and the progress interval must be shorter than that `AckWait`.
On SIGINT/SIGTERM the command stops pulling, cancels active Object Store reads
and `xmllint` subprocesses, NAKs interrupted work, drains the local worker pool,
and shuts down its metrics server.

## Run

The stage topology and Object Store must already be provisioned:

```bash
e-ubl21 worker-xsd \
  --nats-url nats://localhost:4222 \
  --worker-xsd-metrics-addr :9092 \
  --worker-xsd-timeout 30s \
  --worker-xsd-concurrency 1 \
  --worker-xsd-prefetch 1 \
  --worker-xsd-progress-interval 20s
```

The equivalent environment keys use the `EUBL21_` prefix, for example
`EUBL21_WORKER_XSD_CONCURRENCY` and `EUBL21_WORKER_XSD_TIMEOUT`.

This command can run while the EN worker is scaled to zero. Successful XSD
results continue to accumulate, the orchestrator can durably enqueue EN
requests, and those requests remain in the independent EN stream until an EN
worker is available. Activating this split role in place of the transitional
combined worker is a deployment migration decision, not an automatic side
effect of this issue.
