# Production YBC backup control plane

Issue #149 selects **Option A: e-ubl21-owned control plane around the existing Helm universe**. The three zone-specific Helm releases remain database ownership. YugabyteDB Anywhere and the Kubernetes Operator are not introduced: the pinned 2025.2.6.0-b111 multi-release, AZ-overridden universe has not been qualified for import or Operator management.

The intended data flow is:

```text
CronJob/scheduler -> YBC full/incremental backup -> S3-compatible storage
  -> retention/status/evidence -> clean-target restore -> verification
```

The checked-in CronJob manifest is an **interface contract only**, not a qualified implementation: its runner image, MinIO development environment, chain-aware incremental implementation, object-store configuration, retention worker, and restore orchestration are still to be added. It must not be used as production evidence and `make yugabytedb-ybc-control-plane-test` currently performs only manifest preflight.

The implemented runner must invoke YBC, never `ysql_dump`, against all master addresses; associate each incremental with a successful full chain; block increments after full-chain failure; retain controller status, source version, chain identifier, object prefix, manifest checksum and Job logs; and separately expose last-successful-full and last-successful-incremental metrics. Workload identity (or equivalent short-lived federation) is required; static object-store credentials are forbidden in Git, Helm values, ConfigMaps, Secrets, commands and evidence.

The acceptance qualification will use disposable K3d and MinIO with generated namespace, bucket and scoped identity; prove full plus incremental backup, status/evidence, enforced shortened retention, isolated clean-target restore, schema/data and application access; retain output under `regression-results/yugabytedb-ybc-control-plane/`; and run the required regression suite. A local result does not establish cloud-AZ/rack independence.