#!/usr/bin/env bash
set -Eeuo pipefail

REPO="${REPO:-PaddyFrenchman/e-ubl21}"
READY_LABEL="${READY_LABEL:-ready}"
AGENT_LABEL="${AGENT_LABEL:-agent-suitable}"

gh issue list \
  --repo "$REPO" \
  --state open \
  --label "$READY_LABEL" \
  --label "$AGENT_LABEL" \
  --limit 100 \
  --json number,title,body,labels \
| jq -c '
    map(select((.labels | map(.name) | index("blocked")) | not))
    | sort_by(.number)
    | .[0] // empty
  '
