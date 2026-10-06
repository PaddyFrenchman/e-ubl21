#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

REPO="${REPO:-PaddyFrenchman/e-ubl21}"
BASE_BRANCH="${BASE_BRANCH:-development}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-5}"
STOP_AFTER_ONE="${STOP_AFTER_ONE:-0}"
CODEX_LOG_DIR="${CODEX_LOG_DIR:-/tmp/e-ubl21-codex}"

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "ERROR: required command not found: $1" >&2
    exit 2
  }
}

for cmd in git gh jq codex; do
  require "$cmd"
done

mkdir -p "$CODEX_LOG_DIR"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: working tree is not clean." >&2
  git status --short
  exit 2
fi

slugify() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//'
}

has_label() {
  local label="$1"
  gh label list --repo "$REPO" --limit 200 --json name \
    | jq -e --arg label "$label" 'map(.name) | index($label) != null' >/dev/null
}

for label in ready agent-suitable in-progress in-review; do
  if ! has_label "$label"; then
    echo "ERROR: required GitHub label does not exist: $label" >&2
    exit 2
  fi
done

while true; do
  ISSUE_JSON="$(REPO="$REPO" bash codex/select-next-issue.sh)"

  if [[ -z "$ISSUE_JSON" ]]; then
    echo "No eligible issues remain."
    exit 0
  fi

  ISSUE_NUMBER="$(jq -r '.number' <<<"$ISSUE_JSON")"
  ISSUE_TITLE="$(jq -r '.title' <<<"$ISSUE_JSON")"
  ISSUE_BODY="$(jq -r '.body // ""' <<<"$ISSUE_JSON")"
  SLUG="$(slugify "$ISSUE_TITLE")"
  BRANCH="issue/${ISSUE_NUMBER}-${SLUG}"
  TEST_LOG="$CODEX_LOG_DIR/issue-${ISSUE_NUMBER}-tests.log"

  echo
  echo "============================================================"
  echo "Issue #$ISSUE_NUMBER: $ISSUE_TITLE"
  echo "============================================================"

  git checkout "$BASE_BRANCH"
  git pull --ff-only origin "$BASE_BRANCH"

  if git show-ref --verify --quiet "refs/heads/$BRANCH"; then
    git checkout "$BRANCH"
  elif git ls-remote --exit-code --heads origin "$BRANCH" >/dev/null 2>&1; then
    git checkout -b "$BRANCH" "origin/$BRANCH"
  else
    git checkout -b "$BRANCH"
  fi

  gh issue edit "$ISSUE_NUMBER" --repo "$REPO" --add-label in-progress

  attempt=1
  validated=0

  while (( attempt <= MAX_ATTEMPTS )); do
    echo
    echo "Codex implementation attempt $attempt/$MAX_ATTEMPTS"

    if (( attempt == 1 )); then
      PROMPT="$(cat <<EOF
You are implementing GitHub issue #$ISSUE_NUMBER in the e-UBL21 repository.

TITLE:
$ISSUE_TITLE

ISSUE:
$ISSUE_BODY

Before editing:
- Read codex/AGENTS.md and obey it.
- For a large issue, use codex/PLANS.md as the planning contract.
- Inspect the existing implementation and relevant tests.

Implement this issue completely.

Important:
- Work only on this issue.
- Do not commit, push, merge, create a pull request, close the issue, or change labels.
- Run focused tests while developing.
- Do not stop merely because the code compiles.
EOF
)"
    else
      PROMPT="$(cat <<EOF
Continue implementing GitHub issue #$ISSUE_NUMBER.

Read codex/AGENTS.md again.

The repository-wide validation failed. The complete validation log is:
$TEST_LOG

Inspect that log, identify the root cause, and correct the implementation.

Important:
- Do not weaken, delete, skip, or bypass valid tests.
- Distinguish an implementation defect from a genuine environment failure.
- Do not commit, push, merge, create a pull request, close the issue, or change labels.
- Run focused tests after your fixes.
EOF
)"
    fi

    codex exec "$PROMPT"

    if bash codex/validate-issue.sh "$ISSUE_NUMBER"; then
      validated=1
      break
    fi

    attempt=$((attempt + 1))
  done

  if [[ "$validated" -ne 1 ]]; then
    gh issue comment "$ISSUE_NUMBER" \
      --repo "$REPO" \
      --body "Automated Codex development loop stopped after $MAX_ATTEMPTS unsuccessful repository-wide validation attempts. Validation log: `$TEST_LOG`."

    echo "ERROR: issue #$ISSUE_NUMBER did not pass validation." >&2
    exit 1
  fi

  if [[ -z "$(git status --porcelain)" ]]; then
    gh issue comment "$ISSUE_NUMBER" \
      --repo "$REPO" \
      --body "Codex validation completed successfully, but no repository changes were produced. Manual review is required."

    echo "ERROR: validation passed but issue produced no changes." >&2
    exit 1
  fi

  git add -A
  git commit -m "Implement #$ISSUE_NUMBER: $ISSUE_TITLE"
  git push -u origin "$BRANCH"

  PR_URL="$(gh pr create \
    --repo "$REPO" \
    --base "$BASE_BRANCH" \
    --head "$BRANCH" \
    --title "#$ISSUE_NUMBER $ISSUE_TITLE" \
    --body "$(cat <<EOF
Implements #$ISSUE_NUMBER.

## Validation

`bash run-all-tests.sh`

Result: **PASS**

Closes #$ISSUE_NUMBER
EOF
)")"

  gh issue edit "$ISSUE_NUMBER" \
    --repo "$REPO" \
    --remove-label in-progress \
    --add-label in-review

  echo
  echo "Created pull request:"
  echo "$PR_URL"

  git checkout "$BASE_BRANCH"

  if [[ "$STOP_AFTER_ONE" == "1" ]]; then
    exit 0
  fi
done
