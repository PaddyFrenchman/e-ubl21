#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_command() {
    local command_name="$1"
    local install_hint="$2"
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the stage capacity benchmark." >&2
        echo "Install ${install_hint}, then rerun: make benchmark-stages" >&2
        exit 1
    fi
}

require_command go "Go"
require_command java "a Java runtime (OpenJDK 21 recommended)"
require_command xmllint "libxml2-utils (Debian/Ubuntu) or libxml2 (Alpine/macOS)"
require_command ps "procps or an equivalent ps implementation"

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
SAXON_JAR="${SAXON_DIR}/saxon-he-12.7.jar"
RESOLVER_JAR="${SAXON_DIR}/xmlresolver-6.0.17.jar"
if [[ ! -f "$SAXON_JAR" || ! -f "$RESOLVER_JAR" ]]; then
    require_command curl "curl"
    require_command sha256sum "GNU coreutils"
    echo "Pinned Saxon dependencies are missing; downloading checksum-verified copies."
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
CPU_COUNT="unknown"
if command -v nproc >/dev/null 2>&1; then
    CPU_COUNT="$(nproc)"
fi
MEMORY_KIB="unknown"
if [[ -r /proc/meminfo ]]; then
    MEMORY_KIB="$(awk '/^MemTotal:/ {print $2; exit}' /proc/meminfo)"
fi

echo "Split-stage capacity benchmark environment:"
echo "  date_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "  os=$(uname -srm)"
echo "  cpu_count=${CPU_COUNT}"
echo "  memory_kib=${MEMORY_KIB}"
echo "  gomaxprocs=${GOMAXPROCS:-go-default}"
echo "  capacity_samples=${CAPACITY_SAMPLES}"
echo "  measure_tree_rss=${MEASURE_TREE_RSS}"
echo "  $(go version)"
java -version 2>&1 | sed -n '1,3p'
xmllint --version 2>&1 | sed -n '1p'

BENCHMARK_ARGS=(
    go test ./server ./orchestrator
    -p 1
    -run '^$'
    -bench '^(BenchmarkXSDStageCapacity|BenchmarkEN16931StageCapacity|BenchmarkEN16931ProcessCost|BenchmarkOrchestratorCapacity)$'
    -benchtime 1x
    -count 1
    -timeout 30m
)

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
