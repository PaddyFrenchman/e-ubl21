# Codex GitHub Issue Development Loop

This folder contains a bounded automation loop for implementing GitHub issues with Codex.

## Model

GitHub issues are the work queue.

The loop is:

1. select the next open issue labelled `ready` and `agent-suitable`;
2. create or reuse `issue/<number>-<slug>`;
3. invoke Codex for that single issue;
4. run the repository root `run-all-tests.sh`;
5. feed validation failures back to Codex;
6. repeat until validation succeeds or the retry limit is reached;
7. commit and push the implementation;
8. create a pull request against `development`;
9. mark the issue `in-review`;
10. continue with the next eligible issue.

The automation deliberately does **not** merge pull requests.

## Requirements

The workstation must provide:

- `git`
- `gh` authenticated for the repository
- `jq`
- `codex`
- all dependencies required by `../run-all-tests.sh`

## Labels

Issues intended for automated implementation must have both:

- `ready`
- `agent-suitable`

During processing the runner uses:

- `in-progress`
- `in-review`

Use `blocked` for work that must not be selected.

## Run

From the repository root:

```bash
bash codex/codex-agent.sh
```

Optional environment variables:

```bash
REPO=PaddyFrenchman/e-ubl21 \
BASE_BRANCH=development \
MAX_ATTEMPTS=5 \
bash codex/codex-agent.sh
```

To process only one issue:

```bash
STOP_AFTER_ONE=1 bash codex/codex-agent.sh
```

## Safety model

The shell runner owns Git and GitHub mutations. Codex is told not to commit, push,
merge, create pull requests, or close issues.

A failed validation run is passed back to Codex as evidence. The runner stops after
`MAX_ATTEMPTS` rather than looping forever.
