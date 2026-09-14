#!/usr/bin/env bash
set -Eeuo pipefail

admin_user="yugabyte"
admin_password="yugabyte"
admin_database="yugabyte"
app_user="${YSQL_USER:?YSQL_USER is required}"
app_password="${YSQL_PASSWORD:?YSQL_PASSWORD is required}"
app_database="${YSQL_DB:?YSQL_DB is required}"
host="${YSQL_HOST:?YSQL_HOST is required}"
expected_version="${YSQL_EXPECTED_VERSION:-}"
ready_timeout_seconds="${YSQL_READY_TIMEOUT_SECONDS:-300}"

export PGPASSWORD="$admin_password"

deadline=$((SECONDS + ready_timeout_seconds))
server_version=""
until server_version="$(/home/yugabyte/bin/ysqlsh \
    --host "$host" \
    --port 5433 \
    --username "$admin_user" \
    --dbname "$admin_database" \
    --no-psqlrc \
    --no-align \
    --tuples-only \
    --command 'SELECT version()' 2>/dev/null)"; do
    if ((SECONDS >= deadline)); then
        echo "YugabyteDB admin YSQL did not become ready within ${ready_timeout_seconds}s" >&2
        exit 1
    fi
    sleep 2
done
[[ "$server_version" == *"YugabyteDB"* || "$server_version" == *"-YB-"* ]] || {
    echo "YSQL endpoint is not YugabyteDB" >&2
    exit 1
}
if [[ -n "$expected_version" ]]; then
    binary_version="$(/home/yugabyte/bin/yugabyted version)"
    [[ "$binary_version" == *"$expected_version"* ]] || {
        echo "YugabyteDB binary version does not match $expected_version" >&2
        exit 1
    }
    expected_release="${expected_version%%-b*}"
    [[ "$server_version" == *"$expected_release"* ]] || {
        echo "YSQL server release does not match $expected_release" >&2
        exit 1
    }
fi

/home/yugabyte/bin/ysqlsh \
    --host "$host" \
    --port 5433 \
    --username "$admin_user" \
    --dbname "$admin_database" \
    --no-psqlrc \
    --set ON_ERROR_STOP=1 \
    --set app_user="$app_user" \
    --set app_password="$app_password" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_user', :'app_password')
WHERE NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = :'app_user')
\gexec
SELECT format('ALTER ROLE %I WITH LOGIN PASSWORD %L', :'app_user', :'app_password')
\gexec
SQL

database_exists="$(/home/yugabyte/bin/ysqlsh \
    --host "$host" \
    --port 5433 \
    --username "$admin_user" \
    --dbname "$admin_database" \
    --no-psqlrc \
    --no-align \
    --tuples-only \
    --set app_database="$app_database" <<'SQL'
SELECT 1 FROM pg_catalog.pg_database WHERE datname = :'app_database';
SQL
)"

if [[ "$database_exists" != "1" ]]; then
    /home/yugabyte/bin/ysqlsh \
        --host "$host" \
        --port 5433 \
        --username "$admin_user" \
        --dbname "$admin_database" \
        --no-psqlrc \
        --set ON_ERROR_STOP=1 \
        --set app_user="$app_user" \
        --set app_database="$app_database" <<'SQL'
SELECT format('CREATE DATABASE %I OWNER %I', :'app_database', :'app_user')
\gexec
SQL
fi

export PGPASSWORD="$app_password"
/home/yugabyte/bin/ysqlsh \
    --host "$host" \
    --port 5433 \
    --username "$app_user" \
    --dbname "$app_database" \
    --no-psqlrc \
    --tuples-only \
    --command 'SELECT 1' >/dev/null
