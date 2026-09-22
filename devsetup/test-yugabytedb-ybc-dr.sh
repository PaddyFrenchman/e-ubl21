#!/usr/bin/env bash
# Disposable development YBC data-plane qualification.  The backup bind mount
# is host-owned, so it survives deleting and recreating the restore universe.
set -Eeuo pipefail

root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$root/devsetup/lib/docker-engine.sh"
require_docker_compose
compose=(docker compose --project-name eubl21-ybc-dr --file "$root/deploy/local/yugabytedb-dr-compose.yml")
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-ybc-dr/$run_id}"
backup_dir="$(mktemp -d)"
# This is an isolated, disposable development credential. Keep it out of the
# evidence directory and use it for both independently created universes.
generated_ysql_password="ybdr-${run_id}-$(< /proc/sys/kernel/random/uuid)"
ysql_password="yugabyte"
# The image runs YBC as the unprivileged yugabyte user. This is a disposable
# test-only NFS-style share, not a directory containing credentials.
chmod 0777 "$backup_dir"
export EUBL21_YBC_DR_BACKUP_DIR="$backup_dir"
mkdir -p "$artifact_dir"

retain_evidence() {
  mkdir -p "$artifact_dir"
  "${compose[@]}" ps -a >"$artifact_dir/containers.txt" 2>&1 || true
  "${compose[@]}" logs >"$artifact_dir/containers.log" 2>&1 || true
  find "$backup_dir" -maxdepth 4 -type f -printf '%P\n' | sort >"$artifact_dir/backup-files.txt" 2>/dev/null || true
}
cleanup() {
  local status=$? host_uid host_gid
  host_uid="$(id -u)"
  host_gid="$(id -g)"
  retain_evidence
  # Controller writes the mounted NFS hierarchy as the container user. Return
  # it to the invoking user before removing this disposable host directory.
  "${compose[@]}" exec -T source chown -R "$host_uid:$host_gid" /tmp/nfs/nfs >/dev/null 2>&1 || true
  "${compose[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  if [[ "${EUBL21_YBC_DR_KEEP_BACKUP:-0}" != 1 ]]; then
    rm -rf "$backup_dir"
  else
    echo "Retained YBC backup mount for diagnosis: $backup_dir" >&2
  fi
  return "$status"
}
trap cleanup EXIT

ysql() {
  local service=$1; shift
  "${compose[@]}" exec -T -e "PGPASSWORD=$ysql_password" "$service" /home/yugabyte/bin/ysqlsh \
    --host "eubl21-dr-$service" --port 5433 --username yugabyte --dbname yugabyte \
    --no-psqlrc --set ON_ERROR_STOP=1 "$@"
}
ysql_database() {
  local service=$1 database=$2; shift 2
  "${compose[@]}" exec -T -e "PGPASSWORD=$ysql_password" "$service" /home/yugabyte/bin/ysqlsh \
    --host "eubl21-dr-$service" --port 5433 --username yugabyte --dbname "$database" \
    --no-psqlrc --set ON_ERROR_STOP=1 "$@"
}
set_yugabyte_password() {
  local service=$1 current_password=$2
  "${compose[@]}" exec -T -e "PGPASSWORD=$current_password" "$service" /home/yugabyte/bin/ysqlsh \
    --host "eubl21-dr-$service" --port 5433 --username yugabyte --dbname yugabyte \
    --no-psqlrc --set ON_ERROR_STOP=1 \
    --command "ALTER ROLE yugabyte PASSWORD '$generated_ysql_password';"
}
wait_ready() {
  local service=$1 deadline=$((SECONDS + 240))
  until ysql "$service" --tuples-only --no-align --command 'SELECT 1' 2>/dev/null | grep -Fxq 1; do
    (( SECONDS < deadline )) || { "${compose[@]}" logs >&2; echo "$service did not become ready" >&2; exit 1; }
    sleep 2
  done
}
wait_database_ready() {
  local service=$1 database=$2 deadline=$((SECONDS + 240))
  until ysql_database "$service" "$database" --tuples-only --no-align --command 'SELECT 1' 2>/dev/null | grep -Fxq 1; do
    (( SECONDS < deadline )) || { "${compose[@]}" logs >&2; echo "$service/$database did not become ready" >&2; exit 1; }
    sleep 2
  done
}
yugabyted() {
  local service=$1; shift
  "${compose[@]}" exec -T "$service" /home/yugabyte/bin/yugabyted "$@"
}
wait_ybc_task() {
  local operation=$1 service=$2 output_file=$3 deadline=$((SECONDS + 900)) status
  while :; do
    # An in-progress controller task has a non-zero CLI status; inspect its
    # reported state rather than letting `set -e` abort the polling loop.
    status="$(yugabyted "$service" "$operation" --base_dir=/tmp/yb --database=eubl21 --cloud_storage_uri=/nfs/eubl21-ybc --status 2>&1 || true)"
    printf '%s\n' "$status" >"$output_file"
    if [[ "$status" == *"${operation^} is complete."* ]]; then
      return 0
    fi
    if [[ "$status" == *"${operation^} has failed."* ]]; then
      printf '%s\n' "$status" >&2
      return 1
    fi
    (( SECONDS < deadline )) || { printf '%s\n' "$status" >&2; echo "$operation did not complete within 15 minutes" >&2; return 1; }
    sleep 2
  done
}

