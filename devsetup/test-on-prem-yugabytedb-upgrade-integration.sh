#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
upgrade_script="${root_dir}/deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh"
run_id="$(date +%s)-$$"
test_namespace="${EUBL21_UPGRADE_TEST_NAMESPACE:-eubl21-issue-124-${run_id}}"
operator_namespace="cnpg-system"
test_image="${EUBL21_TEST_IMAGE:-localhost/e-ubl21:regression}"
temporary_dir="$(mktemp -d)"
backup_file="${temporary_dir}/legacy.dump"
namespace_owned=0
operator_owned=0

cleanup() {
  if [[ "${namespace_owned}" == "1" ]]; then
    kubectl delete namespace "${test_namespace}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
    kubectl wait --for=delete "namespace/${test_namespace}" --timeout=5m >/dev/null 2>&1 || true
  fi
  if [[ "${operator_owned}" == "1" ]]; then
    helm uninstall cnpg --namespace "${operator_namespace}" --wait >/dev/null 2>&1 || true
    kubectl delete namespace "${operator_namespace}" --ignore-not-found --wait=false >/dev/null 2>&1 || true
    kubectl wait --for=delete "namespace/${operator_namespace}" --timeout=5m >/dev/null 2>&1 || true
    while IFS= read -r crd; do
      if [[ -n "${crd}" ]]; then
        kubectl delete "${crd}" --wait=false >/dev/null 2>&1 || true
        kubectl wait --for=delete "${crd}" --timeout=2m >/dev/null 2>&1 || true
      fi
    done < <(kubectl get crd -o name 2>/dev/null | grep '\.postgresql\.cnpg\.io$' || true)
  fi
  rm -rf "${temporary_dir}"
}
trap cleanup EXIT

[[ "${test_namespace}" == eubl21-issue-124-* ]] || {
  echo "Integration namespace must start with eubl21-issue-124-" >&2
  exit 1
}
for command in helm kubectl grep; do
  command -v "${command}" >/dev/null 2>&1 || {
    echo "${command} is required" >&2
    exit 1
  }
done
if kubectl get crd -o name | grep -q '\.postgresql\.cnpg\.io$' ||
   kubectl get namespace "${operator_namespace}" >/dev/null 2>&1 ||
   helm status cnpg --namespace "${operator_namespace}" >/dev/null 2>&1; then
  echo "Refusing integration test: pre-existing CloudNativePG resources are not test-owned" >&2
  exit 1
fi
if kubectl get namespace "${test_namespace}" >/dev/null 2>&1; then
  echo "Refusing integration test: namespace already exists: ${test_namespace}" >&2
  exit 1
fi

operator_owned=1
helm repo add cnpg https://cloudnative-pg.github.io/charts --force-update >/dev/null
helm repo add yugabytedb https://charts.yugabyte.com --force-update >/dev/null
helm repo update cnpg yugabytedb >/dev/null
helm upgrade --install cnpg cnpg/cloudnative-pg \
  --namespace "${operator_namespace}" \
  --create-namespace \
  --version 0.29.0 \
  --wait \
  --timeout 10m >/dev/null

kubectl create namespace "${test_namespace}" >/dev/null
namespace_owned=1
kubectl create secret generic eubl21-db-auth \
  --namespace "${test_namespace}" \
  --type kubernetes.io/basic-auth \
  --from-literal=username=eubl21 \
  --from-literal=password=issue-124-legacy-password >/dev/null
kubectl apply -f - >/dev/null <<EOF
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: e-ubl21-postgres
  namespace: ${test_namespace}
spec:
  instances: 1
  bootstrap:
    initdb:
      database: eubl21
      owner: eubl21
      secret:
        name: eubl21-db-auth
  storage:
    storageClass: standard
    size: 1Gi
EOF
kubectl wait --for=condition=Ready cluster.postgresql.cnpg.io/e-ubl21-postgres \
  --namespace "${test_namespace}" --timeout=10m >/dev/null

export EUBL21_NAMESPACE="${test_namespace}"
export EUBL21_IMAGE="${test_image}"
export EUBL21_IMAGE_PULL_POLICY=IfNotPresent
export YB_STORAGE_CLASS=standard
export YB_MASTER_STORAGE_SIZE=1Gi
export YB_TSERVER_STORAGE_SIZE=1Gi
export YB_TEST_DISABLE_DISK_FULL_CHECK=1
# shellcheck source=deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh
source "${upgrade_script}"

legacy_uid="$(legacy_cluster_value '{.metadata.uid}')"
inspect_legacy_database >/dev/null

legacy_psql 'CREATE TABLE public.issue_124_unknown_data (id integer PRIMARY KEY); INSERT INTO public.issue_124_unknown_data VALUES (1);' >/dev/null
if inspect_legacy_database >/dev/null 2>&1; then
  echo "Real PostgreSQL data unexpectedly qualified for cutover" >&2
  exit 1
fi
legacy_psql 'DROP TABLE public.issue_124_unknown_data;' >/dev/null

# Force the target-install failure path and prove the source UID and public
# secret remain unchanged.
if (
  install_yugabytedb() { return 1; }
  perform_cutover
) >/dev/null 2>&1; then
  echo "Forced YugabyteDB failure unexpectedly qualified" >&2
  exit 1
fi
[[ "$(legacy_cluster_value '{.metadata.uid}')" == "${legacy_uid}" ]]
[[ -z "$(kubectl get secret eubl21-db-auth --namespace "${test_namespace}" -o jsonpath='{.data.database-url}')" ]]

export_legacy_database "${backup_file}" >/dev/null
perform_cutover >/dev/null
legacy_cluster_exists
[[ -n "$(kubectl get secret eubl21-db-auth --namespace "${test_namespace}" -o jsonpath='{.data.database-url}')" ]]
[[ "$(cutover_state_value status)" == "verified" ]]

# A second cutover must preserve the source identity and remain current.
perform_cutover >/dev/null
[[ "$(legacy_cluster_value '{.metadata.uid}')" == "${legacy_uid}" ]]

export EUBL21_RETIRE_LEGACY_POSTGRESQL="${RETIRE_CONFIRMATION}"
retire_legacy_cluster "${backup_file}" >/dev/null
! legacy_cluster_exists
[[ "$(cutover_state_value status)" == "retired" ]]
retire_legacy_cluster "${backup_file}" >/dev/null

export EUBL21_RETIRE_CLOUDNATIVEPG_OPERATOR="${OPERATOR_RETIRE_CONFIRMATION}"
retire_cloudnativepg_operator >/dev/null

echo "Disposable CloudNativePG-to-YugabyteDB upgrade integration passed."
