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
against the target. The executable production YBC drill is:

```bash
EUBL21_DR_YBC_BACKUP_COMMAND='platform YBC backup command' \
EUBL21_DR_RECREATE_TARGET='platform destroy/recreate clean target command' \
EUBL21_DR_YBC_RESTORE_COMMAND='platform YBC restore command' \
EUBL21_DR_APP_URL='postgresql://application-role@restored-target/eubl21?sslmode=verify-full' \
./devsetup/yugabytedb-dr-drill.sh
```

The recreate command must wait for a genuinely new, clean target. The runner
refuses logical dump/restore commands, verifies application-role access, and
requires complete regression. Run `make yugabytedb-dr-local-test` separately
as the independent logical portability drill.

For development-only YBC data-plane evidence without external infrastructure,
run `make yugabytedb-ybc-dr-test`. It unpacks the pinned YB Controller,
starts a disposable source universe with `--backup_daemon=true`, writes the
backup to a host-mounted NFS-style directory, deletes/recreates the restore
universe, performs `yugabyted restore`, and runs the application database
integration test. Its retained evidence records controller backup/restore
results, versions, location, schema/data verification, and application test.
This is not production scheduling, retention enforcement, object-store
identity, or control-plane orchestration; those remain issue #149.

`make yugabytedb-dr-local-test` remains the independent logical
`ysql_dump`/`ysqlsh` portability and recovery check. It is not YBC evidence.

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
Grafana dashboards show master availability and leader status, live tservers,
Raft replication health, YSQL p99 statement latency, PVC free space, and
storage growth. YSQL error and compaction investigation uses the retained
YSQL and TServer Prometheus endpoints until dedicated dashboard panels are
introduced. Alert immediately for fewer than three live masters or
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
