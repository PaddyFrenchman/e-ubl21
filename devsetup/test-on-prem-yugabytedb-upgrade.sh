#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
upgrade_script="${root_dir}/deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh"
temporary_dir="$(mktemp -d)"
events_file="${temporary_dir}/events"

cleanup() {
  rm -rf "${temporary_dir}"
}
trap cleanup EXIT

export EUBL21_IMAGE=localhost/e-ubl21:upgrade-test
# shellcheck source=deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh
source "${upgrade_script}"

bash -n "${upgrade_script}" "${root_dir}/deploy/on-prem/scripts/install-on-prem.sh"

# Legacy-cluster discovery treats only a successful empty --ignore-not-found
# response as absence. API/RBAC failures are blocking.
if (
  kubectl() { return 1; }
  legacy_cluster_exists
) >/dev/null 2>&1; then
  echo "Legacy cluster discovery treated an API failure as absence/presence" >&2
  exit 1
fi
(
  kubectl() { return 0; }
  ! legacy_cluster_exists
)
(
  kubectl() { printf '%s\n' "cluster.postgresql.cnpg.io/${LEGACY_CLUSTER_NAME}"; }
  legacy_cluster_exists
)

# An empty legacy database qualifies without mutation.
(
  legacy_cluster_exists() { return 0; }
  legacy_psql() { printf '\n'; }
  inspect_legacy_database >/dev/null
)

# Any non-system object blocks cutover, even when the table itself is empty.
(
  legacy_cluster_exists() { return 0; }
  legacy_psql() { printf '%s\n' 'relation|public.customer_invoice'; }
  if inspect_legacy_database >/dev/null 2>&1; then
    echo "Unknown legacy objects unexpectedly qualified" >&2
    exit 1
  else
    [[ "$?" == "2" ]]
  fi
)

# An additional database also blocks retirement of the whole legacy cluster.
(
  legacy_cluster_exists() { return 0; }
  legacy_psql() {
    if [[ "$1" == *"FROM pg_database"* ]]; then
      printf '%s\n' 'database|customer_archive'
    fi
  }
  if inspect_legacy_database >/dev/null 2>&1; then
    echo "Unexpected legacy database qualified for cutover" >&2
    exit 1
  else
    [[ "$?" == "2" ]]
  fi
)

# Upgrade Jobs consume the staging Secret rather than switching the public
# legacy contract before verification.
jobs_file="${temporary_dir}/jobs.yaml"
render_yugabytedb_bootstrap_job >"${jobs_file}"
render_database_migration_job >>"${jobs_file}"
grep -q 'name: eubl21-db-auth-yugabytedb-staging' "${jobs_file}"

# The destructive-retirement check uses a newly created migration Job and the
# verified staging Secret, not historical Job status.
verification_job_file="${temporary_dir}/verification-job.yaml"
(
  run_checked_job() {
    [[ "$1" == eubl21-yugabytedb-retirement-check ]]
    cat >"${verification_job_file}"
  }
  verify_live_yugabytedb_database
)
grep -q 'name: eubl21-yugabytedb-retirement-check' "${verification_job_file}"
grep -q 'name: eubl21-db-auth-yugabytedb-staging' "${verification_job_file}"
if grep -q 'name: eubl21-db-auth"' "${verification_job_file}"; then
  echo "Retirement verification Job used the public Secret" >&2
  exit 1
fi

# Promotion uses the snapshotted resource version to atomically publish only
# the already-verified URL while keeping the legacy credentials unchanged.
secret_file="${temporary_dir}/promoted-secret-command"
(
  kubectl() {
    case "$*" in
      *'.data.username'*) printf '%s' ZXVibDIx ;;
      *'.data.password'*) printf '%s' c2VjcmV0 ;;
      *'.data.database-url'*) printf '%s' cG9zdGdyZXNxbDovL3li ;;
      'patch secret eubl21-db-auth '*) printf '%s\n' "$*" >"${secret_file}" ;;
      *) return 1 ;;
    esac
  }
  promote_staged_database_secret 12345
)
grep -q '^patch secret eubl21-db-auth ' "${secret_file}"
grep -q 'resourceVersion.*12345' "${secret_file}"
grep -q 'database-url.*cG9zdGdyZXNxbDovL3li' "${secret_file}"

