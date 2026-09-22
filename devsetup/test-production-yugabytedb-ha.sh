#!/usr/bin/env bash
set -Eeuo pipefail
root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
chart_url="https://charts.yugabyte.com/yugabyte-2025.2.6.tgz"
chart_sha256="26518b8dfb7389dd6e97bc034df59838236c1a0c15d6f7482a1cf9a7dd4b80a0"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$chart_url" -o "$tmp/chart.tgz"
printf '%s  %s\n' "$chart_sha256" "$tmp/chart.tgz" | sha256sum --check --status
for zone in zone-a zone-b zone-c; do
  values="$root_dir/deploy/production/yugabytedb/$zone.yaml"
  ! rg -n 'REPLACE_|TODO|CHANGEME|<[^>]+>' "$values"
  rg -q 'enabled: true' "$values"
	  rg -A3 '^ybc:' "$values" | rg -q 'enabled: true'
  rg -q 'bootstrapSelfsigned: false' "$values"
  rg -q 'useCustomIssuer: true' "$values"
  rg -q 'name: eubl21-yugabytedb-ca' "$values"
  rg -q 'size: 100Gi' "$values"
  rg -q 'size: 500Gi' "$values"
  rg -q 'requests: \{cpu: "2", memory: 8Gi\}' "$values"
  rg -q 'limits: \{cpu: "2", memory: 8Gi\}' "$values"
  rg -q 'requests: \{cpu: "4", memory: 16Gi\}' "$values"
  rg -q 'limits: \{cpu: "8", memory: 32Gi\}' "$values"
  rg -q 'storageClass: fast-ssd' "$values"
  rg -q 'placement_region: primary-region' "$values"
  helm template "eubl21-yb-$zone" "$tmp/chart.tgz" --namespace "eubl21-yb-$zone" --values "$values" --set-string storage.master.storageClass=standard --set-string storage.tserver.storageClass=standard --set-string gflags.master.placement_region=test --set-string gflags.tserver.placement_region=test --set-string tls.rootCA.cert=ZHVtbXktc2hhcmVkLWNh >"$tmp/$zone.yaml"
  rg -q 'totalMasters: 3' "$root_dir/deploy/production/yugabytedb/$zone.yaml"
  rg -q 'topology.kubernetes.io/zone' "$tmp/$zone.yaml"
done
for rendered in "$tmp"/*.yaml; do
  namespace="$(basename "$rendered" .yaml | sed 's/^/eubl21-yb-/')"
  awk -v default_namespace="$namespace" '
    function reset() { kind=""; name=""; namespace=default_namespace; metadata=0 }
    function emit() {
      if (kind == "") return
      if (kind ~ /^(ClusterRole|ClusterRoleBinding|CustomResourceDefinition|Namespace|Node|PersistentVolume|PriorityClass|StorageClass|ValidatingWebhookConfiguration|MutatingWebhookConfiguration)$/) namespace="_cluster"
      if (name != "") print kind "/" namespace "/" name
    }
    /^---$/ { emit(); reset(); next }
    /^kind:/ { kind=$2; metadata=0; next }
    /^metadata:/ { metadata=1; next }
    metadata && /^  name:/ { name=$2; next }
    metadata && /^  namespace:/ { namespace=$2; next }
    metadata && /^[^ ]/ { metadata=0 }
    END { emit() }
  ' "$rendered"
done | sort | uniq -d | (! grep .)
observability="$root_dir/deploy/production/yugabytedb/observability.yaml"
dashboard="$root_dir/deploy/production/yugabytedb/dashboard.yaml"
rg -q 'http-ysql-met' "$observability"
rg -q 'follower_lag_ms' "$observability"
rg -q 'YugabyteDBConsensusHealth' "$observability"
rg -q 'YugabyteDBYSQLLatencyHigh' "$observability"
rg -q 'handler_latency_yb_ysqlserver_SQLProcessor_' "$observability" "$dashboard"
echo "Production YugabyteDB HA Helm renders and resource identities passed."
