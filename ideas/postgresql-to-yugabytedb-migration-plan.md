# PostgreSQL to YugabyteDB migration plan

## Purpose

This document defines a concrete, issue-by-issue plan for replacing the product's existing PostgreSQL support and PostgreSQL-oriented deployment assumptions with YugabyteDB.

GitHub issues have now been created for every numbered implementation step. Each step below corresponds to exactly one tracked GitHub issue with its own scope, tests, acceptance criteria and dependencies.

## GitHub issue mapping

| Step | Migration ID | GitHub issue |
| ---: | --- | ---: |
| 1 | `MIG-YB-001` | #119 |
| 2 | `MIG-YB-002` | #120 |
| 3 | `MIG-YB-003` | #121 |
| 4 | `MIG-YB-004` | #122 |
| 5 | `MIG-YB-005` | #123 |
| 6 | `MIG-YB-006` | #124 |
| 7 | `MIG-YB-007` | #125 |
| 8 | `MIG-YB-008` | #126 |
| 9 | `MIG-YB-009` | #127 |
| 10 | `MIG-YB-010` | #128 |
| 11 | `MIG-YB-011` | #129 |
| 12 | `MIG-YB-012` | #130 |

The plan is deliberately based on the product **as it exists now**, not on the future third-party-integration architecture alone.

## Current state of the product

Repository review on the `development` branch shows that PostgreSQL is currently present mainly as deployment and architectural scaffolding rather than as a deeply embedded application persistence dependency.

The current on-premises bootstrap explicitly installs:

- CloudNativePG;
- one PostgreSQL instance;
- an `eubl21` database;
- the `eubl21-db-auth` Kubernetes Secret;
- `POSTGRES_STORAGE_SIZE` configuration;
- an `e-ubl21-postgres` cluster resource.

PostgreSQL is also referenced in:

- `deploy/on-prem/README.md`;
- `deploy/on-prem/deployment-on-prem.md`;
- `deploy/on-prem/licensing.md`;
- `deployment-target.md`;
- `README.md`;
- `.github/roadmap-0.md`;
- `docs/SERVER-FUNCTIONALITY-AND-INVOICE-PLATFORM-ARCHITECTURE.md`;
- `ideas/deployment-on-aws.md`;
- `ideas/AWS-KUBERNETES-SCALIBILITY.md`;
- issue #89, which currently proposes moving upload metadata/status from JetStream KV to PostgreSQL.

Repository searches did not find a current Go persistence layer based on `database/sql` or `pgx`, nor a checked-in relational schema/migration set. This means the migration can be performed before substantial PostgreSQL-specific application code exists.

The product's current runtime state remains:

```text
upload metadata/status  -> JetStream KV
raw XML                 -> JetStream Object Store
work/results            -> NATS / JetStream
```

Therefore this migration has **two distinct concerns**:

1. replace the PostgreSQL deployment/platform commitment that already exists;
2. ensure all future relational persistence work is implemented directly against YugabyteDB/YSQL rather than first being built on PostgreSQL and migrated later.

This plan does **not** move JetStream KV metadata to YugabyteDB yet. That remains the responsibility of the future metadata-persistence work represented by issue #89, which must be retargeted from PostgreSQL to YugabyteDB after this migration foundation is complete.

## Target state

The intended database architecture is:

```text
                     e-ubl21
                        |
              PostgreSQL/YSQL protocol
                        |
                        v
                  YugabyteDB
                        |
            authoritative relational data
```

YugabyteDB becomes the only supported relational database technology for the product.

The application should retain PostgreSQL-compatible development practices where they are portable:

- SQL;
- YSQL/PostgreSQL wire protocol;
- `pgx` or another compatible Go driver;
- standard transactions;
- ordinary relational schema design;
- versioned database migrations.

However, all integration tests must execute against YugabyteDB itself. PostgreSQL must not be used as a local substitute for YugabyteDB in CI or developer persistence tests.

Valkey is part of the planned master-data/cache architecture but is **not required to complete this migration**. It should be introduced in its own implementation plan/issue sequence so that database replacement and cache introduction do not become one oversized change.

---

# Migration rules

Every issue created from this plan should obey the following rules.

## Repository workflow inherited from `ISSUES.MD`

This migration plan does **not** define a separate issue, branch, review, integration or closure workflow. Every `MIG-YB-*` issue created from this plan must follow the current **Branch and issue workflow** in `ISSUES.MD` exactly. If wording in this plan ever conflicts with `ISSUES.MD`, then `ISSUES.MD` is authoritative and this plan must be updated to match it.