# Rollback changes only the promoted URL and handles both an earlier URL and
# the legacy Secret shape where the key did not exist.
rollback_file="${temporary_dir}/rollback-secret-command"
(
  kubectl() { printf '%s\n' "$*" >"${rollback_file}"; }
  rollback_public_database_secret new-url old-url
)
grep -q 'test.*database-url.*new-url' "${rollback_file}"
grep -q 'replace.*database-url.*old-url' "${rollback_file}"
(
  kubectl() { printf '%s\n' "$*" >"${rollback_file}"; }
  rollback_public_database_secret new-url ''
)
grep -q 'test.*database-url.*new-url' "${rollback_file}"
grep -q 'remove.*database-url' "${rollback_file}"

# A successful cutover installs in guarded mode and records verification without
# invoking any legacy deletion.
: >"${events_file}"
(
  validate_platform_config() { :; }
  inspect_legacy_database() { printf '%s\n' inspect >>"${events_file}"; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  prepare_staged_database_secret() { printf '%s\n' stage-secret >>"${events_file}"; }
  install_yugabytedb() { printf 'install:%s\n' "$1" >>"${events_file}"; }
  bootstrap_and_migrate_yugabytedb() { printf '%s\n' migrate >>"${events_file}"; }
  verify_yugabytedb_cutover() { printf '%s\n' verify >>"${events_file}"; }
  kubectl() {
    case "$*" in
      *'{.metadata.resourceVersion}'*) printf '%s' 12345 ;;
      *'{.data.database-url}'*) printf '%s' cG9zdGdyZXNxbDovL3li ;;
      *) printf 'forbidden-kubectl:%s\n' "$*" >>"${events_file}"; return 1 ;;
    esac
  }
  promote_staged_database_secret() { [[ "$1" == 12345 ]]; printf '%s\n' promote-secret >>"${events_file}"; }
  verify_promoted_database_secret() { printf '%s\n' verify-promoted-secret >>"${events_file}"; }
  record_cutover_state() { printf 'state:%s:%s\n' "$1" "$2" >>"${events_file}"; }
  perform_cutover >/dev/null
)
grep -Fxq 'install:guarded-upgrade' "${events_file}"
grep -Fxq promote-secret "${events_file}"
grep -Fxq verify-promoted-secret "${events_file}"
grep -Fxq 'state:verified:legacy-uid' "${events_file}"
if grep -q '^forbidden-kubectl:' "${events_file}"; then
  echo "Cutover attempted to mutate the legacy cluster" >&2
  exit 1
fi

# A failed YugabyteDB install cannot reach migration, state recording, or
# retirement; the source cluster remains the rollback path.
: >"${events_file}"
if (
  validate_platform_config() { :; }
  inspect_legacy_database() { :; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  prepare_staged_database_secret() { printf '%s\n' staged-only >>"${events_file}"; }
  kubectl() {
    case "$*" in
      *'{.metadata.resourceVersion}'*) printf '%s' 12345 ;;
      *'{.data.database-url}'*) printf '%s' legacy-url ;;
      *) return 1 ;;
    esac
  }
  install_yugabytedb() { return 1; }
  bootstrap_and_migrate_yugabytedb() { printf '%s\n' forbidden-migrate >>"${events_file}"; }
  promote_staged_database_secret() { printf '%s\n' forbidden-promote >>"${events_file}"; }
  verify_promoted_database_secret() { printf '%s\n' forbidden-verify-promoted >>"${events_file}"; }
  record_cutover_state() { printf '%s\n' forbidden-state >>"${events_file}"; }
  perform_cutover
) >/dev/null 2>&1; then
  echo "Failed YugabyteDB installation unexpectedly qualified" >&2
  exit 1
