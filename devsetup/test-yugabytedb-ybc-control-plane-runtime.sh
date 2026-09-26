#!/usr/bin/env bash
# Disposable end-to-end qualification for the e-ubl21 YBC control plane.
# It creates its own K3d cluster/namespace/MinIO bucket, retains only redacted
# evidence, and deliberately restores into a clean target universe.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
for binary in docker k3d kubectl; do command -v "$binary" >/dev/null || { echo "missing $binary" >&2; exit 2; }; done
run_id="$(date -u +%Y%m%dt%H%M%sz)-$$"; cluster="eybc-${RANDOM}-${RANDOM}"; namespace="ybc-$run_id"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-ybc-control-plane/$run_id}"
minio_source_image="${EUBL21_YBC_MINIO_SOURCE_IMAGE:-eubl21/minio:RELEASE.2025-10-15T17-29-55Z}"
minio_image="eubl21-ybc-minio-runtime:$run_id"
tmp="$(mktemp -d)"; kubeconfig="$tmp/kubeconfig"
mkdir -p "$artifact_dir"
cleanup() {
  status=$?
  if [[ -f "$kubeconfig" ]]; then
    kubectl --kubeconfig "$kubeconfig" --request-timeout=5s -n "$namespace" get all,configmap --ignore-not-found -o yaml >"$artifact_dir/kubernetes.yaml" 2>&1 || true
    kubectl --kubeconfig "$kubeconfig" --request-timeout=5s -n "$namespace" get events --sort-by=.metadata.creationTimestamp >"$artifact_dir/events.txt" 2>&1 || true
  fi
  [[ "${EUBL21_YBC_KEEP_CLUSTER:-0}" == 1 ]] || k3d cluster delete "$cluster" >/dev/null 2>&1 || true
  rm -rf "$tmp"
  exit "$status"
}
trap cleanup EXIT
docker image inspect "$minio_source_image" >/dev/null || {
  echo "missing MinIO binary image: $minio_source_image" >&2
  exit 2
}
docker build \
  --build-arg "MINIO_SOURCE_IMAGE=$minio_source_image" \
  -t "$minio_image" \
  -f "$root/devsetup/Dockerfile.ybc-minio-runtime" \
  "$root"
