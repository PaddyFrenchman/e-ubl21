# UBL-21 Open Issues — Recommended Implementation Order

This document captures all currently open GitHub issues and orders them by implementation dependency rather than issue number.

The sequence is intentionally structured around three principles:

1. **Freeze regression behavior before architectural refactoring.**
2. **Complete the durable asynchronous validation architecture before making Peppol a production stage.**
3. **Finish with release hardening, performance evidence, upgrade policy, and optional envelope validation.**

The order below is the recommended default execution path. Some documentation, corpus, and asset-packaging work can proceed in parallel where dependencies allow.

### Issue-index synchronization rule

Whenever a tracked GitHub issue is created, closed, materially re-scoped, or re-ordered, `ISSUES.MD` must be updated in the same workflow so that the repository issue index remains synchronized with GitHub. Deferred maintenance and future-improvement issues must also be recorded here even when they are intentionally outside the immediate implementation path.

### Current dependency checkpoint

After completion of #60, the **next actionable issue is #59 — TODO-VALIDATION-SPLIT-012 — Add independent Kubernetes deployments and autoscaling**. Issues #48 through #58 and #60 are complete. Issue #45 is **blocked/deferred**, not the next task by issue number. Do not implement #45 until the first supported Peppol validator, pinned release assets, native asynchronous runtime stage, official/reference cross-check, and complete positive/negative Peppol corpus exist. Required predecessors include #37, #38, #64, and the Peppol sequence #66–#78.

An empty, guessed, or implementation-derived Peppol baseline is forbidden. The #45 baseline must freeze already-verified external standards behavior; it must never manufacture the expectations it is intended to protect.

## Branch and issue workflow

The repository uses a GitFlow-lite model with three branch roles:

- `main` is the release-history branch. It is updated only when a release is promoted and every release commit on `main` must be tagged with its release version.
- `development` is the designated default integration branch for ongoing work. The GitHub repository default branch must be set to `development`. All normal issue branches are created from `development` and are integrated back into `development`.
- `release/vX.Y.Z` branches are short-lived release-stabilization branches created from `development` when a specific release is prepared. A permanent `release` branch is not used.

The normal flow is:

```text
issue/<number>-<description>
        |
        v
   development
        |
        v
 release/vX.Y.Z
        |
        v
      main
        |
        v
     tag vX.Y.Z
```

Release-only fixes made on `release/vX.Y.Z` must also be propagated back to `development` so that the next development cycle does not lose them.

### Required repository settings

The branch model is a repository invariant, not a naming convention. GitHub must be configured as follows whenever the repository plan supports enforcement:

- Set `development` as the GitHub default branch.
- Protect `main` from direct day-to-day development pushes, force pushes, and deletion. Changes to `main` must come only from an approved `release/vX.Y.Z` promotion and must pass the release CI gates.
- Protect `development` from force pushes and deletion. Normal changes reach it through issue-branch integration after required CI succeeds.
- Keep linear history on `development` and `main` unless a specific exception is explicitly authorized.
- Require the applicable correctness, Minikube, and standards-validation checks for development and release integration.
- Do not create a permanent `release` branch. Protect short-lived `release/vX.Y.Z` branches as release candidates where repository-plan capabilities permit.

If GitHub server-side branch protection is unavailable for the current repository plan, these rules remain mandatory project policy and must be enforced procedurally by the documented workflow and CI until server-side enforcement becomes available.

### CI branch policy

The repository CI workflows must follow the branch model:

- Pull requests targeting `development`, `release/vX.Y.Z`, or `main` run the relevant correctness and compliance gates.
- Pushes to `development`, `release/vX.Y.Z`, and `main` run the relevant regression/compliance gates.
- `main` CI is therefore a release-verification safety net, not the normal development integration path.
- Cleanup of completed `issue/*` branches is based on containment in `development`, not containment in `main`.

### Issue branch creation

Each GitHub issue must be implemented on its own issue branch created from the latest `origin/development`.

1. Fetch the latest remote state with `git fetch origin --prune`.
2. Update the local development branch with `git switch development` followed by `git pull --ff-only origin development`.
3. Create the issue branch from `development`, for example `git switch -c issue/59-kubernetes-autoscaling`.
4. Implement, test, commit, and push only on the issue branch until integration is explicitly authorized.

Issue branches must not be created from `main` or from a release branch.

### Explicit authorization boundary

Codex must **never close a GitHub issue unless the user explicitly instructs it to close that specific issue**. A request to review, fix, implement, test, commit, push, or otherwise work on an issue is not permission to close it.

Until the user explicitly says to close the issue, Codex must keep the work on the issue branch. It may commit or push that issue branch when requested or when completing the implementation workflow, but it must not begin the closing workflow: do not rebase for integration, switch to `development`, integrate into `development`, push `development`, or change the GitHub issue state.

Mentioning this workflow, asking whether work is committed or pushed, or saying that the workflow should be followed does not authorize issue closure or integration. If the user's intent to close a specific issue is ambiguous, Codex must ask rather than infer permission.

Only when Codex in VS Code is explicitly told to **close a specific issue** must it use the following **rebase-first, fast-forward-only** integration workflow. The intent is to keep `development` linear and avoid merging `development` into the issue branch.

1. Fetch the latest remote state with `git fetch origin --prune` and ensure the working tree is clean.
2. Switch to the issue branch.
3. Rebase the issue branch onto the current `origin/development` with `git rebase origin/development`.
4. Resolve any conflicts on the issue branch. If the rebase cannot be completed safely, abort it and report the problem rather than merging `development` into the issue branch.
5. Run the tests and verification required by the issue/repository.
6. If the rebase rewrote commits on an already-published issue branch, update that remote branch with `git push --force-with-lease origin <issue-branch>`; otherwise use a normal push as appropriate.
7. Switch to `development` and update it with `git pull --ff-only origin development`.
8. Integrate the issue branch with `git merge --ff-only <issue-branch>`.
9. Run the final required verification on the integrated `development` state.
10. Push `development` with `git push origin development`.
11. Only after the push succeeds should Codex close the GitHub issue.

If `git merge --ff-only` fails because `development` advanced after the rebase, return to the issue branch, rebase it onto the new `origin/development`, repeat verification, and try the fast-forward integration again.

**Do not:**

- merge `development` into an issue branch;
- integrate an issue branch directly into `main`;
- integrate an issue branch directly into a `release/vX.Y.Z` branch;
- create a merge commit when integrating an issue branch unless explicitly instructed;
- close the GitHub issue before the rebased branch has been successfully fast-forwarded into `development`, verified, and pushed.

### Release preparation workflow

A release branch is created only when a concrete version is ready for stabilization.

1. Ensure `development` contains all work selected for the release and that required CI/regression gates pass.
2. Create `release/vX.Y.Z` from the current release candidate on `development`.
3. Freeze feature work on that release branch. Only release-blocking fixes, versioning, packaging, documentation, and release-hardening changes belong there.
4. Validate the complete release candidate on `release/vX.Y.Z`.
5. Promote the release branch into protected `main` through the approved release integration path.
6. Tag the resulting release commit on `main` as `vX.Y.Z`.
7. Push the tag and publish any associated release artifacts.
8. Propagate any fixes made only on `release/vX.Y.Z` back into `development`.
9. Delete the release branch after the release is complete and all release-only fixes are safely present in `development`.

`main` must never be used as the day-to-day integration branch. Between releases it remains unchanged.
