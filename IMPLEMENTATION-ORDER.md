# e-ubl21 implementation order

**Project:** e-ubl21  
**Purpose:** Define the mandatory order for addressing the current GitHub Issues and the Git workflow to use for every issue.  
**Tracking authority:** GitHub Issues are authoritative for implementation status. The `TODO-*.md` files remain architecture/specification documents.

---

## 1. General rule

Do not implement the open issues simply in numerical order.

The implementation sequence is dependency-driven:

```text
regression safety net
        |
        v
freeze current monolithic behaviour
        |
        v
split validation stages asynchronously
        |
        v
prove semantic equivalence
        |
        v
implement Peppol rule engine
        |
        v
integrate Peppol as an asynchronous stage
        |
        v
production / Kubernetes / final compliance gates
```

The completed historical issues `#3` through `#31` are retained as project history and must not be reopened merely to reproduce their work.

---

## 2. Mandatory per-issue Git workflow

**Every issue must be addressed in its own freshly recreated branch.**

One issue = one implementation branch.

The project is normally implemented with an AI coding agent such as **Claude** or **Codex** running in the editor. The coding agent must follow the same GitHub workflow as a human developer. It must never treat access to the working tree as permission to work directly on `main`, merge a branch, or close an issue automatically.

### 2.1 Starting an issue

Before starting or restarting work on an issue:

1. identify the GitHub Issue being addressed and read its complete description and acceptance criteria;
2. read the relevant `TODO-*.md`/architecture documents referenced by the issue;
3. switch to `main`;
4. fetch/pull the latest `main`;
5. remove any stale local branch for that issue if it exists;
6. recreate a fresh branch dedicated to the issue from current `main`;
7. immediately rebase that freshly recreated branch onto the latest `origin/main` before implementation begins;
8. verify that the editor/Claude/Codex session is operating on the issue branch, never on `main`;
9. implement only the scope of that GitHub Issue;
10. run the unit, integration and regression tests required by the issue;
11. commit the implementation to the issue branch and push that branch when appropriate;
12. keep the GitHub Issue open while implementation, review and validation are still in progress.

Recommended branch naming:

```text
issue/<github-issue-number>-<short-description>
```

Example:

```text
issue/32-regression-contracts
```

Typical local workflow:

```bash
git switch main
git fetch origin
git pull --ff-only origin main

git branch -D issue/32-regression-contracts 2>/dev/null || true
git switch -c issue/32-regression-contracts

git rebase origin/main
```

The initial rebase is part of **starting the issue** and does not require the special issue-closure confirmation described below.

If the issue branch has already been published and is later rebased, use:

```bash
git push --force-with-lease origin issue/32-regression-contracts
```

`--force-with-lease` must be used instead of an unconditional force push.

No new issue may be implemented directly on `main`.

If `main` changes materially while an issue is being developed, the branch may be rebased during development when necessary. Such a development-time rebase does not by itself mean the issue is ready to close.

### 2.2 Rules for Claude/Codex or another editor coding agent

When Claude, Codex or another coding agent is asked to implement a GitHub Issue, it must:

1. confirm the active Git branch before changing code;
2. refuse to implement the issue directly on `main` and instead use the dedicated issue branch;
3. use the GitHub Issue as the authoritative work item and the referenced roadmap files as the detailed technical specification;
4. keep changes scoped to the active issue unless a required dependency makes additional work unavoidable;
5. if genuinely independent additional work is discovered, create or propose a separate GitHub Issue rather than silently expanding scope;
6. run the tests specified by the issue and any regression gate applicable at that point in the implementation roadmap;
7. report test failures rather than hiding, skipping or automatically blessing changed expected results;
8. never modify committed regression expectations merely because the new implementation produces different results unless the issue explicitly concerns a reviewed standards/baseline change;
9. never merge the issue branch into `main` merely because implementation appears complete;
10. never close the GitHub Issue without the explicit issue-closing workflow below.

A pull request, when used, should reference the issue without prematurely closing it. During development prefer:

```text
Refs #<issue-number>
```

rather than relying on an automatic `Closes #<issue-number>` keyword before the final closure procedure has been approved.

The PR description should summarize:

- the issue being implemented;
- important architectural decisions;
- acceptance criteria addressed;
- tests/regression suites executed;
- any known limitations or follow-up issues.

### 2.3 Mandatory confirmation barrier when closing an issue

