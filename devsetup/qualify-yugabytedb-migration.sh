#!/usr/bin/env bash
# Final, disposable qualification for MIG-YB-012. This invokes the independent
# gates from the preceding migration issues and retains one evidence directory.
set -Eeuo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
run_id="$(date -u +%Y%m%dT%H%M%SZ)-$$"
artifact_dir="${ARTIFACT_DIR:-$root_dir/regression-results/yugabytedb-migration/$run_id}"
logs_dir="$artifact_dir/logs"
mkdir -p "$logs_dir"
artifact_dir="$(cd "$artifact_dir" && pwd)"

suite_names=()
suite_statuses=()
suite_codes=()

run_suite() {
    local name=$1
    shift
    local safe_name="${name//[^a-zA-Z0-9._-]/-}"
    local log="$logs_dir/$safe_name.log"
    local rc

    echo "RUNNING: $name"
    set +e
    "$@" 2>&1 | tee "$log"
    rc=${PIPESTATUS[0]}
    set -e
    suite_names+=("$name")
    suite_codes+=("$rc")
    if (( rc == 0 )); then suite_statuses+=(PASS); else suite_statuses+=(FAIL); fi
}

write_summary() {
    local overall=PASS i
    for ((i = 0; i < ${#suite_names[@]}; i++)); do
        [[ "${suite_statuses[$i]}" == PASS ]] || overall=FAIL
    done
    {
        echo '# YugabyteDB migration qualification'
        echo
        echo "- Candidate commit: \`$(git -C "$root_dir" rev-parse HEAD)\`"
        echo "- Finished (UTC): \`$(date -u +%Y-%m-%dT%H:%M:%SZ)\`"
        echo "- Overall: **$overall**"
        echo
        echo '| Gate | Result | Exit | Log |'
        echo '|---|---:|---:|---|'
        for ((i = 0; i < ${#suite_names[@]}; i++)); do
            printf '| %s | **%s** | %s | `%s` |\n' "${suite_names[$i]}" "${suite_statuses[$i]}" "${suite_codes[$i]}" "logs/${suite_names[$i]//[^a-zA-Z0-9._-]/-}.log"
        done
    } >"$artifact_dir/report.md"
    [[ "$overall" == PASS ]]
}

main() {
    cd "$root_dir"
    [[ -z "$(git status --porcelain)" ]] || { echo 'Refusing to qualify a dirty candidate tree.' >&2; exit 2; }
    printf 'candidate_commit=%s\nstarted_at=%s\n' "$(git rev-parse HEAD)" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >"$artifact_dir/metadata.txt"

    run_suite 'unit tests' go test ./... -count=1
    run_suite 'retired dependency guard' ./devsetup/yugabytedb-compatibility-guard.sh
    run_suite 'fresh install static verification' ./devsetup/test-on-prem-yugabytedb.sh static
    run_suite 'fresh install integration' ./devsetup/test-on-prem-yugabytedb.sh integration
    run_suite 'upgrade static verification' ./devsetup/test-on-prem-yugabytedb-upgrade.sh
    run_suite 'upgrade integration' ./devsetup/test-on-prem-yugabytedb-upgrade-integration.sh
    run_suite 'direct YBC restore drill' ./devsetup/test-yugabytedb-ybc-dr.sh
    run_suite 'YBC control-plane qualification' ./devsetup/test-yugabytedb-ybc-control-plane-runtime.sh
    run_suite 'full semantic and Minikube regression' env REGRESSION_RUN_DIR="$artifact_dir/regression" ./run-regression-tests.sh

    # The guard is enforcing; this audit retains intentional historical and
    # migration references for reviewer inspection.
    rg -n -i 'postgresql|cloudnativepg' --glob '*.md' --glob '*.sh' --glob '*.yaml' --glob '*.yml' . >"$artifact_dir/postgresql-reference-audit.txt" || true
    write_summary
    echo "YugabyteDB migration qualification passed: $artifact_dir"
}

main "$@"
