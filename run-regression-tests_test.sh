#!/usr/bin/env bash
set -Eeuo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT

run_dir="$temporary_dir/run"
mkdir -p "$run_dir/minikube/ubl21/database" "$run_dir/minikube/ubl21/failure-evidence/logs"
cat >"$run_dir/minikube/ubl21/database/summary.json" <<'EOF'
{"failedPhase": "database-readiness"}
EOF
printf 'yugabyte readiness failure\n' >"$run_dir/minikube/ubl21/failure-evidence/logs/e-ubl21-regression.yb-tserver-0.yb-tserver.log"

REGRESSION_RUN_DIR="$run_dir" bash -c '
    source "$1/run-regression-tests.sh"
    suite_names=("minikube regression (ubl21)")
    suite_statuses=("FAIL")
    suite_codes=(1)
    suite_durations=(0)
    suite_logs=("$RUN_DIR/minikube-suite.log")
    suite_artifacts=("$RUN_DIR/minikube/ubl21")
    printf "suite failure\n" >"${suite_logs[0]}"
    write_report
' bash "$root_dir"

report="$run_dir/report.md"
grep -Fq '## YugabyteDB readiness failure-log tails' "$report"
grep -Fq 'yugabyte readiness failure' "$report"
grep -Fq 'failure-evidence/logs/e-ubl21-regression.yb-tserver-0.yb-tserver.log' "$report"