The migration workflow therefore has the same non-negotiable rules as the rest of the repository:

1. When a migration issue is created, closed, materially re-scoped or re-ordered, update `ISSUES.MD` in the same workflow so the repository issue index remains synchronized with GitHub.
2. Start issue work from a synchronized **local `development` branch** that tracks `origin/development`. Do not create issue branches from `main`, a release branch, or a detached `origin/development` ref.
3. Implement each migration issue on its own `issue/<number>-<description>` branch, where `<number>` is the actual GitHub issue number assigned when the issue is created.
4. Commit and push the issue branch, then create or update a pull request targeting `development`.
5. **ChatGPT is the designated PR/code reviewer.** Codex in VS Code is the implementation and fix agent. Review findings are passed to Codex for correction, then ChatGPT re-reviews the updated PR. Repeat until no unresolved blocking findings remain.
6. Creating or reviewing a PR does not authorize integration or issue closure. The issue remains open throughout the review cycle.
7. Do not rebase merely because a review iteration completed, do not merge `development` into the issue branch, and do not integrate the issue branch into `development` during ordinary review work.
8. While the repository's temporary local-regression policy is active, `./run-regression-tests.sh` is the authoritative regression gate wherever regression verification applies. Run it against the exact candidate commit, and rerun it after a rebase or other substantive candidate-changing modification.
9. Neither ChatGPT nor Codex may close or integrate a migration issue unless the user explicitly instructs it to **close that specific issue**. A clean review, passing tests, a pushed branch or an open PR is not closure authorization.
10. Once explicit closure authorization is given, use the `ISSUES.MD` **rebase-and-squash** integration workflow: rebase the issue branch onto current `origin/development`, resolve conflicts on the issue branch, rerun required verification on the exact rebased candidate, push rewritten commits with `--force-with-lease` when required, re-check the PR, then integrate with GitHub **Squash and merge** so the completed issue becomes exactly one commit on `development`. After the squash merge, verify that the resulting squash commit tree matches the reviewed/tested issue-branch candidate tree, complete issue closure, and remove the issue branch only after that equivalence is confirmed.
11. Normal issue integration must not use a merge commit, GitHub's rebase-merge method, or direct integration into `main` or `release/vX.Y.Z`. **Squash and merge is the required PR integration method**, preserving the PR and issue-branch commits as the detailed audit trail while keeping one canonical commit per issue on `development`.

The detailed commands, review-loop rules, temporary CI override, authorization boundary and release workflow remain defined centrally in `ISSUES.MD`. This file specifies migration work; `ISSUES.MD` specifies how that work moves through the repository.

## Preserve product behavior

Replacing PostgreSQL deployment support must not alter invoice validation semantics, API responses, orchestration behavior or regression-corpus results.

## Test the exact implementation candidate

Before any migration issue is closed, the exact candidate commit must pass the tests required by that issue. At minimum, unless the issue explicitly cannot affect Go code:

```bash
go test ./... -count=1
```

and, while the repository's temporary local-regression policy is active, the authoritative local regression gate:

```bash
./run-regression-tests.sh
```

must remain green whenever regression verification applies. A rebase or other substantive change that changes the candidate commit requires the applicable verification to be rerun before integration, exactly as defined in `ISSUES.MD`.

## YugabyteDB is the test database

Any persistence integration test added during this migration must use a real YugabyteDB instance. A PostgreSQL compatibility test may be useful as optional diagnostic evidence, but it must never be the acceptance gate.

## No silent compatibility assumptions

PostgreSQL compatibility does not mean identical PostgreSQL behavior. Schema features, SQL constructs, transaction semantics, indexes, extensions and operational tooling used by the product must be proven against the pinned YugabyteDB version.

## Changes must be reversible during development

Until the final cutover issue is complete, changes should be staged so that a failed migration step can be reverted without damaging existing installations.

---

# Issue sequence

## Step 1 — Record the YugabyteDB architectural decision and migration baseline

Implementation assets:

- [`docs/architecture-decisions/0001-yugabytedb-relational-database.md`](../docs/architecture-decisions/0001-yugabytedb-relational-database.md)
- [`docs/YUGABYTEDB-MIGRATION-BASELINE.md`](../docs/YUGABYTEDB-MIGRATION-BASELINE.md)

