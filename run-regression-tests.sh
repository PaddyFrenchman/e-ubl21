#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${REGRESSION_RUN_DIR:-$ROOT_DIR/regression-results/local/$RUN_STAMP}"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
REPORT="$RUN_DIR/report.md"
LOG_DIR="$RUN_DIR/logs"

mkdir -p "$LOG_DIR" "$RUN_DIR/correctness" "$RUN_DIR/minikube/ubl21" "$RUN_DIR/minikube/en16931"
RUN_DIR="$(cd "$RUN_DIR" && pwd)"
REPORT="$RUN_DIR/report.md"
LOG_DIR="$RUN_DIR/logs"

suite_names=()
suite_statuses=()
suite_codes=()
suite_durations=()
suite_logs=()
suite_artifacts=()

format_duration() {
    local seconds=$1
    printf '%02d:%02d:%02d' "$((seconds / 3600))" "$(((seconds % 3600) / 60))" "$((seconds % 60))"
}

first_line() {
    "$@" 2>&1 | head -n 1 || true
}

require_commands() {
    local missing=()
    local command
    for command in bash git go make java xmllint docker curl diff helm minikube sha256sum grep sed find sort tee tail setsid timeout; do
        if ! command -v "$command" >/dev/null 2>&1; then
            missing+=("$command")
        fi
    done

    if (( ${#missing[@]} > 0 )); then
        echo "ERROR: missing required local regression dependencies:" >&2
        printf '  - %s\n' "${missing[@]}" >&2
        echo >&2
        echo "Install the missing tools, then run this script again." >&2
        exit 2
    fi

    if ! docker info >/dev/null 2>&1; then
        echo "ERROR: Docker is installed but its daemon/runtime is not available." >&2
        exit 2
    fi
}

run_suite() {
    local name=$1
    local artifact_dir=$2
    shift 2

    local safe_name="${name//[^a-zA-Z0-9._-]/-}"
    local log_file="$LOG_DIR/$safe_name.log"
    local started=$SECONDS
    local rc
    local -a pipeline_status

    echo
    echo "======================================================================"
    echo "RUNNING: $name"
    echo "Log:     $log_file"
    echo "Artifacts: $artifact_dir"
    echo "======================================================================"

    set +e
    "$@" 2>&1 | tee "$log_file"
    pipeline_status=("${PIPESTATUS[@]}")
    set -e

    rc=${pipeline_status[0]}
    if (( rc == 0 && pipeline_status[1] != 0 )); then
        rc=${pipeline_status[1]}
        echo "ERROR: failed to write suite log: $log_file" >&2
    fi

    local elapsed=$((SECONDS - started))
    local status="PASS"
    if (( rc != 0 )); then
        status="FAIL"
    fi

    suite_names+=("$name")
    suite_statuses+=("$status")
    suite_codes+=("$rc")
    suite_durations+=("$elapsed")
    suite_logs+=("$log_file")
    suite_artifacts+=("$artifact_dir")

    echo
    echo "$status: $name ($(format_duration "$elapsed"), exit $rc)"
}

append_json_artifacts() {
    local file
    local rel
    local found=0

    while IFS= read -r file; do
        found=1
        rel="${file#"$ROOT_DIR/"}"
        {
            echo
            echo "### \`$rel\`"
            echo '```json'
            cat "$file"
            echo
            echo '```'
        } >> "$REPORT"
    done < <(find "$RUN_DIR" -type f \( -name summary.json -o -name equivalence.json \) | sort)

    if (( found == 0 )); then
        echo "No semantic summary/equivalence JSON files were produced." >> "$REPORT"
    fi
}

append_failed_fixtures() {
    local results
    local failures
    local rel
    local found=0

    while IFS= read -r results; do
        failures="$(grep -E '"result"[[:space:]]*:[[:space:]]*"fail"' "$results" || true)"
        if [[ -z "$failures" ]]; then
            continue
        fi
        found=1
        rel="${results#"$ROOT_DIR/"}"
        {
            echo
            echo "### \`$rel\`"
            sed -nE 's/.*"fixtureId"[[:space:]]*:[[:space:]]*"([^"]+)".*/- `\1`/p' <<<"$failures" | sort -u
        } >> "$REPORT"
    done < <(find "$RUN_DIR" -type f -name results.jsonl | sort)

    if (( found == 0 )); then
        echo "No failed semantic fixture IDs were found in generated result sets." >> "$REPORT"
    fi
}

append_database_readiness_failure_logs() {
    local summary
    local artifact_dir
    local log_file
    local rel
    local found=0

    # A Minikube database failure occurs before the normal suite evidence pass.
    # regression-minikube's EXIT trap captures the affected pod logs in this
    # location, so surface the YugabyteDB tails in the top-level report rather
    # than requiring a reviewer to find them among the artifacts.
    while IFS= read -r summary; do
        if ! grep -Eq '"failedPhase"[[:space:]]*:[[:space:]]*"database-readiness"' "$summary"; then
            continue
        fi
        artifact_dir="$(dirname "$(dirname "$summary")")"
        while IFS= read -r log_file; do
            found=1
            rel="${log_file#"$ROOT_DIR/"}"
            {
                echo
                echo "### YugabyteDB readiness evidence: \`$rel\`"
                echo '```text'
                tail -n 120 "$log_file"
                echo '```'
            } >> "$REPORT"
        done < <(find "$artifact_dir/failure-evidence/logs" -type f -name '*yb-*.log' -print 2>/dev/null | sort)
    done < <(find "$RUN_DIR" -type f -path '*/database/summary.json' | sort)

    if (( found == 0 )); then
        echo "No YugabyteDB readiness-evidence logs were captured for failed database readiness phases." >> "$REPORT"
    fi
}

write_report() {
    local finished_at
    local branch
    local commit
    local dirty
    local overall="PASS"
    local i
    local status

    finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
    commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
    if [[ -n "$(git status --porcelain 2>/dev/null || true)" ]]; then
        dirty="yes"
    else
        dirty="no"
    fi

    for status in "${suite_statuses[@]}"; do
        if [[ "$status" != "PASS" ]]; then
            overall="FAIL"
            break
        fi
    done

    {
        echo "# e-ubl21 local regression report"
        echo
        echo "- Overall: **$overall**"
        echo "- Finished (UTC): \`$finished_at\`"
        echo "- Git branch: \`$branch\`"
        echo "- Git commit: \`$commit\`"
        echo "- Working tree dirty: \`$dirty\`"
        echo "- Minikube profile: \`$MINIKUBE_PROFILE\`"
        echo "- Results directory: \`${RUN_DIR#"$ROOT_DIR/"}\`"
        echo
        echo "## Toolchain"
        echo
        echo '```text'
        echo "Go:       $(first_line go version)"
        echo "Java:     $(first_line java -version)"
        echo "Docker:   $(first_line docker --version)"
        echo "Minikube: $(first_line minikube version --short)"
        echo '```'
        echo
        echo "## Suite summary"
        echo
        echo "| Suite | Result | Exit | Duration | Artifacts | Log |"
        echo "|---|---:|---:|---:|---|---|"
    } > "$REPORT"

    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        printf '| %s | **%s** | %s | %s | `%s` | `%s` |\n' \
            "${suite_names[$i]}" \
            "${suite_statuses[$i]}" \
            "${suite_codes[$i]}" \
            "$(format_duration "${suite_durations[$i]}")" \
            "${suite_artifacts[$i]#"$ROOT_DIR/"}" \
            "${suite_logs[$i]#"$ROOT_DIR/"}" >> "$REPORT"
    done

    {
        echo
        echo "## Semantic summaries and equivalence evidence"
    } >> "$REPORT"
    append_json_artifacts

    {
        echo
        echo "## Failed fixture IDs"
    } >> "$REPORT"
    append_failed_fixtures

    local failures_present=0
    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        if [[ "${suite_statuses[$i]}" == "FAIL" ]]; then
            if (( failures_present == 0 )); then
                {
                    echo
                    echo "## Failure log tails"
                    echo
                    echo "The last 120 lines of each failed suite are included below for rapid triage."
                } >> "$REPORT"
                failures_present=1
            fi
            {
                echo
                echo "### ${suite_names[$i]}"
                echo '```text'
                tail -n 120 "${suite_logs[$i]}"
                echo '```'
            } >> "$REPORT"
        fi
    done

    if (( failures_present != 0 )); then
        {
            echo
            echo "## YugabyteDB readiness failure-log tails"
            echo
            echo "The last 120 lines of each captured YugabyteDB pod log are included when database readiness failed."
        } >> "$REPORT"
        append_database_readiness_failure_logs
    fi
}

print_summary() {
    local i
    local overall=0

    echo
    echo "======================================================================"
    echo "REGRESSION SUMMARY"
    echo "======================================================================"
    printf '%-34s %-6s %-6s %-10s\n' "SUITE" "RESULT" "EXIT" "DURATION"
    printf '%-34s %-6s %-6s %-10s\n' "----------------------------------" "------" "------" "----------"
    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        printf '%-34s %-6s %-6s %-10s\n' \
            "${suite_names[$i]}" \
            "${suite_statuses[$i]}" \
            "${suite_codes[$i]}" \
            "$(format_duration "${suite_durations[$i]}")"
        if [[ "${suite_statuses[$i]}" != "PASS" ]]; then
            overall=1
        fi
    done
    echo
    echo "Full report: $REPORT"
    echo "Artifacts:   $RUN_DIR"
    echo "Minikube profile '$MINIKUBE_PROFILE' is retained for faster subsequent local runs."

    if (( overall == 0 )); then
        echo "OVERALL: PASS"
    else
        echo "OVERALL: FAIL"
    fi

    return "$overall"
}

main() {
require_commands

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "e-ubl21 local regression run"
echo "Started:    $STARTED_AT"
echo "Repository: $ROOT_DIR"
echo "Results:    $RUN_DIR"
echo "Profile:    $MINIKUBE_PROFILE"

# Keep the local runner aligned with the two GitHub regression workflows:
# regression-correctness.yml and regression-minikube.yml.
run_suite \
    "validation chaos/crash/replay" \
    "$RUN_DIR" \
    make test-validation-chaos

# When supplied by the YBC DR drill, execute the complete Go application
# suite with its database integration tests bound to the recovered target.
# The normal Minikube suites remain independent topology regression evidence.
if [[ -n "${EUBL21_RESTORED_DATABASE_URL:-}" ]]; then
run_suite \
    "restored database application regression" \
    "$RUN_DIR/restored-database" \
    env EUBL21_TEST_DATABASE_URL="$EUBL21_RESTORED_DATABASE_URL" \
        go test ./... -count=1
fi

run_suite \
    "correctness regression (ubl21 + en16931)" \
    "$RUN_DIR/correctness" \
    env ARTIFACT_DIR="$RUN_DIR/correctness" make regression-correctness

run_suite \
    "minikube regression (ubl21)" \
    "$RUN_DIR/minikube/ubl21" \
    env \
        MINIKUBE_PROFILE="$MINIKUBE_PROFILE" \
        MANIFEST="createsamples/samples/manifest.json" \
        PROFILE=all \
        ARTIFACT_DIR="$RUN_DIR/minikube/ubl21" \
        make regression-minikube

run_suite \
    "minikube regression (en16931)" \
    "$RUN_DIR/minikube/en16931" \
    env \
        MINIKUBE_PROFILE="$MINIKUBE_PROFILE" \
        MANIFEST="createsamples/samples-en16931/manifest.json" \
        PROFILE=all \
        ARTIFACT_DIR="$RUN_DIR/minikube/en16931" \
        make regression-minikube

write_report
if print_summary; then
    exit 0
fi
exit 1
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
