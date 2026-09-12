# YugabyteDB migration baseline

This document freezes the repository state from which the YugabyteDB migration
program begins. It is the evidence and inventory for MIG-YB-001 / issue #119;
it is not an instruction to change the current runtime before the dependent
migration issues are implemented.

## Frozen repository state

- Baseline branch: `development`
- Baseline commit: `f5ed46a7dc30a07e11069f93a6d48b9307222b5d`
- Commit subject: `Track YugabyteDB migration issues in issue index`
- Commit date: `2026-09-12T20:24:15+01:00`
- Relational application layer: none found
- Checked-in relational schema or schema migration set: none found

At this commit, the authoritative runtime state remains:

```text
upload metadata/status  -> JetStream KV
raw XML                 -> JetStream Object Store
work/results            -> NATS / JetStream
```

Repository searches found no Go import of `database/sql` or `pgx` and no SQL
migration files. PostgreSQL is therefore an installed platform component and a
future architecture assumption, not the current application system of record.

## Implemented deployment identifiers to migrate

The following contracts exist in `deploy/on-prem/scripts/install-on-prem.sh`.
They must be handled by the new-install work in #123 and the guarded upgrade
work in #124.

| Kind | Current identifier/value | Migration significance |
|---|---|---|
| Environment variable | `POSTGRES_STORAGE_SIZE` (default `50Gi`) | Replace with a YugabyteDB operational storage setting; document upgrade mapping. |
| Environment variable | `CLOUDNATIVEPG_CHART_VERSION` (default `0.29.0`) | Remove from the e-ubl21 installation path after YugabyteDB replacement is qualified. |
| Helm repository | `cnpg` / `https://cloudnative-pg.github.io/charts` | Remove from new installations. |
| Helm release | `cnpg` in namespace `cnpg-system` | Detect and preserve during upgrades until explicit retirement. |
| Kubernetes API/kind | `postgresql.cnpg.io/v1`, `Cluster` | Detect as a legacy deployment resource. |
| Kubernetes resource | `e-ubl21-postgres` in the e-ubl21 namespace | Preserve and inspect on upgrade; never delete unknown data automatically. |
| Secret | `eubl21-db-auth`, type `kubernetes.io/basic-auth` | Generic name can be preserved where safe; current keys are `username` and `password`. |
| Database and owner | `eubl21` / `eubl21` | Recreate through YSQL bootstrap without assuming any legacy database is empty. |
| Installer functions | `install_cloudnativepg`, `install_postgresql_cluster` | Replace in the new-install path and recognize in upgrade history. |
| Operator check | `clusters.postgresql.cnpg.io` | Replace for new-install readiness; retain only where needed for legacy detection. |

The installer currently creates a one-instance cluster and generates the
database password if `eubl21-db-auth` does not already exist. No application
pod currently consumes this database as a business-state dependency.

## Complete reference inventory

The inventory was produced with case-sensitive repository searches for each
required term, excluding `.git` and generated `regression-results` content:

```bash
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'PostgreSQL' .
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'postgres' .
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'CloudNativePG' .
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'CLOUDNATIVEPG' .
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'POSTGRES_' .
rg -l --hidden --glob '!.git/**' --glob '!regression-results/**' 'e-ubl21-postgres' .
```

Every matching file is classified below. Historical migration references are
retained deliberately; active product claims are updated by later issues after
the deployment and operational replacements exist.

| File | Classification and required treatment |
|---|---|
| `.github/roadmap-0.md` | Active roadmap; replace PostgreSQL persistence assumptions in #129. |
| `README.md` | Active product roadmap/capabilities; replace PostgreSQL baseline claims in #129. |
| `ISSUES.MD` | Governance and issue history; #89 wording is deliberately retained until #129, while migration-history references remain. |
| `deploy/on-prem/scripts/install-on-prem.sh` | Live deployment implementation; replace new-install behavior in #123 and protect legacy resources in #124. |
| `deploy/on-prem/README.md` | Active on-prem documentation; update with the installer in #123 and finish the audit in #129. |
| `deploy/on-prem/deployment-on-prem.md` | Active deployment design and settings; update across #123, #124, #127, and #129. |
| `deploy/on-prem/licensing.md` | Active distribution/licensing inventory; remove retired components and add the selected YugabyteDB distribution obligations in #129. |
| `deployment-target.md` | Active cloud architecture; replace managed-PostgreSQL targets in #127/#129. |
| `docs/SERVER-FUNCTIONALITY-AND-INVOICE-PLATFORM-ARCHITECTURE.md` | Active application architecture; retarget future relational schema work in #129. |
| `docs/YUGABYTEDB-MIGRATION-BASELINE.md` | This frozen inventory; references are intentional migration evidence. |
| `docs/architecture-decisions/0001-yugabytedb-relational-database.md` | Accepted target-platform decision; PostgreSQL references explain the replaced alternative and YSQL compatibility. |
| `e-invoice/E-INVOICE.md` | Product concept architecture; reconcile its PostgreSQL diagram in #129. |
| `ideas/AWS-KUBERNETES-SCALIBILITY.md` | AWS design proposal; replace Aurora/PostgreSQL assumptions in #127/#129. |
| `ideas/deployment-on-aws.md` | AWS deployment proposal; replace Aurora/PostgreSQL assumptions in #127/#129. |
| `ideas/postgresql-to-yugabytedb-migration-plan.md` | Authoritative migration plan; intentional historical and target-state references remain. |
| `ideas/third-party-integration.md` | Future architecture and migration rationale; intentional migration passages remain, while any stale target wording is audited in #129. |
| `ideas/yugabytedb-client-party-master-data-schema.md` | YugabyteDB target design; its `postgres` match is explanatory compatibility text, not a PostgreSQL dependency. |
| `ideas/sitesentinel_micro_saas_idea.md` | Unrelated product idea; PostgreSQL references do not describe e-ubl21 and require no migration. |

## Issue #89 boundary

Issue #89 remains unimplemented. Its current index wording intentionally says
PostgreSQL until MIG-YB-011 / #129 updates the tracked future design after the
YugabyteDB foundation is in place. The later target separation is:

```text
metadata/status  -> YugabyteDB
raw XML          -> S3-compatible object storage
work/results     -> NATS / JetStream
```

MIG-YB-001 does not move JetStream KV metadata, raw XML, or work/results.

## Baseline verification

Both required commands passed in a clean detached worktree at the frozen
baseline commit before infrastructure implementation started:

| Gate | Result | Evidence |
|---|---:|---|
| `go test ./... -count=1` | PASS | Clean worktree at `f5ed46a7dc30a07e11069f93a6d48b9307222b5d`. |
| `./run-regression-tests.sh` | PASS | `regression-results/local/mig-yb-001-baseline-f5ed46a-clean/report.md` |

The regression report records:

- finished at `2026-09-12T19:58:58Z`;
- commit `f5ed46a7dc30a07e11069f93a6d48b9307222b5d`;
- working tree dirty: `no`;
- validation chaos/crash/replay: PASS;
- correctness UBL 2.1 and EN 16931: PASS, with 71/71 and 32/32
  fixtures semantically equivalent respectively;
- Minikube UBL 2.1: PASS, 71/71 equivalent;
- Minikube EN 16931: PASS, 32/32 equivalent.

The report and its logs are generated local evidence and remain under the
repository's ignored `regression-results/` tree rather than being committed.
