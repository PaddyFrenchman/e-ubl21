#!/usr/bin/env bash
set -Eeuo pipefail
root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
chart_url="https://charts.yugabyte.com/yugabyte-2025.2.6.tgz"
chart_sha256="26518b8dfb7389dd6e97bc034df59838236c1a0c15d6f7482a1cf9a7dd4b80a0"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$chart_url" -o "$tmp/chart.tgz"
printf '%s  %s\n' "$chart_sha256" "$tmp/chart.tgz" | sha256sum --check --status
for zone in zone-a zone-b zone-c; do
  helm template "eubl21-yb-$zone" "$tmp/chart.tgz" --namespace "eubl21-yb-$zone" --values "$root_dir/deploy/production/yugabytedb/$zone.yaml" --set-string storage.master.storageClass=standard --set-string storage.tserver.storageClass=standard >"$tmp/$zone.yaml"
  rg -q 'totalMasters: 3' "$root_dir/deploy/production/yugabytedb/$zone.yaml"
done
awk '/^kind:/{kind=$2} /^  name:/{name=$2} /^  namespace:/{print kind "/" $2 "/" name}' "$tmp"/*.yaml | sort | uniq -d | (! grep .)
echo "Production YugabyteDB HA Helm renders and resource identities passed."