fi
grep -Fxq staged-only "${events_file}"
if grep -q '^forbidden-' "${events_file}"; then
  echo "Failed cutover advanced beyond staging" >&2
  exit 1
fi

# A finalization failure after URL promotion restores the previous public URL
# and cannot publish a verified cutover record.
: >"${events_file}"
if (
  validate_platform_config() { :; }
  inspect_legacy_database() { :; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  prepare_staged_database_secret() { :; }
  install_yugabytedb() { :; }
  bootstrap_and_migrate_yugabytedb() { :; }
  verify_yugabytedb_cutover() { :; }
  kubectl() {
    case "$*" in
      *'{.metadata.resourceVersion}'*) printf '%s' 12345 ;;
      *eubl21-db-auth-yugabytedb-staging*'{.data.database-url}'*) printf '%s' new-url ;;
      *eubl21-db-auth*'{.data.database-url}'*) printf '%s' old-url ;;
      *) return 1 ;;
    esac
  }
  promote_staged_database_secret() { printf 'promote:%s\n' "$1" >>"${events_file}"; }
  verify_promoted_database_secret() { :; }
  record_cutover_state() { return 1; }
  rollback_public_database_secret() { printf 'rollback:%s:%s\n' "$1" "$2" >>"${events_file}"; }
  perform_cutover
) >/dev/null 2>&1; then
  echo "Failed cutover finalization unexpectedly qualified" >&2
  exit 1
fi
grep -Fxq 'promote:12345' "${events_file}"
grep -Fxq 'rollback:new-url:old-url' "${events_file}"

# Export produces a non-empty validated archive and refuses overwrite.
backup_file="${temporary_dir}/legacy.dump"
(
  legacy_cluster_exists() { return 0; }
  legacy_primary_pod() { printf '%s\n' legacy-primary; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  kubectl() {
    case "$*" in
      *pg_dump*) printf '%s' test-custom-format-archive ;;
      *pg_restore*) cat >/dev/null ;;
      *) return 1 ;;
    esac
  }
  export_legacy_database "${backup_file}" >/dev/null
)
[[ -s "${backup_file}" ]]
[[ -s "${backup_file}.eubl21-metadata" ]]
grep -Fxq 'legacy_cluster_uid=legacy-uid' "${backup_file}.eubl21-metadata"
(
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  legacy_primary_pod() { printf '%s\n' legacy-primary; }
  kubectl() { cat >/dev/null; }
  verify_backup_archive "${backup_file}"
)
tampered_backup="${temporary_dir}/tampered.dump"
cp "${backup_file}" "${tampered_backup}"
cp "${backup_file}.eubl21-metadata" "${tampered_backup}.eubl21-metadata"
printf '%s' tampered >>"${tampered_backup}"
if (
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  verify_backup_archive "${tampered_backup}"
) >/dev/null 2>&1; then
  echo "Backup checksum mismatch unexpectedly qualified" >&2
  exit 1
fi
if (
  legacy_cluster_exists() { return 0; }
  export_legacy_database "${backup_file}"
) >/dev/null 2>&1; then
  echo "Export overwrote an existing backup" >&2
  exit 1
fi