### Proposed issue title

`MIG-YB-001 — Adopt YugabyteDB as the product relational database and freeze migration baseline`

### GitHub issue

`#119`

### Goal

Create the formal architectural baseline before implementation begins.

### Work

- Add an ADR selecting YugabyteDB as the product's relational database.
- Record that PostgreSQL/CloudNativePG is being retired as an e-ubl21 deployment dependency.
- Define supported modes:
  - developer/small on-prem: single-node YugabyteDB;
  - production on-prem HA: multi-node YugabyteDB;
  - cloud/SaaS: multi-node and eventually multi-region YugabyteDB.
- Pin the first supported YugabyteDB version or version range.
- Inventory every PostgreSQL/CloudNativePG reference in code, deployment assets, workflows and documentation.
- Record current environment variables, resource names and secrets that will require migration.
- Record that issue #89 is not implemented in this migration but must later target YugabyteDB instead of PostgreSQL.
- Capture the current exact regression baseline before infrastructure changes begin.

### Tests/evidence

- `go test ./... -count=1`.
- `./run-regression-tests.sh`.
- Save the regression report for the exact baseline commit.
- Verify that the PostgreSQL inventory is complete with repository searches for at least:
  - `PostgreSQL`;
  - `postgres`;
  - `CloudNativePG`;
  - `CLOUDNATIVEPG`;
  - `POSTGRES_`;
  - `e-ubl21-postgres`.

### Acceptance

- ADR exists and clearly selects YugabyteDB.
- Supported deployment modes are documented.
- Pinned YugabyteDB version policy exists.
- PostgreSQL/CloudNativePG inventory is complete.
- Baseline unit/regression evidence is retained.

### Dependencies

None.

---

## Step 2 — Introduce a generic application database configuration contract

### Proposed issue title

`MIG-YB-002 — Introduce YugabyteDB/YSQL application database configuration and connectivity`

### GitHub issue

`#120`

### Goal

Make application-level database configuration vendor-neutral in naming but explicitly tested against YugabyteDB.

### Work

Introduce a small database package/configuration contract without yet moving any existing business state into the database.

Preferred application configuration:

```text
EUBL21_DATABASE_URL
```

or equivalent discrete settings such as:

```text
EUBL21_DB_HOST
EUBL21_DB_PORT
EUBL21_DB_NAME
EUBL21_DB_USER
EUBL21_DB_PASSWORD
EUBL21_DB_SSLMODE
```

Avoid new application-facing names such as `POSTGRES_*` or `YUGABYTE_*` where the setting is simply a SQL connection property. YugabyteDB-cluster operational settings may use `YB_*` names in deployment tooling.

Implementation should include:

- chosen Go YSQL/PostgreSQL-compatible driver, preferably `pgx` unless a better project-specific reason exists;
- connection pool creation;
- explicit connect/query timeouts;
- TLS configuration support;
- startup connectivity probe/helper;
- graceful connection-pool shutdown;
- typed configuration validation;
- structured errors that do not leak credentials.

No production business path should depend on the database yet.

### Unit tests

Add unit tests for:

- connection-string/config parsing;
- missing mandatory settings;
- default port/database behavior;
- TLS/SSL mode parsing;
- password redaction from logs/errors;
- timeout validation;
- invalid URLs and malformed configuration.

### Integration tests

Against a real YugabyteDB instance:

- connect successfully;
- `SELECT 1` succeeds;
- pool exhaustion/timeouts behave predictably;
- wrong credentials fail cleanly;
- unavailable host fails within bounded time;
- TLS configuration is exercised where supported by the selected test topology.

### Regression

- `go test ./... -count=1`.
- Existing `./run-regression-tests.sh` remains unchanged and green.

### Acceptance

- application database package exists;
- it connects to YugabyteDB through YSQL;
- unit and integration tests pass against the pinned YugabyteDB version;
- no business feature requires the database yet;
- no new PostgreSQL-specific configuration names are introduced.

### Dependencies

Step 1.

### Implementation

Implemented by the vendor-neutral [`database`](../database) package, root
`EUBL21_DATABASE_*` configuration, and the
[`YugabyteDB/YSQL application connectivity`](../docs/YUGABYTEDB-CONNECTIVITY.md)
contract. The integration test requires a real YugabyteDB target and rejects a
PostgreSQL substitute. No production business path uses the pool at this stage.

---

## Step 3 — Add a reproducible local YugabyteDB developer/test runtime

