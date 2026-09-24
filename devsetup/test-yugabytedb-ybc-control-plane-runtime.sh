#!/usr/bin/env bash
# Disposable end-to-end qualification for the e-ubl21 YBC control plane.
# It creates its own K3d cluster/namespace/MinIO bucket, retains only redacted
# evidence, and deliberately restores into a clean target universe.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for binary in docker k3d kubectl; do command -v "$binary" >/dev/null || { echo "missing $binary" >&2; exit 2; }; done
run_id="$(date -u +%Y%m%dt%H%M%sz)-$$"; cluster="eybc-${RANDOM}-${RANDOM}"; namespace="ybc-$run_id"
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
# Create two independent single-node universes. The target exists before the
# drill but is intentionally empty; it is never used as the backup source.
"${ctx[@]}" apply -f - <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata: {name: yb-source, labels: {app: yb-source}}
spec:
  replicas: 1
  selector: {matchLabels: {app: yb-source}}
  template:
    metadata: {labels: {app: yb-source}}
    spec:
      containers:
      - name: yugabyte
        image: yugabytedb/yugabyte:2025.2.6.0-b111
        command: [bash, -ec, 'mkdir -p ybc && tar -xzf share/ybc-*-linux-x86_64.tar.gz -C ybc --strip-components=1 && exec bin/yugabyted start --base_dir=/tmp/yb --background=false --backup_daemon=true']
        ports: [{containerPort: 7100}, {containerPort: 5433}, {containerPort: 18018}]
---
apiVersion: v1
kind: Service
metadata: {name: yb-source}
spec: {selector: {app: yb-source}, ports: [{name: master, port: 7100}, {name: ysql, port: 5433}, {name: controller, port: 18018}]}
---
apiVersion: apps/v1
kind: Deployment
metadata: {name: yb-target, labels: {app: yb-target}}
spec:
  replicas: 1
  selector: {matchLabels: {app: yb-target}}
  template:
    metadata: {labels: {app: yb-target}}
    spec:
      containers:
      - name: yugabyte
        image: yugabytedb/yugabyte:2025.2.6.0-b111
        command: [bash, -ec, 'mkdir -p ybc && tar -xzf share/ybc-*-linux-x86_64.tar.gz -C ybc --strip-components=1 && exec bin/yugabyted start --base_dir=/tmp/yb --background=false --backup_daemon=true']
        ports: [{containerPort: 7100}, {containerPort: 5433}, {containerPort: 18018}]
---
apiVersion: v1
kind: Service
metadata: {name: yb-target}
spec: {selector: {app: yb-target}, ports: [{name: master, port: 7100}, {name: ysql, port: 5433}, {name: controller, port: 18018}]}
YAML
"${ctx[@]}" wait --for=condition=Available deployment/yb-source deployment/yb-target --timeout=10m
"${ctx[@]}" create secret generic s3-test-identity --from-literal=AWS_ACCESS_KEY_ID="$minio_user" --from-literal=AWS_SECRET_ACCESS_KEY="$minio_password"
"${ctx[@]}" create job minio-init --image=minio/mc:RELEASE.2025-02-21T16-00-46Z -- sh -ec "mc alias set local http://minio:9000 '$minio_user' '$minio_password'; mc mb --ignore-existing local/eubl21-ybc"
"${ctx[@]}" wait --for=condition=complete job/minio-init --timeout=180s
"${ctx[@]}" create configmap ybc-policy --from-literal=database=eubl21 --from-literal=controllerAddress=yb-source --from-literal=objectStoreUri=s3://eubl21-ybc/evidence --from-literal=objectStoreEndpoint=http://minio:9000 --from-literal=objectStoreRegion=us-east-1 --from-literal='retention={"keepFull":1}'
docker build -t "eubl21-ybc-control-plane:$run_id" -f "$root/deploy/production/yugabytedb/ybc-control-plane/Dockerfile" "$root/deploy/production/yugabytedb/ybc-control-plane"
k3d image import "eubl21-ybc-control-plane:$run_id" -c "$cluster"
run_control_plane() {
  local name=$1; shift
  "${ctx[@]}" create job "$name" --image="eubl21-ybc-control-plane:$run_id" -- "$@"
  "${ctx[@]}" patch "job/$name" --type=strategic -p '{"spec":{"template":{"spec":{"volumes":[{"name":"policy","configMap":{"name":"ybc-policy"}}],"containers":[{"name":"'"$name"'","volumeMounts":[{"name":"policy","mountPath":"/etc/eubl21","readOnly":true}]}]}}}'
  "${ctx[@]}" set env "job/$name" --from=secret/s3-test-identity
  "${ctx[@]}" wait --for=condition=complete "job/$name" --timeout=15m
  "${ctx[@]}" logs "job/$name" >"$artifact_dir/$name.log"
}
# Full snapshot, then a distinct post-full write, then its incremental chain.
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -c 'CREATE DATABASE eubl21;'
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -c "CREATE TABLE qualification (id int PRIMARY KEY, payload text); INSERT INTO qualification VALUES (1, 'full');"
run_control_plane ybc-full backup --kind=full --policy=/etc/eubl21
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -c "INSERT INTO qualification VALUES (2, 'incremental');"
run_control_plane ybc-incremental backup --kind=incremental --policy=/etc/eubl21
run_control_plane ybc-retention retention --policy=/etc/eubl21
run_control_plane ybc-restore restore --policy=/etc/eubl21 --target-controller=yb-target
"${ctx[@]}" exec deploy/yb-target -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -Atc 'SELECT payload FROM qualification ORDER BY id' | tee "$artifact_dir/restored-data.txt"
grep -Fxq full "$artifact_dir/restored-data.txt"; grep -Fxq incremental "$artifact_dir/restored-data.txt"
"${ctx[@]}" get configmap ybc-policy -o yaml >"$artifact_dir/policy.yaml"
echo "K3d/MinIO YBC full/incremental/retention/isolated-restore qualification passed: $artifact_dir"