# Retirement requires an exact confirmation, a matching verified source UID,
# a validated backup, a second empty inspection, and live YugabyteDB checks.
: >"${events_file}"
(
  export EUBL21_RETIRE_LEGACY_POSTGRESQL="${RETIRE_CONFIRMATION}"
  cutover_state_value() {
    case "$1" in
      status) printf '%s\n' verified ;;
      legacy_cluster_uid) printf '%s\n' legacy-uid ;;
      *) printf '\n' ;;
    esac
  }
  legacy_cluster_exists() { return 0; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  verify_backup_archive() { [[ "$1" == "${backup_file}" ]]; }
  inspect_legacy_database() { printf '%s\n' reinspection >>"${events_file}"; }
  verify_yugabytedb_cutover() { printf '%s\n' yb-verified >>"${events_file}"; }
  verify_promoted_database_secret() { printf '%s\n' secret-verified >>"${events_file}"; }
  verify_live_yugabytedb_database() { printf '%s\n' live-ysql-verified >>"${events_file}"; }
  record_cutover_state() { printf 'state:%s:%s:%s\n' "$1" "$2" "$3" >>"${events_file}"; }
  kubectl() {
    if [[ "$1" == "get" && "$2" == "secret" ]]; then
      return 0
    fi
    if [[ "$1" == "delete" && "$2" == "cluster.postgresql.cnpg.io" && "$3" == "${LEGACY_CLUSTER_NAME}" ]]; then
      printf '%s\n' cluster-deleted >>"${events_file}"
      return 0
    fi
    return 1
  }
  retire_legacy_cluster "${backup_file}" >/dev/null
)
grep -Fxq reinspection "${events_file}"
grep -Fxq yb-verified "${events_file}"
grep -Fxq secret-verified "${events_file}"
grep -Fxq live-ysql-verified "${events_file}"
grep -q '^state:retiring:legacy-uid:' "${events_file}"
grep -Fxq cluster-deleted "${events_file}"
grep -q '^state:retired:legacy-uid:' "${events_file}"

# If deletion failed after the retiring record was written, the same source UID
# and backup checksum re-run every gate and retry deletion without rewriting the
# transition record.
: >"${events_file}"
expected_backup_sha256="$(sha256sum "${backup_file}" | awk '{print $1}')"
(
  export EUBL21_RETIRE_LEGACY_POSTGRESQL="${RETIRE_CONFIRMATION}"
  cutover_state_value() {
    case "$1" in
      status) printf '%s\n' retiring ;;
      legacy_cluster_uid) printf '%s\n' legacy-uid ;;
      backup_sha256) printf '%s\n' "${expected_backup_sha256}" ;;
    esac
  }
  legacy_cluster_exists() { return 0; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  verify_backup_archive() { [[ "$1" == "${backup_file}" ]]; }
  inspect_legacy_database() { printf '%s\n' retry-reinspection >>"${events_file}"; }
  verify_yugabytedb_cutover() { printf '%s\n' retry-yb-verified >>"${events_file}"; }
  verify_promoted_database_secret() { printf '%s\n' retry-secret-verified >>"${events_file}"; }
  verify_live_yugabytedb_database() { printf '%s\n' retry-live-ysql-verified >>"${events_file}"; }
  record_cutover_state() { printf 'retry-state:%s:%s:%s\n' "$1" "$2" "$3" >>"${events_file}"; }
  kubectl() {
    if [[ "$1" == "get" && "$2" == "secret" ]]; then
      return 0
    fi
    if [[ "$1" == "delete" && "$2" == "cluster.postgresql.cnpg.io" && "$3" == "${LEGACY_CLUSTER_NAME}" ]]; then
      printf '%s\n' retry-cluster-deleted >>"${events_file}"
      return 0
    fi
    return 1
  }
  retire_legacy_cluster "${backup_file}" >/dev/null
)
grep -Fxq retry-reinspection "${events_file}"
grep -Fxq retry-yb-verified "${events_file}"
grep -Fxq retry-secret-verified "${events_file}"
grep -Fxq retry-live-ysql-verified "${events_file}"
grep -Fxq retry-cluster-deleted "${events_file}"
if grep -q '^retry-state:retiring:' "${events_file}"; then
  echo "Retirement retry rewrote the retiring transition" >&2
  exit 1
fi
grep -q '^retry-state:retired:legacy-uid:' "${events_file}"

