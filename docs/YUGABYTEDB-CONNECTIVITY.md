# YugabyteDB/YSQL application connectivity

MIG-YB-002 / issue #120 introduces the application's relational database
configuration and connection-pool boundary. It is intentionally a foundation:
no serve, worker, orchestration, upload, or validation path requires a database
yet, and JetStream remains the current system of record.

## Application configuration

The application-facing settings are vendor-neutral:

| Environment variable | Flag | Default | Purpose |
|---|---|---:|---|
| `EUBL21_DATABASE_URL` | `--database-url` | unset | `postgres` or `postgresql` URL for the YugabyteDB YSQL endpoint |
| `EUBL21_DATABASE_CONNECT_TIMEOUT` | `--database-connect-timeout` | `5s` | connection and startup-probe deadline |
| `EUBL21_DATABASE_QUERY_TIMEOUT` | `--database-query-timeout` | `5s` | query, statement, and pool-acquisition deadline |
| `EUBL21_DATABASE_MAX_CONNECTIONS` | `--database-max-connections` | `10` | maximum pool size |
| `EUBL21_DATABASE_MIN_CONNECTIONS` | `--database-min-connections` | `0` | minimum pool size |

When a URL is configured, it must contain a user and host. Missing connection
properties receive these defaults:

- port `5433`, YugabyteDB's YSQL port;
- database `eubl21`;
- `sslmode=require`.

For example:

```text
EUBL21_DATABASE_URL=postgresql://eubl21:REDACTED@yb.example:5433/eubl21?sslmode=verify-full
```

The standard pgx/libpq SSL modes are accepted: `disable`, `allow`, `prefer`,
`require`, `verify-ca`, and `verify-full`. Production endpoints should use
certificate verification (`verify-full` where hostname verification is
possible). Plaintext local test instances must opt in explicitly with
`sslmode=disable`; the secure default is not weakened for local convenience.

Do not introduce application settings named `POSTGRES_*` or `YUGABYTE_*` for
ordinary connection properties. Later deployment tooling may use `YB_*` for
YugabyteDB cluster-operational settings.

## Go package contract

The [`database`](../database) package uses pgx v5 and provides:

- typed URL and pool-setting parsing with bounded-value validation;
- `Open` for lazy pool creation and `Connect` for pool creation plus an
  immediate startup probe;
- `Probe`, `ScanRow`, `Exec`, and bounded pool `Acquire` helpers;
- idempotent, graceful pool shutdown through `Client.Close`;
- structured configuration, connectivity, query, and pool error kinds.

The normalized connection string remains private and the configuration's
string representation omits all user information and passwords. Returned errors contain a stable
operation and kind but do not render the underlying driver error, preventing
connection credentials from entering normal logs. Deadline checks remain
available through `errors.Is(err, context.DeadlineExceeded)`.

## YugabyteDB integration test

The integration test is opt-in until #121 supplies the reproducible repository
runtime. Point it only at a real YugabyteDB instance with YSQL authentication
enabled:

```bash
EUBL21_TEST_DATABASE_URL='postgresql://eubl21:REDACTED@127.0.0.1:5433/eubl21?sslmode=disable' \
  go test ./database -run TestYugabyteDBIntegration -count=1
```

The test rejects a PostgreSQL substitute by checking `version()` for a
YugabyteDB/YB build marker. It covers a successful startup probe, `SELECT 1`,
bounded pool exhaustion, wrong credentials, and an unavailable endpoint. A
plaintext topology also proves that `sslmode=require` is enforced and cannot
silently downgrade. For a TLS-enabled topology, provide a
certificate-verifying URL to exercise successful verification:

```bash
EUBL21_TEST_DATABASE_TLS_URL='postgresql://eubl21:REDACTED@yb.example:5433/eubl21?sslmode=verify-full&sslrootcert=/path/to/ca.crt'
```

The initial qualification target is the accepted YugabyteDB 2025.2 LTS series.
Issue #121 will pin and automate the exact container/runtime build; this issue
does not pre-empt that lifecycle work.
