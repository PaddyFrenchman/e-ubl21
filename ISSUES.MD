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

After completion of #68, the **next actionable issue is #69 — TODO-PEPPOL-004 — Define and implement deterministic Schematron compilation**. Issues #48 through #63 and #65 through #68 are complete. Issue #64 remains open but is intentionally downstream of the Peppol validator/asset work, so it is not the next task. Issue #45 is **blocked/deferred**, not the next task by issue number. Do not implement #45 until the first supported Peppol validator, pinned release assets, native asynchronous runtime stage, official/reference cross-check, and complete positive/negative Peppol corpus exist. Required predecessors include #37, #38, #64, and the Peppol sequence #66–#78.

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

### Local development branch requirement

Every working clone must maintain a **local** `development` branch that tracks `origin/development`. `origin/development` is the remote-tracking ref; it is not the day-to-day working branch and issue work must not be started from it in detached-HEAD state.

For a clone that does not yet have a local `development` branch:

```bash
git fetch origin --prune
git switch -c development --track origin/development
```

If the local `development` branch already exists, ensure that its upstream is correct:

```bash
git switch development
git branch --set-upstream-to=origin/development development
```

Verify the relationship with:

```bash
git branch -vv
```

The `development` entry must show `[origin/development]`. Before starting any new issue branch, synchronize the local integration branch:

```bash
git switch development
git pull --ff-only origin development
```

The intended working relationship is therefore:

```text
local development <--> origin/development
         |
         +--> issue/<number>-<description>
```

### Issue branch creation

Each GitHub issue must be implemented on its own issue branch created from the synchronized **local `development` branch**.

1. Fetch the latest remote state with `git fetch origin --prune`.
2. Switch to the local integration branch with `git switch development`.
3. Synchronize it with `git pull --ff-only origin development`.
4. Create the issue branch from local `development`, for example `git switch -c issue/61-safe-deployment-migration`.
5. Implement and test on the issue branch.
6. Commit the reviewable work and push the issue branch.
7. Create a pull request from the issue branch into `development`.

Issue branches must not be created from `main`, from a release branch, or directly from the `origin/development` remote-tracking ref.

### Pull-request review cycle

The pull request is the normal code-review surface for an issue. **Creating, updating, or reviewing a pull request is part of implementation and is not part of the issue-closing workflow.** The issue remains open throughout the review cycle.

The review roles are deliberately separated:

- **ChatGPT is the designated code reviewer.** It reviews the complete PR diff, issue acceptance criteria, tests, CI results, architecture, regressions, migration/safety behavior, and existing review history.
- **Codex in VS Code is the implementation and fix agent.** It implements the issue, runs local verification, commits/pushes work, and addresses findings supplied from the ChatGPT review.
- **Codex must not perform a second independent full code review by default.** The user submits ChatGPT's review findings to Codex so that Codex can fix them without duplicating the complete review and consuming unnecessary coding-agent credits.
- While addressing a finding, Codex may explain why it believes a specific finding is incorrect or may surface a newly discovered implementation problem. That is not permission to start another broad PR review unless the user explicitly requests one.

The required review loop is:

1. Codex or the user implements the current issue on its issue branch and runs the appropriate local tests.
2. Commit and push the issue branch.
3. Create or update the pull request targeting `development`.
4. The user asks **ChatGPT to review the PR**.
5. ChatGPT reviews the complete current PR against `development`, the GitHub issue/acceptance criteria, relevant repository contracts and tests, current CI state, and prior unresolved findings. ChatGPT produces a complete review report and may post it to the PR when requested.
6. The user submits ChatGPT's review report to Codex. Codex addresses the identified findings, adds or updates tests where required, runs verification, then commits and pushes the fixes to the same issue branch. Codex does not repeat the full review unless explicitly instructed.
7. The user asks ChatGPT to re-review the updated PR. ChatGPT verifies the fixes, reviews the new diff for regressions or newly introduced defects, checks the relevant CI/tests, and updates the review verdict.
8. Findings that are fully addressed are recorded as resolved; unresolved findings remain blocking or advisory according to severity. New findings are added when necessary.
9. Steps 6 through 8 repeat until the implementation fully satisfies the issue and ChatGPT reports no unresolved blocking findings.

The PR and its review comments are retained as the audit trail connecting each finding to the commit that addressed it. Review comments must not be deleted merely because they are resolved.

During this review loop:

- do not ask Codex to run a redundant complete review after ChatGPT has already reviewed the same PR state unless the user explicitly wants an additional review;
- do not rebase the issue branch onto `development` merely because a review iteration completed;
- do not fast-forward the issue branch into `development`;
- do not close the PR as merged;
- do not close the GitHub issue;
- do not interpret a clean ChatGPT PR review as authorization to integrate or close the issue.

### Explicit authorization boundary

Neither ChatGPT nor Codex must **ever close a GitHub issue unless the user explicitly instructs it to close that specific issue**. A request to implement, test, commit, push, create a PR, review a PR, add comments, address review findings, or otherwise continue the review cycle is not permission to close or integrate the issue.

Until the user explicitly says to close the issue, the work remains on the issue branch and in its pull request. The branch may be committed and pushed repeatedly, and ChatGPT may review the PR repeatedly, but the closing/integration workflow must not begin: do not rebase for integration, switch to `development` for integration, fast-forward the issue branch into `development`, push the integrated `development`, or change the GitHub issue state.

