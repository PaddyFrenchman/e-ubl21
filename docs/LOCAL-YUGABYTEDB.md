# Local YugabyteDB runtime

Issue #121 provides a reproducible, single-node YugabyteDB/YSQL runtime for
development and database integration tests. It uses the exact supported image
`yugabytedb/yugabyte:2025.2.6.0-b111`; floating tags are not permitted.

This topology is local test infrastructure, not a production deployment. It
has replication factor one, publishes plaintext YSQL on loopback, and uses the
deterministic credentials `eubl21` / `eubl21-local-only`. Do not reuse these
credentials outside this isolated runtime.

The Compose volume shares the container engine's host filesystem. YugabyteDB's
production percentage-based write floor can reject an empty test database on a
large but mostly occupied developer disk, so this isolated node disables that
server-side threshold. The lifecycle wrapper instead checks both the repository
filesystem and the filesystem actually mounted for container-volume data, and
refuses to start if either has less than 2 GiB free. Monitor long-running local
workloads yourself. This deliberately does not define or alter any production
storage policy.

## Prerequisites

- Docker Engine with the official Docker Compose v2 plugin. See
  [`DOCKER-DEVELOPMENT.md`](DOCKER-DEVELOPMENT.md) for setup and verification.
- Enough local resources for a single YugabyteDB node.
- Go 1.25 for the application integration test.

No host installation of PostgreSQL, `ysqlsh`, or YugabyteDB is required. The
repository wrapper runs `ysqlsh` inside the pinned container.

## Common lifecycle

Start YugabyteDB, wait for authenticated YSQL readiness, apply all schema
migrations, verify the pinned build, and run `SELECT 1`:

```bash
make database-up
```

`database-up` safely checks and reapplies the migration runner on every call.
Run `make database-migrate` when an already-running database only needs its
schema advanced. The migration design, history, locking, and recovery policy
are documented in
[`YUGABYTEDB-SCHEMA-MIGRATIONS.md`](YUGABYTEDB-SCHEMA-MIGRATIONS.md).

Run the existing application database integration test against that runtime:

```bash
make database-test
```

Stop and later restart the node without deleting its data:

```bash
make database-stop
make database-up
```

The service has no automatic restart policy. `database-stop` explicitly stops
it through Compose with YugabyteDB's configured grace period, so it remains
stopped until `database-up` or `database-restart` is run. Its named volume,
static Yugabyte node hostname, and hostname-based advertise identity are
retained across that lifecycle even if the engine assigns a different private
IP.

Remove the local container, Compose network, and persistent database volume:

```bash
make database-reset
```

`database-reset` is destructive, but its scope is limited to the
`eubl21-yugabytedb-local` Compose project.

The application URL can be obtained without starting the runtime:

```bash
./devsetup/yugabytedb-local.sh url
```

The YSQL host port defaults to `5433`, and the local YugabyteDB UI is available
at <http://127.0.0.1:15433>. To avoid a host conflict, select different ports:

```bash
YB_LOCAL_YSQL_PORT=15433 YB_LOCAL_UI_PORT=25433 make database-up
```

Use the same `YB_LOCAL_YSQL_PORT` value for subsequent wrapper commands. The
readiness deadline defaults to 120 seconds and can be changed with the positive
integer `YB_LOCAL_READY_TIMEOUT` setting.

## Lifecycle acceptance test

The following target is intentionally destructive to the local Compose project:

```bash
make database-verify
```

It automates the issue acceptance scenarios in order:

1. removes any prior local state and starts cleanly;
2. proves readiness fails within a bounded deadline while the node is stopped;
3. creates a marker table, restarts the node, and verifies the data survived;
4. resets the persistent volume and verifies the marker was removed;
5. runs the connectivity and schema-migration integration gates through the
   application database package;
6. removes the disposable runtime on exit, including after a failed check.

The local runtime deliberately uses `sslmode=disable`. The application keeps
its secure `sslmode=require` default; plaintext is explicitly selected only in
the URL produced for this isolated test topology. TLS qualification remains
available through `EUBL21_TEST_DATABASE_TLS_URL` as documented in
[`YUGABYTEDB-CONNECTIVITY.md`](YUGABYTEDB-CONNECTIVITY.md).
