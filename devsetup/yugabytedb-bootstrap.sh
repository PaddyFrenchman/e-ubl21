#!/usr/bin/env bash
set -Eeuo pipefail

admin_user="yugabyte"
admin_password="yugabyte"
admin_database="yugabyte"
app_user="${YSQL_USER:?YSQL_USER is required}"
app_password="${YSQL_PASSWORD:?YSQL_PASSWORD is required}"
app_database="${YSQL_DB:?YSQL_DB is required}"
host="${YSQL_HOST:?YSQL_HOST is required}"

export PGPASSWORD="$admin_password"

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