Mentioning this workflow, asking whether work is committed or pushed, or saying that the issue appears complete does not authorize issue closure or integration. If the user's intent to close a specific issue is ambiguous, ChatGPT or Codex must ask rather than infer permission.

Only when explicitly told to **close a specific issue** must the following **rebase-first, fast-forward-only** integration workflow begin. The intent is to keep `development` linear while preserving the PR as the completed review record.

1. Fetch the latest remote state with `git fetch origin --prune` and ensure the working tree is clean.
2. Switch to the issue branch.
3. Rebase the issue branch onto the current `origin/development` with `git rebase origin/development`.
4. Resolve any conflicts on the issue branch. If the rebase cannot be completed safely, abort it and report the problem rather than merging `development` into the issue branch.
5. Run the tests and verification required by the issue/repository on the rebased branch.
6. A rebase normally rewrites existing commits; it does **not** require an additional commit unless resolving the rebase requires an actual content change. If published commits were rewritten, push the rebased issue branch with `git push --force-with-lease origin <issue-branch>`; otherwise use a normal push as appropriate.
7. Re-check the PR after the rebase to ensure the reviewed implementation remains unchanged in substance and no new blocking finding or conflict-resolution regression was introduced. ChatGPT remains the designated reviewer for any substantive post-rebase review; Codex should not perform a redundant full review unless explicitly requested.
8. Switch to `development` and update it with `git pull --ff-only origin development`.
9. Integrate the issue branch with `git merge --ff-only <issue-branch>`.
10. Run the final required verification on the integrated `development` state.
11. Push `development` with `git push origin development`.
12. Close the PR as completed/merged according to the repository's chosen PR bookkeeping method.
13. Only after the integrated `development` push succeeds should the GitHub issue be closed.
14. Delete the completed issue branch only after its commits are confirmed contained in `development`.

If `git merge --ff-only` fails because `development` advanced after the rebase, return to the issue branch, rebase it onto the new `origin/development`, repeat verification and the post-rebase PR check, then try the fast-forward integration again.

**Do not:**

- merge `development` into an issue branch;
- integrate an issue branch directly into `main`;
- integrate an issue branch directly into a `release/vX.Y.Z` branch;
- create a merge commit when integrating an issue branch unless explicitly instructed;
- use the GitHub PR merge button to bypass the documented rebase-first, fast-forward-only integration policy;
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

---

## Phase 1 — Freeze deterministic regression behavior

### 1. #32 — TODO-REGRESSION-001 — Freeze regression corpus and result contracts

**Goal:** Define stable fixture-manifest and normalized regression-result schemas before extending generation.

**Scope:** Define stable fixture IDs, profile/stage names, expected rule-ID representation, imported-fixture provenance, and volatile fields excluded from semantic comparison. Add schema-validation tests.

**Acceptance:** A regression fixture and its expected result can be reviewed without reading Go test code.

**Dependency:** Prerequisite for the validation-stage split and Peppol regression work.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/32

### 2. #33 — TODO-REGRESSION-002 — Make UBL generation deterministic and manifest-driven

**Goal:** Reproducibly generate the complete UBL 2.1 positive corpus for all 65 main document types.

Require deterministic values, ordering and namespaces, SHA-256 per fixture, automatic manifest entries, and independent `xmllint` validation.

**Acceptance:** Two consecutive clean generations are byte-identical and every generated positive sample is XSD-valid.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/33

### 3. #34 — TODO-REGRESSION-003 — Formalize UBL targeted-negative generation

**Goal:** Turn XSD-negative cases into deterministic manifest-defined fixtures.

Add a mutation catalogue for missing required elements, invalid datatypes, bad namespaces, wrong order, invalid enumerations, malformed XML, and record why every fixture must fail.

**Acceptance:** Every UBL-negative fixture predictably fails XSD and no later semantic stage executes.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/34

### 4. #35 — TODO-REGRESSION-004 — Integrate EN 16931 generator into unified corpus CLI

**Goal:** Expose the existing EN semantic generator through the common regression-corpus interface.

Add `--profile en16931`, positive Invoice/CreditNote generation, pinned official fixtures where applicable, common manifest entries, and ruleset metadata.

**Acceptance:** EN fixtures can be generated and enumerated independently of server test packages.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/35

### 5. #36 — TODO-REGRESSION-005 — Formalize EN 16931 targeted-negative corpus

**Goal:** Convert the existing EN rule-targeted negatives into manifest-defined regression fixtures with expected rule IDs and mutation provenance.

**Acceptance:** Every EN-negative fixture remains XSD-valid and fails EN 16931 with at least its expected official rule ID.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/36

### 6. #39 — TODO-REGRESSION-008 — Implement normalized regression runner

**Goal:** Run manifest fixtures through the authenticated public API and durable result path, normalize volatile fields, compare expected versus actual, and emit `results.jsonl`, `summary.json`, and `junit.xml`.

**Acceptance:** One command produces a deterministic semantic verdict for the complete selected corpus and exits non-zero on mismatch.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/39

### 7. #40 — TODO-REGRESSION-009 — Add regression evidence and log collector

**Goal:** Retain enough diagnostics to investigate failed regression runs.

Capture service logs, Kubernetes events, selected NATS state, bounded metrics, and environment metadata while keeping correctness results normalized and invoice bodies out of logs.

**Acceptance:** A failed fixture can be diagnosed from the regression artifact without immediately reproducing the CI environment.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/40

