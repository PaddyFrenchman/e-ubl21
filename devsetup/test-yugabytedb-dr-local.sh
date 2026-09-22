#!/usr/bin/env bash
# Development evidence only. This is not the off-cluster production DR drill.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$root/devsetup/lib/docker-engine.sh"
command -v docker >/dev/null
require_docker_compose
compose=(docker compose --project-name eubl21-yugabytedb-dr --file "$root/deploy/local/yugabytedb-dr-compose.yml")
port="${EUBL21_DR_LOCAL_TARGET_PORT:-15435}"
backup_dir="$(mktemp -d)"
run_id="$(date -u +%y%m%d%H%M%S)-$$"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-dr-local/$run_id}"
cleanup() {
  local status=$?
  if (( status != 0 )); then
    mkdir -p "$artifact_dir"
    "${compose[@]}" ps -a >"$artifact_dir/containers.txt" 2>&1 || true
    "${compose[@]}" logs >"$artifact_dir/containers.log" 2>&1 || true
    echo "Local DR qualification failed; diagnostics retained in $artifact_dir" >&2
  fi
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$backup_dir"
  return "$status"
}
trap cleanup EXIT

"${compose[@]}" up --detach source
ysql() {
  local service="$1"; shift
  "${compose[@]}" exec -T -e PGPASSWORD=yugabyte "$service" /home/yugabyte/bin/ysqlsh \
    --host "eubl21-dr-$service" --port 5433 --username yugabyte --dbname yugabyte \
    --no-psqlrc --set ON_ERROR_STOP=1 "$@"
}
for service in source; do
  deadline=$((SECONDS + 180))
  until ysql "$service" --tuples-only --no-align --command 'SELECT 1' 2>/dev/null | grep -Fxq 1; do
    (( SECONDS < deadline )) || { "${compose[@]}" logs >&2; echo "$service did not become ready" >&2; exit 1; }
    sleep 2
  done
done

ysql source --command "CREATE TABLE eubl21_dr_qualification (id integer PRIMARY KEY, payload text NOT NULL); INSERT INTO eubl21_dr_qualification VALUES (128, 'eubl21-yugabytedb-dr-qualification');"
backup="$backup_dir/eubl21.sql"
"${compose[@]}" exec -T -e PGPASSWORD=yugabyte source /home/yugabyte/postgres/bin/ysql_dump \
  --host eubl21-dr-source --port 5433 --username yugabyte --dbname yugabyte >"$backup"
sha256sum "$backup" >"$backup.sha256"
sha256sum --check --status "$backup.sha256"
# Required DR sequence: the target did not exist while the backup was made.
"${compose[@]}" rm --stop --force target >/dev/null 2>&1 || true
"${compose[@]}" up --detach target
deadline=$((SECONDS + 180))
until ysql target --tuples-only --no-align --command 'SELECT 1' 2>/dev/null | grep -Fxq 1; do
  (( SECONDS < deadline )) || { "${compose[@]}" logs >&2; echo "target did not become ready" >&2; exit 1; }
  sleep 2
done
"${compose[@]}" exec -T -e PGPASSWORD=yugabyte target /home/yugabyte/bin/ysqlsh \
  --host eubl21-dr-target --port 5433 --username yugabyte --dbname yugabyte \
  --no-psqlrc --set ON_ERROR_STOP=1 <"$backup"
ysql target --tuples-only --no-align --command "SELECT payload FROM eubl21_dr_qualification WHERE id = 128" | grep -Fxq eubl21-yugabytedb-dr-qualification
EUBL21_TEST_DATABASE_URL="postgresql://yugabyte:yugabyte@127.0.0.1:$port/yugabyte?sslmode=disable" \
  go test ./database -run '^TestYugabyteDBIntegration$' -count=1
echo "Disposable local YugabyteDB DR development qualification passed."