# A different backup cannot resume an interrupted retirement.
if (
  export EUBL21_RETIRE_LEGACY_POSTGRESQL="${RETIRE_CONFIRMATION}"
  cutover_state_value() {
    case "$1" in
      status) printf '%s\n' retiring ;;
      legacy_cluster_uid) printf '%s\n' legacy-uid ;;
      backup_sha256) printf '%064d\n' 0 ;;
    esac
  }
  legacy_cluster_exists() { return 0; }
  legacy_cluster_value() { printf '%s\n' legacy-uid; }
  verify_backup_archive() { :; }
  inspect_legacy_database() { :; }
  verify_yugabytedb_cutover() { :; }
  verify_promoted_database_secret() { :; }
  verify_live_yugabytedb_database() { :; }
  kubectl() { return 0; }
  retire_legacy_cluster "${backup_file}"
) >/dev/null 2>&1; then
  echo "Interrupted retirement accepted a different backup" >&2
  exit 1
fi

# Missing confirmation blocks retirement before any state-changing command.
: >"${events_file}"
if (
  unset EUBL21_RETIRE_LEGACY_POSTGRESQL
  kubectl() { printf '%s\n' forbidden >>"${events_file}"; }
  retire_legacy_cluster "${backup_file}"
) >/dev/null 2>&1; then
  echo "Retirement accepted without explicit confirmation" >&2
  exit 1
fi
[[ ! -s "${events_file}" ]]

# A completed retirement is safely idempotent.
(
  export EUBL21_RETIRE_LEGACY_POSTGRESQL="${RETIRE_CONFIRMATION}"
  legacy_cluster_exists() { return 1; }
  cutover_state_value() {
    case "$1" in
      status) printf '%s\n' retired ;;
      legacy_cluster_uid) printf '%s\n' legacy-uid ;;
    esac
  }
  retire_legacy_cluster "${backup_file}" >/dev/null
)

# Operator removal is separately confirmed and blocked by any remaining CNPG
# cluster in any namespace.
: >"${events_file}"
if (
  export EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR="${OPERATOR_RETIRE_CONFIRMATION}"
  cutover_state_value() { printf '%s\n' retired; }
  kubectl() { printf '%s\n' 'cluster.postgresql.cnpg.io/another-cluster'; }
  helm() { printf '%s\n' forbidden-uninstall >>"${events_file}"; }
  retire_cloudnativepg_operator
) >/dev/null 2>&1; then
  echo "Operator retirement ignored another CloudNativePG cluster" >&2
  exit 1
fi
[[ ! -s "${events_file}" ]]

# Failure to inventory clusters is blocking; it is never interpreted as an
# empty cluster list.
: >"${events_file}"
if (
  export EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR="${OPERATOR_RETIRE_CONFIRMATION}"
  cutover_state_value() { printf '%s\n' retired; }
  kubectl() { return 1; }
  helm() { printf '%s\n' forbidden-uninstall >>"${events_file}"; }
  retire_cloudnativepg_operator
) >/dev/null 2>&1; then
  echo "Operator retirement ignored a failed cluster inventory" >&2
  exit 1
fi
[[ ! -s "${events_file}" ]]

: >"${events_file}"
(
  export EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR="${OPERATOR_RETIRE_CONFIRMATION}"
  cutover_state_value() { printf '%s\n' retired; }
  kubectl() { return 0; }
  helm() {
    if [[ "$1" == "status" ]]; then
      return 0
    fi
    if [[ "$1" == "uninstall" && "$2" == "cnpg" ]]; then
      printf '%s\n' operator-uninstalled >>"${events_file}"
      return 0
    fi
    return 1
  }
  retire_cloudnativepg_operator >/dev/null
)
grep -Fxq operator-uninstalled "${events_file}"

echo "Guarded on-prem PostgreSQL-to-YugabyteDB upgrade tests passed."