### 8. #41 — TODO-REGRESSION-010 — Create local Minikube regression deployment

**Goal:** Make Minikube the canonical local distributed integration environment.

Add Kubernetes deployment assets, image build/load automation, readiness checks, one-command corpus execution/cleanup, and local result/log collection.

**Acceptance:** `make regression-minikube` runs the selected corpus through the real Kubernetes topology and returns non-zero on regression.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/41

### 9. #42 — TODO-REGRESSION-011 — Add GitHub Actions correctness regression gate

**Goal:** Run deterministic regression on pull requests and pushes affecting `development`, `release/vX.Y.Z`, and `main`, with `development` as the normal integration branch and `main` reserved for release verification.

Generate and verify corpus hashes, run required runtimes with skipping disabled, execute the runner, upload artifacts on success/failure, and publish JUnit/test summaries.

**Acceptance:** Any unexpected standards-outcome change produces a failing GitHub check naming affected fixture IDs before development integration or release promotion.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/42

### 10. #43 — TODO-REGRESSION-012 — Add GitHub Minikube/Kubernetes regression gate

**Goal:** Exercise the deployed Kubernetes topology in GitHub using the same Minikube-oriented runner used locally.

Build/load candidate image, deploy NATS and e-ubl21 components, run the corpus, upload evidence, and tear down.

**Acceptance:** Local and GitHub Minikube runs share the same semantic regression logic and both fail on fixture mismatches.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/43

### 11. #44 — TODO-REGRESSION-013 — Establish pre/post validation-split equivalence gate

**Goal:** Prove the distributed validation refactor does not change standards behavior.

Capture a normalized monolithic baseline, run the same manifest against the split-stage architecture, and require identical outcomes and rule IDs. Separately test asynchronous queue, restart, and scale-to-zero behavior.

**Acceptance:** `TODO-VALIDATION-SPLIT` cannot complete while any fixture changes semantic result unexpectedly.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/44

---

## Phase 2 — Build the durable asynchronous validation architecture

### 12. #48 — TODO-VALIDATION-SPLIT-001 — Freeze service boundaries and state ownership

**Goal:** Define API, orchestrator, XSD worker, EN worker, and future Peppol-worker responsibilities before runtime changes.

Freeze that stage workers cannot finalize whole-document validity or choose the next stage. Document shared and stage-specific resources and crash boundaries.

**Acceptance:** A written ownership table leaves no state mutation shared by unrelated roles without an explicit CAS/idempotency contract.

**Dependency:** Regression corpus and baseline should be established first. This dependency is satisfied by #44. **#48 through #63 and #65 through #68 are complete; #69 is the current next actionable issue. #64 remains open for later Peppol-stage integration.**

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/48

### 13. #49 — TODO-VALIDATION-SPLIT-002 — Introduce durable validation execution and plan IDs

**Goal:** Separate immutable upload identity from a particular validation execution.

Persist validation ID/generation, requested profile, frozen ordered stage list, selected rule versions, and define revalidation identity semantics.

**Acceptance:** One upload can support one or more unambiguous validation executions without overwriting historical evidence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/49

### 14. #50 — TODO-VALIDATION-SPLIT-003 — Define versioned StageRequest and StageResult contracts

**Goal:** Create stable versioned queue contracts shared by all stage services.

Include validation/upload identity, stage/ordinal, plan/generation/ruleset version, timestamps, bounded diagnostics, outcome/reason enums, and deterministic idempotency keys. Reject unknown versions fail-closed.

**Acceptance:** Validators exchange stage messages without depending on another worker's internal types.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/50

### 15. #51 — TODO-VALIDATION-SPLIT-004 — Create namespace-derived stage queue topology

**Goal:** Provision durable namespace-derived request/result subjects and consumers independently for XSD, EN 16931, and future Peppol stages.

Provide independent AckWait, MaxAckPending, retention, and migration safety.

**Acceptance:** XSD and EN worker deployments can be stopped, scaled, and upgraded independently while their queues retain work durably.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/51

### 16. #52 — TODO-VALIDATION-SPLIT-005 — Implement validation orchestrator state machine

**Goal:** Move sequencing and finalization out of validators.

The orchestrator loads the frozen ValidationPlan, publishes first/next StageRequests, consumes StageResults, CAS-updates stage state, stops on standards failure, and finalizes the authoritative execution outcome. Publishing the canonical external `ValidationResult` remains owned by #56.

**Acceptance:** No validation worker directly invokes or queues a later stage.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/52

### 17. #53 — TODO-VALIDATION-SPLIT-006 — Make orchestrator transitions crash-safe and idempotent

**Goal:** Prevent lost or duplicated stage transitions across crash windows between state persistence, request publication, and acknowledgements.

Use CAS state, deterministic message IDs, durable transition intent/reconciliation, and duplicate request/result tolerance.

**Acceptance:** Killing the orchestrator at any transition boundary cannot lose work, skip a stage, advance twice, or create duplicate final results.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/53

### 18. #54 — TODO-VALIDATION-SPLIT-007 — Extract XSD validation into worker-xsd

**Goal:** Make UBL 2.1 XSD validation an independent service consuming only XSD StageRequests, fetching XML from Object Store, and publishing durable XSD StageResults before ACK.

**Acceptance:** XSD workers can continue processing while EN workers are stopped, with downstream EN work remaining durable.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/54