k3d cluster create "$cluster" --wait
# K3d advertises the API listener as 0.0.0.0 in this environment, which a
# client cannot connect to. Preserve the generated credentials and port while
# using loopback as the API endpoint.
k3d kubeconfig get "$cluster" | sed 's#https://0\.0\.0\.0:#https://127.0.0.1:#' >"$kubeconfig"
k3d image import "$minio_image" -c "$cluster"
ctx=(kubectl --kubeconfig "$kubeconfig" -n "$namespace")
wait_for_kubernetes_api() {
  local timeout_seconds="${EUBL21_YBC_KUBERNETES_READY_TIMEOUT_SECONDS:-180}"
  local deadline
  local last_error

  [[ "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || {
    echo "EUBL21_YBC_KUBERNETES_READY_TIMEOUT_SECONDS must be a positive integer" >&2
    exit 2
  }
  deadline=$((SECONDS + timeout_seconds))
  until last_error="$(kubectl --kubeconfig "$kubeconfig" --request-timeout=5s \
      get --raw=/readyz 2>&1)" && [[ "$last_error" == *ok* ]]; do
    if (( SECONDS >= deadline )); then
      printf '%s\n' "$last_error" >"$artifact_dir/kubernetes-api-readiness.txt"
      echo "Kubernetes API for k3d cluster $cluster did not become ready within ${timeout_seconds}s; see $artifact_dir/kubernetes-api-readiness.txt" >&2
      return 1
    fi
    sleep 2
  done
}

wait_for_kubernetes_api
ysql_on_deployment() {
  local deployment=$1
  local pod_ip
  shift

  pod_ip="$("${ctx[@]}" get pod -l "app=$deployment" \
    -o jsonpath='{.items[0].status.podIP}')"
  [[ "$pod_ip" =~ ^[0-9a-fA-F:.]+$ ]] || {
    echo "could not resolve a Pod IP for deployment/$deployment" >&2
    return 1
  }
  "${ctx[@]}" exec "deployment/$deployment" -- \
    env PGPASSWORD=yugabyte /home/yugabyte/bin/ysqlsh --host "$pod_ip" --port 5433 "$@"
}

wait_for_ysql() {
  local deployment=$1
  local timeout_seconds="${EUBL21_YBC_YSQL_READY_TIMEOUT_SECONDS:-600}"
  local deadline
  local last_error

  [[ "$timeout_seconds" =~ ^[1-9][0-9]*$ ]] || {
    echo "EUBL21_YBC_YSQL_READY_TIMEOUT_SECONDS must be a positive integer" >&2
    exit 2
  }
  deadline=$((SECONDS + timeout_seconds))
  until last_error="$(ysql_on_deployment "$deployment" \
      --username yugabyte --dbname yugabyte --no-psqlrc --no-align \
      --tuples-only --command 'SELECT 1' 2>&1)"; do
    if (( SECONDS >= deadline )); then
      echo "YSQL on deployment/$deployment did not become ready within ${timeout_seconds}s" >&2
      printf '%s\n' "$last_error" >&2
      return 1
    fi
    sleep 2
  done
}
"${ctx[@]}" create namespace "$namespace"
# Generated, namespace-local MinIO credentials are used only by the disposable
# qualification. They are never printed or copied to the evidence directory.
minio_user="minio-$run_id"; minio_password="$(od -An -N24 -tx1 /dev/urandom | tr -d ' \n')"
"${ctx[@]}" create secret generic minio-test-identity --from-literal=MINIO_ROOT_USER="$minio_user" --from-literal=MINIO_ROOT_PASSWORD="$minio_password"
"${ctx[@]}" apply -f - <<YAML
apiVersion: apps/v1
kind: Deployment
metadata: {name: minio}
spec:
  replicas: 1
  selector: {matchLabels: {app: minio}}
  template:
    metadata: {labels: {app: minio}}
    spec:
      containers:
      - name: minio
        image: $minio_image
        command: [/minio]
        args: [server, /data]
        envFrom: [{secretRef: {name: minio-test-identity}}]
        ports: [{containerPort: 9000}]
---
apiVersion: v1
kind: Service
metadata: {name: minio}
spec: {selector: {app: minio}, ports: [{port: 9000, targetPort: 9000}]}
YAML
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
        command: [bash, -ec, 'mkdir -p ybc && tar -xzf share/ybc-*-linux-x86_64.tar.gz -C ybc --strip-components=1 && exec bin/yugabyted start --base_dir=/tmp/yb --background=false --ysql_enable_auth=true --backup_daemon=true']
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
        command: [bash, -ec, 'mkdir -p ybc && tar -xzf share/ybc-*-linux-x86_64.tar.gz -C ybc --strip-components=1 && exec bin/yugabyted start --base_dir=/tmp/yb --background=false --ysql_enable_auth=true --backup_daemon=true']
        ports: [{containerPort: 7100}, {containerPort: 5433}, {containerPort: 18018}]
---
apiVersion: v1
kind: Service
metadata: {name: yb-target}
spec: {selector: {app: yb-target}, ports: [{name: master, port: 7100}, {name: ysql, port: 5433}, {name: controller, port: 18018}]}
YAML
"${ctx[@]}" wait --for=condition=Available deployment/yb-source deployment/yb-target --timeout=10m
wait_for_ysql yb-source
"${ctx[@]}" create secret generic s3-test-identity --from-literal=AWS_ACCESS_KEY_ID="$minio_user" --from-literal=AWS_SECRET_ACCESS_KEY="$minio_password"
docker build -t "eubl21-ybc-control-plane:$run_id" -f "$root/deploy/production/yugabytedb/ybc-control-plane/Dockerfile" "$root/deploy/production/yugabytedb/ybc-control-plane"
k3d image import "eubl21-ybc-control-plane:$run_id" -c "$cluster"
"${ctx[@]}" apply -f - <<YAML
apiVersion: batch/v1
kind: Job
metadata:
  name: minio-init
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: minio-init
        image: eubl21-ybc-control-plane:$run_id
        command: [aws]
        args: [--endpoint-url, http://minio:9000, --region, us-east-1, s3api, create-bucket, --bucket, eubl21-ybc]
        envFrom: [{secretRef: {name: s3-test-identity}}]
YAML
"${ctx[@]}" wait --for=condition=complete job/minio-init --timeout=180s
"${ctx[@]}" create configmap ybc-policy --from-literal=database=eubl21 --from-literal=controllerAddress=yb-source --from-literal=objectStoreUri=s3://eubl21-ybc/evidence --from-literal=objectStoreEndpoint=http://minio:9000 --from-literal=objectStoreRegion=us-east-1 --from-literal='retention={"keepFull":1}'
run_control_plane() {
  local name=$1; shift
  local runner_command
  local job_status
  printf -v runner_command '%q ' /usr/local/bin/eubl21-ybc-control-plane "$@"
  # The full Pod template is supplied at creation; Jobs are never patched after
  # admission, so the mounted policy and short-lived test identity are immutable.
  "${ctx[@]}" apply -f - <<YAML
apiVersion: batch/v1
kind: Job
metadata:
  name: $name
spec:
  template:
    spec:
      restartPolicy: Never
      volumes: [{name: policy, configMap: {name: ybc-policy}}]
      containers:
      - name: $name
        image: eubl21-ybc-control-plane:$run_id
        command: [/bin/sh, -ec]
        args:
        - |
          exec $runner_command
        env:
        - name: AWS_DEFAULT_REGION
          value: us-east-1
        # YBC reads this endpoint directly for S3-compatible stores; without
        # it, credential validation is sent to AWS instead of local MinIO.
        - name: AWS_ENDPOINT
          value: http://minio:9000
        envFrom: [{secretRef: {name: s3-test-identity}}]
        volumeMounts: [{name: policy, mountPath: /etc/eubl21, readOnly: true}]
YAML
  # `kubectl wait --for=condition=complete` does not stop when a Job has
  # already failed; it waits for its entire timeout. Poll both terminal states
  # so a controller error is reported promptly and its logs are retained.
  while :; do
    job_status="$("${ctx[@]}" get "job/$name" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{" "}{end}')"
    case " $job_status " in
      *' Complete=True '*) break;;
      *' Failed=True '*)
        "${ctx[@]}" logs "job/$name" --all-containers=true >"$artifact_dir/$name.log" 2>&1 || true
        echo "job/$name failed; see $artifact_dir/$name.log" >&2
        return 1
        ;;
    esac
    sleep 2
  done
  "${ctx[@]}" logs "job/$name" --all-containers=true >"$artifact_dir/$name.log"
}
# Full snapshot, then a distinct post-full write, then its incremental chain.
ysql_on_deployment yb-source -c 'CREATE DATABASE eubl21;'
ysql_on_deployment yb-source -d eubl21 -c "CREATE TABLE qualification (id int PRIMARY KEY, payload text); INSERT INTO qualification VALUES (1, 'full');"
run_control_plane ybc-full backup --kind=full --policy=/etc/eubl21
ysql_on_deployment yb-source -d eubl21 -c "INSERT INTO qualification VALUES (2, 'incremental');"
# Destroy the untouched target and create a fresh universe only after both
# source backups exist; restoration can therefore never be in-place.
"${ctx[@]}" delete deployment yb-target --wait=true
"${ctx[@]}" apply -f - <<'YAML'
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
        command: [bash, -ec, 'mkdir -p ybc && tar -xzf share/ybc-*-linux-x86_64.tar.gz -C ybc --strip-components=1 && exec bin/yugabyted start --base_dir=/tmp/yb --background=false --ysql_enable_auth=true --backup_daemon=true']
        ports: [{containerPort: 7100}, {containerPort: 5433}, {containerPort: 18018}]