### Proposed issue title

`MIG-YB-003 — Add reproducible local YugabyteDB runtime for development and tests`

### GitHub issue

`#121`

### Goal

Make it trivial for developers and tests to run the exact supported YugabyteDB version.

### Work

Provide one-command local lifecycle support using the deployment mechanism best aligned with the current project, for example container-based local runtime and/or Kubernetes manifests.

Required capabilities:

```text
start YugabyteDB
wait until YSQL is ready
create/bootstrap the eubl21 database/user
run smoke query
stop
clean persistent state
```

Pin the image/version. Do not use an unpinned `latest` image.

Provide deterministic credentials for isolated local tests only, or generate credentials and expose them to the calling script in a controlled way.

### Tests

Automate:

- clean start from no data;
- readiness timeout/failure handling;
- restart with persistent volume and data preserved;
- clean reset removes test state;
- YSQL endpoint is reachable by the application database package from Step 2.

### Regression

- `go test ./... -count=1`.
- `./run-regression-tests.sh` remains green.

### Acceptance

A new contributor can clone the repository and start the pinned YugabyteDB test environment with a documented command, run the DB integration tests, and tear it down without manually installing PostgreSQL or YugabyteDB on the host.

### Dependencies

Steps 1-2.

### Implementation

Implemented by the pinned single-node Compose topology in
[`deploy/local/yugabytedb-compose.yml`](../deploy/local/yugabytedb-compose.yml),
the [`devsetup/yugabytedb-local.sh`](../devsetup/yugabytedb-local.sh) lifecycle
wrapper, Make targets, and the
[`local YugabyteDB runtime`](../docs/LOCAL-YUGABYTEDB.md) guide. The destructive
`make database-verify` target covers clean start, bounded readiness failure,
persistent restart, clean reset, smoke/version checks, and connectivity through
the application database package.

---

## Step 4 — Add versioned schema migration infrastructure validated on YugabyteDB

### Proposed issue title

`MIG-YB-004 — Establish YugabyteDB schema migration framework and compatibility gates`

### GitHub issue

`#122`

### Goal

Introduce database schema evolution before business tables are created.

### Work

Select and document a migration mechanism compatible with YugabyteDB/YSQL.

Requirements:

- ordered versioned migrations;
- migration history table;
- deterministic empty-database bootstrap;
- idempotent migration execution;
- protection against concurrent migration races;
- explicit failure behavior;
- documented forward-fix/rollback policy;
- no dependence on unsupported PostgreSQL extensions;
- migrations executed against YugabyteDB in tests.

Because the current product does not yet appear to have relational business tables, the first migration may contain only the minimum bootstrap/schema metadata necessary to prove the mechanism.

### Unit tests

Test migration runner configuration and version ordering without requiring a database where practical.
### Integration tests

Against fresh YugabyteDB:

1. create empty database;
2. run migrations;
3. verify expected schema/version;
4. run migrations again and prove no unintended change;
5. simulate one failed migration and prove failure is reported without silently marking it complete;
6. start from an older migration state and upgrade to current.

### Regression

- `go test ./... -count=1`.
- full local regression unchanged.

### Acceptance

- migrations are a first-class repository asset;
- a blank YugabyteDB database can be deterministically brought to current schema;
- repeated execution is safe;
- migration integration tests run against YugabyteDB.

### Dependencies

Steps 2-3.

Implemented by the contiguous embedded SQL assets in
[`database/migrations`](../database/migrations), the checksummed advisory-lock
migration runner and real-YugabyteDB compatibility tests in
[`database`](../database), the `database migrate` command, the automatic local
runtime migration step, and the documented forward-only recovery policy in
[`YUGABYTEDB-SCHEMA-MIGRATIONS.md`](../docs/YUGABYTEDB-SCHEMA-MIGRATIONS.md).

---

## Step 5 — Replace CloudNativePG/PostgreSQL in the on-prem installer

### Proposed issue title

`MIG-YB-005 — Replace CloudNativePG/PostgreSQL with YugabyteDB in on-prem installation`

### GitHub issue

`#123`

### Goal

Make new on-prem installations deploy YugabyteDB instead of CloudNativePG/PostgreSQL.

### Current items to replace

The current installer contains PostgreSQL-specific elements including:

```text
CLOUDNATIVEPG_CHART_VERSION
POSTGRES_STORAGE_SIZE
install_cloudnativepg
install_postgresql_cluster
e-ubl21-postgres
```

