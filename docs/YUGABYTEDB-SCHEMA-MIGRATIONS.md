# YugabyteDB schema migrations

Issue #122 establishes a forward-only schema migration mechanism for the YSQL
database before relational business tables are introduced. Migration SQL is a
versioned product asset embedded in the application binary from
`database/migrations/`.

## Running migrations

The normal local start path creates the application database and role, waits
for YSQL, and applies migrations before reporting success:

```bash
make database-up
```

To migrate an already-running local database explicitly:

```bash
make database-migrate
```

For another environment, use the application command with its normal database
configuration and a bounded migration deadline:

```bash
EUBL21_DATABASE_URL='postgresql://USER:REDACTED@HOST:5433/eubl21?sslmode=verify-full' \
  e-ubl21 database migrate --timeout 2m
```

Startup services do not implicitly migrate a deployed database. Deployment
automation must run this command as a dedicated, successful pre-start/pre-rollout
step rather than allowing every application replica to mutate schema at startup.

## Asset and history contract

Migration filenames must match `NNNNNN_name.sql`, begin at `000001`, and remain
contiguous. The runner sorts and validates the complete set before connecting.
The current bootstrap creates the `eubl21` schema and its minimal
`schema_metadata` marker; it creates no business tables.

`public.eubl21_schema_migrations` records each version, name, SHA-256 checksum,
state, and timestamps. On every run, existing rows must be an exact immutable
prefix of the embedded files. A changed checksum/name, version gap, migration
newer than the running binary, or non-`applied` row stops execution.

The runner acquires a session-level YSQL advisory lock on one dedicated pool
connection before creating or reading history and holds it until the run ends.
This serializes concurrent migration jobs without requiring a PostgreSQL
extension. The real-database gate runs two migrators concurrently and proves
that only one applies the pending version.

## Failure and recovery policy

The pinned YugabyteDB build supports session advisory locks, but DDL is not
assumed to be transactional: qualification proves a table created before a
later failing statement can survive `ROLLBACK`. The runner therefore writes an
`applying` history row before executing SQL, changes it to `applied` only after
the entire file succeeds, and records `failed` when execution returns an error.
An interrupted `applying` row and a `failed` row both block later runs with an
operator-repair error. They are never silently retried or marked complete.

Recovery is deliberate:

1. stop rollout and inspect the history row plus all partial schema effects; failed rows retain the SQLSTATE and normalized primary YSQL message, while excluding extended server detail, hints, and SQL text;
2. restore the pre-migration backup when rollback is required; there are no
   automatic down migrations;
3. for an unshipped migration, return the schema to its prior state, delete only
   that reviewed failed history row, correct the SQL, and rerun qualification;
4. for a shipped migration that must be preserved, complete and verify its
   exact intended schema under an audited runbook before marking that version
   applied, then deliver any additional correction as a new forward migration.

Never edit or renumber an applied migration. Normal product changes append a
new migration, and destructive changes require an independently reviewed data
preservation/backup plan.

## Authoring rules

- Use portable YSQL/PostgreSQL syntax supported by the pinned YugabyteDB LTS.
- Do not depend on PostgreSQL extensions or superuser-only session setup.
- Give every object an explicit schema and every constraint an explicit,
  deterministic definition.
- Keep each migration bounded and safe to diagnose after partial DDL.
- Qualify every new sequence from blank, repeated, and prior-version states on
  real YugabyteDB before integration.

## Compatibility gate

The migration integration test requires disposable-database administrator and
application URLs. The repository lifecycle supplies both automatically:

```bash
make database-test
```

The gate creates a separate empty database per scenario and removes it after
testing. It covers fresh bootstrap, repeat execution, failed migration state,
upgrade from the previous schema version, and concurrent runners. The broader
destructive `make database-verify` lifecycle also runs this gate after checking
restart persistence and clean-volume reset behavior.
