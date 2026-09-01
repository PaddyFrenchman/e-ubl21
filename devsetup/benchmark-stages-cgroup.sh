#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CONTAINER_CLI="${CONTAINER_CLI:-docker}"
CAPACITY_IMAGE="${CAPACITY_IMAGE:-e-ubl21:stage-capacity}"
CAPACITY_SAMPLES="${CAPACITY_SAMPLES:-100}"
if ! command -v "$CONTAINER_CLI" >/dev/null 2>&1; then
    echo "ERROR: ${CONTAINER_CLI} is required for the constrained EN 16931 benchmark." >&2
    exit 1
fi
if [[ ! "$CAPACITY_SAMPLES" =~ ^[0-9]+$ ]] || (( CAPACITY_SAMPLES < 100 || CAPACITY_SAMPLES > 500 )); then
    echo "ERROR: CAPACITY_SAMPLES must be from 100 through 500." >&2
    exit 2
fi

"$CONTAINER_CLI" build --target capacity-benchmark -t "$CAPACITY_IMAGE" .
"$CONTAINER_CLI" run --rm \
    --cpus=2 \
    --memory=2g \
    --memory-swap=2g \
    --pids-limit=2048 \
    --env "EUBL21_CAPACITY_SAMPLES=${CAPACITY_SAMPLES}" \
    --env EUBL21_REQUIRE_EN16931_TEST_RUNTIME=1 \
    "$CAPACITY_IMAGE" \
    sh -euc '
        echo "Constrained EN 16931 benchmark cgroup:"
        echo "  cpu.max=$(cat /sys/fs/cgroup/cpu.max)"
        echo "  memory.max=$(cat /sys/fs/cgroup/memory.max)"
        echo "  capacity_samples=${EUBL21_CAPACITY_SAMPLES}"
        echo "  $(go version)"
        java -version 2>&1 | sed -n "1,3p"
        xmllint --version 2>&1 | sed -n "1p"
        go run ./tools/verify-validation-assets -saxon-dir ./devsetup/tools/saxon
        cd /src/server
        status=0
        /usr/local/bin/stage-capacity.test -test.run "^$" \
            -test.bench "^(BenchmarkEN16931StageCapacity|BenchmarkEN16931ProcessCost)$" \
            -test.benchtime 1x -test.count 1 -test.timeout 90m || status=$?
        echo "capacity_cgroup_memory_peak_bytes=$(cat /sys/fs/cgroup/memory.peak)"
        sed "s/^/capacity_cgroup_cpu_/" /sys/fs/cgroup/cpu.stat
        exit "$status"
    '
