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
minio_user="minio-$run_id"; minio_password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
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
docker build -t "eubl21-ybc-control-plane:$run_id" -f "$root/deploy/production/yugabytedb/ybc-control-plane/Dockerfile" "$root/deploy/production/yugabytedb/ybc-control-plane"
k3d image import "eubl21-ybc-control-plane:$run_id" -c "$cluster"
"${ctx[@]}" create job minio-init --image="eubl21-ybc-control-plane:$run_id" --overrides='{"spec":{"template":{"spec":{"restartPolicy":"Never","containers":[{"name":"minio-init","command":["aws"],"args":["--endpoint-url","http://minio:9000","--region","us-east-1","s3api","create-bucket","--bucket","eubl21-ybc"],"envFrom":[{"secretRef":{"name":"s3-test-identity"}}]}]}}}'
"${ctx[@]}" wait --for=condition=complete job/minio-init --timeout=180s
"${ctx[@]}" create configmap ybc-policy --from-literal=database=eubl21 --from-literal=controllerAddress=yb-source --from-literal=objectStoreUri=s3://eubl21-ybc/evidence --from-literal=objectStoreEndpoint=http://minio:9000 --from-literal=objectStoreRegion=us-east-1 --from-literal='retention={"keepFull":1}'
run_control_plane() {
  local name=$1; shift
  # The full Pod template is supplied at creation; Jobs are never patched after
  # admission, so the mounted policy and short-lived test identity are immutable.
  "${ctx[@]}" create job "$name" --image="eubl21-ybc-control-plane:$run_id" --overrides='{"spec":{"template":{"spec":{"restartPolicy":"Never","volumes":[{"name":"policy","configMap":{"name":"ybc-policy"}}],"containers":[{"name":"'"$name"'","envFrom":[{"secretRef":{"name":"s3-test-identity"}}],"volumeMounts":[{"name":"policy","mountPath":"/etc/eubl21","readOnly":true}]}]}}}' -- "$@"
  "${ctx[@]}" wait --for=condition=complete "job/$name" --timeout=15m
  "${ctx[@]}" logs "job/$name" >"$artifact_dir/$name.log"
  "${ctx[@]}" logs "job/$name" >"$artifact_dir/$name.log"
}
# Full snapshot, then a distinct post-full write, then its incremental chain.
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -c 'CREATE DATABASE eubl21;'
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -c "CREATE TABLE qualification (id int PRIMARY KEY, payload text); INSERT INTO qualification VALUES (1, 'full');"
run_control_plane ybc-full backup --kind=full --policy=/etc/eubl21
"${ctx[@]}" exec deploy/yb-source -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -c "INSERT INTO qualification VALUES (2, 'incremental');"
# Destroy the untouched target and create a fresh universe only after both
# source backups exist; restoration can therefore never be in-place.
"${ctx[@]}" delete deployment yb-target --wait=true
"${ctx[@]}" apply -f - <<'YAML'
# Exercise the real Go database client against the clean restored target. The
# port-forward is retained only for the duration of this assertion and its log
# is evidence; credentials are not printed.
"${ctx[@]}" port-forward service/yb-target 15435:5433 >"$artifact_dir/target-port-forward.log" 2>&1 & pf=$!
trap 'kill "$pf" >/dev/null 2>&1 || true; cleanup' EXIT
sleep 3
EUBL21_TEST_DATABASE_URL='postgresql://yugabyte@127.0.0.1:15435/eubl21?sslmode=disable' go test ./database -run '^TestYugabyteDBIntegration$' -count=1 | tee "$artifact_dir/application-integration.txt"
kill "$pf" >/dev/null 2>&1 || true
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
YAML
"${ctx[@]}" wait --for=condition=Available deployment/yb-target --timeout=10m
run_control_plane ybc-restore restore --policy=/etc/eubl21 --target-controller=yb-target
run_control_plane ybc-retention retention --policy=/etc/eubl21
run_control_plane ybc-restore restore --policy=/etc/eubl21 --target-controller=yb-target
"${ctx[@]}" exec deploy/yb-target -- /home/yugabyte/bin/ysqlsh -h 127.0.0.1 -d eubl21 -Atc 'SELECT payload FROM qualification ORDER BY id' | tee "$artifact_dir/restored-data.txt"
grep -Fxq full "$artifact_dir/restored-data.txt"; grep -Fxq incremental "$artifact_dir/restored-data.txt"
"${ctx[@]}" get configmap ybc-policy -o yaml >"$artifact_dir/policy.yaml"
echo "K3d/MinIO YBC full/incremental/retention/isolated-restore qualification passed: $artifact_dir"