### Work

- Remove CloudNativePG chart/repository installation from the e-ubl21 bootstrap path.
- Add YugabyteDB installation for the supported single-server baseline.
- Pin YugabyteDB chart/image/version.
- Replace `POSTGRES_STORAGE_SIZE` with an appropriate YugabyteDB storage setting.
- Preserve or safely migrate the existing generic `eubl21-db-auth` secret contract where possible.
- Create the `eubl21` database/user using YugabyteDB/YSQL.
- Add readiness checks that fail installation if YSQL is not usable.
- Run schema migrations after readiness when the migration framework from Step 4 exists.
- Keep K3s, MetalLB, cert-manager, KEDA, NATS, object storage and observability behavior unchanged unless YugabyteDB integration strictly requires a change.

### Installer tests

At minimum:

- shell syntax validation;
- static validation/rendering of generated Kubernetes/Helm resources;
- clean install on supported Linux baseline in a disposable environment where feasible;
- second execution is idempotent;
- YugabyteDB is ready after installation;
- expected database/user exists;
- CloudNativePG operator and `e-ubl21-postgres` are not created on a fresh install.

### Regression

After installation, run API health checks and the applicable validation regression suites against the deployed product.

### Acceptance

A clean supported on-prem installation contains YugabyteDB and no CloudNativePG/PostgreSQL cluster created by e-ubl21.

### Dependencies

Steps 1, 3 and 4.

Implementation in progress through the checksum-qualified official YugabyteDB
chart, single-server values, preserved generic database secret, bounded YSQL
bootstrap/readiness and migration Jobs in the on-prem installer, plus static and
disposable two-pass installer qualification in
[`test-on-prem-yugabytedb.sh`](../devsetup/test-on-prem-yugabytedb.sh).

---

## Step 6 — Add a safe upgrade path for existing PostgreSQL-oriented installations

### Proposed issue title

`MIG-YB-006 — Add safe on-prem upgrade path from PostgreSQL/CloudNativePG deployment to YugabyteDB`

### GitHub issue

`#124`

### Goal

Avoid turning the new installer into a destructive upgrade for anyone who already ran the old bootstrap.

### Important current-state observation

The repository currently does not appear to use the installed PostgreSQL instance as an active application system of record. Nevertheless, the migration must not assume deployed databases are empty.

### Work

When an existing installation contains `e-ubl21-postgres` or CloudNativePG resources:

1. detect the old deployment;
2. inspect whether the `eubl21` database contains user/application tables or data;
3. never delete unknown/non-empty data automatically;
4. if no business data exists, allow controlled retirement after YugabyteDB is healthy;
5. if data exists, stop and require an explicit export/import path;
6. provide a documented/manual or scripted PostgreSQL-to-YugabyteDB data transfer procedure for recognized schemas if/when such schemas exist;
7. preserve a rollback window until the YugabyteDB deployment is verified;
8. only remove CloudNativePG resources after successful verification and explicit operator intent.

### Tests

Test at least these scenarios:

- old PostgreSQL deployment exists but contains no application tables;
- old PostgreSQL contains unexpected table/data and migration refuses destructive removal;
- YugabyteDB install fails and PostgreSQL remains untouched;
- successful YugabyteDB cutover followed by explicit PostgreSQL retirement;
- re-running the migration script is safe.

### Acceptance

No supported upgrade path can silently destroy an existing PostgreSQL database.

### Dependencies

Step 5.

---

## Step 7 — Add YugabyteDB to the Minikube/Kubernetes regression topology

### Proposed issue title

`MIG-YB-007 — Integrate YugabyteDB into Minikube regression topology`

### GitHub issue

`#125`

### Goal

Ensure local Kubernetes regression uses the same database technology as deployment.

### Current state

The Minikube topology currently contains e-ubl21 application resources and NATS manifests but no YugabyteDB manifest.

### Work

- Add a pinned YugabyteDB resource/chart configuration suitable for regression.
- Add database Secret/config injection.
- Wait for YugabyteDB/YSQL readiness before launching DB-dependent components/tests.
- Apply schema migrations.
- Add cleanup behavior for YugabyteDB resources/PVCs.
- Ensure repeated local regression can reuse or clean the Minikube profile deterministically.
- Keep runtime resource usage reasonable for developer machines.

### Regression-runner integration

Extend `run-regression-tests.sh` so the report records at least:

