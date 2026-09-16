#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR="${ISSUE124_RUN_DIR:-$ROOT_DIR/regression-results/issue-124/$RUN_STAMP}"
LOG_DIR="$RUN_DIR/logs"
REPORT="$RUN_DIR/report.md"
MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-docker}"
TEST_IMAGE="${EUBL21_TEST_IMAGE:-localhost/e-ubl21:regression}"

mkdir -p "$LOG_DIR"
RUN_DIR="$(cd "$RUN_DIR" && pwd)"
LOG_DIR="$RUN_DIR/logs"
REPORT="$RUN_DIR/report.md"

suite_names=()
suite_statuses=()
suite_codes=()
suite_durations=()
suite_logs=()

format_duration() {
    local seconds=$1
    printf '%02d:%02d:%02d' "$((seconds / 3600))" "$(((seconds % 3600) / 60))" "$((seconds % 60))"
}

first_line() {
    "$@" 2>&1 | head -n 1 || true
}

require_commands() {
    local command
    local missing=()

    for command in \
        bash git go make java xmllint docker curl diff minikube kubectl helm \
        grep rg sed find sort tee tail setsid sha256sum awk; do
        if ! command -v "$command" >/dev/null 2>&1; then
            missing+=("$command")
        fi
    done

    if (( ${#missing[@]} > 0 )); then
        echo "ERROR: missing required #124 qualification dependencies:" >&2
        printf '  - %s\n' "${missing[@]}" >&2
        exit 2
    fi

    if ! docker info >/dev/null 2>&1; then
        echo "ERROR: Docker is installed but its daemon/runtime is unavailable." >&2
        exit 2
    fi
}

run_suite() {
    local name=$1
    shift

    local safe_name="${name//[^a-zA-Z0-9._-]/-}"
    local log_file="$LOG_DIR/$safe_name.log"
    local started=$SECONDS
    local rc
    local -a pipeline_status

    echo
    echo "======================================================================"
    echo "RUNNING: $name"
    echo "Log:     $log_file"
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

    echo
    echo "$status: $name ($(format_duration "$elapsed"), exit $rc)"
    return 0
}

prepare_minikube_integration_context() {
    local kubeconfig_copy="$RUN_DIR/minikube-kubeconfig"

    if ! minikube status -p "$MINIKUBE_PROFILE" --format='{{.Host}}' 2>/dev/null | grep -qx Running; then
        minikube start -p "$MINIKUBE_PROFILE" --driver="$MINIKUBE_DRIVER"
    fi

    # Copy the effective kubeconfig before selecting the test context. This keeps
    # the user's normal current-context untouched while letting direct kubectl
    # and Helm commands in the on-prem integration tests use the same Minikube.
    kubectl config view --raw --flatten >"$kubeconfig_copy"
    chmod 0600 "$kubeconfig_copy"
    export KUBECONFIG="$kubeconfig_copy"

    if ! kubectl config get-contexts -o name | grep -Fxq "$MINIKUBE_PROFILE"; then
        echo "ERROR: Minikube context '$MINIKUBE_PROFILE' is not present in the effective kubeconfig." >&2
        return 1
    fi
    kubectl config use-context "$MINIKUBE_PROFILE" >/dev/null
    kubectl get nodes >/dev/null

    # The normal regression runner builds this image. Rebuild only if it is not
    # present locally, then explicitly load it into the qualification profile so
    # the two on-prem integration suites do not depend on a previous test order.
    if ! docker image inspect "$TEST_IMAGE" >/dev/null 2>&1; then
        docker build --tag "$TEST_IMAGE" .
    fi
    docker save "$TEST_IMAGE" | minikube -p "$MINIKUBE_PROFILE" image load -
}

write_report() {
    local branch commit dirty finished_at overall="PASS" i

    branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
    commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
    finished_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    if [[ -n "$(git status --porcelain 2>/dev/null || true)" ]]; then
        dirty="yes"
    else
        dirty="no"
    fi

    for ((i = 0; i < ${#suite_statuses[@]}; i++)); do
        if [[ "${suite_statuses[$i]}" != "PASS" ]]; then
            overall="FAIL"
            break
        fi
    done

    {
        echo "# Issue #124 local qualification report"
        echo
        echo "- Overall: **$overall**"
        echo "- Finished (UTC): \`$finished_at\`"
        echo "- Git branch: \`$branch\`"
        echo "- Git commit: \`$commit\`"
        echo "- Working tree dirty: \`$dirty\`"
        echo "- Minikube profile: \`$MINIKUBE_PROFILE\`"
        echo "- Test image: \`$TEST_IMAGE\`"
        echo "- Results directory: \`${RUN_DIR#"$ROOT_DIR/"}\`"
        echo
        echo "## Qualification scope"
        echo
        echo "This runner qualifies issue #124 by combining its dedicated guarded-upgrade tests with the existing repository regression runner. It also re-runs the clean on-prem YugabyteDB installer gates because #124 changes shared installer functions."
        echo
        echo "## Toolchain"
        echo
        echo '```text'
        echo "Go:       $(first_line go version)"
        echo "Java:     $(first_line java -version)"
        echo "Docker:   $(first_line docker --version)"
        echo "Minikube: $(first_line minikube version --short)"
        echo "kubectl:  $(first_line kubectl version --client)"
        echo "Helm:     $(first_line helm version --short)"
        echo '```'
        echo
        echo "## Suite summary"
        echo
        echo "| Suite | Result | Exit | Duration | Log |"
        echo "|---|---:|---:|---:|---|"
    } >"$REPORT"

    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        printf '| %s | **%s** | %s | %s | `%s` |\n' \
            "${suite_names[$i]}" \
            "${suite_statuses[$i]}" \
            "${suite_codes[$i]}" \
            "$(format_duration "${suite_durations[$i]}")" \
            "${suite_logs[$i]#"$ROOT_DIR/"}" >>"$REPORT"
    done

    if [[ -f "$RUN_DIR/full-regression/report.md" ]]; then
        {
            echo
            echo "## Full regression report"
            echo
            echo "The existing repository regression runner produced:"
            echo
            echo "- \`${RUN_DIR#"$ROOT_DIR/"}/full-regression/report.md\`"
        } >>"$REPORT"
    fi

    local failure_heading_written=0
    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        if [[ "${suite_statuses[$i]}" != "PASS" ]]; then
            if (( failure_heading_written == 0 )); then
                {
                    echo
                    echo "## Failure log tails"
                    echo
                    echo "The last 120 lines of each failed suite are included for triage."
                } >>"$REPORT"
                failure_heading_written=1
            fi
            {
                echo
                echo "### ${suite_names[$i]}"
                echo '```text'
                tail -n 120 "${suite_logs[$i]}"
                echo '```'
            } >>"$REPORT"
        fi
    done
}

print_summary() {
    local i overall=0

    echo
    echo "======================================================================"
    echo "ISSUE #124 QUALIFICATION SUMMARY"
    echo "======================================================================"
    printf '%-44s %-6s %-6s %-10s\n' "SUITE" "RESULT" "EXIT" "DURATION"
    printf '%-44s %-6s %-6s %-10s\n' "--------------------------------------------" "------" "------" "----------"

    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        printf '%-44s %-6s %-6s %-10s\n' \
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
    echo "All logs:    $LOG_DIR"

    if (( overall == 0 )); then
        echo "OVERALL: PASS"
    else
        echo "OVERALL: FAIL"
    fi
    return "$overall"
}

require_commands

STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "e-ubl21 issue #124 local qualification"
echo "Started:          $STARTED_AT"
echo "Repository:       $ROOT_DIR"
echo "Results:          $RUN_DIR"
echo "Minikube profile: $MINIKUBE_PROFILE"
echo "Test image:       $TEST_IMAGE"

# Fast, non-destructive gates first. These prove the #124 state machine and the
# shared fresh-install rendering/validation logic before any real cluster work.
run_suite \
    "#124 guarded-upgrade behavioral tests" \
    make on-prem-database-upgrade-test

run_suite \
    "on-prem YugabyteDB static installer gate" \
    make on-prem-database-test

# Reuse the repository-wide runner rather than duplicating its correctness,
# chaos/crash/replay, and two Minikube semantic regression suites.
run_suite \
    "full repository regression suite" \
    env \
        REGRESSION_RUN_DIR="$RUN_DIR/full-regression" \
        MINIKUBE_PROFILE="$MINIKUBE_PROFILE" \
        bash ./run-regression-tests.sh

# The on-prem integration scripts use direct kubectl/Helm commands. Prepare an
# isolated kubeconfig that points at the dedicated Minikube qualification
# profile, and make sure the application image is available there.
run_suite \
    "prepare isolated Minikube integration context" \
    prepare_minikube_integration_context

# Only execute destructive-in-a-disposable-namespace integration gates when the
# isolated Minikube context preparation passed. The underlying scripts also
# refuse to touch pre-existing CloudNativePG resources they do not own.
context_status="${suite_statuses[${#suite_statuses[@]}-1]}"
if [[ "$context_status" == "PASS" ]]; then
    run_suite \
        "on-prem YugabyteDB clean-install integration" \
        env \
            KUBECONFIG="$RUN_DIR/minikube-kubeconfig" \
            EUBL21_TEST_IMAGE="$TEST_IMAGE" \
            make on-prem-database-integration

    run_suite \
        "#124 CloudNativePG-to-YugabyteDB integration" \
        env \
            KUBECONFIG="$RUN_DIR/minikube-kubeconfig" \
            EUBL21_TEST_IMAGE="$TEST_IMAGE" \
            make on-prem-database-upgrade-integration
else
    echo "Skipping on-prem integration suites because Minikube context preparation failed." >&2
fi

write_report
if print_summary; then
    exit 0
fi
exit 1
