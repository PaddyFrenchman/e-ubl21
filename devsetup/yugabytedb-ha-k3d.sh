#!/usr/bin/env bash
# Disposable three-agent K3d qualification for production YugabyteDB HA.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
k3d_bin="${K3D_BIN:-$(go env GOPATH)/bin/k3d}"
run_id="${EUBL21_HA_RUN_ID:-$(date -u +%y%m%d%H%M%S)-$$}"
# K3d restricts cluster names to 32 characters.
cluster="${EUBL21_HA_K3D_CLUSTER:-eubl21-ha-$run_id}"
tmp="$(mktemp -d)"; kubeconfig="$tmp/kubeconfig"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-ha/$run_id}"
cleanup() {
  mkdir -p "$artifact_dir"
  if [[ -f "$kubeconfig" ]]; then
    KUBECONFIG="$kubeconfig" kubectl get pods -A -o wide >"$artifact_dir/pods.txt" 2>&1 || true
    KUBECONFIG="$kubeconfig" kubectl get events -A --sort-by=.lastTimestamp >"$artifact_dir/events.txt" 2>&1 || true
    KUBECONFIG="$kubeconfig" kubectl logs -n cert-manager -l app.kubernetes.io/instance=cert-manager --all-containers=true >"$artifact_dir/cert-manager.log" 2>&1 || true
    for ns in eubl21-yb-zone-a eubl21-yb-zone-b eubl21-yb-zone-c; do
      KUBECONFIG="$kubeconfig" kubectl get pods -n "$ns" -o yaml >"$artifact_dir/$ns-pods.yaml" 2>&1 || true
      KUBECONFIG="$kubeconfig" kubectl describe pods -n "$ns" >"$artifact_dir/$ns-describe.txt" 2>&1 || true
      KUBECONFIG="$kubeconfig" kubectl logs -n "$ns" -l app=yb-master --all-containers=true --prefix >"$artifact_dir/$ns-master.log" 2>&1 || true
      KUBECONFIG="$kubeconfig" kubectl logs -n "$ns" -l app=yb-tserver --all-containers=true --prefix >"$artifact_dir/$ns-tserver.log" 2>&1 || true
    done
    helm list -A >"$artifact_dir/helm-releases.txt" 2>&1 || true
    kubectl get certificate,clusterissuer -A -o yaml >"$artifact_dir/tls-resources.yaml" 2>&1 || true
  fi
  "$k3d_bin" cluster delete "$cluster" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT
command -v helm >/dev/null; command -v kubectl >/dev/null; [[ -x "$k3d_bin" ]]
[[ "$("$k3d_bin" version | head -n1)" == "k3d version v5.9.0" ]] || {
  echo "K3d v5.9.0 release binary is required; development builds are unsupported" >&2
  exit 2
}
mkdir -p "$artifact_dir"
"$root/devsetup/test-production-yugabytedb-ha.sh"
cluster_create_timeout="${EUBL21_HA_K3D_CREATE_TIMEOUT:-5m}"
echo "Creating disposable K3d cluster (timeout: $cluster_create_timeout)..."
"$k3d_bin" cluster create "$cluster" --agents 0 --wait --timeout "$cluster_create_timeout" \
  --image rancher/k3s:v1.32.5-k3s1 \
  --runtime-ulimit nofile=1048576:1048576 \
  --k3s-arg '--disable=traefik@server:0' \
  --kubeconfig-update-default=false --kubeconfig-switch-context=false
for n in 0 1 2; do
  "$k3d_bin" node create "$cluster-agent-$n" --cluster "$cluster" --role agent --wait --timeout "$cluster_create_timeout" \
    --runtime-ulimit nofile=1048576:1048576
done
"$k3d_bin" kubeconfig get "$cluster" >"$kubeconfig"
export KUBECONFIG="$kubeconfig"
echo "Waiting for all K3d nodes to register..."
deadline=$((SECONDS + 300))
while true; do
  registered_nodes="$(kubectl get nodes --no-headers 2>/dev/null | wc -l || true)"
  ready_nodes="$(kubectl get nodes --no-headers 2>/dev/null | awk '$2 == "Ready" { count++ } END { print count+0 }' || true)"
  if [[ "$registered_nodes" -eq 4 && "$ready_nodes" -eq 4 ]]; then
    break
  fi
  (( SECONDS < deadline )) || { echo "K3d nodes did not all register within 5 minutes (registered=$registered_nodes ready=$ready_nodes)" >&2; exit 1; }
  sleep 2
done
zones=(zone-a zone-b zone-c)
for n in 0 1 2; do
  kubectl label node "k3d-$cluster-agent-$n" topology.kubernetes.io/zone="${zones[$n]}" --overwrite
  # The pinned chart currently renders this deprecated selector as well.
  kubectl label node "k3d-$cluster-agent-$n" failure-domain.beta.kubernetes.io/zone="${zones[$n]}" --overwrite
done
helm repo add jetstack https://charts.jetstack.io --force-update >/dev/null
helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true --wait --timeout 10m
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: eubl21-yb-ha-ca
  namespace: cert-manager
spec:
  isCA: true
  commonName: eubl21-yb-ha-ca
  secretName: eubl21-yb-ha-ca
  issuerRef: {name: eubl21-yb-ha-bootstrap, kind: ClusterIssuer}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: eubl21-yb-ha-bootstrap}
