# Guarded on-prem PostgreSQL-to-YugabyteDB upgrade

This runbook is the only supported path from an on-prem installation created by
the former CloudNativePG bootstrap to the single-node YugabyteDB baseline. The
fresh installer intentionally stops when it finds `e-ubl21-postgres`.

The guarded path never deletes PostgreSQL during inspection or YugabyteDB
cutover. Retirement is a separate command requiring a verified cutover record,
an unchanged source-cluster identity, a validated backup, a second empty-source
inspection, live YugabyteDB verification and an exact confirmation value.

## Current data-transfer boundary

The old installer created the `eubl21` database and role but did not create an
application schema. This release therefore recognizes only an empty legacy
database as eligible for automated cutover. Any non-system schema, table,
partition, view, materialized view, sequence, foreign table or large object is
treated as unknown business data and blocks cutover, even when a table has no
rows. Any additional connectable database also blocks cutover, and the default
`postgres` database is inspected alongside `eubl21`; retirement never assumes
that unrecognized databases are disposable. Nonstandard roles, tablespaces,
routines and user-defined types are likewise blocking customizations. The
allowlist contains only PostgreSQL built-in roles, `eubl21`, the replication
role created by the old manifest and CloudNativePG's documented
`cnpg_metrics_exporter` role.

There is deliberately no generic automatic restore into YugabyteDB. A direct
`pg_restore` could overwrite migration-owned objects or rely on PostgreSQL
features that have not passed the YugabyteDB compatibility gate. When
inspection reports objects:

1. export the database with the command below;
2. keep CloudNativePG running and do not run `retire`;
3. inventory the archive with `pg_restore --list BACKUP_FILE` in a secured
   PostgreSQL environment;
4. define and review an explicit table/schema mapping and YugabyteDB-compatible
   import for the recognized application schema;
5. qualify row counts, constraints and application behavior before adding that
   mapping to this runbook.

As of this release there is no recognized legacy business schema, so step 4
requires a new reviewed migration change rather than an operator override.

## Prerequisites

- Use the Kubernetes context for the existing on-prem K3s installation.
- Confirm the legacy resource is named `e-ubl21-postgres` in the `e-ubl21`
  namespace.
- Make a secured directory on storage that is independent of the K3s host.
- Make the pinned e-ubl21 image available to K3s; it supplies schema migration
  `000001` and later.
- Keep the server and database unavailable to configuration changes throughout
  inspection and cutover. A new source object after cutover blocks retirement.

Start by recording status:

```bash
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh status
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh inspect
```

`inspect` is read-only. Failure to reach the primary or query the database is a
blocking result, never an assumption that the database is empty.

## Export the source database

Choose a new path; the script refuses to overwrite either a final or partial
backup file. It runs `pg_dump --format=custom` inside the existing primary,
writes with a restrictive umask, and validates the archive with `pg_restore
--list` before publishing the final filename. It also writes
`BACKUP_FILE.eubl21-metadata`, binding the archive checksum to the immutable
legacy Cluster UID. Keep the two files together; retirement requires both.

```bash
mkdir -p /secure/off-host/e-ubl21
chmod 700 /secure/off-host/e-ubl21
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh export \
  /secure/off-host/e-ubl21/postgresql-before-yugabyte.dump
```

Copy the backup, metadata sidecar and printed SHA-256 digest off-host. The
backup contains sensitive application data and must be encrypted and
access-controlled.

## Install and verify YugabyteDB

Cutover requires the legacy inspection to be empty. It reuses the generic
`eubl21-db-auth` username/password through an upgrade-only staging Secret,
installs the pinned YugabyteDB chart in guarded-upgrade mode, waits for YSQL,
creates the application identity/database, runs all schema migrations and
verifies both completion Jobs. The public `eubl21-db-auth` Secret receives the
YugabyteDB URL only after those checks pass.

```bash
EUBL21_IMAGE=registry.example/e-ubl21@sha256:REPLACE_ME \
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh cutover
```

