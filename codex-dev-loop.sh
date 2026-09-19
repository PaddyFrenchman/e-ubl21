#!/usr/bin/env bash
# codex-dev-loop.sh
#
# Credit-conscious development loop for Codex CLI:
#   task/issue -> Terra implementation -> local quality gates -> independent review
#   -> Terra fixes -> re-test -> bounded re-review
#
# Designed for Git repositories and GitHub issues, but a local task file can be used.
# The script deliberately keeps Git mutations (commit/push) outside Codex.

set -Eeuo pipefail
IFS=$'\n\t'

SCRIPT_NAME="$(basename "$0")"

# ------------------------------- Defaults -----------------------------------
BASE_BRANCH="${BASE_BRANCH:-development}"
REMOTE="${REMOTE:-origin}"

IMPLEMENT_MODEL="${IMPLEMENT_MODEL:-gpt-5.6-terra}"
IMPLEMENT_EFFORT="${IMPLEMENT_EFFORT:-low}"
FIX_MODEL="${FIX_MODEL:-gpt-5.6-terra}"
FIX_EFFORT="${FIX_EFFORT:-low}"

# auto | sol-high | sol-medium | terra-low
REVIEW_POLICY="${REVIEW_POLICY:-auto}"
REVIEW_MODEL_OVERRIDE="${REVIEW_MODEL:-}"
REVIEW_EFFORT_OVERRIDE="${REVIEW_EFFORT:-}"

MAX_LOCAL_FIX_CYCLES="${MAX_LOCAL_FIX_CYCLES:-2}"
MAX_REVIEW_CYCLES="${MAX_REVIEW_CYCLES:-2}"
AUTO_ESCALATE_LOW_CONFIDENCE="${AUTO_ESCALATE_LOW_CONFIDENCE:-1}"

# Optional local commands/settings.
TEST_CMD="${TEST_CMD:-}"
RUN_STATICCHECK="${RUN_STATICCHECK:-1}"
RUN_GOLANGCI_LINT="${RUN_GOLANGCI_LINT:-1}"

# Safe by default. Enable explicitly if desired.
AUTO_COMMIT="${AUTO_COMMIT:-0}"
AUTO_PUSH="${AUTO_PUSH:-0}"
BRANCH_NAME="${BRANCH_NAME:-}"

# Review escalation threshold based on added+deleted lines.
LARGE_DIFF_LINES="${LARGE_DIFF_LINES:-800}"