### 19. #55 — TODO-VALIDATION-SPLIT-008 — Extract EN 16931 validation into worker-en16931

**Goal:** Make EN 16931 an independently scalable service consuming only EN StageRequests after durable XSD-pass evidence, fetching XML independently, and publishing a durable EN StageResult before ACK.

**Acceptance:** EN workers can scale to zero and back without synchronously blocking API/XSD processing; queued EN work survives.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/55

### 20. #56 — TODO-VALIDATION-SPLIT-009 — Adapt final-result persistence to orchestrator ownership

**Goal:** Make the orchestrator the sole publisher of canonical ValidationResult while preserving ordered stage evidence, first-result permanence, reconciliation, and raw-content retention semantics.

**Acceptance:** Final `valid` exists only when every required frozen-plan stage has a persisted `passed` result; intermediate stages cannot appear as final.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/56

### 21. #57 — TODO-VALIDATION-SPLIT-010 — Implement independent stage retry and DLQ policies

**Goal:** Make operational retries, attempt counters, DLQ entries, and replay attributable per validation stage.

Preserve standards-invalid versus operational-failure semantics and prevent replay from bypassing prerequisites.

**Acceptance:** An EN runtime outage does not consume XSD retry budget, and stage replay can resume safely without unnecessary earlier-stage execution unless a new validation generation is requested.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/57

### 22. #58 — TODO-VALIDATION-SPLIT-011 — Add per-stage telemetry and queue-delay measurement

**Goal:** Expose per-stage requests/outcomes, service duration, queue-wait time, in-flight count, retries, DLQ/queue depth, redelivery, request age, throughput, and result-publication/orchestrator latency with bounded labels.

**Acceptance:** Operators can distinguish an intrinsically slow validator from an undersized pool where documents mostly wait in queue.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/58

### 23. #60 — TODO-VALIDATION-SPLIT-013 — Add distributed tracing and correlation across stage events

**Goal:** Propagate correlation and trace metadata through API, orchestrator, StageRequest, and StageResult events.

Record queue wait, XML fetch, validator execution, publication, and transition timing without sensitive invoice content.

**Acceptance:** One upload can be followed end-to-end through all services and asynchronous queues without timestamp guesswork.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/60

### 24. #59 — TODO-VALIDATION-SPLIT-012 — Add independent Kubernetes deployments and autoscaling

**Goal:** Deploy API, orchestrator, XSD worker, EN worker, and later Peppol worker as independently scalable Kubernetes workloads with independent resources, concurrency, prefetch, timeouts, readiness, and autoscaling.

**Acceptance:** EN load can increase EN replica count without changing XSD/API/orchestrator replicas; scaling is driven primarily by queue depth/age and measured service time.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/59

### 25. #61 — TODO-VALIDATION-SPLIT-014 — Migrate existing deployments safely

**Goal:** Move from the monolithic worker to asynchronous stages without mixed-generation processing or incomplete stage evidence.

Define generation markers, stop-and-drain barrier, pending-upload reconciliation, and rollback.

**Acceptance:** Migration cannot produce a document marked valid under the new model with missing required stage evidence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/61

### 26. #62 — TODO-VALIDATION-SPLIT-015 — Add chaos, crash and replay integration tests

**Goal:** Prove distributed transitions converge correctly under orchestrator/worker crashes, lost ACKs, duplicate results, unavailable Object Store/result streams, scale-to-zero, stale generations, and concurrent replay.

**Acceptance:** Every required failure scenario converges to exactly one correct durable terminal state and one permanent final result when applicable.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/62

### 27. #63 — TODO-VALIDATION-SPLIT-016 — Benchmark each stage and establish scaling baselines

**Goal:** Benchmark XSD, EN, orchestrator, and later Peppol independently.

Capture docs/sec, p50/p95/p99 service time, CPU/memory per document, process/JVM startup contribution, safe concurrency, and effects of document size and semantic complexity.

**Acceptance:** Each deployment has documented initial resources, concurrency, and scaling thresholds derived from measured stage behavior.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/63

### 28. #65 — TODO-VALIDATION-SPLIT-018 — Final end-to-end distributed validation gate

**Status:** Complete / closed.

**Goal:** Prove the split deployment preserves the existing validation contract while adding independent scaling and fault isolation.

**Required scenarios:**

- EN 16931 plan: public upload -> XSD PASS -> EN PASS -> final valid.
- XSD-invalid Invoice: final invalid and EN request count remains zero.
- EN-invalid Invoice: XSD PASS -> EN invalid -> final invalid.
- EN outage: XSD continues draining while EN queue accumulates; restoring EN drains it.
- Peppol plan when available: XSD PASS -> EN PASS -> Peppol PASS -> final valid.
- Scale EN workers to zero, prove durable accumulation, then restore workers and complete validation.

**Acceptance:** Each stage runs in an independently deployable process, consumes only its own durable StageRequests, and publishes durable StageResults. The orchestrator alone advances and finalizes. Failures, restarts, and scale-to-zero cannot lose work, skip validation, or create duplicate final results.

**Regression dependency:** Must satisfy #44 pre/post semantic equivalence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/65

---

## Phase 3 — Freeze the Peppol product contract and official assets

### 29. #66 — TODO-PEPPOL-001 — Freeze Peppol validation scope and contract

**Status:** Complete / closed.

**Goal:** Define exactly what e-ubl21 means by `Peppol valid` before implementing the validator.

