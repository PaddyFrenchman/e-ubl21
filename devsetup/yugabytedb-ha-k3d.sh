#!/usr/bin/env bash
# Disposable three-agent K3d qualification for production YugabyteDB HA.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
k3d_bin="${K3D_BIN:-$(go env GOPATH)/bin/k3d}"
cluster="${EUBL21_HA_K3D_CLUSTER:-eubl21-yb-ha}"
tmp="$(mktemp -d)"; kubeconfig="$tmp/kubeconfig"
cleanup() { "$k3d_bin" cluster delete "$cluster" >/dev/null 2>&1 || true; rm -rf "$tmp"; }
trap cleanup EXIT
command -v helm >/dev/null; command -v kubectl >/dev/null; [[ -x "$k3d_bin" ]]
"$root/devsetup/test-production-yugabytedb-ha.sh"
"$k3d_bin" cluster create "$cluster" --agents 3 --wait
"$k3d_bin" kubeconfig write "$cluster" >"$kubeconfig"
export KUBECONFIG="$kubeconfig"
zones=(zone-a zone-b zone-c)
for n in 0 1 2; do kubectl label node "k3d-$cluster-agent-$n" topology.kubernetes.io/zone="${zones[$n]}" --overwrite; done
helm repo add jetstack https://charts.jetstack.io --force-update >/dev/null
helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true --wait
kubectl apply -f - <<'EOF'
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata: {name: eubl21-yb-ha-ca}
spec: {selfSigned: {}}
EOF
for zone in zone-a zone-b zone-c; do
  ns="eubl21-yb-$zone"; kubectl create namespace "$ns"
  helm upgrade --install "eubl21-yb-$zone" https://charts.yugabyte.com/yugabyte-2025.2.6.tgz --namespace "$ns" -f "$root/deploy/production/yugabytedb/$zone.yaml"     --set-string storage.master.storageClass=local-path --set-string storage.tserver.storageClass=local-path     --set-string gflags.master.placement_region=local --set-string gflags.tserver.placement_region=local     --set tls.certManager.useClusterIssuer=true --set tls.certManager.bootstrapSelfsigned=false --set-string tls.certManager.clusterIssuer=eubl21-yb-ha-ca     --set resource.master.requests.cpu=500m --set resource.master.requests.memory=1Gi --set resource.master.limits.cpu=1 --set resource.master.limits.memory=2Gi     --set resource.tserver.requests.cpu=500m --set resource.tserver.requests.memory=1Gi --set resource.tserver.limits.cpu=1 --set resource.tserver.limits.memory=2Gi
  kubectl -n "$ns" rollout status statefulset/yb-master --timeout=10m
  kubectl -n "$ns" rollout status statefulset/yb-tserver --timeout=10m
done
echo "K3d three-zone YugabyteDB TLS topology qualified; failure/reconnect phase requires the generated client URL wiring."