# ------------------------------- Utilities ----------------------------------
log()  { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
info() { printf '    %s\n' "$*"; }
warn() { printf '\033[1;33mWARN:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

usage() {
  cat <<USAGE
Usage:
  $SCRIPT_NAME <issue-number>
  $SCRIPT_NAME <task-file>

Examples:
  ./$SCRIPT_NAME 125
  ./$SCRIPT_NAME ./issue-125.md

Useful overrides:
  BASE_BRANCH=development
  TEST_CMD='./run-regression-tests.sh'
  REVIEW_POLICY=auto|sol-high|sol-medium|terra-low
  MAX_REVIEW_CYCLES=2
  MAX_LOCAL_FIX_CYCLES=2
  AUTO_COMMIT=1
  AUTO_PUSH=1
  BRANCH_NAME='issue/125-safe-on-prem-upgrade'

Model overrides:
  IMPLEMENT_MODEL=gpt-5.6-terra IMPLEMENT_EFFORT=low
  REVIEW_MODEL=gpt-5.6-sol REVIEW_EFFORT=high

Exit codes:
  0  clean review / completed successfully
  1  setup, Git, or Codex execution error
  2  review still requires changes after retry budget
  3  local quality gates still failing after repair budget
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

is_uint() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

slugify() {
  printf '%s' "$1" \
    | tr '[:upper:]' '[:lower:]' \
    | sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//' \
    | cut -c1-55
}

working_tree_is_clean() {
  [[ -z "$(git status --porcelain)" ]]
}

track_new_files_for_diff() {
  # Make untracked implementation files visible to `git diff <base>` without
  # staging their contents. Intent-to-add is local index metadata only.
  local files=()
  while IFS= read -r -d '' f; do
    files+=("$f")
  done < <(git ls-files --others --exclude-standard -z)

  if (( ${#files[@]} > 0 )); then
    git add -N -- "${files[@]}"
  fi
}

json_escape_toml_string() {
  # Emit a TOML quoted string. Sufficient for the simple enum/model values used here.
  printf '"%s"' "${1//\"/\\\"}"
}

# -------------------------- Argument / prerequisite checks -------------------
[[ $# -eq 1 ]] || { usage; exit 1; }
TASK_SOURCE="$1"

require_cmd git
require_cmd codex
require_cmd jq

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "Run this script inside a Git repository."
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

working_tree_is_clean || die "Working tree is not clean. Commit/stash existing changes before starting the automated cycle."

git remote get-url "$REMOTE" >/dev/null 2>&1 || die "Git remote '$REMOTE' does not exist."

# Keep orchestration artifacts local without modifying the repository's .gitignore.
mkdir -p .codex-cycle
if [[ -w .git/info/exclude ]] && ! grep -qxF '.codex-cycle/' .git/info/exclude 2>/dev/null; then
  printf '\n.codex-cycle/\n' >> .git/info/exclude
fi

# ----------------------------- Resolve task ----------------------------------
ISSUE_NUMBER=""
ISSUE_TITLE=""
ISSUE_URL=""

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
RUN_DIR=".codex-cycle/$RUN_ID"
mkdir -p "$RUN_DIR"
TASK_FILE="$RUN_DIR/task.md"

if is_uint "$TASK_SOURCE"; then
  require_cmd gh
  ISSUE_NUMBER="$TASK_SOURCE"
  log "Loading GitHub issue #$ISSUE_NUMBER"

  ISSUE_JSON="$RUN_DIR/issue.json"
  gh issue view "$ISSUE_NUMBER" --json number,title,body,url > "$ISSUE_JSON"

  ISSUE_TITLE="$(jq -r '.title' "$ISSUE_JSON")"
  ISSUE_URL="$(jq -r '.url' "$ISSUE_JSON")"

  {
    printf '# GitHub Issue #%s: %s\n\n' "$ISSUE_NUMBER" "$ISSUE_TITLE"
    printf 'URL: %s\n\n' "$ISSUE_URL"
    printf '## Requirements\n\n'
    jq -r '.body // ""' "$ISSUE_JSON"
  } > "$TASK_FILE"
else
  [[ -f "$TASK_SOURCE" ]] || die "Task file not found: $TASK_SOURCE"
  cp "$TASK_SOURCE" "$TASK_FILE"
  ISSUE_TITLE="$(basename "$TASK_SOURCE")"
fi

# ------------------------------ Git setup ------------------------------------
log "Refreshing $REMOTE/$BASE_BRANCH"
git fetch "$REMOTE" "$BASE_BRANCH"
BASE_REF="$REMOTE/$BASE_BRANCH"

git rev-parse --verify "$BASE_REF" >/dev/null 2>&1 || die "Base ref '$BASE_REF' not found."

CURRENT_BRANCH="$(git branch --show-current)"
[[ -n "$CURRENT_BRANCH" ]] || die "Detached HEAD is not supported."

if [[ "$CURRENT_BRANCH" == "$BASE_BRANCH" && -n "$ISSUE_NUMBER" ]]; then
  if [[ -n "$BRANCH_NAME" ]]; then
    TARGET_BRANCH="$BRANCH_NAME"
  else
    ISSUE_SLUG="$(slugify "$ISSUE_TITLE")"
    TARGET_BRANCH="issue/${ISSUE_NUMBER}-${ISSUE_SLUG:-work}"
  fi

  log "Selecting issue branch $TARGET_BRANCH"

  if git show-ref --verify --quiet "refs/heads/$TARGET_BRANCH"; then
    git switch "$TARGET_BRANCH"
  elif git ls-remote --exit-code --heads "$REMOTE" "$TARGET_BRANCH" >/dev/null 2>&1; then
    git fetch "$REMOTE" "$TARGET_BRANCH:$TARGET_BRANCH"
    git switch "$TARGET_BRANCH"
  else
    git switch -c "$TARGET_BRANCH" "$BASE_REF"
  fi
  CURRENT_BRANCH="$TARGET_BRANCH"
fi

if [[ "$CURRENT_BRANCH" != "$BASE_BRANCH" ]]; then
  log "Rebasing $CURRENT_BRANCH onto $BASE_REF"
  if ! git rebase "$BASE_REF"; then
    warn "Rebase stopped because of conflicts. Resolve them, then rerun the script."
    exit 1
  fi
else
  warn "You are working directly on '$BASE_BRANCH'. A task file was supplied, so no issue branch was created."
fi

# Snapshot the exact base commit used for this run. All reviews compare against it.
BASE_COMMIT="$(git rev-parse "$BASE_REF")"
printf '%s\n' "$BASE_COMMIT" > "$RUN_DIR/base-commit.txt"
printf '%s\n' "$CURRENT_BRANCH" > "$RUN_DIR/branch.txt"

# --------------------------- Codex invocation --------------------------------
LEDGER="$RUN_DIR/usage.tsv"
printf 'stage\tmodel\teffort\tinput_tokens\tcached_input_tokens\toutput_tokens\treasoning_output_tokens\n' > "$LEDGER"

record_usage() {
  local stage="$1" model="$2" effort="$3" events="$4"
  local stats

  if [[ ! -s "$events" ]]; then
    printf '%s\t%s\t%s\t0\t0\t0\t0\n' "$stage" "$model" "$effort" >> "$LEDGER"
    return 0
  fi

  stats="$(jq -s '
    reduce (.[] | select(.type == "turn.completed") | .usage) as $u
      ({input_tokens:0,cached_input_tokens:0,output_tokens:0,reasoning_output_tokens:0};
       .input_tokens += ($u.input_tokens // 0) |
       .cached_input_tokens += ($u.cached_input_tokens // 0) |
       .output_tokens += ($u.output_tokens // 0) |
       .reasoning_output_tokens += ($u.reasoning_output_tokens // 0))
  ' "$events")"

  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
    "$stage" "$model" "$effort" \
    "$(jq -r '.input_tokens' <<<"$stats")" \
    "$(jq -r '.cached_input_tokens' <<<"$stats")" \
    "$(jq -r '.output_tokens' <<<"$stats")" \
    "$(jq -r '.reasoning_output_tokens' <<<"$stats")" \
    >> "$LEDGER"
}

run_codex() {
  local stage="$1" model="$2" effort="$3" sandbox="$4" prompt_file="$5" last_message="$6" events="$7"
  local effort_toml
  effort_toml="$(json_escape_toml_string "$effort")"

  log "Codex: $stage [$model / $effort / $sandbox]"

  # --json gives us token accounting. -o captures only the final agent message.
  # approval_policy=never is appropriate here because this script is non-interactive;
  # the sandbox remains the hard permission boundary.
  if ! codex exec \
      --ephemeral \
      --model "$model" \
      --sandbox "$sandbox" \
      -c "model_reasoning_effort=$effort_toml" \
      -c 'approval_policy="never"' \
      --json \
      -o "$last_message" \
      "$(cat "$prompt_file")" \
      > "$events"; then
    record_usage "$stage" "$model" "$effort" "$events"
    die "Codex stage '$stage' failed. See $events"
  fi

  record_usage "$stage" "$model" "$effort" "$events"

  if [[ "$sandbox" == "workspace-write" ]]; then
    track_new_files_for_diff
  fi
}

run_codex_structured_review() {
  local stage="$1" model="$2" effort="$3" prompt_file="$4" result_json="$5" events="$6"
  local effort_toml
  effort_toml="$(json_escape_toml_string "$effort")"

  log "Codex: $stage [$model / $effort / read-only]"

  if ! codex exec \
      --ephemeral \
      --model "$model" \
      --sandbox read-only \
      -c "model_reasoning_effort=$effort_toml" \
      -c 'approval_policy="never"' \
      --json \
      --output-schema "$REVIEW_SCHEMA" \
      -o "$result_json" \
      "$(cat "$prompt_file")" \
      > "$events"; then
    record_usage "$stage" "$model" "$effort" "$events"
    die "Codex review stage '$stage' failed. See $events"
  fi

  record_usage "$stage" "$model" "$effort" "$events"
  jq -e . "$result_json" >/dev/null || die "Review result is not valid JSON: $result_json"
}

# ----------------------------- Quality gates ---------------------------------
run_quality_gates_raw() {
  if [[ -n "$TEST_CMD" ]]; then
    info "TEST_CMD: $TEST_CMD"
    bash -lc "$TEST_CMD"
  elif [[ -x ./run-regression-tests.sh ]]; then
    info "Detected ./run-regression-tests.sh"
    ./run-regression-tests.sh
  elif [[ -f go.mod ]]; then
    info "Detected Go module: go test ./..."
    go test ./...
  else
    warn "No TEST_CMD, executable run-regression-tests.sh, or go.mod detected. Skipping test command."
  fi

  if [[ -f go.mod && "$RUN_STATICCHECK" == "1" ]] && command -v staticcheck >/dev/null 2>&1; then
    info "staticcheck ./..."
    staticcheck ./...
  fi

  if [[ -f go.mod && "$RUN_GOLANGCI_LINT" == "1" ]] && command -v golangci-lint >/dev/null 2>&1; then
    info "golangci-lint run"
    golangci-lint run
  fi
}

run_quality_gates() {
  local log_file="$1"
  log "Local quality gates (no AI credits)"

  set +e
  run_quality_gates_raw > "$log_file" 2>&1
  local rc=$?
  set -e

  cat "$log_file"
  return "$rc"
}

repair_quality_gates() {
  local context="$1"
  local attempt gate_log prompt last events

  for ((attempt=1; attempt<=MAX_LOCAL_FIX_CYCLES; attempt++)); do
    gate_log="$RUN_DIR/gates-${context}-${attempt}.log"
    if run_quality_gates "$gate_log"; then
      return 0
    fi

    prompt="$RUN_DIR/fix-gates-${context}-${attempt}.prompt.md"
    last="$RUN_DIR/fix-gates-${context}-${attempt}.last.txt"
    events="$RUN_DIR/fix-gates-${context}-${attempt}.events.jsonl"

    {
      cat <<'PROMPT'
You are the implementation/fix agent. Repair the current working tree so that the local quality gates pass.

Constraints:
- Make the smallest correct changes.
- Do not refactor unrelated code.
- Follow AGENTS.md and repository architecture/documentation.
- Do not commit, push, rebase, or alter Git history.
- Do not touch .codex-cycle.
- The outer orchestration script will rerun the full quality gates, so run only targeted tests needed to diagnose the failure.

The most recent gate output is below:

```text
PROMPT
      tail -n 250 "$gate_log"
      printf '\n```\n'
    } > "$prompt"

    run_codex "quality-gate-fix-$context-$attempt" "$FIX_MODEL" "$FIX_EFFORT" workspace-write "$prompt" "$last" "$events"
  done

  # One final check after the last repair attempt.
  gate_log="$RUN_DIR/gates-${context}-final.log"
  if run_quality_gates "$gate_log"; then
    return 0
  fi

  return 1
}

# ----------------------------- Implementation --------------------------------
IMPLEMENT_PROMPT="$RUN_DIR/implementation.prompt.md"
IMPLEMENT_LAST="$RUN_DIR/implementation.last.txt"
IMPLEMENT_EVENTS="$RUN_DIR/implementation.events.jsonl"

{
  cat <<PROMPT
You are the implementation agent for the current Git repository.

Implement the task in:
  $TASK_FILE

Repository/base information:
- Base branch: $BASE_BRANCH
- Base commit for this cycle: $BASE_COMMIT
- Current branch: $CURRENT_BRANCH

Execution policy:
- Read the task file and relevant repository documentation, including AGENTS.md if present.
- Make the smallest complete and correct implementation.
- Preserve existing architecture and public behaviour unless the requirement explicitly changes it.
- Add or update focused tests when appropriate.
- Do not refactor unrelated code.
- Do not commit, push, rebase, switch branches, or alter Git history.
- Do not touch .codex-cycle.
- Do not run broad/full regression suites. The outer script runs them after you finish. You may run narrowly targeted tests when useful.
- Finish by briefly stating what you changed and any assumptions.
PROMPT
} > "$IMPLEMENT_PROMPT"

run_codex "implementation" "$IMPLEMENT_MODEL" "$IMPLEMENT_EFFORT" workspace-write \
  "$IMPLEMENT_PROMPT" "$IMPLEMENT_LAST" "$IMPLEMENT_EVENTS"

if git diff --quiet "$BASE_COMMIT"; then
  die "Implementation produced no changes relative to the base commit."
fi

if ! repair_quality_gates "post-implementation"; then
  warn "Local quality gates still fail after $MAX_LOCAL_FIX_CYCLES repair attempts. Sol review was NOT invoked, preserving review credits."
  exit 3
fi

# ------------------------------ Review policy --------------------------------
changed_files() {
  git diff --name-only "$BASE_COMMIT" --
}

is_docs_only_change() {
  local f
  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    case "$f" in
      *.md|*.mdx|*.txt|*.rst|LICENSE|LICENSE.*|NOTICE|NOTICE.*|docs/*|documentation/*)
        ;;
      *)
        return 1
        ;;
    esac
  done < <(changed_files)
  return 0
}

is_high_risk_change() {
  local files total_lines
  files="$(changed_files)"
  total_lines="$(git diff --numstat "$BASE_COMMIT" -- \
    | awk '{a += ($1 ~ /^[0-9]+$/ ? $1 : 0); d += ($2 ~ /^[0-9]+$/ ? $2 : 0)} END {print a+d+0}')"

  if (( total_lines >= LARGE_DIFF_LINES )); then
    return 0
  fi

  # Conservative path/name signals for changes that deserve the stronger review tier.
  grep -Eqi '(^|/)(auth|security|crypto|storage|database|db|migration|migrations|api|openapi|schema|concurrency|locking|transaction|transactions|k8s|kubernetes|deploy|deployment)(/|$)|(^|/)(Dockerfile|docker-compose[^/]*|.*\.sql$|.*\.proto$|openapi[^/]*\.(ya?ml|json)$)' <<< "$files"
}

choose_review_tier() {
  case "$REVIEW_POLICY" in
    sol-high)
      REVIEW_MODEL_SELECTED="gpt-5.6-sol"; REVIEW_EFFORT_SELECTED="high"; REVIEW_REASON="forced sol-high policy" ;;
    sol-medium)
      REVIEW_MODEL_SELECTED="gpt-5.6-sol"; REVIEW_EFFORT_SELECTED="medium"; REVIEW_REASON="forced sol-medium policy" ;;
    terra-low)
      REVIEW_MODEL_SELECTED="gpt-5.6-terra"; REVIEW_EFFORT_SELECTED="low"; REVIEW_REASON="forced terra-low policy" ;;
    auto)
      if is_docs_only_change; then
        REVIEW_MODEL_SELECTED="gpt-5.6-terra"; REVIEW_EFFORT_SELECTED="low"; REVIEW_REASON="documentation-only change"
      elif is_high_risk_change; then
        REVIEW_MODEL_SELECTED="gpt-5.6-sol"; REVIEW_EFFORT_SELECTED="high"; REVIEW_REASON="high-risk path or large diff"
      else
        REVIEW_MODEL_SELECTED="gpt-5.6-sol"; REVIEW_EFFORT_SELECTED="medium"; REVIEW_REASON="ordinary code change"
      fi
      ;;
    *) die "Unknown REVIEW_POLICY: $REVIEW_POLICY" ;;
  esac

  [[ -z "$REVIEW_MODEL_OVERRIDE" ]] || REVIEW_MODEL_SELECTED="$REVIEW_MODEL_OVERRIDE"
  [[ -z "$REVIEW_EFFORT_OVERRIDE" ]] || REVIEW_EFFORT_SELECTED="$REVIEW_EFFORT_OVERRIDE"
}

choose_review_tier
log "Review tier: $REVIEW_MODEL_SELECTED / $REVIEW_EFFORT_SELECTED"
info "$REVIEW_REASON"

git diff --stat "$BASE_COMMIT" -- | tee "$RUN_DIR/diff.stat"
git diff --name-only "$BASE_COMMIT" -- > "$RUN_DIR/changed-files.txt"

# ------------------------------ Review schema --------------------------------
REVIEW_SCHEMA="$RUN_DIR/review-schema.json"
cat > "$REVIEW_SCHEMA" <<'JSON'
{
  "type": "object",
  "properties": {
    "status": {
      "type": "string",
      "enum": ["CLEAN", "CHANGES_REQUIRED"]
    },
    "confidence": {
      "type": "string",
      "enum": ["high", "medium", "low"]
    },
    "summary": {
      "type": "string"
    },
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "severity": {
            "type": "string",
            "enum": ["critical", "high", "medium", "low"]
          },
          "file": { "type": "string" },
          "line": { "type": "string" },
          "problem": { "type": "string" },
          "recommendation": { "type": "string" }
        },
        "required": ["severity", "file", "line", "problem", "recommendation"],
        "additionalProperties": false
      }
    }
  },
  "required": ["status", "confidence", "summary", "findings"],
  "additionalProperties": false
}
JSON

# ------------------------------ Review loop ----------------------------------
review_cycle=1
escalation_used=0

while (( review_cycle <= MAX_REVIEW_CYCLES )); do
  REVIEW_PROMPT="$RUN_DIR/review-${review_cycle}.prompt.md"
  REVIEW_RESULT="$RUN_DIR/review-${review_cycle}.json"
  REVIEW_EVENTS="$RUN_DIR/review-${review_cycle}.events.jsonl"

  {
    cat <<PROMPT
You are the independent senior code reviewer. Do not modify any files.

Review the current working tree against base commit:
  $BASE_COMMIT

The task/requirements are in:
  $TASK_FILE

Credit/context discipline:
- Start with: git diff --find-renames $BASE_COMMIT --
- Focus on changed code and the requirement.
- Inspect surrounding files only when needed to verify behaviour, contracts, or call sites.
- Do not perform broad repository exploration without a concrete reason.
- Local quality gates have already passed. Do not rerun broad/full regression suites.
- You may run cheap/read-only targeted commands if they materially help the review.
- Ignore .codex-cycle.

Review for concrete defects, especially:
- requirement mismatches and functional correctness
- regressions and compatibility
- concurrency/race/deadlock hazards
- security and trust-boundary problems
- resource lifetime/leaks
- incorrect error handling
- API/schema/data-model compatibility
- edge cases and failure modes
- inadequate or missing tests for changed behaviour
- architectural violations that can cause real maintenance or correctness problems

Do not request changes for subjective style preferences or speculative refactors.
If there are no actionable defects, return CLEAN with an empty findings array.
If actionable defects exist, return CHANGES_REQUIRED and precise findings.
PROMPT
  } > "$REVIEW_PROMPT"

  run_codex_structured_review "review-$review_cycle" "$REVIEW_MODEL_SELECTED" "$REVIEW_EFFORT_SELECTED" \
    "$REVIEW_PROMPT" "$REVIEW_RESULT" "$REVIEW_EVENTS"

  log "Review $review_cycle result"
  jq . "$REVIEW_RESULT"

  status="$(jq -r '.status' "$REVIEW_RESULT")"
  confidence="$(jq -r '.confidence' "$REVIEW_RESULT")"

  if [[ "$status" == "CLEAN" ]]; then
    # Spend extra review credits only when a cheaper tier explicitly says it is uncertain.
    if [[ "$AUTO_ESCALATE_LOW_CONFIDENCE" == "1" && "$confidence" == "low" && "$REVIEW_EFFORT_SELECTED" != "high" && "$escalation_used" == "0" && "$review_cycle" -lt "$MAX_REVIEW_CYCLES" ]]; then
      warn "Clean review reported low confidence. Escalating once to Sol/high."
      REVIEW_MODEL_SELECTED="gpt-5.6-sol"
      REVIEW_EFFORT_SELECTED="high"
      escalation_used=1
      ((review_cycle++))
      continue
    fi

    log "Independent review is CLEAN"
    break
  fi

  if (( review_cycle == MAX_REVIEW_CYCLES )); then
    warn "Review still requires changes and the review budget is exhausted."
    break
  fi

  FIX_PROMPT="$RUN_DIR/review-fix-${review_cycle}.prompt.md"
  FIX_LAST="$RUN_DIR/review-fix-${review_cycle}.last.txt"
  FIX_EVENTS="$RUN_DIR/review-fix-${review_cycle}.events.jsonl"

  {
    cat <<PROMPT
You are the implementation/fix agent.

An independent reviewer found actionable defects in the current implementation.
The review report is:
  $REVIEW_RESULT

Instructions:
- Read the report and inspect the relevant changed code.
- Fix substantiated findings only.
- Make the smallest correct changes.
- Preserve the original issue/task intent.
- Do not refactor unrelated code.
- Add/update focused tests when needed for the defect.
- Do not commit, push, rebase, switch branches, or alter Git history.
- Do not touch .codex-cycle.
- Do not run broad/full regression suites; the outer script will do that.
PROMPT
  } > "$FIX_PROMPT"

  run_codex "review-fix-$review_cycle" "$FIX_MODEL" "$FIX_EFFORT" workspace-write \
    "$FIX_PROMPT" "$FIX_LAST" "$FIX_EVENTS"

  if ! repair_quality_gates "post-review-$review_cycle"; then
    warn "Local gates failed after applying review fixes. Stopping before another Sol review."
    exit 3
  fi

  ((review_cycle++))
done

FINAL_REVIEW_RESULT="$RUN_DIR/review-${review_cycle}.json"
if [[ ! -f "$FINAL_REVIEW_RESULT" ]]; then
  # Can happen when an escalation increments beyond the configured budget.
  FINAL_REVIEW_RESULT="$(find "$RUN_DIR" -maxdepth 1 -name 'review-*.json' | sort -V | tail -n 1)"
fi

FINAL_STATUS="CHANGES_REQUIRED"
if [[ -n "$FINAL_REVIEW_RESULT" && -f "$FINAL_REVIEW_RESULT" ]]; then
  FINAL_STATUS="$(jq -r '.status' "$FINAL_REVIEW_RESULT")"
fi

# ------------------------------ Optional Git ---------------------------------
if [[ "$FINAL_STATUS" == "CLEAN" && "$AUTO_COMMIT" == "1" ]]; then
  log "Creating Git commit"
  git add -A
  if [[ -n "$ISSUE_NUMBER" ]]; then
    COMMIT_MSG="Issue #$ISSUE_NUMBER: $ISSUE_TITLE"
  else
    COMMIT_MSG="Implement $ISSUE_TITLE"
  fi
  git commit -m "$COMMIT_MSG"
fi

if [[ "$FINAL_STATUS" == "CLEAN" && "$AUTO_PUSH" == "1" ]]; then
  [[ "$AUTO_COMMIT" == "1" ]] || die "AUTO_PUSH=1 requires AUTO_COMMIT=1."
  log "Pushing $CURRENT_BRANCH to $REMOTE"
  git push -u "$REMOTE" "$CURRENT_BRANCH"
fi

# ------------------------------- Summary -------------------------------------
log "Cycle summary"
printf 'Repository:       %s\n' "$REPO_ROOT"
printf 'Branch:           %s\n' "$CURRENT_BRANCH"
printf 'Base:             %s (%s)\n' "$BASE_BRANCH" "$BASE_COMMIT"
printf 'Review status:    %s\n' "$FINAL_STATUS"
printf 'Artifacts:        %s\n' "$RUN_DIR"
printf '\nUsage ledger:\n'

if command -v column >/dev/null 2>&1; then
  column -t -s $'\t' "$LEDGER"
else
  cat "$LEDGER"
fi

printf '\nTotals:\n'
jq -Rn \
  '[inputs | split("\t")] as $rows |
   ($rows[1:] // []) |
   reduce .[] as $r
     ({input_tokens:0,cached_input_tokens:0,output_tokens:0,reasoning_output_tokens:0};
      .input_tokens += (($r[3] // "0")|tonumber) |
      .cached_input_tokens += (($r[4] // "0")|tonumber) |
      .output_tokens += (($r[5] // "0")|tonumber) |
      .reasoning_output_tokens += (($r[6] // "0")|tonumber))' \
  < "$LEDGER"

printf '\nFinal diff stat:\n'
git diff --stat "$BASE_COMMIT" --

if [[ "$FINAL_STATUS" == "CLEAN" ]]; then
  printf '\n\033[1;32mPASS:\033[0m implementation, local gates, and independent review are clean.\n'
  exit 0
fi

printf '\n\033[1;31mSTOP:\033[0m review budget exhausted with outstanding findings.\n'
printf 'Inspect: %s\n' "$FINAL_REVIEW_RESULT"
exit 2