**Scope:**

- First supported BIS: Peppol BIS Billing 3.0.
- UBL 2.1 Invoice and CreditNote syntax.
- Pin the mandatory Peppol release used at implementation time.
- Define Profile 01 and Profile 02 behavior.
- Define explicit versus automatic profile selection.
- Define final `valid`, `invalid`, and `processing_failed` semantics.
- Define warning/fatal/error behavior and Peppol version fields exposed to clients.

**Acceptance:** A documented truth table maps document type + requested profile + stage outcomes to final status.

**Contract:** [`docs/PEPPOL-VALIDATION-CONTRACT.md`](docs/PEPPOL-VALIDATION-CONTRACT.md)

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/66

### 30. #67 — TODO-PEPPOL-002 — Pin official Peppol release and provenance

**Status:** Complete / closed.

**Goal:** Make the exact Peppol standards implementation reproducible and establish `standard/` as the repository-local standards documentation/provenance entry point for Peppol and EN 16931.

**Scope:**

- Pin the immutable official OpenPeppol release/tag/archive and record BIS version, publication date, mandatory-from date, authoritative source URL/tag/archive, Schematron filenames, SHA-256 validation-asset digests, code-list versions/publication date, and licence/provenance.
- Never use a moving `main`, `master`, or `upcoming` branch as production truth.
- Create at least:

```text
standard/
├── README.md
├── en16931/
│   └── README.md
└── peppol/
    ├── README.md
    └── download.sh
```

- Make `standard/README.md` the canonical guide to implemented standards, committed versus downloaded material, runtime asset locations, versioning/provenance rules, and licensing constraints.
- Under `standard/en16931/`, document the implemented EN 16931 standard/version, authoritative CEN/EC access references, the pinned official ConnectingEurope/CEN-TC434 validation release, provenance, licence, checksums, and links to existing repository validation assets. Commit local documentation or validation-source material only where redistribution is permitted; do not commit copyrighted normative CEN documents without established redistribution rights.
- Do not mirror OpenPeppol rendered documentation in Git. Version `standard/peppol/download.sh` instead. It must retrieve only the authoritative pinned OpenPeppol release into a deterministic local subdirectory, verify version/provenance and SHA-256 where applicable, fail clearly on mismatch, and be safe to rerun.
- Document the pinned BIS release, authoritative sources, licence/redistribution constraints, downloader usage, expected local layout, and relationship to runtime Peppol validation assets in `standard/peppol/README.md`.
- Update `.gitignore` as necessary so downloaded non-redistributable Peppol documentation cannot be accidentally committed.
- Do not duplicate large runtime validation assets under `standard/` when they already live under an authoritative path such as `validation-assets/`; link and document them instead.

**Acceptance:** A clean checkout can identify exactly which Peppol release bytes the server is intended to validate against, exposes `standard/` as the canonical Peppol/EN 16931 documentation and provenance entry point, contains sufficient EN 16931 references/provenance/permitted local material to identify the implemented release, contains a version-controlled reproducible Peppol downloader rather than redistribution-restricted documentation, records licences/source URLs/checksums for audit, and prevents downloaded restricted material from being committed.

**Provenance:** [`standard/README.md`](standard/README.md), with detailed
[`EN 16931`](standard/en16931/README.md) and
[`Peppol`](standard/peppol/README.md) records.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/67

### 31. #68 — TODO-PEPPOL-003 — Package Peppol Schematron, code lists and example assets

**Status:** Complete / closed.

**Goal:** Bring official Peppol Billing rule sources into the same controlled asset lifecycle as EN 16931.

Pin the official Peppol UBL Schematron, required includes/resources, relevant code-list sources, official positive examples, provenance, and digests. Avoid duplicating the existing EN 16931 ruleset unless the official Peppol package requires it.

**Acceptance:** The validation-asset verifier fails on any missing file, byte change, wrong version, or provenance mismatch.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/68

### 32. #69 — TODO-PEPPOL-004 — Define and implement deterministic Schematron compilation

**Goal:** Produce a reproducible executable Peppol validation artefact compatible with the existing Saxon runtime.

Prefer an authoritative executable XSLT if available. Otherwise pin the ISO Schematron compilation toolchain and compile deterministically at build/release time. Store the compiled XSLT digest, prohibit runtime network compilation, add an SVRL smoke transform, and verify rule IDs, severity/role, diagnostics, and namespace bindings survive compilation.

**Acceptance:** The same pinned source/compiler inputs produce the accepted runtime artefact and it executes offline through Saxon.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/69

### 33. #70 — TODO-PEPPOL-005 — Extend layered validation result model for Peppol

**Goal:** Represent Peppol independently from UBL XSD and EN 16931.

Add a stable `peppol-bis-billing` layer with BIS version, resolved profile identifier, validity/outcome, issues, timestamp, and duration. Reuse the structured issue model without flattening validation layers.

**Acceptance:** A result can unambiguously show UBL PASS, EN 16931 PASS, and Peppol FAIL with relevant Peppol rule evidence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/70

### 34. #71 — TODO-PEPPOL-006 — Implement Peppol profile applicability and identifier checks

**Goal:** Prevent silent under-validation and ensure the requested Peppol Billing profile is actually tested.

Safely parse `cbc:CustomizationID` and `cbc:ProfileID`, recognize supported Billing customization and Profile 01/Profile 02 identifiers, define precise auto-detection behavior, and keep profile resolution separate from rule execution.