- YugabyteDB version;
- DB integration-suite result;
- migration/bootstrap result;
- Minikube DB readiness failures in failure log tails.

Do not remove the existing suites:

- validation chaos/crash/replay;
- correctness UBL + EN 16931;
- Minikube UBL;
- Minikube EN 16931.

### Tests

- DB smoke test inside Minikube;
- migrations execute successfully;
- DB pod restart does not corrupt schema;
- application regression still produces equivalent UBL/EN results;
- clean topology rebuild succeeds from zero state.

### Acceptance

`./run-regression-tests.sh` exercises a YugabyteDB-backed Kubernetes topology and remains a single comprehensive local regression entry point.

### Dependencies

Steps 3-5.

---

## Step 8 — Add YugabyteDB CI integration and compatibility gates

### Proposed issue title

`MIG-YB-008 — Add YugabyteDB unit/integration/regression CI gates`

### GitHub issue

`#126`

### Goal

Prevent future code from accidentally becoming PostgreSQL-only or Yugabyte-incompatible.

### Work

Add or extend CI workflows to include:

- ordinary Go unit tests;
- DB integration tests against the pinned YugabyteDB version;
- schema migration tests;
- Minikube/Yugabyte regression where hosted resources permit;
- deployment-manifest/Helm rendering checks;
- PostgreSQL-specific dependency/reference guard where appropriate.

### Required CI commands/evidence

At minimum:

```bash
go test ./... -count=1
```

plus the YugabyteDB integration target and the existing correctness/regression targets.

If hosted Minikube regression remains temporarily non-blocking under the project's current local-gate policy, the local exact-head regression evidence must still be required before closing migration issues that affect deployed topology.

### Compatibility guard

Add a lightweight repository guard that fails if new prohibited production references are introduced, for example:

```text
CloudNativePG
e-ubl21-postgres
POSTGRES_STORAGE_SIZE
```

Allow historical migration documentation where explicitly whitelisted.

### Acceptance

A pull request cannot silently introduce a relational persistence change that has never run against YugabyteDB.

### Dependencies

Steps 4 and 7.

---

## Step 9 — Define production YugabyteDB Kubernetes topology and cloud deployment contracts

### Proposed issue title

`MIG-YB-009 — Define production YugabyteDB HA and multi-region deployment architecture`

### GitHub issue

`#127`

### Goal

Replace the PostgreSQL/Aurora/Azure-PostgreSQL assumptions in production architecture with explicit YugabyteDB deployment models.

### Work

Define and document separately:

#### Single-server / developer

```text
1 YugabyteDB node
no HA guarantee
simple storage
```

#### Production on-prem

```text
3+ YugabyteDB nodes
replication factor appropriate to deployment
anti-affinity/failure-domain rules
persistent storage per node
```

#### Cloud/SaaS

```text
multi-node YugabyteDB
availability-zone placement
future multi-region placement
region-aware tenant/data-locality strategy
```

Document:

- CPU/memory requests and limits;
- persistent-volume requirements;
- placement/anti-affinity;
- service endpoints;
- TLS between application and YSQL;
- credential management;
- Kubernetes disruption budgets;
- scaling model;
- failure-domain assumptions;
- database version upgrade strategy.

Update architecture currently referring to:

- Aurora PostgreSQL / RDS;
- Azure Database for PostgreSQL;
- PostgreSQL via CloudNativePG.

### Tests/evidence

- render all Kubernetes/Helm configurations;
- validate no resource-name collisions;
- deploy the reference HA topology in a representative Kubernetes environment where feasible;
- terminate one DB node and prove cluster availability according to the documented HA target;
- application reconnect behavior is tested.

### Acceptance

Every supported deployment class has an explicit YugabyteDB design and there is no production architecture that still requires PostgreSQL.

### Dependencies

Steps 5, 7 and 8.

---

## Step 10 — Add backup, restore, disaster-recovery, security and observability procedures

### Proposed issue title

`MIG-YB-010 — Establish YugabyteDB backup/restore, DR, security and observability`

### GitHub issue

`#128`

### Goal

Reach operational parity with the expectations previously documented for PostgreSQL and make YugabyteDB production-operable.

### Work

Define and implement:

- backup mechanism and schedule;
- off-machine/off-cluster backup destination;
- retention policy;
- restoration to a clean cluster;
- point-in-time/recovery expectations if supported and selected;
- encryption in transit;
- credential rotation;
- least-privilege database users;
- metrics scraping;
- dashboards;
- alerts for availability, replication health, disk pressure, latency and storage growth;
- runbooks for node loss, storage exhaustion and failed migrations.

