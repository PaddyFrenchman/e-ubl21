#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ISSUE_NUMBER="${1:-unknown}"
LOG_DIR="${CODEX_LOG_DIR:-/tmp/e-ubl21-codex}"
LOG_FILE="$LOG_DIR/issue-${ISSUE_NUMBER}-tests.log"

mkdir -p "$LOG_DIR"
cd "$ROOT_DIR"

set +e
bash ./run-all-tests.sh 2>&1 | tee "$LOG_FILE"
status=${PIPESTATUS[0]}
set -e

printf '\nValidation log: %s\n' "$LOG_FILE"
exit "$status"
