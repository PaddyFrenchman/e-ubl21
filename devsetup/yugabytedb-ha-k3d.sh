#!/usr/bin/env bash
# Disposable three-agent K3d qualification for production YugabyteDB HA.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
k3d_bin="${K3D_BIN:-$(go env GOPATH)/bin/k3d}"
cluster="${EUBL21_HA_K3D_CLUSTER:-eubl21-yb-ha}"
tmp="$(mktemp -d)"; kubeconfig="$tmp/kubeconfig"
artifact_dir="${ARTIFACT_DIR:-$root/regression-results/yugabytedb-ha/$(date -u +%Y%m%dT%H%M%SZ)}"
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
  fi
  "$k3d_bin" cluster delete "$cluster" >/dev/null 2>&1 || true
  rm -rf "$tmp"
}
trap cleanup EXIT
command -v helm >/dev/null; command -v kubectl >/dev/null; [[ -x "$k3d_bin" ]]
"$root/devsetup/test-production-yugabytedb-ha.sh"
"$k3d_bin" cluster create "$cluster" --agents 3 --wait
"$k3d_bin" kubeconfig get "$cluster" >"$kubeconfig"
export KUBECONFIG="$kubeconfig"
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
kind: ClusterIssuer
metadata: {name: eubl21-yb-ha-ca}
spec: {selfSigned: {}}
EOF
for zone in zone-a zone-b zone-c; do
  ns="eubl21-yb-$zone"; kubectl create namespace "$ns"
  helm upgrade --install "eubl21-yb-$zone" https://charts.yugabyte.com/yugabyte-2025.2.6.tgz --namespace "$ns" -f "$root/deploy/production/yugabytedb/$zone.yaml"     --set-string storage.master.storageClass=local-path --set-string storage.tserver.storageClass=local-path     --set-string gflags.master.placement_region=local --set-string gflags.tserver.placement_region=local     --set tls.certManager.bootstrapSelfsigned=false --set tls.certManager.useCustomIssuer=true --set-string tls.certManager.customIssuer.kind=ClusterIssuer --set-string tls.certManager.customIssuer.name=eubl21-yb-ha-ca --set-string tls.certManager.customIssuer.group=cert-manager.io     --set resource.master.requests.cpu=500m --set resource.master.requests.memory=1Gi --set resource.master.limits.cpu=1 --set resource.master.limits.memory=2Gi     --set resource.tserver.requests.cpu=500m --set resource.tserver.requests.memory=1Gi --set resource.tserver.limits.cpu=1 --set resource.tserver.limits.memory=2Gi
  kubectl -n "$ns" rollout status statefulset/yb-master --timeout=10m
  kubectl -n "$ns" rollout status statefulset/yb-tserver --timeout=10m
done

# The chart generates this client credential from the shared ClusterIssuer.
for key in ca.crt tls.crt tls.key; do
  kubectl -n eubl21-yb-zone-a get secret yugabyte-tls-client-cert -o "jsonpath={.data.$key}" | base64 -d >"$tmp/$key"
done
chmod 0600 "$tmp/tls.key"

# Keep one local endpoint available while kubectl reconnects its forwarding
# tunnel when the selected tserver disappears. The Go test retains its one
# pool throughout this operation.
( while true; do kubectl -n eubl21-yb-zone-a port-forward service/yb-tservers 15433:5433 >/dev/null 2>&1 || true; sleep 1; done ) &
forwarder=$!
trap 'kill "$forwarder" 2>/dev/null || true; cleanup' EXIT
until nc -z 127.0.0.1 15433; do sleep 1; done
url="postgresql://yugabyte:yugabyte@127.0.0.1:15433/yugabyte?sslmode=verify-ca&sslrootcert=$tmp/ca.crt&sslcert=$tmp/tls.crt&sslkey=$tmp/tls.key"
EUBL21_HA_DATABASE_URL="$url" \
EUBL21_HA_TSERVER_POD=yb-tserver-0 EUBL21_HA_TSERVER_NAMESPACE=eubl21-yb-zone-a \
EUBL21_HA_MASTER_POD=yb-master-0 EUBL21_HA_MASTER_NAMESPACE=eubl21-yb-zone-a \
EUBL21_HA_READY_SELECTOR='app=yb-tserver' EUBL21_HA_TSERVER_COUNT=3 \
"$root/devsetup/test-production-yugabytedb-ha-integration.sh"
echo "K3d three-zone YugabyteDB TLS HA qualification passed."
