#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for command_name in go java ps; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the Peppol capacity benchmark." >&2
        exit 1
    fi
done

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
if [[ ! -f "${SAXON_DIR}/saxon-he-12.7.jar" || ! -f "${SAXON_DIR}/xmlresolver-6.0.17.jar" ]]; then
    for command_name in curl sha256sum; do
        command -v "$command_name" >/dev/null 2>&1 || {
            echo "ERROR: ${command_name} is required to download the pinned Saxon runtime." >&2
            exit 1
        }
    done
    "${ROOT_DIR}/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi
go run ./tools/verify-validation-assets -saxon-dir "$SAXON_DIR"

CAPACITY_SAMPLES="${CAPACITY_SAMPLES:-100}"
MEASURE_TREE_RSS="${MEASURE_TREE_RSS:-0}"
if [[ ! "$CAPACITY_SAMPLES" =~ ^[0-9]+$ ]] || (( CAPACITY_SAMPLES < 100 || CAPACITY_SAMPLES > 500 )); then
    echo "ERROR: CAPACITY_SAMPLES must be from 100 through 500." >&2
    exit 2
fi
if [[ "$MEASURE_TREE_RSS" != "0" && "$MEASURE_TREE_RSS" != "1" ]]; then
    echo "ERROR: MEASURE_TREE_RSS must be 0 or 1." >&2
    exit 2
fi

echo "Peppol stage capacity benchmark environment:"
echo "  date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "  os=$(uname -srm)"
echo "  cpu_count=$(getconf _NPROCESSORS_ONLN 2>/dev/null || echo unknown)"
echo "  capacity_samples=${CAPACITY_SAMPLES}"
echo "  measure_tree_rss=${MEASURE_TREE_RSS}"
echo "  $(go version)"
java -version 2>&1 | sed -n '1,3p'

benchmark_tree_rss_kib() {
    local root_pid="$1"
    ps -eo pid=,ppid=,rss= | awk -v root="$root_pid" '
        { parent[$1] = $2; rss[$1] = $3 }
        function include(pid, child) {
            if (seen[pid]) return
            seen[pid] = 1
            total += rss[pid]
            for (child in parent) if (parent[child] == pid) include(child)
        }
        END { include(root); print total + 0 }
    '
}

BENCHMARK_ARGS=(
    go test ./server -run '^$' -bench '^BenchmarkPeppolStageCapacity$'
    -benchtime 1x -count 1 -timeout 60m
)
set +e
if [[ -x /usr/bin/time ]]; then
    EUBL21_CAPACITY_SAMPLES="$CAPACITY_SAMPLES" EUBL21_REQUIRE_EN16931_TEST_RUNTIME=1 /usr/bin/time \
        -f $'capacity_wall_seconds=%e\ncapacity_max_single_process_rss_kib=%M\ncapacity_user_seconds=%U\ncapacity_system_seconds=%S' \
        "${BENCHMARK_ARGS[@]}" &
else
    EUBL21_CAPACITY_SAMPLES="$CAPACITY_SAMPLES" EUBL21_REQUIRE_EN16931_TEST_RUNTIME=1 "${BENCHMARK_ARGS[@]}" &
fi
BENCHMARK_PID=$!
PEAK_TREE_RSS_KIB=0
if [[ "$MEASURE_TREE_RSS" == "1" ]]; then
    while kill -0 "$BENCHMARK_PID" 2>/dev/null; do
        CURRENT_TREE_RSS_KIB="$(benchmark_tree_rss_kib "$BENCHMARK_PID")"
        if (( CURRENT_TREE_RSS_KIB > PEAK_TREE_RSS_KIB )); then
            PEAK_TREE_RSS_KIB="$CURRENT_TREE_RSS_KIB"
        fi
        sleep 0.1
    done
fi
wait "$BENCHMARK_PID"
BENCHMARK_STATUS=$?
set -e

if [[ "$MEASURE_TREE_RSS" == "1" ]]; then
    echo "capacity_peak_process_tree_rss_kib=${PEAK_TREE_RSS_KIB}"
fi
exit "$BENCHMARK_STATUS"