When the user says, for example:

```text
close issue #32
```

or otherwise requests that an issue be completed/closed, **do not immediately rebase, merge or close it**.

First inspect the issue branch and current `main`, summarize whether the implementation appears ready for finalization, and then ask an explicit confirmation question equivalent to:

> Confirm final rebase of the issue branch onto the latest `origin/main`, followed by the required post-rebase tests and, if they pass, merge and closure of the GitHub Issue?

**Stop at that point and wait for explicit confirmation.**

A request to close the issue is **not itself authorization to perform the final rebase**. The separate confirmation response is required.

Only after the user confirms may the finalization sequence begin:

1. fetch the latest `origin/main`;
2. verify the working tree and issue branch are clean/understood;
3. rebase the issue branch onto the latest `origin/main`;
4. if the rebase produces conflicts that require a semantic/architectural choice, stop and obtain guidance rather than silently choosing a potentially incorrect resolution;
5. rerun all tests required by the issue **after the final rebase**;
6. rerun the applicable regression gate and produce/inspect regression evidence when required;
7. push the rebased branch with `--force-with-lease` if the branch was already published;
8. verify the relevant GitHub Actions checks when they exist;
9. merge through the normal GitHub review/merge process;
10. verify that the merge is present on `main`;
11. close the GitHub Issue with reason `completed` only when all acceptance criteria are satisfied;
12. delete the merged issue branch locally and remotely when appropriate.

The final closure flow is therefore:

```text
user requests issue closure
        |
        v
inspect branch / tests / acceptance criteria
        |
        v
ASK FOR CONFIRMATION TO REBASE
        |
        +---- no / not yet ---> stop; leave issue open
        |
        v explicit confirmation
fetch latest main
        |
        v
final rebase onto origin/main
        |
        v
post-rebase tests + regression evidence
        |
        v
GitHub checks / review
        |
        v
merge to main
        |
        v
verify merge
        |
        v
close issue as completed
        |
        v
delete issue branch
```

A failing test, regression mismatch, unresolved conflict, unmet acceptance criterion or failed mandatory GitHub check stops the closure flow. The issue remains open until corrected.

---

# 3. Phase A — Build the regression safety net first

The first objective is to freeze what the current server accepts and rejects before its architecture is changed.

## Step A1 — Regression contracts

### Issue #32 — `TODO-REGRESSION-001`

**Freeze regression corpus and result contracts.**

Define stable manifest/result schemas, fixture IDs, profile/stage names, rule-ID expectations and normalization rules.

This issue is the foundation for every later regression task.

---

## Step A2 — UBL 2.1 positive corpus

### Issue #33 — `TODO-REGRESSION-002`

**Make UBL generation deterministic and manifest-driven.**

Generate deterministic positive UBL 2.1 fixtures with stable SHA-256 values and independent XSD verification.

---

## Step A3 — UBL 2.1 negative corpus

### Issue #34 — `TODO-REGRESSION-003`

**Formalize UBL targeted-negative generation.**

Produce deterministic XSD failures with explicit expected outcomes and prove semantic stages do not run afterward.

---

## Step A4 — EN 16931 positive corpus integration

### Issue #35 — `TODO-REGRESSION-004`

**Integrate the EN 16931 generator into the unified corpus CLI.**

Expose existing semantic Invoice/CreditNote generation through the common regression corpus.

---

## Step A5 — EN 16931 targeted negatives

### Issue #36 — `TODO-REGRESSION-005`

**Formalize the EN 16931 targeted-negative corpus.**

Make the existing expected EN rule failures first-class manifest data.

---

## Step A6 — Regression runner

### Issue #39 — `TODO-REGRESSION-008`

**Implement the normalized regression runner.**

The runner must exercise the public API and durable result path, compare actual results with the fixture manifest and produce:

```text
results.jsonl
summary.json
junit.xml
```

Raw logs are diagnostics, not the semantic oracle.

---

## Step A7 — Regression evidence

### Issue #40 — `TODO-REGRESSION-009`

**Add regression evidence and log collection.**

Collect enough logs, runtime metadata, NATS state and metrics to explain failures without contaminating deterministic result comparison.

---

## Step A8 — Local Minikube regression environment

### Issue #41 — `TODO-REGRESSION-010`

**Create the local Minikube regression deployment.**

The target developer entry point is:

```bash
make regression-minikube
```

