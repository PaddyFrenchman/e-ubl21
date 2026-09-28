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
app_password="app-${run_id}-$(< /proc/sys/kernel/random/uuid)"
migration_password="migrator-${run_id}-$(< /proc/sys/kernel/random/uuid)"
app_user="eubl21"
migration_user="eubl21_migrator"
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
ysql_as_database() {
  local service=$1 user=$2 password=$3 database=$4; shift 4
  "${compose[@]}" exec -T -e "PGPASSWORD=$password" "$service" /home/yugabyte/bin/ysqlsh \
    --host "eubl21-dr-$service" --port 5433 --username "$user" --dbname "$database" \
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
"${compose[@]}" cp "$root/devsetup/yugabytedb-bootstrap.sh" source:/tmp/yugabytedb-bootstrap.sh
"${compose[@]}" exec -T -e "YSQL_HOST=eubl21-dr-source" -e "YSQL_ADMIN_PASSWORD=$ysql_password" -e "YSQL_USER=$app_user" -e "YSQL_PASSWORD=$app_password" -e "YSQL_MIGRATION_USER=$migration_user" -e "YSQL_MIGRATION_PASSWORD=$migration_password" -e YSQL_DB=eubl21 -e YSQL_EXPECTED_VERSION=2025.2.6.0-b111 source bash /tmp/yugabytedb-bootstrap.sh
source_port="${EUBL21_DR_LOCAL_SOURCE_PORT:-15434}"
run_migrations() {
  local deadline=$((SECONDS + 60)) migration_output
  until migration_output="$(EUBL21_DATABASE_URL="postgresql://$migration_user:$migration_password@127.0.0.1:$source_port/eubl21?sslmode=disable" go run . database migrate 2>&1)"; do
    if (( SECONDS >= deadline )); then
      printf "%s\n" "$migration_output" >&2
      return 1
    fi
    sleep 2
  done
  printf "%s\n" "$migration_output"
}

run_migrations
# Seed representative application-schema data through the DDL identity, then
# prove the application identity receives only its granted data access.
ysql_as_database source "$migration_user" "$migration_password" eubl21 --command "CREATE TABLE eubl21.ybc_dr_qualification (id integer PRIMARY KEY, payload text NOT NULL); GRANT SELECT ON eubl21.ybc_dr_qualification TO $app_user; INSERT INTO eubl21.ybc_dr_qualification VALUES (128, 'eubl21-ybc-dr-qualification');"

# Keep raw upload content outside the database backup, as in production. The
# restored metadata must reconcile to this independently retained object.
recovery_upload_id='0123456789abcdef0123456789abcdef'
recovery_object_dir="$artifact_dir/recovery-objects"
recovery_object_key="uploads/$recovery_upload_id/document.xml"
mkdir -p "$(dirname "$recovery_object_dir/$recovery_object_key")"
printf 'YSQL upload metadata recovery qualification\n' >"$recovery_object_dir/$recovery_object_key"
recovery_checksum="$(sha256sum "$recovery_object_dir/$recovery_object_key" | awk '{print $1}')"
ysql_as_database source "$migration_user" "$migration_password" eubl21 --command "INSERT INTO eubl21.upload_metadata (queue_namespace, upload_id, revision, document_type, received_at, content_object_key, content_sha256, status, processing_attempts, record) VALUES ('recovery.qualification', '$recovery_upload_id', 1, 'Invoice', '2026-09-28T00:00:00Z', '$recovery_object_key', '$recovery_checksum', 'enqueue_pending', 0, '{\"id\":\"$recovery_upload_id\",\"documentType\":\"Invoice\",\"status\":\"enqueue_pending\",\"receivedAt\":\"2026-09-28T00:00:00Z\",\"contentSha256\":\"$recovery_checksum\"}'::jsonb);"

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
# YBC restores the database, not cluster-global login roles. Recreate the
# least-privilege identities in the fresh target before restoring its grants.
ysql target --command "CREATE ROLE $app_user LOGIN PASSWORD '$app_password'; CREATE ROLE $migration_user LOGIN PASSWORD '$migration_password';"
yugabyted target restore --base_dir=/tmp/yb --database=eubl21 --cloud_storage_uri=/nfs/eubl21-ybc | tee "$artifact_dir/restore-result.txt"
wait_ybc_task restore target "$artifact_dir/restore-status.txt"
ysql_as_database target "$app_user" "$app_password" eubl21 --tuples-only --no-align --command "SELECT payload FROM eubl21.ybc_dr_qualification WHERE id = 128" | grep -Fxq eubl21-ybc-dr-qualification
ysql_as_database target "$app_user" "$app_password" eubl21 --tuples-only --no-align --command "SELECT upload_id || '|' || revision || '|' || status || '|' || content_object_key || '|' || content_sha256 FROM eubl21.upload_metadata WHERE queue_namespace = 'recovery.qualification'" | tee "$artifact_dir/restored-upload-metadata.txt" | grep -Fxq "$recovery_upload_id|1|enqueue_pending|$recovery_object_key|$recovery_checksum"

port="${EUBL21_DR_LOCAL_TARGET_PORT:-15435}"
EUBL21_TEST_DATABASE_URL="postgresql://$app_user:$app_password@127.0.0.1:$port/eubl21?sslmode=disable" \
  go test ./database -run '^TestYugabyteDBIntegration$' -count=1 | tee "$artifact_dir/application-integration.txt"
EUBL21_TEST_DATABASE_URL="postgresql://$app_user:$app_password@127.0.0.1:$port/eubl21?sslmode=disable" \
  EUBL21_RECOVERY_OBJECT_DIR="$recovery_object_dir" \
  go test ./queue -run '^TestYSQLUploadMetadataRecoveryQualification$' -count=1 | tee "$artifact_dir/upload-metadata-recovery.txt"
EUBL21_TEST_DATABASE_URL="postgresql://$app_user:$app_password@127.0.0.1:$port/eubl21?sslmode=disable" \
  go test ./... -count=1 | tee "$artifact_dir/unit-tests.txt"
rotated_app_password="rotated-${run_id}-$(< /proc/sys/kernel/random/uuid)"
ysql_database target eubl21 --command "ALTER ROLE $app_user PASSWORD '$rotated_app_password';"
ysql_as_database target "$app_user" "$rotated_app_password" eubl21 --tuples-only --no-align --command 'SELECT 1' | grep -Fxq 1
! ysql_as_database target "$app_user" "$app_password" eubl21 --tuples-only --no-align --command 'SELECT 1' >/dev/null 2>&1
app_password="$rotated_app_password"
EUBL21_RESTORED_DATABASE_URL="postgresql://$app_user:$app_password@127.0.0.1:$port/eubl21?sslmode=disable" \
  REGRESSION_RUN_DIR="$artifact_dir/regression" ./run-regression-tests.sh | tee "$artifact_dir/application-regression.txt"
printf 'source_version=%s\ntarget_version=%s\nbackup_identifier=%s\nbackup_location=%s\nbackup_status=passed\nrestore_status=passed\nschema_data_verification=passed\nupload_metadata_recovery=passed\napplication_integration=passed\ncredential_rotation=passed\napplication_regression=passed\n' \
  "$source_version" "$target_version" "nfs/eubl21-ybc" "$backup_dir" >"$artifact_dir/summary.txt"
echo "Disposable YBC DR qualification passed; evidence retained in $artifact_dir"
