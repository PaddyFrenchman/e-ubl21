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

The configured concurrency plus prefetch must not exceed the provisioned
Peppol request consumer `MaxAckPending`, and the progress interval must remain
below its `AckWait`. Kubernetes uses the Peppol request consumer lag for
independent scale-to-zero activation and exposes `/metrics` and `/healthz` on
port 9095.

Public HTTP selection of `peppol-bis-billing` is deliberately separate and is
owned by issue #75. Until that contract lands, the durable internal profile and
worker can be exercised by orchestrator-created plans without changing legacy
client behavior.

Durable replay, failure, shutdown, and EN-only-to-Peppol upgrade rules are
specified in [`PEPPOL-DURABILITY-MIGRATION.md`](PEPPOL-DURABILITY-MIGRATION.md).
