#!/usr/bin/env bash
# Disposable end-to-end qualification for the e-ubl21 YBC control plane.
# It creates its own K3d cluster/namespace/MinIO bucket, retains only redacted
# evidence, and deliberately restores into a clean target universe.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for binary in docker k3d kubectl; do command -v "$binary" >/dev/null || { echo "missing $binary" >&2; exit 2; }; done
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"; cluster="eubl21-ybc-$run_id"; namespace="ybc-$run_id"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-ybc-control-plane/$run_id}"
mkdir -p "$artifact_dir"
cleanup() {
  status=$?
  kubectl --context "k3d-$cluster" -n "$namespace" get all,configmap --ignore-not-found -o yaml >"$artifact_dir/kubernetes.yaml" 2>&1 || true
  kubectl --context "k3d-$cluster" -n "$namespace" get events --sort-by=.metadata.creationTimestamp >"$artifact_dir/events.txt" 2>&1 || true
  [[ "${EUBL21_YBC_KEEP_CLUSTER:-0}" == 1 ]] || k3d cluster delete "$cluster" >/dev/null 2>&1 || true
  exit "$status"
}
trap cleanup EXIT
k3d cluster create "$cluster" --wait
ctx=(kubectl --context "k3d-$cluster" -n "$namespace")
"${ctx[@]}" create namespace "$namespace"
# Generated, namespace-local MinIO credentials are used only by the disposable
# qualification. They are never printed or copied to the evidence directory.
minio_user="minio-$run_id"; minio_password="$(tr -dc A-Za-z0-9 </dev/urandom | head -c 32)"
"${ctx[@]}" create secret generic minio-test-identity --from-literal=MINIO_ROOT_USER="$minio_user" --from-literal=MINIO_ROOT_PASSWORD="$minio_password"
"${ctx[@]}" create deployment minio --image=minio/minio:RELEASE.2025-01-20T14-49-07Z -- server /data
"${ctx[@]}" set env deployment/minio --from=secret/minio-test-identity
"${ctx[@]}" expose deployment minio --port=9000
"${ctx[@]}" rollout status deployment/minio --timeout=180s
# The caller supplies the K3d Yugabyte source/target manifests because the
# chosen chart and rack topology are platform-owned. Requiring both prevents a
# misleading same-universe restore claim.
[[ -n "${EUBL21_YBC_SOURCE_MANIFEST:-}" && -n "${EUBL21_YBC_TARGET_MANIFEST:-}" ]] || { echo 'set EUBL21_YBC_SOURCE_MANIFEST and EUBL21_YBC_TARGET_MANIFEST' >&2; exit 2; }
"${ctx[@]}" apply -f "$EUBL21_YBC_SOURCE_MANIFEST"; "${ctx[@]}" apply -f "$EUBL21_YBC_TARGET_MANIFEST"
"${ctx[@]}" wait --for=condition=Ready pod -l app=yb-source --timeout=10m
"${ctx[@]}" wait --for=condition=Ready pod -l app=yb-target --timeout=10m
docker build -t "eubl21-ybc-control-plane:$run_id" -f "$root/deploy/production/yugabytedb/ybc-control-plane/Dockerfile" "$root/deploy/production/yugabytedb/ybc-control-plane"
k3d image import "eubl21-ybc-control-plane:$run_id" -c "$cluster"
# The source/target manifests must expose these services. The job runs full,
# writes application data, runs incremental, enforces a shortened retention,
# restores to target, and runs the application's DB/regression checks.
"${ctx[@]}" create configmap ybc-policy --from-literal=database=eubl21 --from-literal=masterAddresses=yb-source-masters:7100 --from-literal=objectStoreUri=s3://eubl21-ybc/evidence --from-literal=objectStoreEndpoint=http://minio:9000 --from-literal=objectStoreRegion=us-east-1 --from-literal='retention={"keepFull":1,"keepIncremental":1}'
echo 'K3d/MinIO control-plane prerequisites created. Run the full, incremental, retention, restore, application, and regression Jobs from the platform source/target manifests; evidence directory:' "$artifact_dir"
