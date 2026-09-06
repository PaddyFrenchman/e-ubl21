#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ARTIFACT_DIR="${ARTIFACT_DIR:-performance-results/local}"
CAPACITY_SAMPLES="${CAPACITY_SAMPLES:-100}"
RUN_CGROUP="${RUN_CGROUP:-0}"
PERFORMANCE_BASELINE="${PERFORMANCE_BASELINE:-}"
PERFORMANCE_REGRESSION_THRESHOLD="${PERFORMANCE_REGRESSION_THRESHOLD:-25}"
PERFORMANCE_MAX_TREE_RSS_KIB="${PERFORMANCE_MAX_TREE_RSS_KIB:-0}"
PERFORMANCE_MAX_CGROUP_MEMORY_BYTES="${PERFORMANCE_MAX_CGROUP_MEMORY_BYTES:-0}"

if [[ "$RUN_CGROUP" != "0" && "$RUN_CGROUP" != "1" ]]; then
    echo "ERROR: RUN_CGROUP must be 0 or 1." >&2
    exit 2
fi
if [[ ! "$CAPACITY_SAMPLES" =~ ^[0-9]+$ ]] || (( CAPACITY_SAMPLES < 100 || CAPACITY_SAMPLES > 500 )); then
    echo "ERROR: CAPACITY_SAMPLES must be from 100 through 500." >&2
    exit 2
fi

mkdir -p "$ARTIFACT_DIR"
benchmark_status=0

set +e
MEASURE_TREE_RSS=1 CAPACITY_SAMPLES="$CAPACITY_SAMPLES" \
    make benchmark-stages 2>&1 | tee "$ARTIFACT_DIR/host.log"
host_status=${PIPESTATUS[0]}
set -e
if (( host_status != 0 )); then
    benchmark_status=$host_status
fi

inputs=(-input "host=$ARTIFACT_DIR/host.log")
if [[ "$RUN_CGROUP" == "1" ]]; then
    set +e
    CAPACITY_SAMPLES="$CAPACITY_SAMPLES" \
        make benchmark-stages-cgroup 2>&1 | tee "$ARTIFACT_DIR/constrained.log"
    constrained_status=${PIPESTATUS[0]}
    set -e
    if (( constrained_status != 0 && benchmark_status == 0 )); then
        benchmark_status=$constrained_status
    fi
    inputs+=(-input "constrained=$ARTIFACT_DIR/constrained.log")
fi

comparison=()
if [[ -n "$PERFORMANCE_BASELINE" && -f "$PERFORMANCE_BASELINE" ]]; then
    cp "$PERFORMANCE_BASELINE" "$ARTIFACT_DIR/prior-evidence.json"
    comparison=(-baseline "$PERFORMANCE_BASELINE")
fi

export GITHUB_SHA="${GITHUB_SHA:-$(git rev-parse HEAD)}"
export GITHUB_EVENT_NAME="${GITHUB_EVENT_NAME:-local}"

summary_status=0
go run ./tools/performance-evidence \
    "${inputs[@]}" \
    "${comparison[@]}" \
    -output "$ARTIFACT_DIR/evidence.json" \
    -markdown "$ARTIFACT_DIR/summary.md" \
    -regression-threshold-percent "$PERFORMANCE_REGRESSION_THRESHOLD" \
    -max-tree-rss-kib "$PERFORMANCE_MAX_TREE_RSS_KIB" \
    -max-cgroup-memory-bytes "$PERFORMANCE_MAX_CGROUP_MEMORY_BYTES" || summary_status=$?

if (( benchmark_status != 0 )); then
    exit "$benchmark_status"
fi
exit "$summary_status"