### Disaster-recovery test

Perform a full drill:

```text
create representative data
      |
      v
backup
      |
      v
destroy/recreate target database
      |
      v
restore
      |
      v
verify schema + data + application access
```

### Regression

Run unit and complete application regression after restored state is brought online.

### Acceptance

A documented and tested restore can reconstruct a usable YugabyteDB-backed e-ubl21 environment from retained backup material.

### Dependencies

Steps 4 and 9.

---

## Step 11 — Remove PostgreSQL/CloudNativePG product references and update issue #89 design

### Proposed issue title

`MIG-YB-011 — Remove stale PostgreSQL architecture and retarget future metadata persistence to YugabyteDB`

### GitHub issue

`#129`

### Goal

Ensure the repository tells one coherent architectural story after the technical migration.

### Work

Update all relevant documentation, scripts and diagrams, including at least:

- `README.md`;
- `.github/roadmap-0.md`;
- `deployment-target.md`;
- `deploy/on-prem/README.md`;
- `deploy/on-prem/deployment-on-prem.md`;
- `deploy/on-prem/licensing.md`;
- `docs/SERVER-FUNCTIONALITY-AND-INVOICE-PLATFORM-ARCHITECTURE.md`;
- `ideas/deployment-on-aws.md`;
- `ideas/AWS-KUBERNETES-SCALIBILITY.md`;
- any other repository references discovered by the inventory.

Update licensing documentation to remove CloudNativePG/PostgreSQL from the e-ubl21 bundled stack where they are no longer shipped and add YugabyteDB's applicable licensing/notice obligations for the selected distribution.

### Issue #89

Do **not** implement the JetStream-KV-to-database migration in this issue.

Instead, update the future architecture represented by #89 so it becomes conceptually:

```text
metadata/status  -> YugabyteDB
raw XML          -> S3-compatible object storage
work/results     -> NATS / JetStream
```

Its later implementation must build on the YugabyteDB connection, schema migration, deployment and test infrastructure created by this plan.

### Guard/test

Repository-wide searches should find no stale production claim that PostgreSQL is the intended e-ubl21 relational database, except historical/migration text that intentionally discusses the replacement.

### Acceptance

The active product documentation consistently identifies YugabyteDB as the relational database.

### Dependencies

Steps 5, 8, 9 and 10.

---

## Step 12 — Final migration qualification and PostgreSQL retirement

### Proposed issue title

`MIG-YB-012 — Qualify YugabyteDB migration and retire PostgreSQL support`

### GitHub issue

`#130`

### Goal

Prove that the migration is complete before declaring PostgreSQL unsupported.

### Required final verification

#### Unit tests

```bash
go test ./... -count=1
```

All tests must pass against the exact candidate commit.

#### Database integration tests

- connection/config tests;
- schema bootstrap;
- migration repeatability;
- restart/persistence;
- authentication failures;
- timeout/failover behavior applicable to the supported topology.

#### Full regression

```bash
./run-regression-tests.sh
```

The complete report must be retained and must show no semantic regression in existing UBL/EN 16931 corpora.

#### Fresh-install test

From a clean supported host/cluster:

1. run the on-prem installer;
2. verify YugabyteDB is installed and ready;
3. verify PostgreSQL/CloudNativePG is absent;
4. verify application health;
5. run representative validation;
6. restart the system and reverify health.

#### Upgrade test

From a deployment created by the last PostgreSQL-oriented installer:

1. execute the supported migration path;
2. prove no old data is silently removed;
3. bring YugabyteDB online;
4. verify application behavior;
5. retire old PostgreSQL only through the documented explicit path.

#### Backup/restore test

Repeat the tested restore drill on the release candidate.

#### Documentation/reference audit

Search for stale PostgreSQL/CloudNativePG production dependencies and confirm remaining matches are intentional historical/migration references.

### Acceptance

The PostgreSQL-to-YugabyteDB migration is complete only when all of the following are true:

