#!/usr/bin/env bash
set -Eeuo pipefail

repo="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
export PATH="/home/a/go/bin:$PATH"

docker image inspect eubl21/minio:RELEASE.2025-10-15T17-29-55Z >/dev/null

export EUBL21_YBC_KEEP_CLUSTER="${EUBL21_YBC_KEEP_CLUSTER:-1}"
export ARTIFACT_DIR="${ARTIFACT_DIR:-$repo/regression-results/yugabytedb-ybc-control-plane}"

exec "$repo/devsetup/test-yugabytedb-ybc-control-plane-runtime.sh"