Under explicit Peppol validation, missing or unsupported identifiers are a Peppol standards failure, never EN-only success.

**Acceptance:** A caller requesting Peppol validation cannot obtain `valid` by omitting or corrupting required Peppol identifiers.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/71

---

## Phase 4 — Build deterministic Peppol regression data

### 35. #37 — TODO-REGRESSION-006 — Add Peppol semantic sample model and positive generator

**Goal:** Extend semantic Invoice/CreditNote generation to Peppol BIS Billing.

Add Peppol CustomizationID/ProfileID, legal endpoint/party schemes and code lists, Profile 01/02 variants, pinned provenance, and official examples where permitted.

**Acceptance:** Deterministic candidate Peppol Invoice/CreditNote fixtures can be generated before or alongside validator implementation.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/37

### 36. #38 — TODO-REGRESSION-007 — Add Peppol targeted-negative mutation catalogue

**Goal:** Generate Peppol-specific failures that still pass UBL XSD and EN 16931.

Define deterministic one-defect mutations for identifier, profile, code-list, common, and country-specific rule families and record verified expected Peppol rule IDs.

**Acceptance:** Every Peppol-negative fixture reaches the Peppol stage and fails with expected rule evidence once the validator exists.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/38

### 37. #76 — TODO-PEPPOL-011 — Build positive Peppol conformity corpus

**Goal:** Prove valid real-world Peppol Billing Invoice/CreditNote documents pass UBL 2.1, EN 16931, and Peppol.

**Coverage:** Official examples, Profile 01, Profile 02 where supported, representative VAT categories, payment means, identifier schemes, allowances/charges, attachments, and country-specific examples covered by the pinned ruleset.

Every fixture must declare expected ordered stage outcomes in the regression manifest.

**Acceptance:** A mandatory real-runtime regression target runs the positive corpus with missing dependencies treated as failure rather than skip.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/76

### 38. #77 — TODO-PEPPOL-012 — Build rule-targeted negative Peppol corpus

**Goal:** Prove the Peppol layer catches profile-specific failures that UBL XSD + EN 16931 alone accept.

Create deterministic XSD-valid and EN-valid mutations for identifiers, endpoint/EAS and party schemes, code lists, payment restrictions, syntax/cardinality, currency consistency, arithmetic rules, attachment/MIME restrictions, common rules, and selected country rules.

Retain controls proving XSD-invalid input never reaches EN/Peppol and EN-invalid input never reaches Peppol.

**Acceptance:** Every targeted Peppol negative reaches the Peppol stage and fails with its expected official rule evidence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/77

---

## Phase 5 — Implement and integrate Peppol validation

### 39. #72 — TODO-PEPPOL-007 — Implement Peppol validator adapter

**Goal:** Execute the pinned Peppol UBL validation artefact and return a structured Peppol StageResult.

Define a `PeppolValidator` interface, reuse/generalize the secure Saxon runner, stream XML over stdin, bound SVRL/stderr, reuse the bounded SVRL parser, preserve official Peppol rule IDs, and classify malformed output, runtime crashes, missing assets, timeouts, and cancellation as operational failures.

Warnings and successful reports remain visible without incorrectly becoming fatal.

**Acceptance:** Known Peppol-valid input passes; known Peppol-invalid input fails with expected official rule IDs; runtime failures are operational rather than standards-invalid.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/72

### 40. #75 — TODO-PEPPOL-010 — Extend HTTP/OpenAPI validation profile contract

**Goal:** Allow clients to request and inspect Peppol validation explicitly.

Add the chosen `validationProfile` request contract while preserving existing XML-upload compatibility. Expose requested/resolved profile in status responses and document the Peppol stage in OpenAPI with examples for successful and failed paths. Keep raw SVRL internal.

**Acceptance:** The public API can distinguish `EN 16931 valid` from `Peppol BIS Billing valid` without ambiguity.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/75

### 41. #64 — TODO-VALIDATION-SPLIT-017 — Integrate Peppol as a native asynchronous stage

**Goal:** Implement Peppol as `worker-peppol` with its own StageRequest/StageResult subjects, queued only after durable EN pass evidence, with independent timeout, retry, DLQ, telemetry, and autoscaling.

**Acceptance:** The orchestrator executes XSD -> durable result -> EN -> durable result -> Peppol -> durable result -> final, with every transition asynchronous.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/64

### 42. #73 — TODO-PEPPOL-008 — Integrate Peppol as the third validation stage

**Goal:** Integrate Peppol after durable XSD and EN 16931 success without reintroducing synchronous stage chaining.

Use `worker-peppol` and the validation-plan orchestrator. Peppol runs only for applicable explicit/resolved requests and only after durable XSD + EN pass evidence. Operational failure follows its own retry path. No partial final result may be published while Peppol is pending or running.

**Acceptance:** A Peppol-requested document cannot reach terminal `valid` with only XSD + EN evidence, and no validator calls the next validator synchronously.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/73

### 43. #74 — TODO-PEPPOL-009 — Integrate durable replay, failure and migration semantics

**Goal:** Make Peppol validation safe under retries, worker replacement, duplicate delivery, replay, and upgrades in the asynchronous stage architecture.

Require Peppol stage evidence when the frozen ValidationPlan includes it; never reinterpret legacy EN-only results as Peppol-valid. Define upgrade/migration barriers, stage-specific retry/DLQ/replay, shutdown behavior, stale-generation handling, and duplicate request/result behavior while preserving exactly one permanent final result.

