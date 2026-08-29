# Independent EN 16931 worker

**Status:** Implemented process contract for `TODO-VALIDATION-SPLIT-008`

`e-ubl21 worker-en16931` is the independently deployable EN 16931 semantic
validation role. It consumes only the `en16931` request consumer, streams the
immutable XML object identified by `uploadId`, executes the pinned Saxon/XSLT
adapter, and publishes bounded durable stage evidence.

The worker never runs XSD, selects or dispatches another stage, authorizes a
retry, mutates execution/upload state, or publishes a final document result.

## Authorization and prerequisite evidence

Before opening XML or starting Java, the worker loads the exact durable
`uploadId`/`validationId`/`generation` execution and rejects the request unless
all of these remain true:

- document, plan, plan version, ordinal, stage, and ruleset match the frozen
  execution;
- EN 16931 is the current queued stage and the request ID/attempt is the
  currently authorized attempt;
- every earlier stage has a terminal `passed` state and matching immutable
  `StageResult` evidence;
- the earlier stages include passing UBL 2.1 XSD evidence.

This makes delayed requests from an old plan, generation, ordinal, or attempt
incapable of invoking Saxon. Malformed, misrouted, or stale requests are
terminated. A transient failure while reading execution/prerequisite evidence
NAKs the delivery so authorization is never guessed.

## Durable processing contract

For an authorized request, processing is ordered as follows:

1. Confirm the requested ruleset is the pinned EN 16931 release and the
   document type is supported.
2. Stream the upload from Object Store into the existing restricted Saxon
   adapter under one attempt deadline.
3. Preserve the bounded ordered SVRL issue set and map the result to `passed`,
   `standards-invalid`, or a typed `operational-failure`.
4. Publish the deterministic `StageResult` and wait for JetStream's durable
   publication acknowledgement.
5. ACK the EN request.

Result publication failure leaves the request redeliverable. Shutdown cancels
the Object Store read and Java subprocess and NAKs interrupted work. Duplicate
recomputation uses the same deterministic result identity and converges on the
orchestrator's immutable evidence slot.

Missing content, Java/assets, Saxon failure, malformed SVRL, output limits,
timeouts, and cancellation remain operational failures rather than invoice
verdicts. External diagnostics are normalized to valid UTF-8 and bounded before
publication. SVRL parsing already enforces issue count, per-field, aggregate
text, and output-size limits.

Warning/warn `failed-assert` entries are preserved as advisory evidence in a
`passed` result. Fatal, error, empty, or unknown failed-assert severities are
standards failures. Structurally invalid normalized issue evidence is published
as `operational-failure`/`malformed-output` and ACKed, preventing deterministic
validator defects from becoming an endless request-redelivery loop.

An operational result is immutable evidence for one EN attempt. The
orchestrator applies the frozen attempt budget and may authorize another EN
attempt without rerunning XSD. Stage delivery counters, DLQ publication, and
operator replay remain owned by `TODO-VALIDATION-SPLIT-010` (#57); this command
does not create a competing retry/replay contract.

## Startup and scaling

Startup performs read-only checks for the EN request stream/consumer, EN result
publication stream, Object Store, execution/evidence buckets, and the complete
pinned Saxon runtime. The checksum and smoke-transform gate must pass before
the worker pulls requests. The orchestrator result consumer is not part of EN
worker readiness.

The worker reads the actual request consumer `AckWait` and `MaxAckPending`.
Concurrency plus prefetch must not exceed `MaxAckPending`, and the progress
interval must be shorter than `AckWait`.

```bash
e-ubl21 worker-en16931 \
  --nats-url nats://localhost:4222 \
  --worker-en16931-metrics-addr :9093 \
  --worker-en16931-timeout 30s \
  --worker-en16931-concurrency 1 \
  --worker-en16931-prefetch 1 \
  --worker-en16931-progress-interval 20s
```

Use `e-ubl21 worker-en16931 --help` for the pinned Java, Saxon, XML Resolver,
configuration, and XSLT path flags. Environment names use the `EUBL21_` prefix
and uppercase underscore conversion.

The EN deployment may scale to zero while XSD workers and the orchestrator
continue. EN requests remain durable in their independent stream and are
processed when this role scales back up. Activating the split roles in place
of the transitional combined worker remains a later deployment migration
decision.
