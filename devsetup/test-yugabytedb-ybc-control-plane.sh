#!/usr/bin/env bash
# Preflight the e-ubl21-owned YBC control-plane contract. The image invoked by
# the CronJobs performs the K3d + MinIO runtime qualification and retains its
# evidence under regression-results/yugabytedb-ybc-control-plane.
set -Eeuo pipefail
root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$root/deploy/production/yugabytedb/backup-control-plane.yaml"
command -v kubectl >/dev/null
kubectl apply --dry-run=client -f "$manifest" >/dev/null
rg -q 'eubl21-ybc-backup-full' "$manifest"
rg -q 'eubl21-ybc-backup-incremental' "$manifest"
rg -q 'concurrencyPolicy: Forbid' "$manifest"
rg -q -- '--kind=full' "$manifest"
rg -q -- '--kind=incremental' "$manifest"
rg -q '35 daily, 12 monthly, 7 yearly' "$manifest"
echo "YBC control-plane manifest preflight passed."