**Acceptance:** No crash, replay, duplicate, or upgrade path can publish or preserve a false Peppol-valid result.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/74

### 44. #79 — TODO-PEPPOL-014 — Harden, observe and benchmark the Peppol stage

**Goal:** Give the Peppol stage the same production guarantees as XSD and EN 16931 while measuring it independently for autoscaling.

Add Peppol-specific timeout, cancellation/process termination, bounded SVRL/stderr/issues/diagnostics, structured stage logs, bounded Prometheus metrics, queue wait and service duration. Benchmark valid, invalid, large/high-line-count inputs and concurrency.

**Acceptance:** Peppol validation is bounded, independently observable, and has measured service/capacity data suitable for Kubernetes sizing and autoscaling.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/79

### 45. #78 — TODO-PEPPOL-013 — Cross-check against official/reference validation behavior

**Goal:** Reduce the risk that locally compiled/executed Peppol rules differ semantically from the official release.

Compare official example outcomes and, where practical, an independent reference validator using the exact same release. Freeze representative expected rules, verify compilation does not lose active rules/patterns/assertions, and allow only non-semantic formatting differences.

**Acceptance:** There is explicit evidence that e-ubl21 executes the pinned official Peppol rule set faithfully rather than merely producing internally consistent results.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/78

### 46. #45 — TODO-REGRESSION-014 — Establish Peppol release baseline

**Goal:** Freeze expected behavior for the first supported Peppol release.

Complete the positive/negative corpus, cross-check official/reference validation, record release/digests, and produce the first accepted normalized Peppol result set.

**Blocked:** Do not implement until #37, #38, #64, and #66–#78 have supplied the real pinned Peppol release assets, validator/runtime stage, official/reference evidence, and complete positive/negative corpus. An empty, guessed, placeholder, or automatically blessed baseline does not satisfy this issue.

**Acceptance:** `TODO-PEPPOL` cannot complete until the full Peppol corpus matches committed expectations.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/45

### 47. #80 — TODO-PEPPOL-015 — Production packaging and Peppol upgrade policy

**Goal:** Make Peppol validation reproducible, offline, and safely upgradeable.

Package all runtime assets in the production image; verify checksums before `worker-peppol` consumes work; log BIS/release identifiers and digests; add licence/provenance notices; add a `--network none` image smoke test and mandatory GitHub gate; document release upgrades including publication and mandatory-from dates.

**Acceptance:** A clean image contains everything required for offline Peppol Billing validation and reports the exact ruleset/release being executed.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/80

### 48. #81 — TODO-PEPPOL-016 — Final public end-to-end Peppol compliance gate

**Goal:** Prove the public server contract for Peppol Billing through the full asynchronous durable pipeline.

**Required scenarios include:**

- Valid Invoice and CreditNote: XSD PASS -> EN PASS -> Peppol PASS -> final valid.
- XSD-invalid request: no EN/Peppol stage.
- EN-invalid request: no Peppol stage.
- EN-valid/Peppol-invalid input exposes structured Peppol rule evidence.
- Missing/wrong identifiers under explicit Peppol profile are invalid and never silently downgraded.
- Valid Profile 01 and Profile 02.
- Operational failure exercises retry, DLQ, replay, and recovery.
- Worker/orchestrator interruption cannot create a partial final result.
- Duplicate/replayed stage messages still produce exactly one final result.
- EN-only backward compatibility remains EN-valid without being labelled Peppol-valid.

**Acceptance:** When Peppol validation is requested, `valid` is possible only after durable PASS evidence for UBL 2.1 XSD, pinned EN 16931, and pinned Peppol BIS Billing. The normalized Peppol regression baseline must pass first.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/81

---

## Phase 6 — Release engineering, performance, and standards lifecycle

### 49. #46 — TODO-REGRESSION-015 — Add release/nightly stress and performance evidence

**Goal:** Retain performance and capacity evidence separately from deterministic semantic correctness.

Exercise large, line-heavy, and near-limit fixtures and record stage queue wait, execution/end-to-end latency, and resource use on nightly, on-demand, and release runs.

**Acceptance:** Performance evolution is visible without flaky timing assertions contaminating the correctness gate.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/46

### 50. #47 — TODO-REGRESSION-016 — Document standards upgrade workflow around corpus changes

**Goal:** Prevent UBL, EN, or Peppol upgrades from silently rewriting expected behavior.

Require asset pinning, candidate regeneration, old/new result comparison, classification/review of every changed fixture, deliberate manifest updates, and retained before/after evidence.

**Acceptance:** Checksum or ruleset upgrades cannot merely bless changed tests; behavior differences require explicit reviewed regression evidence.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/47

---

## Phase 7 — Optional Peppol envelope validation

### 51. #82 — TODO-PEPPOL-017 — Optional Peppol enveloping/SBDH conformity validation

**Goal:** Optionally extend beyond payload conformity to Peppol Business Message Envelope/SBDH validation while keeping envelope validity separate from Invoice/CreditNote payload validity.

Pin official envelope/SBDH validation artefacts, define a separate `peppol-envelope` stage, validate sender/receiver/document/process identifiers against payload values, add positive and negative envelope corpora, package/version envelope rules independently, and integrate them as another asynchronous ValidationPlan stage when enabled.

