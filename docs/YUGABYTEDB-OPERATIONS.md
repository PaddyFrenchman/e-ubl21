# YugabyteDB backup, DR, security and observability

This is the production operating contract for the three-zone universe defined
in `YUGABYTEDB-HA-ARCHITECTURE.md`. It applies to the pinned YugabyteDB 2025.2
release and is deliberately separate from the disposable K3d HA test.

## Recovery policy

Production uses YugabyteDB Controller (YBC) distributed snapshots to an
operator-owned, encrypted object-store bucket in a different failure domain
from the universe. The bucket is private, versioned, and has an immutable
retention lock. Database pods receive only a workload-identity policy scoped
to their universe prefix; long-lived cloud keys are forbidden.

Take an incremental backup every six hours and a full backup every day. Retain
35 daily, 12 monthly, and 7 yearly recovery points. Alert on any failed or
late backup. Retain the backup manifest, checksum, source universe/version,
and restore evidence with each recovery point. The retention controller may
delete only backups beyond all three retention windows.

PITR is not enabled by default. Enable it only after storage-cost and recovery
objective approval; it cannot replace retained off-cluster backups. PITR
retention is 24 hours when enabled, and operators must verify that a restore
target is on a compatible or newer supported YugabyteDB release.

## Restore drill

At least quarterly, create an isolated target universe and restore a retained
backup. It must be a clean cluster/database, not a destructive in-place test.
Then run schema/data checks and the application database integration test
against the target. The executable logical drill is:

```bash
EUBL21_DR_SOURCE_URL='postgresql://...' \
EUBL21_DR_RESTORE_URL='postgresql://...' \
EUBL21_DR_BACKUP_DIR=/mnt/off-cluster-backups/e-ubl21 \
EUBL21_DR_RUN_REGRESSION=1 \
./devsetup/yugabytedb-dr-drill.sh
```

The target URL must name an empty database created from `template0`; the
runner refuses a matching source URL or an existing qualification table. For a
production exercise, run the YBC snapshot restore first, then this logical
drill as the independent schema/data/application-access proof. Store the
generated SHA-256 sidecar and command output as drill evidence.

For development-only evidence without external infrastructure, run
`make yugabytedb-dr-local-test`. It starts disposable source and target
containers, retains the dump on the host outside both containers, restores it,
and runs the application database integration test. It is explicitly not a
substitute for the production off-cluster drill.

## Security

TLS is mandatory for client-to-server and node-to-node traffic. The production
values use a shared, platform-managed CA; rotate client certificates before
expiry and rotate the CA with overlap, one zone at a time. Keep master,
tserver, and metrics endpoints private; expose only the application YSQL
service through the approved private network path.

The application uses a non-superuser login role with only `CONNECT`, `USAGE`
on its schemas, and table/sequence rights granted by migrations. A separately
held migration role owns DDL privileges. A backup role is used only by the
backup workload; it is not an application credential. Rotate every credential
through a new Kubernetes Secret version, roll clients, validate the new login,
then revoke the old password/certificate. Never place passwords, client keys,
or object-store credentials in Git, ConfigMaps, dashboards, or shell history.

## Observability and alerts

Prometheus scrapes each master on port 7000 and each tserver on port 9000 at
`/prometheus-metrics`; node-exporter supplies host disk/CPU/memory metrics.
Grafana dashboards must show master availability/leader status, live tservers,
Raft replication health, YSQL latency and errors, PVC free space, compaction,
and storage growth. Alert immediately for fewer than three live masters or
tservers, failed backup, or replication/consensus health loss. Page for disk
free space below 30%; warn at 40%. Page for sustained YSQL p99 latency above
the approved SLO and warn on sustained storage growth that exhausts provisioned
capacity within 30 days.

Apply `deploy/production/yugabytedb/observability.yaml` after the
Prometheus Operator is installed, and apply `dashboard.yaml` in the monitoring
namespace used by Grafana. The `release: monitoring` selector matches the
repository's `kube-prometheus-stack` installation name; alter it only when the
monitoring release is deliberately renamed.

Runbooks: for node loss, preserve evidence, confirm quorum, replace only one
failure domain at a time, and wait for re-replication. For storage exhaustion,
stop nonessential writes, expand storage before the 30% floor, and verify
rebalancing. For failed migrations, stop rollout, retain logs and migration
version, restore a pre-migration recovery point to an isolated target, and
perform rollback only from the reviewed runbook.

YugabyteDB documents Prometheus endpoints for masters and tservers, consistent
YSQL dumps, and distributed snapshots/PITR in its [metrics documentation](https://docs.yugabyte.com/stable/launch-and-manage/monitor-and-alert/metrics/), [YSQL dump documentation](https://docs.yugabyte.com/stable/admin/ysql-dump/), and [backup and restore documentation](https://docs.yugabyte.com/stable/manage/backup-restore/).