spec: {selfSigned: {}}
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: eubl21-yb-ha-ca}
spec:
  ca: {secretName: eubl21-yb-ha-ca}
EOF
kubectl -n cert-manager wait --for=condition=Ready certificate/eubl21-yb-ha-ca --timeout=5m
kubectl wait --for=condition=Ready clusterissuer/eubl21-yb-ha-ca --timeout=5m
kubectl -n cert-manager get secret eubl21-yb-ha-ca -o jsonpath='{.data.tls\.crt}' >"$tmp/shared-ca.b64"
[[ -s "$tmp/shared-ca.b64" ]] || { echo "generated shared CA is missing" >&2; exit 1; }
printf 'tls:\n  rootCA:\n    cert: %s\n' "$(<"$tmp/shared-ca.b64")" >"$tmp/shared-ca-values.yaml"
for zone in zone-a zone-b zone-c; do
  ns="eubl21-yb-$zone"; kubectl create namespace "$ns"
  helm upgrade --install "eubl21-yb-$zone" https://charts.yugabyte.com/yugabyte-2025.2.6.tgz --namespace "$ns" -f "$root/deploy/production/yugabytedb/$zone.yaml" -f "$tmp/shared-ca-values.yaml"     --set-string storage.master.storageClass=local-path --set-string storage.tserver.storageClass=local-path     --set-string gflags.master.placement_region=local --set-string gflags.tserver.placement_region=local     --set tls.certManager.bootstrapSelfsigned=false --set tls.certManager.useCustomIssuer=true --set-string tls.certManager.customIssuer.kind=ClusterIssuer --set-string tls.certManager.customIssuer.name=eubl21-yb-ha-ca --set-string tls.certManager.customIssuer.group=cert-manager.io     --set preflight.skipUlimit=true     --set resource.master.requests.cpu=500m --set resource.master.requests.memory=1Gi --set resource.master.limits.cpu=1 --set resource.master.limits.memory=2Gi     --set resource.tserver.requests.cpu=500m --set resource.tserver.requests.memory=1Gi --set resource.tserver.limits.cpu=1 --set resource.tserver.limits.memory=2Gi
  kubectl -n "$ns" rollout status statefulset/yb-master --timeout=10m
  kubectl -n "$ns" rollout status statefulset/yb-tserver --timeout=10m
  kubectl -n "$ns" wait --for=condition=Ready certificate/yb-master-yugabyte-tls-cert --timeout=5m
  kubectl -n "$ns" wait --for=condition=Ready certificate/yb-tserver-yugabyte-tls-cert --timeout=5m
  kubectl -n "$ns" wait --for=condition=Ready certificate/yugabyte-tls-client-cert --timeout=5m
  helm get manifest "eubl21-yb-$zone" -n "$ns" >"$artifact_dir/$zone-rendered.yaml"
done

# The chart generates this client credential from the shared ClusterIssuer.
kubectl -n cert-manager get secret eubl21-yb-ha-ca -o jsonpath='{.data.tls\.crt}' | base64 -d >"$tmp/ca.crt"
[[ -s "$tmp/ca.crt" ]] || { echo "generated shared CA is missing" >&2; exit 1; }
kubectl -n eubl21-yb-zone-a get secret yugabyte-tls-client-cert -o jsonpath='{.data.tls\.crt}' | base64 -d >"$tmp/tls.crt"
kubectl -n eubl21-yb-zone-a get secret yugabyte-tls-client-cert -o jsonpath='{.data.tls\.key}' | base64 -d >"$tmp/tls.key"
[[ -s "$tmp/tls.crt" && -s "$tmp/tls.key" ]] || { echo "generated TLS client secret is incomplete" >&2; exit 1; }
chmod 0600 "$tmp/tls.key"

