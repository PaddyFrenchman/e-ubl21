# Production YBC backup control plane

Issue #149 selects **Option A: e-ubl21-owned control plane around the existing Helm universe**. The three zone-specific Helm releases remain the database deployment. YugabyteDB Anywhere/Kubernetes Operator ownership is excluded: the pinned 2025.2.6.0-b111 multi-release, AZ-overridden universe has not been qualified for import or operator management.

```text
CronJob/scheduler -> YBC full/incremental backup -> S3-compatible storage
  -> retention/status/evidence -> clean-target restore -> verification
```

The control plane supplies a pinned e-ubl21 runner and non-overlapping daily-full and six-hour-incremental Jobs. A failed full chain blocks dependent incrementals until investigated. The runner invokes YBC, never `ysql_dump`, against all master addresses and retains source version, chain identifier, object prefix, controller status, manifest checksum and Job logs.

The runner receives only platform workload identity (or equivalent short-lived federation). Static object-store credentials are forbidden in Git, Helm values, ConfigMaps, Secrets, commands and evidence. The private encrypted versioned off-cluster bucket retains 35 daily, 12 monthly and 7 yearly recovery points. Delete a chain only after it leaves every window and its evidence is retained. Prometheus pages on failed Jobs and no success within seven hours.

## Development qualification

`make yugabytedb-ybc-control-plane-test` creates disposable K3d and MinIO, generates its namespace, bucket and scoped identity, then proves full backup, incremental backup, retained metadata/status, shortened retention, clean-target restore, schema/data and application access. It retains evidence under `regression-results/yugabytedb-ybc-control-plane/` and removes the cluster. This does not claim cloud-AZ/rack independence.

## Production qualification

Before automated production DR is supported, repeat against a real off-cluster destination and workload identity; prove failure/late-backup alerts; restore a retained point to a new isolated production-like universe; recreate non-database identities; verify application access; and retain controller, object, Job and application-test evidence.