The current monolithic server must be exercised first so it provides the pre-refactor reference behaviour.

---

## Step A9 — GitHub deterministic regression gate

### Issue #42 — `TODO-REGRESSION-011`

**Add the GitHub Actions correctness regression gate.**

Pull requests and pushes to `main` must fail when deterministic validation behaviour changes unexpectedly.

---

## Step A10 — Capture the monolithic baseline

### Issue #44 — `TODO-REGRESSION-013` — PART 1

Run the completed regression corpus against the existing monolithic validation worker and preserve the normalized accepted baseline.

**Do not close issue #44 yet.**

It remains open until the split implementation is complete and the same corpus produces semantically identical results.

This baseline is the safety barrier before changing validation topology.

---

# 4. Phase B — Split the validation stages

Only begin the architecture split after Phase A has produced a trustworthy baseline.

The intended architecture is:

```text
API
 |
 v
Validation Orchestrator
 |
 +--> XSD StageRequest queue
 |       |
 |       v
 |    worker-xsd
 |       |
 |       v
 |    XSD StageResult queue
 |
 +--> EN16931 StageRequest queue
 |       |
 |       v
 |    worker-en16931
 |       |
 |       v
 |    EN16931 StageResult queue
 |
 +--> future Peppol StageRequest queue
         |
         v
      worker-peppol
         |
         v
      Peppol StageResult queue
```

A validation worker must never synchronously call the next validator.

The orchestrator alone advances the plan and finalizes the document.

---

## Step B1

### Issue #48 — `TODO-VALIDATION-SPLIT-001`

Freeze service boundaries and state ownership.

---

## Step B2

### Issue #49 — `TODO-VALIDATION-SPLIT-002`

Introduce durable validation execution identity and frozen ValidationPlan data.

---

## Step B3

### Issue #50 — `TODO-VALIDATION-SPLIT-003`

Define versioned `StageRequest` and `StageResult` contracts.

---

## Step B4

### Issue #51 — `TODO-VALIDATION-SPLIT-004`

Create the namespace-derived durable stage queue topology.

---

## Step B5

### Issue #52 — `TODO-VALIDATION-SPLIT-005`

Implement the validation orchestrator state machine.

---

## Step B6

### Issue #53 — `TODO-VALIDATION-SPLIT-006`

Make orchestrator transitions crash-safe and idempotent before extracting the validators.

---

## Step B7

### Issue #54 — `TODO-VALIDATION-SPLIT-007`

Extract UBL XSD validation into `worker-xsd`.

---

## Step B8

### Issue #55 — `TODO-VALIDATION-SPLIT-008`

Extract EN 16931 validation into `worker-en16931`.

---

## Step B9

### Issue #56 — `TODO-VALIDATION-SPLIT-009`

Move permanent final-result ownership exclusively to the orchestrator.

---

## Step B10

### Issue #57 — `TODO-VALIDATION-SPLIT-010`

Implement independent stage retry, DLQ and replay policies.

---

## Step B11

### Issue #58 — `TODO-VALIDATION-SPLIT-011`

Add per-stage telemetry and explicit queue-wait measurement.

The telemetry must make it possible to distinguish:

```text
slow validator execution
```

from:

```text
undersized worker pool / excessive queue wait
```

---

## Step B12

### Issue #60 — `TODO-VALIDATION-SPLIT-013`

Add distributed tracing and correlation before expanding the production deployment topology.

Once validation is distributed, the pipeline must be traceable end-to-end through asynchronous boundaries.

---

## Step B13

### Issue #59 — `TODO-VALIDATION-SPLIT-012`

Add independent Kubernetes deployments and autoscaling.

Create independently scalable deployments for at least:

```text
api
orchestrator
worker-xsd
worker-en16931
```

Peppol will be added later as another independent deployment.

---

## Step B14

### Issue #61 — `TODO-VALIDATION-SPLIT-014`

Define and implement the safe migration from monolithic workers to the split topology.

---

## Step B15

### Issue #62 — `TODO-VALIDATION-SPLIT-015`

Run chaos/crash/replay integration scenarios.

---

## Step B16

### Issue #63 — `TODO-VALIDATION-SPLIT-016`

Benchmark each isolated stage and establish initial scaling/resource baselines.

---

# 5. Phase C — Prove that the split did not change validation semantics

## Step C1

### Issue #44 — `TODO-REGRESSION-013` — PART 2