"${compose[@]}" up --detach source
wait_ready source
set_yugabyte_password source yugabyte
ysql_password="$generated_ysql_password"
source_version="$(yugabyted source version)"
ysql source --command 'CREATE DATABASE eubl21;'
wait_database_ready source eubl21
ysql_database source eubl21 --command "CREATE TABLE eubl21_ybc_dr_qualification (id integer PRIMARY KEY, payload text NOT NULL); INSERT INTO eubl21_ybc_dr_qualification VALUES (128, 'eubl21-ybc-dr-qualification');"

# This is a real YBC request, not a logical dump.  Status is queried after it
# completes to retain controller-level confirmation with the evidence.
# yugabyted parses the last two path components as the NFS bucket and backup
# directory, so the mount root alone is invalid for YBC.
yugabyted source backup --base_dir=/tmp/yb --database=eubl21 --cloud_storage_uri=/nfs/eubl21-ybc | tee "$artifact_dir/backup-result.txt"
wait_ybc_task backup source "$artifact_dir/backup-status.txt"
# YBC creates the NFS hierarchy first; it can contain directories, symlinks,
# and data files depending on the YBC release.  In all cases it must be
# material outside either database universe before the target is recreated.
backup_entry="$(find "$backup_dir" -mindepth 1 -print -quit)"
[[ -n "$backup_entry" ]]

# A target container did not exist at backup time. Delete it explicitly, then
# recreate a clean universe before issuing the real controller restore.
"${compose[@]}" rm --stop --force target >/dev/null 2>&1 || true
"${compose[@]}" up --detach target
ysql_password="yugabyte"
wait_ready target
set_yugabyte_password target yugabyte
ysql_password="$generated_ysql_password"
target_version="$(yugabyted target version)"
yugabyted target restore --base_dir=/tmp/yb --database=eubl21 --cloud_storage_uri=/nfs/eubl21-ybc | tee "$artifact_dir/restore-result.txt"
wait_ybc_task restore target "$artifact_dir/restore-status.txt"
ysql_database target eubl21 --tuples-only --no-align --command "SELECT payload FROM eubl21_ybc_dr_qualification WHERE id = 128" | grep -Fxq eubl21-ybc-dr-qualification

port="${EUBL21_DR_LOCAL_TARGET_PORT:-15435}"
EUBL21_TEST_DATABASE_URL="postgresql://yugabyte:$ysql_password@127.0.0.1:$port/eubl21?sslmode=disable" \
  go test ./database -run '^TestYugabyteDBIntegration$' -count=1 | tee "$artifact_dir/application-integration.txt"
REGRESSION_RUN_DIR="$artifact_dir/regression" ./run-regression-tests.sh | tee "$artifact_dir/application-regression.txt"
printf 'source_version=%s\ntarget_version=%s\nbackup_identifier=%s\nbackup_location=%s\nbackup_status=passed\nrestore_status=passed\nschema_data_verification=passed\napplication_integration=passed\napplication_regression=passed\n' \
  "$source_version" "$target_version" "nfs/eubl21-ybc" "$backup_dir" >"$artifact_dir/summary.txt"
echo "Disposable YBC DR qualification passed; evidence retained in $artifact_dir"