base_port=$((20000 + RANDOM % 20000))
forwarders=()
cleanup_forwarders() { for pid in "${forwarders[@]:-}"; do kill "$pid" 2>/dev/null || true; done; }
trap 'cleanup_forwarders; cleanup' EXIT
for index in 0 1 2; do
  zone="${zones[$index]}"; port=$((base_port + index))
  ( while true; do kubectl -n "eubl21-yb-$zone" port-forward "service/yb-tservers" "$port:5433" >/dev/null 2>&1 || true; sleep 1; done ) &
  forwarders+=("$!")
done
for index in 0 1 2; do until nc -z 127.0.0.1 "$((base_port + index))"; do sleep 1; done; done
backend_file="$tmp/proxy-backends"
printf '127.0.0.1:%s\n' "$base_port" >"$backend_file"
all_backends="127.0.0.1:$base_port,127.0.0.1:$((base_port + 1)),127.0.0.1:$((base_port + 2))"
go run "$root/devsetup/ha-tcp-proxy.go" -backends-file="$backend_file" >"$tmp/proxy-address" 2>"$artifact_dir/tcp-proxy.log" &
proxy_pid=$!
trap 'kill "$proxy_pid" 2>/dev/null || true; cleanup_forwarders; cleanup' EXIT
deadline=$((SECONDS + 60)); until [[ -s "$tmp/proxy-address" ]]; do (( SECONDS < deadline )) || { echo "TCP HA gateway did not start" >&2; exit 1; }; sleep 1; done
proxy_port="$(sed 's/.*://' "$tmp/proxy-address")"
# pgx accepts TLS file paths as URL query values; percent-encode them so the
# generated path cannot be mistaken for URL syntax.
url="postgresql://yugabyte:yugabyte@127.0.0.1:$proxy_port/yugabyte?sslmode=verify-ca&sslrootcert=$(printf %s "$tmp/ca.crt" | sed 's#/#%2F#g')&sslcert=$(printf %s "$tmp/tls.crt" | sed 's#/#%2F#g')&sslkey=$(printf %s "$tmp/tls.key" | sed 's#/#%2F#g')"
# Kubernetes marks the StatefulSet Ready before the chart's final tserver
# startup delay and a newly formed RF-3 universe can take several minutes to
# elect leaders. Do not race that process with the coordinated failover test.
ysql_ready_log="$artifact_dir/ysql-readiness.log"
ysql_ready_timeout_seconds="${EUBL21_HA_YSQL_READY_TIMEOUT_SECONDS:-600}"
[[ "$ysql_ready_timeout_seconds" =~ ^[1-9][0-9]*$ ]] || { echo "EUBL21_HA_YSQL_READY_TIMEOUT_SECONDS must be a positive integer" >&2; exit 2; }
deadline=$((SECONDS + ysql_ready_timeout_seconds))
until EUBL21_TEST_DATABASE_URL="$url" go test ./database -run '^TestYugabyteDBIntegration$/connect_and_query$' -count=1 >"$ysql_ready_log" 2>&1; do
  if (( SECONDS >= deadline )); then
    cat "$ysql_ready_log" >&2
    echo "YSQL did not become ready through the TLS HA gateway within ${ysql_ready_timeout_seconds}s" >&2
    exit 1
  fi
  sleep 2
done
EUBL21_HA_DATABASE_URL="$url" \
EUBL21_HA_TSERVER_POD=yb-tserver-0 EUBL21_HA_TSERVER_NAMESPACE=eubl21-yb-zone-a \
EUBL21_HA_MASTER_POD=yb-master-0 EUBL21_HA_MASTER_NAMESPACE=eubl21-yb-zone-a \
EUBL21_HA_READY_SELECTOR='app=yb-tserver' EUBL21_HA_TSERVER_COUNT=3 \
EUBL21_HA_FAILURE_NODE="k3d-$cluster-agent-0" EUBL21_HA_BACKENDS_FILE="$backend_file" EUBL21_HA_FAILOVER_BACKENDS="$all_backends" \
"$root/devsetup/test-production-yugabytedb-ha-integration.sh"
echo "K3d three-zone YugabyteDB TLS HA qualification passed."
