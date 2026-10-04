#!/usr/bin/env bash
# Collect reproducible evidence for GitHub issue #167. This script never
# promotes generated fixtures and never treats uncovered schema targets as an
# approved residual. It runs every automated part of the qualification gate,
# retains command logs, and exits non-zero when a required check fails or a
# human residual-coverage decision is still outstanding.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

RUN_STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACT_DIR="${ARTIFACT_DIR:-regression-results/issue-167/$RUN_STAMP}"
STRUCTURAL_FIXTURE_ID="${STRUCTURAL_FIXTURE_ID:-ubl21.applicationresponse.cardinality-5}"
mkdir -p "$ARTIFACT_DIR/logs"
ARTIFACT_DIR="$(cd "$ARTIFACT_DIR" && pwd)"

for command in git go make xmllint docker java; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "ERROR: required command not found: $command" >&2
        exit 2
    }
done

git status --short >"$ARTIFACT_DIR/git-status.before.txt"
git diff --binary >"$ARTIFACT_DIR/worktree.before.patch"
git rev-parse HEAD >"$ARTIFACT_DIR/commit.txt"
git log -1 --format='%H%n%s%n%b' >"$ARTIFACT_DIR/commit-description.txt"
cp docs/STRUCTURAL-CORPUS-RESIDUALS.md "$ARTIFACT_DIR/residual-register.before.md"

declare -a CHECKS=()
declare -a STATUSES=()

run_check() {
    local name=$1
    shift
    local log="$ARTIFACT_DIR/logs/$name.log"
    local started finished status
    started="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '== %s ==\n' "$name" | tee "$log"
    "$@" >>"$log" 2>&1
    status=$?
    finished="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    CHECKS+=("$name|$started|$finished|$log")
    STATUSES+=("$status")
    if (( status == 0 )); then
        echo "PASS: $name"
    else
        echo "FAIL ($status): $name — see $log" >&2
    fi
    return 0
}

# Regeneration validates generated expected-valid fixtures, checks intended
# XSD-negative diagnostics, and compares a second deterministic generation to
# the tracked corpora. It refuses dirty generator/XSD inputs by design.
run_check corpus-regenerate make corpus-regenerate

# Write the source-of-truth coverage artifacts even while gaps remain. Stdout
# is the optional per-fixture audit stream; it is intentionally discarded here
# because report.json/report.md are the retained issue evidence.
run_check coverage-report bash -c '
  go run ./createsamples coverage --xsd XSD \
    --samples createsamples/samples-structural \
    --json "$1/coverage.json" --markdown "$1/coverage.md" \
    --require-cardinality=false >/dev/null
' bash "$ARTIFACT_DIR"

# Exercise generator boundaries independently of the corpus lifecycle.
run_check structural-generator-tests go test ./createsamples -run 'Test(Large.*|GenerateSignatureExtension.*|TrackedStructuralCorpusHasValidHashes|SchemaCoverage.*)' -count=1

# These tests construct controlled runner failures and verify the redacted
# diagnostic bundle and Codex handoff material rather than relying on an
# accidental production failure.
run_check diagnostics-handoff-tests go test ./regression -run 'Test.*(Diagnostic|Failure|Artifact)' -count=1

# This starts the local authenticated API/NATS/YugabyteDB topology and submits
# all tracked UBL, EN 16931, and structural fixtures. Its evidence includes
# fixture/submitted/terminal counts, run maps, logs, metrics, and failures.
run_check full-runtime-corpus env ARTIFACT_DIR="$ARTIFACT_DIR/correctness-full" ./devsetup/regression-correctness.sh

# Reproduce one high-cardinality structural fixture through the exact same
# topology. Override STRUCTURAL_FIXTURE_ID when investigating a specific case.
run_check single-fixture-reproduction env STRUCTURAL_FIXTURE_ID="$STRUCTURAL_FIXTURE_ID" ARTIFACT_DIR="$ARTIFACT_DIR/correctness-single-fixture" ./devsetup/regression-correctness.sh

git status --short >"$ARTIFACT_DIR/git-status.after.txt"
git diff --binary >"$ARTIFACT_DIR/worktree.after.patch"

residuals=0
if [[ -f "$ARTIFACT_DIR/coverage.json" ]]; then
    residuals="$(grep -c '"approved": false' "$ARTIFACT_DIR/coverage.json" || true)"
fi

{
    printf '# Issue #167 qualification summary\n\n'
    printf 'Evidence directory: `%s`\n\n' "$ARTIFACT_DIR"
    printf '| Check | Started | Finished | Status | Log |\n|---|---|---|---:|---|\n'
    for i in "${!CHECKS[@]}"; do
        IFS='|' read -r name started finished log <<<"${CHECKS[$i]}"
        printf '| %s | %s | %s | %s | [%s](%s) |\n' "$name" "$started" "$finished" "${STATUSES[$i]}" "log" "${log#$ARTIFACT_DIR/}"
    done
    printf '\nUnapproved cardinality residuals in generated coverage: **%s**.\n' "$residuals"
    printf '\nThis automation does not approve residual coverage or close issue #167. Review `coverage.json`, `coverage.md`, and `docs/STRUCTURAL-CORPUS-RESIDUALS.md` before making that decision.\n'
} >"$ARTIFACT_DIR/summary.md"

failed=0
for status in "${STATUSES[@]}"; do
    (( status == 0 )) || failed=1
done
if (( residuals > 0 )); then
    echo "QUALIFICATION INCOMPLETE: $residuals unapproved cardinality residuals remain." >&2
    failed=1
fi

echo "Issue #167 evidence: $ARTIFACT_DIR"
echo "Summary: $ARTIFACT_DIR/summary.md"
exit "$failed"