Run exactly the same regression manifest against the split-stage architecture.

Require semantic equivalence with the monolithic baseline:

```text
same fixture
same requested profile
same final validity
same ordered required stage outcomes
same expected standards rule IDs
```

Architecture-specific volatile values such as timing, pod names, JetStream sequences and trace IDs are excluded from semantic comparison.

Only close issue #44 after the equivalence gate passes.

If the split changes an unexpected standards outcome, fix the split. Do not automatically update the accepted baseline to match the new behaviour.

---

# 6. Phase D — Build the Peppol standards engine and sample corpus

Peppol production integration must use the split asynchronous architecture. Do not add Peppol as a synchronous call inside the old/current combined validation worker.

---

## Step D1

### Issue #66 — `TODO-PEPPOL-001`

Freeze Peppol validation scope and contract.

---

## Step D2

### Issue #67 — `TODO-PEPPOL-002`

Pin the official Peppol release and immutable provenance.

---

## Step D3

### Issue #37 — `TODO-REGRESSION-006`

Add the Peppol semantic sample model and positive generator.

This follows Peppol scope/release selection so generated identifiers, code lists and profiles target the actual supported release.

---

## Step D4

### Issue #68 — `TODO-PEPPOL-003`

Package the official Peppol Schematron, code lists and reference/example assets.

---

## Step D5

### Issue #69 — `TODO-PEPPOL-004`

Implement deterministic Schematron-to-runtime-XSLT handling.

---

## Step D6

### Issue #70 — `TODO-PEPPOL-005`

Extend the layered validation result model with an explicit Peppol layer.

---

## Step D7

### Issue #71 — `TODO-PEPPOL-006`

Implement Peppol profile applicability and identifier checks.

---

## Step D8

### Issue #72 — `TODO-PEPPOL-007`

Implement the standalone Peppol validator adapter.

At this stage the rule engine can be tested independently of final production orchestration.

---

## Step D9

### Issue #38 — `TODO-REGRESSION-007`

Finalize the Peppol targeted-negative mutation catalogue.

Do this after the real Peppol validator exists so expected rule IDs are verified against the pinned official rule set rather than guessed.

Every Peppol-negative fixture must satisfy:

```text
UBL 2.1 XSD          PASS
EN 16931             PASS
Peppol BIS Billing   FAIL expected Peppol rule
```

---

# 7. Phase E — Integrate Peppol into the asynchronous validation platform

## Step E1

### Issue #64 — `TODO-VALIDATION-SPLIT-017`

Integrate Peppol as a native asynchronous validation-stage role.

---

## Step E2

### Issue #73 — `TODO-PEPPOL-008`

Complete Peppol third-stage integration through the ValidationPlan/orchestrator model.

Required flow:

```text
XSD PASS
   |
   v
queued EN request
   |
   v
EN PASS
   |
   v
queued Peppol request
   |
   v
Peppol StageResult
   |
   v
orchestrator finalization
```

---

## Step E3

### Issue #74 — `TODO-PEPPOL-009`

Integrate Peppol durable replay, operational failure and migration semantics.

---

## Step E4

### Issue #75 — `TODO-PEPPOL-010`

Extend HTTP/OpenAPI with an explicit validation-profile contract and Peppol stage results.

---

# 8. Phase F — Establish Peppol conformity evidence

## Step F1

### Issue #76 — `TODO-PEPPOL-011`

Complete the positive Peppol conformity corpus using the common regression generator.

---

## Step F2

### Issue #77 — `TODO-PEPPOL-012`

Complete the targeted negative Peppol conformity corpus.

---

## Step F3

### Issue #78 — `TODO-PEPPOL-013`

Cross-check e-ubl21 results against official/reference validation behaviour.

The objective is to avoid validating only the internal consistency of our own generator and our own validator.

---

## Step F4

### Issue #45 — `TODO-REGRESSION-014`

Freeze the first accepted normalized Peppol release baseline.

Do not declare Peppol conformity complete before this baseline is accepted.

---

# 9. Phase G — Production hardening and final gates

## Step G1

### Issue #79 — `TODO-PEPPOL-014`

Harden, instrument and benchmark the independent Peppol stage.

---

## Step G2

### Issue #80 — `TODO-PEPPOL-015`

Complete production packaging and the Peppol upgrade policy.

---

## Step G3

### Issue #43 — `TODO-REGRESSION-012`