- new installations deploy YugabyteDB;
- existing installations have a safe documented upgrade path;
- no e-ubl21 application persistence test uses PostgreSQL as its acceptance database;
- all schema migrations are validated on YugabyteDB;
- Minikube regression includes YugabyteDB;
- CI contains YugabyteDB compatibility gates;
- cloud/on-prem production topology is documented;
- backup/restore has been demonstrated;
- application unit tests pass;
- existing semantic regression tests pass unchanged;
- active deployment documentation no longer requires PostgreSQL or CloudNativePG;
- issue #89 is retargeted to YugabyteDB for future metadata persistence.

### Dependencies

Steps 1-11.

---

# Dependency graph

```text
MIG-YB-001  Architecture decision + baseline
      |
      v
MIG-YB-002  App DB config/connectivity
      |
      +-------------------+
      v                   |
MIG-YB-003  Local YB      |
      |                   |
      v                   |
MIG-YB-004  Migrations <--+
      |
      v
MIG-YB-005  New on-prem installer
      |
      +------------------+
      v                  v
MIG-YB-006  Upgrade    MIG-YB-007  Minikube regression
                         |
                         v
                    MIG-YB-008  CI gates
                         |
                         v
                    MIG-YB-009  Production topology
                         |
                         v
                    MIG-YB-010  Backup/DR/security/obs
                         |
                         v
                    MIG-YB-011  Docs + #89 retarget
                         |
                         v
                    MIG-YB-012  Final qualification
```

Some issues may be developed in parallel after their explicit dependencies are satisfied, but they should not be closed out of order if doing so would leave an untested or unsupported migration state.

---

# Test matrix for the full migration

| Area | Required evidence |
| --- | --- |
| Go unit tests | `go test ./... -count=1` |
| DB config unit tests | parsing, validation, redaction, timeouts, TLS settings |
| YugabyteDB integration | real YSQL connect/query/auth/restart behavior |
| Schema migrations | empty bootstrap, repeat run, upgrade from old version, failure handling |
| Local runtime | start/readiness/restart/clean lifecycle |
| On-prem install | clean K3s install, idempotent rerun, DB readiness |
| Existing-install upgrade | detect old PG, refuse destructive unknown-data removal, explicit retirement |
| Minikube | YugabyteDB deployed/readied/migrated before DB-dependent tests |
| Existing correctness regression | UBL + EN 16931 equivalence remains unchanged |
| Chaos/crash/replay | existing validation resilience suite remains green |
| Hosted CI | Yugabyte integration and migration gates |
| HA | node-loss/reconnect behavior against documented production topology |
| Backup/restore | successful restore and post-restore application verification |
| Documentation | repository-wide PostgreSQL/CloudNativePG audit |

---

# Explicit non-goals of this migration

To keep the issue series focused, the following work should **not** be smuggled into the PostgreSQL-to-YugabyteDB migration unless a specific dependency makes it unavoidable:

- moving upload metadata/status from JetStream KV to YugabyteDB;
- moving raw XML from JetStream Object Store to S3-compatible storage;
- adding third-party accounting adapters;
- creating canonical customer/supplier/product master-data tables;
- implementing Valkey caching;
- implementing NATS cache invalidation;
- multi-region tenant placement policies beyond the deployment foundation required to prove YugabyteDB topology;
- building the semantic-resolution UI.

Those features should consume the database platform established here after the migration is qualified.

---

# Relationship to issue #89

Issue #89 currently describes a future move from JetStream KV to PostgreSQL. That design assumption becomes obsolete once this plan is adopted.

The clean sequencing is:

```text
this migration plan
PostgreSQL support -> YugabyteDB platform
             |
             v
future issue #89 implementation
JetStream KV metadata -> YugabyteDB
```

This avoids performing two migrations at once. First establish and qualify YugabyteDB as infrastructure. Then move business metadata onto it as a separately reviewable functional change.

---

# Recommended execution discipline

Each migration issue must be implemented on its own issue branch and, once the user explicitly authorizes closure of that specific issue, integrated into `development` using the rebase-and-squash workflow defined in `ISSUES.MD`. The issue branch may contain multiple implementation and review-fix commits, but the completed issue must appear on `development` as exactly **one squash commit**. After the squash merge, the resulting commit tree must match the reviewed/tested candidate tree. Ordinary PR review iterations do not authorize integration or closure.

No issue should rely on "the next issue will fix it" for:

- broken installation;
- failing unit tests;
- failing regression tests;
- unsafe upgrade behavior;
- missing readiness checks;
- undocumented credentials/secrets;
- incompatible schema changes.

The migration should advance as a sequence of small, reversible stones across the river rather than one heroic leap with a database strapped to its back.