YAML
"${ctx[@]}" wait --for=condition=Available deployment/yb-target --timeout=10m
wait_for_ysql yb-target
run_control_plane ybc-restore restore --policy=/etc/eubl21 --target-controller=yb-target
# Yugabyted binds YSQL to the Pod IP, whereas kubectl port-forward connects to
# loopback in the Pod's network namespace. Relay only this disposable test
# connection from loopback to the current target Pod IP.
target_pod="$("${ctx[@]}" get pod -l app=yb-target -o jsonpath='{.items[0].metadata.name}')"
target_pod_ip="$("${ctx[@]}" get pod "$target_pod" -o jsonpath='{.status.podIP}')"
"${ctx[@]}" exec "pod/$target_pod" -- env TARGET_POD_IP="$target_pod_ip" python3 -c '
import os
import socket
import threading

listener = socket.socket()
listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
listener.bind(("127.0.0.1", 15435))
listener.listen()
target = (os.environ["TARGET_POD_IP"], 5433)

def relay(source, destination):
    try:
        while data := source.recv(65536):
            destination.sendall(data)
    finally:
        destination.shutdown(socket.SHUT_WR)

while True:
    client, _ = listener.accept()
    upstream = socket.create_connection(target)
    threading.Thread(target=relay, args=(client, upstream), daemon=True).start()
    threading.Thread(target=relay, args=(upstream, client), daemon=True).start()
' >"$artifact_dir/target-ysql-loopback-relay.log" 2>&1 & relay=$!
"${ctx[@]}" port-forward "pod/$target_pod" 15435:15435 >"$artifact_dir/target-port-forward.log" 2>&1 & pf=$!
trap 'kill "$pf" "$relay" >/dev/null 2>&1 || true; cleanup' EXIT
sleep 3
EUBL21_TEST_DATABASE_URL='postgresql://yugabyte:yugabyte@127.0.0.1:15435/eubl21?sslmode=disable' go test ./database -run '^TestYugabyteDBIntegration$' -count=1 | tee "$artifact_dir/application-integration.txt"
kill "$pf" "$relay" >/dev/null 2>&1 || true
run_control_plane ybc-retention retention --policy=/etc/eubl21
run_control_plane ybc-restore restore --policy=/etc/eubl21 --target-controller=yb-target
ysql_on_deployment yb-target -d eubl21 -Atc 'SELECT payload FROM qualification ORDER BY id' | tee "$artifact_dir/restored-data.txt"
grep -Fxq full "$artifact_dir/restored-data.txt"; grep -Fxq incremental "$artifact_dir/restored-data.txt"
"${ctx[@]}" get configmap ybc-policy -o yaml >"$artifact_dir/policy.yaml"
echo "K3d/MinIO YBC full/incremental/retention/isolated-restore qualification passed: $artifact_dir"