Only after all checks succeed does the script write the
`eubl21-yugabytedb-cutover` ConfigMap with status `verified` and the immutable
UID of the inspected legacy cluster. If installation, bootstrap, migration or
verification fails, the legacy Cluster resource is not changed and no verified
state is recorded. Promotion uses the snapshotted Kubernetes Secret resource
version, so a concurrent public-Secret change blocks cutover. If final Secret
verification or cutover-state recording fails after promotion, the script
restores the previous public database URL; a concurrent change also makes that
rollback fail closed if it changes the promoted URL, with an explicit repair
instruction. The staging Secret may remain for diagnosis and a safe retry, but
is not consumed by the legacy deployment.

The command is safe to rerun. A successful rerun upgrades the same pinned Helm
release, reapplies idempotent bootstrap/migrations and refreshes the verified
record for the same source cluster.

## Rollback window

After cutover, `e-ubl21-postgres`, its pods and its persistent volumes remain in
place. Do not retire it until the operator has independently verified:

- YugabyteDB StatefulSets are ready;
- bootstrap and migration Jobs are complete;
- schema version is current;
- the application image and future production deployment use the YugabyteDB
  URL in `eubl21-db-auth`;
- the validated PostgreSQL backup is stored off-host and restore-tested under
  the site's recovery policy.

If YugabyteDB qualification fails, stop using the YugabyteDB endpoint and keep
the legacy cluster. The upgrade script performs no automatic traffic switch and
does not delete YugabyteDB during rollback.

## Explicitly retire the legacy cluster

Deleting a CloudNativePG Cluster can also remove its pods and persistent
volumes. This step is destructive and cannot be recovered without the backup.
The script verifies the sidecar checksum and source UID, revalidates the backup
through the live PostgreSQL primary, repeats the empty-database inspection and
rechecks YugabyteDB immediately before deletion. The final target check starts
a new, authenticated migration command through the promoted public database
URL. This opens a fresh YSQL connection and validates the complete migration
history against the immutable migrations embedded in the application image;
previously completed Jobs alone are not accepted as current database health.

```bash
EUBL21_RETIRE_LEGACY_POSTGRESQL=delete-e-ubl21-postgres-after-verified-backup \
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh retire \
  /secure/off-host/e-ubl21/postgresql-before-yugabyte.dump
```

The cutover record moves through `retiring` to `retired`. If deletion fails
while the cluster still exists, rerunning the same command with the same backup
repeats every verification gate and safely retries deletion. If Kubernetes has
already removed the cluster, the retry verifies the recorded backup checksum
before completing the `retired` record. API/RBAC failures are blocking and are
never interpreted as cluster absence. The backup SHA-256 is recorded, but the
backup itself is never stored in Kubernetes.

The CloudNativePG operator remains installed so other clusters are never
silently affected. After the required rollback-retention period, remove only
the old `cnpg` Helm release with a second explicit command:

```bash
EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR=uninstall-cnpg-after-all-clusters-are-retired \
deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh retire-operator
```

This refuses to proceed while any CloudNativePG Cluster exists in any
namespace. Failure to inventory clusters is also blocking rather than being
treated as an empty result. It preserves the `cnpg-system` namespace and CRDs
because they can contain independently managed state; remove those only through
a separate, site-reviewed Kubernetes maintenance procedure.

## Qualification

Run the non-destructive scenario suite:

```bash
make on-prem-database-upgrade-test
EUBL21_TEST_IMAGE=localhost/e-ubl21:regression make on-prem-database-upgrade-integration
```

It covers an empty source, an unknown relation, failed YugabyteDB installation,
atomic Secret promotion and rollback, validated export/no-overwrite, fully
gated explicit retirement, fresh target-schema verification, interrupted
retirement recovery, failed cluster inventory, missing confirmation and
idempotent rerun. The normal fresh-install and schema gates remain required as
applicable. The integration target additionally creates a real legacy
CloudNativePG cluster, repeats the safety scenarios, performs the two-pass
YugabyteDB cutover, explicitly retires the source and operator, and cleans up
only resources whose prior absence it verified. It refuses to run on a
Kubernetes context that already contains CloudNativePG resources.
