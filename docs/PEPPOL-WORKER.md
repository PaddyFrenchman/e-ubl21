# Independent Peppol Billing worker

`e-ubl21 worker-peppol` is the independently deployable third-stage validator.
It consumes only `peppol-bis-billing` `StageRequest` messages and publishes a
durable `StageResult`; it never invokes XSD, EN 16931, or the orchestrator
synchronously.

Before execution, the worker verifies that the request is the current
authorized ordinal in the frozen validation plan and reloads permanent passing
evidence for every prerequisite. A Peppol request therefore cannot run until
both UBL 2.1 XSD and EN 16931 have durable passing evidence. Stale, superseded,
or cross-stage requests are terminated without running Saxon.

The worker streams immutable upload content into the checksum-verified pinned
Peppol XSLT. Standards failures and advisory reports are returned as bounded
structured evidence. Missing content/assets, malformed SVRL, process failures,
resource limits, and timeouts are operational outcomes. The orchestrator alone
authorizes retries, advances the plan, and publishes the permanent final
result.

Run locally after the split topology is provisioned and activated:

```bash
e-ubl21 worker-peppol \
  --worker-peppol-metrics-addr :9095 \
  --worker-peppol-timeout 30s \
  --worker-peppol-concurrency 1 \
  --worker-peppol-prefetch 1 \
  --worker-peppol-progress-interval 20s
```

The 45-second deployment timeout covers Object Store streaming, profile
resolution, the Saxon process, and bounded SVRL parsing for one attempt. It is
shorter than the provisioned 120-second request `AckWait`; 20-second progress
heartbeats preserve ownership while work is active. Shutdown cancellation
propagates into the attempt context and terminates the Java child before the
request is NAKed for another worker.

Peppol uses the shared semantic-runtime bounds: 4 MiB maximum SVRL, 256 issues,
bounded rule/severity/location/test/message/diagnostic fields, 256 KiB total
issue text, 8 KiB worker diagnostic, and truncated Saxon stderr. Output or
profile-resolution evidence exceeding a bound is an operational
`resource-limit`, never a partial standards result. Malformed XML/SVRL remains
the separate bounded `malformed-output` reason.

The configured concurrency plus prefetch must not exceed the provisioned
Peppol request consumer `MaxAckPending`, and the progress interval must remain
below its `AckWait`. Kubernetes uses the Peppol request consumer lag for
independent scale-to-zero activation and exposes `/metrics` and `/healthz` on
port 9095.

Prometheus exposes bounded Peppol dimensions on the shared stage request,
service-duration, queue-wait, request-age, in-flight, retry, redelivery,
queue-depth, DLQ-depth, runtime-error, startup-error, and durable-publication
series documented in [`STAGE-TELEMETRY.md`](STAGE-TELEMETRY.md). Completion
logs include upload/validation/request correlation, document type, selected
profile and version, stage and ruleset, Peppol BIS version/dates, outcome,
bounded reason, issue count, and duration. XML content and rule IDs are never
metric labels and invoice content is never logged.

The measured production defaults are concurrency/prefetch 1/1, 1 CPU and
768 MiB requested, 2 CPU and 2 GiB limited, and KEDA lag threshold 4. Run
`CAPACITY_SAMPLES=100 make benchmark-peppol` for the host matrix and
`CAPACITY_SAMPLES=100 make benchmark-peppol-cgroup` for the deployment limits.
The recorded service, durable publication, memory, throttling, and scaling
decision evidence is in [`STAGE-CAPACITY.md`](STAGE-CAPACITY.md).

Durable replay, failure, shutdown, and EN-only-to-Peppol upgrade rules are
specified in [`PEPPOL-DURABILITY-MIGRATION.md`](PEPPOL-DURABILITY-MIGRATION.md).
