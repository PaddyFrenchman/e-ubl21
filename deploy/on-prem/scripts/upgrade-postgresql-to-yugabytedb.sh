#!/usr/bin/env bash
set -Eeuo pipefail

# Guarded migration for installations created by the pre-#123 on-prem
# bootstrap. The legacy PostgreSQL cluster is preserved through inspection and
# cutover. Retirement is a separate, explicitly confirmed operation.

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=deploy/on-prem/scripts/install-on-prem.sh
source "${SCRIPT_DIR}/install-on-prem.sh"

CUTOVER_STATE_CONFIGMAP="${CUTOVER_STATE_CONFIGMAP:-eubl21-yugabytedb-cutover}"
LEGACY_CLUSTER_NAME="${LEGACY_CLUSTER_NAME:-e-ubl21-postgres}"
PUBLIC_DATABASE_SECRET_NAME="eubl21-db-auth"
EUBL21_DATABASE_SECRET_NAME="eubl21-db-auth-yugabytedb-staging"
RETIRE_CONFIRMATION="delete-e-ubl21-postgres-after-verified-backup"
OPERATOR_RETIRE_CONFIRMATION="uninstall-cnpg-after-all-clusters-are-retired"

usage() {
  cat <<'EOF'
Usage: upgrade-postgresql-to-yugabytedb.sh COMMAND [ARGUMENT]

Commands:
  inspect                 Classify the legacy eubl21 database without changing it.
  export BACKUP_FILE      Write and validate a PostgreSQL custom-format backup.
  cutover                 Install/verify YugabyteDB while preserving PostgreSQL.
  retire BACKUP_FILE      Delete only the verified legacy cluster after explicit confirmation.
  retire-operator         Uninstall the CNPG Helm release after every cluster is gone.
  status                  Show the legacy cluster, YugabyteDB, and cutover record.

Required for cutover:
  EUBL21_IMAGE=<immutable application image>

Required for retire:
  EUBL21_RETIRE_LEGACY_POSTGRESQL=delete-e-ubl21-postgres-after-verified-backup

Required for retire-operator:
  EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR=uninstall-cnpg-after-all-clusters-are-retired
EOF
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

require_upgrade_tools() {
  require_command kubectl
  require_command helm
}

legacy_cluster_exists() {
  local resource_name
  resource_name="$(kubectl get cluster.postgresql.cnpg.io "${LEGACY_CLUSTER_NAME}" \
    --namespace "${EUBL21_NAMESPACE}" --ignore-not-found -o name)" ||
    fail "Could not determine whether legacy Cluster/${LEGACY_CLUSTER_NAME} exists; refusing to assume it is absent"
  [[ -n "${resource_name}" ]]
}

legacy_cluster_value() {
  local jsonpath="$1"
  kubectl get cluster.postgresql.cnpg.io "${LEGACY_CLUSTER_NAME}" \
    --namespace "${EUBL21_NAMESPACE}" -o "jsonpath=${jsonpath}"
}

legacy_primary_pod() {
  local primary
  primary="$(legacy_cluster_value '{.status.currentPrimary}')"
  [[ -n "${primary}" ]] || fail "Legacy PostgreSQL cluster has no current primary; repair it before migration"
  kubectl get pod "${primary}" --namespace "${EUBL21_NAMESPACE}" >/dev/null
  printf '%s\n' "${primary}"
}

legacy_psql() {
  local statement="$1" database="${2:-eubl21}" primary
  primary="$(legacy_primary_pod)"
  kubectl exec "${primary}" --namespace "${EUBL21_NAMESPACE}" --container postgres -- \
    psql --username postgres --dbname "${database}" --no-psqlrc --no-align --tuples-only \
      --set ON_ERROR_STOP=1 --command "${statement}"
}

inspect_legacy_database() {
  local database database_findings finding findings="" query unexpected_databases
  legacy_cluster_exists || fail "Legacy ${LEGACY_CLUSTER_NAME} cluster was not found in ${EUBL21_NAMESPACE}"
  unexpected_databases="$(legacy_psql "SELECT 'database|' || quote_ident(datname) FROM pg_database WHERE datallowconn AND NOT datistemplate AND datname NOT IN ('eubl21','postgres') UNION ALL SELECT 'role|' || quote_ident(rolname) FROM pg_roles WHERE rolname NOT IN ('postgres','streaming_replica','cnpg_metrics_exporter','eubl21') AND rolname NOT LIKE 'pg_%' UNION ALL SELECT 'tablespace|' || quote_ident(spcname) FROM pg_tablespace WHERE spcname NOT IN ('pg_default','pg_global') ORDER BY 1;" postgres)" ||
    fail "Could not inventory databases, roles, and tablespaces in the legacy cluster; PostgreSQL remains untouched"
  findings+="${unexpected_databases}"
  query="SELECT 'schema|' || quote_ident(nspname) FROM pg_namespace WHERE nspname NOT IN ('pg_catalog','information_schema','public') AND nspname NOT LIKE 'pg_toast%' AND nspname NOT LIKE 'pg_temp_%' UNION ALL SELECT 'relation|' || quote_ident(n.nspname) || '.' || quote_ident(c.relname) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE c.relkind IN ('r','p','v','m','S','f') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp_%' UNION ALL SELECT 'routine|' || quote_ident(n.nspname) || '.' || quote_ident(p.proname) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp_%' UNION ALL SELECT 'type|' || quote_ident(n.nspname) || '.' || quote_ident(t.typname) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace WHERE t.typtype IN ('b','c','d','e','m','r') AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp_%' UNION ALL SELECT 'large-object|' || oid::text FROM pg_largeobject_metadata ORDER BY 1;"
  for database in eubl21 postgres; do
    database_findings="$(legacy_psql "${query}" "${database}")" ||
      fail "Could not inspect the legacy ${database} database; PostgreSQL remains untouched"
    if [[ -n "${database_findings}" ]]; then
      findings+=$'\n'
      while IFS= read -r finding; do
        findings+="${database}|${finding}"$'\n'
      done <<<"${database_findings}"
    fi
  done
  findings="$(printf '%s\n' "${findings}" | sed '/^[[:space:]]*$/d')"
  if [[ -n "${findings}" ]]; then
    printf '%s\n' "Legacy eubl21 database contains non-system objects:" >&2
    while IFS= read -r finding; do
      printf '  %s\n' "${finding}" >&2
    done <<<"${findings}"
    warn "Cutover is blocked. Export the database and use a reviewed data-transfer plan; nothing was deleted."
    return 2
  fi
  log "Legacy cluster contains only expected databases/roles/tablespaces and no user schemas, relations, routines, types, sequences, or large objects"
}

cutover_state_value() {
  local key="$1"
  kubectl get configmap "${CUTOVER_STATE_CONFIGMAP}" --namespace "${EUBL21_NAMESPACE}" \
    -o "jsonpath={.data.${key}}" 2>/dev/null || true
}

record_cutover_state() {
  local status="$1" legacy_uid="$2" backup_sha256="${3:-none}"
  kubectl create configmap "${CUTOVER_STATE_CONFIGMAP}" \
    --namespace "${EUBL21_NAMESPACE}" \
    --from-literal=status="${status}" \
    --from-literal=legacy_cluster_name="${LEGACY_CLUSTER_NAME}" \
    --from-literal=legacy_cluster_uid="${legacy_uid}" \
    --from-literal=legacy_inspection=empty \
    --from-literal=yugabytedb_chart_version="${YUGABYTEDB_CHART_VERSION}" \
    --from-literal=yugabytedb_image="${YUGABYTEDB_IMAGE_REPOSITORY}:${YUGABYTEDB_IMAGE_TAG}" \
    --from-literal=backup_sha256="${backup_sha256}" \
    --from-literal=updated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --dry-run=client -o yaml | kubectl apply -f -
}

verify_yugabytedb_cutover() {
  local bootstrap_complete migration_complete
  helm status yugabyte --namespace "${EUBL21_NAMESPACE}" >/dev/null
  kubectl rollout status statefulset/yb-master --namespace "${EUBL21_NAMESPACE}" --timeout=10m
  kubectl rollout status statefulset/yb-tserver --namespace "${EUBL21_NAMESPACE}" --timeout=10m
  bootstrap_complete="$(kubectl get job eubl21-yugabytedb-bootstrap --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}')"
  migration_complete="$(kubectl get job eubl21-database-migrate --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}')"
  [[ "${bootstrap_complete}" == "True" ]] || fail "YugabyteDB bootstrap verification is not complete"
  [[ "${migration_complete}" == "True" ]] || fail "YugabyteDB migration verification is not complete"
  kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" \
    -o jsonpath='{.data.database-url}' | grep -q . || fail "Database secret has no YugabyteDB URL"
}

apply_database_secret() {
  local secret_name="$1" username_base64="$2" password_base64="$3" database_url_base64="${4:-}"
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: ${secret_name}
  namespace: ${EUBL21_NAMESPACE}
type: kubernetes.io/basic-auth
data:
  username: ${username_base64}
  password: ${password_base64}
  database-url: "${database_url_base64}"
EOF
}

prepare_staged_database_secret() {
  local username_base64 password_base64 username
  username_base64="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.username}')"
  password_base64="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.password}')"
  [[ -n "${username_base64}" && -n "${password_base64}" ]] || fail "Legacy ${PUBLIC_DATABASE_SECRET_NAME} is missing credentials"
  username="$(printf '%s' "${username_base64}" | base64 --decode)"
  [[ "${username}" == "eubl21" ]] || fail "Legacy ${PUBLIC_DATABASE_SECRET_NAME} username must be eubl21"
  apply_database_secret "${EUBL21_DATABASE_SECRET_NAME}" "${username_base64}" "${password_base64}"
}

promote_staged_database_secret() {
  local expected_resource_version="$1" username_base64 password_base64 database_url_base64
  username_base64="$(kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.username}')"
  password_base64="$(kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.password}')"
  database_url_base64="$(kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.database-url}')"
  [[ -n "${username_base64}" && -n "${password_base64}" && -n "${database_url_base64}" ]] ||
    fail "Staged YugabyteDB credentials are incomplete"
  kubectl patch secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" \
    --type=json \
    -p "[{\"op\":\"test\",\"path\":\"/metadata/resourceVersion\",\"value\":\"${expected_resource_version}\"},{\"op\":\"add\",\"path\":\"/data/database-url\",\"value\":\"${database_url_base64}\"}]" >/dev/null
}

rollback_public_database_secret() {
  local promoted_database_url_base64="$1" previous_database_url_base64="$2" patch
  if [[ -n "${previous_database_url_base64}" ]]; then
    patch="[{\"op\":\"test\",\"path\":\"/data/database-url\",\"value\":\"${promoted_database_url_base64}\"},{\"op\":\"replace\",\"path\":\"/data/database-url\",\"value\":\"${previous_database_url_base64}\"}]"
  else
    patch="[{\"op\":\"test\",\"path\":\"/data/database-url\",\"value\":\"${promoted_database_url_base64}\"},{\"op\":\"remove\",\"path\":\"/data/database-url\"}]"
  fi
  kubectl patch secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" \
    --type=json -p "${patch}" >/dev/null
}

verify_promoted_database_secret() {
  local key public_value staged_value
  for key in username password database-url; do
    staged_value="$(kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o "jsonpath={.data.${key}}")" ||
      fail "Could not read ${key} from the staged YugabyteDB Secret"
    public_value="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o "jsonpath={.data.${key}}")" ||
      fail "Could not read ${key} from the public database Secret"
    [[ -n "${staged_value}" && "${public_value}" == "${staged_value}" ]] ||
      fail "Public database Secret does not match the verified YugabyteDB ${key}"
  done
}

verify_live_yugabytedb_database() {
  # A new migration command opens a fresh authenticated YSQL connection through
  # the verified staged migration identity and validates the complete history against the
  # immutable migrations embedded in EUBL21_IMAGE. It is idempotent when the
  # schema is current and fails on missing, failed, changed, or newer history.
  render_database_migration_job \
    eubl21-yugabytedb-retirement-check "${EUBL21_DATABASE_SECRET_NAME}" |
    run_checked_job eubl21-yugabytedb-retirement-check
}

perform_cutover() {
  local legacy_uid previous_database_url_base64 public_secret_resource_version promoted_database_url_base64
  validate_platform_config
  inspect_legacy_database || fail "Legacy PostgreSQL inspection did not qualify for cutover"
  legacy_uid="$(legacy_cluster_value '{.metadata.uid}')"
  [[ -n "${legacy_uid}" ]] || fail "Could not record the legacy cluster identity"
  prepare_staged_database_secret

  previous_database_url_base64="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.database-url}')" ||
    fail "Could not snapshot the public database URL before cutover"
  public_secret_resource_version="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.metadata.resourceVersion}')" ||
    fail "Could not snapshot the public database Secret version before cutover"
  [[ -n "${public_secret_resource_version}" ]] || fail "Public database Secret has no resource version"

  log "Installing and verifying YugabyteDB; legacy PostgreSQL will remain available"
  if ! (
    install_yugabytedb guarded-upgrade || exit 1
    bootstrap_and_migrate_yugabytedb || exit 1
    verify_yugabytedb_cutover || exit 1
  ); then
    fail "YugabyteDB cutover failed; legacy PostgreSQL and ${PUBLIC_DATABASE_SECRET_NAME} remain unchanged"
  fi
  promoted_database_url_base64="$(kubectl get secret "${EUBL21_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" -o jsonpath='{.data.database-url}')" ||
    fail "Could not read the verified staged YugabyteDB URL"
  [[ -n "${promoted_database_url_base64}" ]] || fail "Verified staged YugabyteDB URL is empty"
  promote_staged_database_secret "${public_secret_resource_version}" ||
    fail "Could not atomically promote the verified YugabyteDB URL; legacy PostgreSQL remains untouched"
  if ! (
    verify_promoted_database_secret || exit 1
    record_cutover_state verified "${legacy_uid}" || exit 1
  ); then
    rollback_public_database_secret "${promoted_database_url_base64}" "${previous_database_url_base64}" ||
      fail "Cutover finalization failed and the public database URL could not be rolled back; stop and repair the Secret before retrying"
    fail "Cutover finalization failed; the public database URL was restored and legacy PostgreSQL remains untouched"
  fi
  log "Cutover verified. PostgreSQL remains intact for rollback; export a backup before explicit retirement."
}

export_legacy_database() {
  local backup_file="$1" backup_sha256 legacy_uid metadata_file partial_file partial_metadata_file primary
  legacy_cluster_exists || fail "Legacy ${LEGACY_CLUSTER_NAME} cluster was not found"
  [[ -n "${backup_file}" ]] || fail "A backup file path is required"
  metadata_file="${backup_file}.eubl21-metadata"
  [[ ! -e "${backup_file}" ]] || fail "Refusing to overwrite existing backup: ${backup_file}"
  [[ ! -e "${metadata_file}" ]] || fail "Refusing to overwrite existing backup metadata: ${metadata_file}"
  [[ -d "$(dirname -- "${backup_file}")" ]] || fail "Backup directory does not exist: $(dirname -- "${backup_file}")"
  partial_file="${backup_file}.partial"
  partial_metadata_file="${metadata_file}.partial"
  [[ ! -e "${partial_file}" ]] || fail "Refusing to overwrite partial backup: ${partial_file}"
  [[ ! -e "${partial_metadata_file}" ]] || fail "Refusing to overwrite partial backup metadata: ${partial_metadata_file}"
  primary="$(legacy_primary_pod)"
  legacy_uid="$(legacy_cluster_value '{.metadata.uid}')"
  [[ -n "${legacy_uid}" ]] || fail "Could not bind the backup to the legacy cluster identity"
  umask 077
  log "Exporting the legacy eubl21 database without modifying it"
  kubectl exec "${primary}" --namespace "${EUBL21_NAMESPACE}" --container postgres -- \
    pg_dump --username postgres --dbname eubl21 --format=custom --no-owner --no-privileges >"${partial_file}" ||
    fail "PostgreSQL export failed; inspect ${partial_file} before removing it"
  [[ -s "${partial_file}" ]] || fail "PostgreSQL export was empty"
  kubectl exec -i "${primary}" --namespace "${EUBL21_NAMESPACE}" --container postgres -- \
    pg_restore --list <"${partial_file}" >/dev/null || fail "PostgreSQL export validation failed; inspect ${partial_file}"
  backup_sha256="$(sha256sum "${partial_file}" | awk '{print $1}')"
  printf 'format=eubl21-postgresql-backup-v1\nlegacy_cluster_uid=%s\ndatabase=eubl21\nbackup_sha256=%s\n' \
    "${legacy_uid}" "${backup_sha256}" >"${partial_metadata_file}"
  mv -- "${partial_file}" "${backup_file}"
  mv -- "${partial_metadata_file}" "${metadata_file}"
  log "Validated backup written to ${backup_file} (sha256 ${backup_sha256}; metadata ${metadata_file})"
}

verify_backup_archive() {
  local backup_file="$1" expected_sha256 legacy_uid metadata_file metadata_uid primary
  [[ -f "${backup_file}" && -s "${backup_file}" ]] || fail "A non-empty backup file is required: ${backup_file}"
  metadata_file="${backup_file}.eubl21-metadata"
  [[ -f "${metadata_file}" && -s "${metadata_file}" ]] || fail "Backup metadata is required: ${metadata_file}"
  grep -Fxq 'format=eubl21-postgresql-backup-v1' "${metadata_file}" || fail "Backup metadata format is not recognized"
  grep -Fxq 'database=eubl21' "${metadata_file}" || fail "Backup metadata does not identify the eubl21 database"
  metadata_uid="$(awk -F= '$1 == "legacy_cluster_uid" {print $2}' "${metadata_file}")"
  expected_sha256="$(awk -F= '$1 == "backup_sha256" {print $2}' "${metadata_file}")"
  legacy_uid="$(legacy_cluster_value '{.metadata.uid}')"
  [[ -n "${metadata_uid}" && "${metadata_uid}" == "${legacy_uid}" ]] || fail "Backup belongs to a different legacy cluster"
  [[ "${expected_sha256}" =~ ^[0-9a-f]{64}$ ]] || fail "Backup metadata SHA-256 is invalid"
  printf '%s  %s\n' "${expected_sha256}" "${backup_file}" | sha256sum --check --status || fail "Backup checksum does not match its metadata"
  primary="$(legacy_primary_pod)"
  kubectl exec -i "${primary}" --namespace "${EUBL21_NAMESPACE}" --container postgres -- \
    pg_restore --list <"${backup_file}" >/dev/null || fail "Backup archive validation failed"
}

retire_legacy_cluster() {
  local backup_file="$1" state_status recorded_uid recorded_backup_sha legacy_uid backup_sha256 secret_owner_uids
  [[ "${EUBL21_RETIRE_LEGACY_POSTGRESQL:-}" == "${RETIRE_CONFIRMATION}" ]] ||
    fail "Explicit retirement confirmation is missing; see --help"
  state_status="$(cutover_state_value status)"
  recorded_uid="$(cutover_state_value legacy_cluster_uid)"
  recorded_backup_sha="$(cutover_state_value backup_sha256)"

  if ! legacy_cluster_exists; then
    if [[ "${state_status}" == "retired" ]]; then
      log "Legacy PostgreSQL cluster is already retired"
      return
    fi
    if [[ "${state_status}" == "retiring" ]]; then
      [[ -n "${recorded_uid}" && "${recorded_backup_sha}" =~ ^[0-9a-f]{64}$ ]] ||
        fail "The interrupted retirement record is incomplete; refusing to mark the legacy cluster retired"
      [[ -f "${backup_file}" && -s "${backup_file}" ]] ||
        fail "The recorded retirement backup is required to complete recovery: ${backup_file}"
      backup_sha256="$(sha256sum "${backup_file}" | awk '{print $1}')"
      [[ "${backup_sha256}" == "${recorded_backup_sha}" ]] ||
        fail "The supplied backup does not match the interrupted retirement record"
      record_cutover_state retired "${recorded_uid}" "${recorded_backup_sha}"
      log "Recorded completion of the previously initiated retirement"
      return
    fi
    fail "Legacy cluster is absent without a completed retirement record"
  fi

  [[ "${state_status}" == "verified" || "${state_status}" == "retiring" ]] ||
    fail "A verified or interrupted-retirement YugabyteDB cutover record is required before retirement"
  legacy_uid="$(legacy_cluster_value '{.metadata.uid}')"
  [[ -n "${recorded_uid}" && "${legacy_uid}" == "${recorded_uid}" ]] ||
    fail "Legacy cluster identity changed after cutover; refusing retirement"
  verify_backup_archive "${backup_file}"
  inspect_legacy_database
  verify_yugabytedb_cutover
  verify_promoted_database_secret
  verify_live_yugabytedb_database
  secret_owner_uids="$(kubectl get secret "${PUBLIC_DATABASE_SECRET_NAME}" --namespace "${EUBL21_NAMESPACE}" \
    -o jsonpath='{range .metadata.ownerReferences[*]}{.uid}{"\n"}{end}')" ||
    fail "Could not verify ownership of ${PUBLIC_DATABASE_SECRET_NAME}"
  if grep -Fxq "${legacy_uid}" <<<"${secret_owner_uids}"; then
    fail "${PUBLIC_DATABASE_SECRET_NAME} is owned by the legacy cluster; refusing retirement"
  fi
  backup_sha256="$(sha256sum "${backup_file}" | awk '{print $1}')"
  if [[ "${state_status}" == "retiring" ]]; then
    [[ "${recorded_backup_sha}" =~ ^[0-9a-f]{64}$ && "${backup_sha256}" == "${recorded_backup_sha}" ]] ||
      fail "The supplied backup does not match the interrupted retirement record"
    log "Resuming the previously verified retirement for Cluster/${LEGACY_CLUSTER_NAME}"
  else
    record_cutover_state retiring "${legacy_uid}" "${backup_sha256}"
  fi

  warn "Explicit confirmation accepted; deleting only Cluster/${LEGACY_CLUSTER_NAME}. The CNPG operator is retained."
  kubectl delete cluster.postgresql.cnpg.io "${LEGACY_CLUSTER_NAME}" \
    --namespace "${EUBL21_NAMESPACE}" --wait=true
  record_cutover_state retired "${legacy_uid}" "${backup_sha256}"
  log "Legacy PostgreSQL cluster retired. Keep the validated backup for the required rollback retention period."
}

retire_cloudnativepg_operator() {
  local remaining_clusters state_status
  [[ "${EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR:-}" == "${OPERATOR_RETIRE_CONFIRMATION}" ]] ||
    fail "Explicit CloudNativePG operator retirement confirmation is missing; see --help"
  state_status="$(cutover_state_value status)"
  [[ "${state_status}" == "retired" ]] || fail "Legacy cluster retirement must be recorded before operator removal"
  remaining_clusters="$(kubectl get clusters.postgresql.cnpg.io --all-namespaces -o name)" ||
    fail "Could not inventory CloudNativePG clusters; refusing to uninstall the operator"
  [[ -z "${remaining_clusters}" ]] || fail "CloudNativePG clusters still exist; refusing to uninstall the operator"
  if helm status cnpg --namespace cnpg-system >/dev/null 2>&1; then
    helm uninstall cnpg --namespace cnpg-system --wait
    log "CloudNativePG Helm release removed; its namespace and CRDs were preserved for safety"
  else
    log "CloudNativePG Helm release is already absent"
  fi
}

show_status() {
  if legacy_cluster_exists; then
    printf 'Legacy PostgreSQL cluster: present (uid=%s)\n' "$(legacy_cluster_value '{.metadata.uid}')"
  else
    printf 'Legacy PostgreSQL cluster: absent\n'
  fi
  if helm status yugabyte --namespace "${EUBL21_NAMESPACE}" >/dev/null 2>&1; then
    printf 'YugabyteDB Helm release: present\n'
  else
    printf 'YugabyteDB Helm release: absent\n'
  fi
  printf 'Cutover state: %s\n' "$(cutover_state_value status | sed 's/^$/not-recorded/')"
}

main() {
  local command="${1:-}" argument="${2:-}"
  case "${command}" in
    -h|--help|help) usage; return ;;
  esac
  require_upgrade_tools
  case "${command}" in
    inspect) inspect_legacy_database ;;
    export) export_legacy_database "${argument}" ;;
    cutover) perform_cutover ;;
    retire) retire_legacy_cluster "${argument}" ;;
    retire-operator) retire_cloudnativepg_operator ;;
    status) show_status ;;
    *) usage >&2; exit 2 ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
