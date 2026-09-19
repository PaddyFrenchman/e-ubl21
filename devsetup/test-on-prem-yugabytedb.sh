#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
installer="$root_dir/deploy/on-prem/scripts/install-on-prem.sh"
values_file="$root_dir/deploy/on-prem/yugabytedb-values.yaml"
chart_url="https://charts.yugabyte.com/yugabyte-2025.2.6.tgz"
chart_sha256="26518b8dfb7389dd6e97bc034df59838236c1a0c15d6f7482a1cf9a7dd4b80a0"
test_namespace="${EUBL21_TEST_NAMESPACE:-eubl21-issue-123-$(date +%s)-$$}"
test_image="${EUBL21_TEST_IMAGE:-localhost/e-ubl21:regression}"
mode="${1:-static}"
temporary_dir="$(mktemp -d)"
namespace_created=0

cleanup() {
    if [[ "$mode" == "integration" && "$namespace_created" == "1" ]]; then
        kubectl delete namespace "$test_namespace" --ignore-not-found --wait=true >/dev/null 2>&1 || true
    fi
    rm -rf "$temporary_dir"
}
trap cleanup EXIT

require_command() {
    command -v "$1" >/dev/null 2>&1 || {
        echo "$1 is required" >&2
        exit 1
    }
}

assert_contains() {
    local pattern="$1" file="$2"
    rg -q -- "$pattern" "$file" || {
        echo "Expected $file to contain: $pattern" >&2
        exit 1
    }
}

require_command curl
require_command helm
require_command rg
require_command sha256sum

bash -n "$installer" "$root_dir/devsetup/yugabytedb-bootstrap.sh"
if rg -q 'helm repo add cnpg|helm upgrade --install cnpg|cnpg/cloudnative-pg|kind:[[:space:]]+Cluster' "$installer"; then
    echo "Fresh installer still contains CloudNativePG installation logic" >&2
    exit 1
fi
if EUBL21_IMAGE= bash -c 'source "$1"; validate_platform_config' _ "$installer" >/dev/null 2>&1; then
    echo "Installer accepted an empty migration image" >&2
    exit 1
fi
if EUBL21_IMAGE=eubl21:test POSTGRES_STORAGE_SIZE=50Gi bash -c \
    'source "$1"; validate_platform_config' _ "$installer" >/dev/null 2>&1; then
    echo "Installer silently accepted a retired PostgreSQL setting" >&2
    exit 1
fi
if EUBL21_IMAGE=eubl21:test YB_TEST_DISABLE_DISK_FULL_CHECK=1 bash -c \
    'source "$1"; validate_platform_config' _ "$installer" >/dev/null 2>&1; then
    echo "Installer allowed the test-only disk override in the production namespace" >&2
    exit 1
fi
encoded_password="$(bash -c 'source "$1"; urlencode "a+b/c="' _ "$installer")"
[[ "$encoded_password" == "a%2Bb%2Fc%3D" ]] || {
    echo "Database password URL encoding failed: $encoded_password" >&2
    exit 1
}
curl -fsSL "$chart_url" -o "$temporary_dir/yugabyte.tgz"
printf '%s  %s\n' "$chart_sha256" "$temporary_dir/yugabyte.tgz" | sha256sum --check --status

helm template yugabyte "$temporary_dir/yugabyte.tgz" \
    --namespace e-ubl21 \
    --values "$values_file" \
    --set-string Image.repository=yugabytedb/yugabyte \
    --set-string Image.tag=2025.2.6.0-b111 \
    --set-string storage.master.size=10Gi \
    --set-string storage.tserver.size=50Gi \
    --set-string storage.master.storageClass=local-path \
    --set-string storage.tserver.storageClass=local-path \
    >"$temporary_dir/yugabyte-rendered.yaml"

assert_contains 'name: yb-master' "$temporary_dir/yugabyte-rendered.yaml"
assert_contains 'name: yb-tserver' "$temporary_dir/yugabyte-rendered.yaml"
assert_contains 'image: "yugabytedb/yugabyte:2025.2.6.0-b111"' "$temporary_dir/yugabyte-rendered.yaml"
assert_contains 'storage: 10Gi' "$temporary_dir/yugabyte-rendered.yaml"
assert_contains 'storage: 50Gi' "$temporary_dir/yugabyte-rendered.yaml"
assert_contains '--ysql_enable_auth=true' "$temporary_dir/yugabyte-rendered.yaml"
if rg -q 'postgresql\.cnpg\.io|e-ubl21-postgres|cloudnative-pg' "$temporary_dir/yugabyte-rendered.yaml"; then
    echo "Rendered clean-install resources contain a legacy PostgreSQL dependency" >&2
    exit 1
fi

EUBL21_NAMESPACE=e-ubl21 EUBL21_IMAGE="$test_image" bash -c \
    'source "$1"; render_yugabytedb_bootstrap_job; printf "\n---\n"; render_database_migration_job' \
    _ "$installer" >"$temporary_dir/eubl21-database-jobs.yaml"
assert_contains 'name: eubl21-yugabytedb-bootstrap' "$temporary_dir/eubl21-database-jobs.yaml"
assert_contains 'name: eubl21-database-migrate' "$temporary_dir/eubl21-database-jobs.yaml"
assert_contains 'key: database-url' "$temporary_dir/eubl21-database-jobs.yaml"

# Keep static verification independent of the active Kubernetes context.
# Recent kubectl versions perform API discovery for `create --dry-run=client`,
# so even that command fails when a developer has a stale or offline context.
# The generated manifest is exercised against a live API server by integration
# mode below.

if [[ "$mode" == "static" ]]; then
    echo "On-prem YugabyteDB static installer verification passed."
    exit 0
fi
[[ "$mode" == "integration" ]] || {
    echo "Usage: $0 [static|integration]" >&2
    exit 2
}
[[ "$test_namespace" == eubl21-issue-123* ]] || {
    echo "Integration namespace must start with eubl21-issue-123" >&2
    exit 1
}
require_command kubectl

kubectl create namespace "$test_namespace"
namespace_created=1
helm repo add yugabytedb https://charts.yugabyte.com --force-update >/dev/null
helm repo update yugabytedb >/dev/null

export EUBL21_NAMESPACE="$test_namespace"
export EUBL21_IMAGE="$test_image"
export YB_STORAGE_CLASS="${EUBL21_TEST_STORAGE_CLASS:-standard}"
export YB_TEST_DISABLE_DISK_FULL_CHECK=1
# shellcheck disable=SC1090
source "$installer"

for run in first second; do
    echo "Running $run disposable on-prem YugabyteDB installation"
    install_yugabytedb
    bootstrap_and_migrate_yugabytedb
done

kubectl get statefulset yb-master yb-tserver --namespace "$test_namespace" >/dev/null
kubectl get job eubl21-yugabytedb-bootstrap eubl21-database-migrate --namespace "$test_namespace" >/dev/null
if kubectl get cluster.postgresql.cnpg.io e-ubl21-postgres --namespace "$test_namespace" >/dev/null 2>&1; then
    echo "Disposable installation unexpectedly created e-ubl21-postgres" >&2
    exit 1
fi

echo "On-prem YugabyteDB clean-install and idempotency verification passed."