Add the GitHub Minikube/Kubernetes topology regression gate.

The same semantic regression logic used locally must run in GitHub.

---

## Step G4

### Issue #65 — `TODO-VALIDATION-SPLIT-018`

Run the final distributed validation gate, including scale-to-zero and durable asynchronous-stage behaviour.

---

## Step G5

### Issue #81 — `TODO-PEPPOL-016`

Run the final public end-to-end Peppol compliance gate.

This is the mandatory Peppol payload-validation completion milestone.

A Peppol-requested Invoice/CreditNote may be reported `valid` only after:

```text
UBL 2.1 XSD          PASS
EN 16931             PASS
Peppol BIS Billing   PASS
```

with durable evidence for every required stage.

---

# 10. Phase H — Long-running governance and optional expansion

## Step H1

### Issue #46 — `TODO-REGRESSION-015`

Add nightly/release stress and performance evidence.

Keep deterministic semantic correctness separate from performance trend analysis.

---

## Step H2

### Issue #47 — `TODO-REGRESSION-016`

Finalize the standards-upgrade workflow around corpus/result changes.

All later UBL, EN 16931 and Peppol upgrades must produce reviewed before/after regression evidence.

---

## Step H3

### Issue #82 — `TODO-PEPPOL-017`

Add optional Peppol envelope/SBDH conformity validation only after Peppol Billing payload validation is complete.

Envelope validation is a separate concern and should become another asynchronous validation stage rather than being folded into the Peppol payload worker.

---

# 11. Condensed master sequence

The mandatory master order is:

```text
REGRESSION FOUNDATION

#32
 -> #33
 -> #34
 -> #35
 -> #36
 -> #39
 -> #40
 -> #41
 -> #42
 -> #44 PART 1: freeze monolithic baseline


VALIDATION SPLIT

#48
 -> #49
 -> #50
 -> #51
 -> #52
 -> #53
 -> #54
 -> #55
 -> #56
 -> #57
 -> #58
 -> #60
 -> #59
 -> #61
 -> #62
 -> #63
 -> #44 PART 2: prove equivalence and close #44


PEPPOL ENGINE + GENERATOR

#66
 -> #67
 -> #37
 -> #68
 -> #69
 -> #70
 -> #71
 -> #72
 -> #38


PEPPOL ASYNCHRONOUS INTEGRATION

#64
 -> #73
 -> #74
 -> #75


PEPPOL REGRESSION EVIDENCE

#76
 -> #77
 -> #78
 -> #45


PRODUCTION / FINAL GATES

#79
 -> #80
 -> #43
 -> #65
 -> #81


LATER / GOVERNANCE

#46
 -> #47
 -> #82
```

---

# 12. Issue completion rule

An issue is not complete merely because its code exists.

When implementation appears complete, leave the issue open until the user explicitly requests closure and then completes the confirmation barrier in section 2.3.

Before an issue may be closed:

1. its implementation is committed on its dedicated branch;
2. the user has requested issue closure;
3. the agent has inspected the issue branch/current `main` and asked for explicit confirmation to perform the final rebase;
4. the user has explicitly confirmed that final rebase;
5. the branch has then been rebased onto the latest current `origin/main`;
6. all acceptance criteria from the GitHub Issue and relevant `TODO-*.md` specification are satisfied;
7. required tests pass after the final rebase;
8. regression evidence is produced and matches the accepted expectations when applicable;
9. mandatory GitHub Actions checks pass when they exist;
10. the change is merged to `main`;
11. the merge is verified on `main`;
12. only then is the GitHub Issue closed as `completed`;
13. the merged issue branch is deleted.

The coding agent must never interpret phrases such as `implementation is done`, `looks good`, or `tests pass` as authorization for final rebase/merge/closure. The closure request and the subsequent explicit rebase confirmation are separate control points.

If implementation discovers a new independent body of work, create a new GitHub Issue instead of silently expanding the active issue indefinitely.

---

# 13. Source documents

Detailed architecture and acceptance criteria remain in:

```text
TODO-REGRESSION-CORPUS.md
TODO-VALIDATION-SPLIT.md
TODO-PEPPOL.md
TODO-EN-16931.md       # completed historical EN 16931 roadmap
TODO.md                 # completed historical repository TODOs
```

This document defines **execution order and workflow**. The GitHub Issues define **work status**. The roadmap documents define **technical scope and rationale**.