**Acceptance:** The server can validate a complete Peppol business-message envelope without conflating envelope failures with Peppol Billing payload failures.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/82

---

## Phase 8 — Deferred maintenance and future platform improvements

These issues are intentionally tracked but are **not part of the current immediate implementation sequence** unless their revisit trigger becomes active.

### 52. #85 — MAINTENANCE — Harden hosted regression CI runner environment

**Goal:** Track non-blocking hosted CI maintenance, including Minikube/Docker disk-pressure risk and GitHub Actions runtime deprecation warnings.

**Revisit:** When hosted regression jobs become unstable due to disk pressure, or before GitHub removes the compatibility path for currently used action runtimes.

**Acceptance:** Hosted Minikube CI has an explicit stable disk-capacity strategy and the regression workflows no longer emit the tracked action-runtime deprecation warnings.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/85

### 53. #87 — FUTURE IMPROVEMENT — Evaluate Perses as Grafana replacement for on-prem deployments

**Goal:** Evaluate Perses as a permissively licensed dashboard/visualization layer for commercial on-prem deployments, potentially replacing Grafana OSS while retaining Prometheus/OpenTelemetry metrics.

**Revisit:** Before the first commercially distributed on-prem release, or earlier if dashboard implementation work begins.

**Acceptance:** Select Perses, retain Grafana under an approved licensing model, select another permissive dashboard, or deliberately ship without a dashboard. Capture the accepted decision in an ADR and deployment documentation.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/87

### 54. #88 — FUTURE IMPROVEMENT — Replace JetStream raw XML storage with S3-compatible object storage

**Goal:** Move raw XML persistence out of NATS JetStream Object Store into a backend-neutral S3-compatible storage layer, leaving NATS focused on messaging/orchestration.

**Configuration principle:** The object-storage backend must be selected by configuration rather than hard-wired into application logic. Amazon S3 is the cloud target, while **SeaweedFS is the proposed standard local/on-prem S3-compatible alternative**. Customer-provided S3-compatible stores, including Ceph RGW, should also be supportable through the same endpoint/bucket/credential contract.

**Revisit:** Before the first production-grade on-prem release, or earlier if storage is generalized for multi-environment deployment.

**Acceptance:** Adopt S3-compatible raw-content storage with SeaweedFS available as the local/on-prem configuration option, explicitly retain JetStream Object Store, or document another selected storage architecture in an ADR and deployment documentation.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/88

### 55. #89 — FUTURE IMPROVEMENT — Move upload metadata/status from JetStream KV to PostgreSQL

**Goal:** Evaluate PostgreSQL as the durable system of record for upload metadata/status, queryable lifecycle state, retry state, retention metadata, and future multi-tenant reporting.

**Target separation:** PostgreSQL stores metadata/status; S3-compatible storage stores raw XML; NATS/JetStream handles messaging and orchestration. The raw-content backend remains configurable, with **SeaweedFS proposed for local/on-prem deployment** and Amazon S3 for AWS.

**Revisit:** Before the first production-grade on-prem release, or earlier if persistence is generalized for multi-environment deployment.

**Acceptance:** Adopt PostgreSQL, explicitly retain JetStream KV, or document another metadata persistence architecture. Any accepted decision must be reflected in an ADR, deployment, retention, backup, and licensing documentation.

Issue: https://github.com/PaddyFrenchman/e-ubl21/issues/89

---

## Dependency summary

```text
Regression contracts/corpus
        |
        v
Normalized regression runner + CI gates
        |
        v
Pre/post semantic equivalence baseline
        |
        v
Validation service boundaries + durable plan IDs
        |
        v
Stage contracts + durable queues
        |
        v
Orchestrator + XSD/EN workers
        |
        v
Crash safety + retries + telemetry + Kubernetes
        |
        v
Distributed validation gate
        |
        v
Peppol scope + pinned assets + deterministic compilation
        |
        v
Peppol corpus + validator
        |
        v
worker-peppol + public API integration
        |
        v
Reference cross-check + regression baseline
        |
        v
Production packaging + final compliance gate
        |
        v
Performance/release lifecycle
        |
        v
Optional SBDH/envelope validation
```

The Phase 8 items are deliberately outside this immediate dependency chain. They are future product/platform choices with explicit revisit triggers rather than prerequisites for the current validation/Peppol roadmap.

## Notes

- Issues #64 and #73 are closely related and should be implemented as one coordinated Peppol-stage integration effort, while preserving both acceptance contracts.
- Issues #37/#76 and #38/#77 are paired: the regression issues define reusable generation capability; the Peppol issues define the conformity corpus that consumes it.
- Issues #44, #48–#63, and #65–#68 are complete; **#69 is therefore the current next actionable issue.** #64 remains open and intentionally follows the Peppol engine work because it integrates `worker-peppol` into the asynchronous runtime.
- Issue #45 is the safety rail for declaring the first Peppol release supported and remains blocked until its real Peppol prerequisites are complete.
- Issue #82 is intentionally last in the immediate standards roadmap because it extends transport-envelope conformity beyond the core UBL/EN/Peppol Billing payload contract.
- Issues #87, #88, and #89 are deliberately retained as future product-platform improvements and must not silently become current requirements without an explicit architecture decision.
- Issue #88 and #89 should be evaluated together when the future persistence architecture is revisited, while remaining separate issues because raw-content storage and metadata persistence have different migration and failure semantics.