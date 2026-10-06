# Codex Development Agent Contract

## Scope

A GitHub issue is the authoritative specification for the current development task.

Work on exactly one issue at a time.

## Required workflow

1. Read the complete issue title and body.
2. Read relevant existing documentation and source code before editing.
3. Identify the explicit and implicit acceptance criteria.
4. Inspect existing tests covering the affected behavior.
5. Implement the complete issue.
6. Add or update tests where required.
7. Run focused tests during development.
8. Leave the working tree in a coherent, reviewable state.

## Completion criteria

An issue is not complete merely because:

- the code compiles;
- one test passes;
- a partial implementation exists;
- a commit could be created;
- the happy path works.

Treat implementation as complete only when:

- all issue requirements are addressed;
- relevant tests pass;
- error paths are handled;
- existing behavior is not unintentionally regressed;
- documentation is updated where the issue changes documented behavior.

The outer orchestration layer runs the complete repository regression suite.

## Git ownership

The orchestration layer owns Git and GitHub state.

Do not:

- commit;
- push;
- merge;
- rebase;
- create or update pull requests;
- close issues;
- change issue labels.

You may inspect Git history and diffs.

## Test failures

When given a failing test log:

1. inspect the failure;
2. identify the root cause;
3. distinguish implementation defects from environment failures;
4. fix implementation defects;
5. run the narrowest relevant tests;
6. continue until the defect is resolved.

Never disable, skip, weaken, delete, or rewrite a valid test merely to make validation green.

If the failure is genuinely environmental and cannot be corrected from the repository,
state that clearly in your final output for the orchestration layer.

## Change discipline

- Avoid unrelated refactors.
- Preserve public APIs unless the issue explicitly changes them.
- Follow existing project patterns before introducing new abstractions.
- Prefer small, reviewable changes.
- Do not introduce generated artifacts or large fixture sets unless the issue explicitly requires them.
- Do not silently relax validation, security, durability, or correctness guarantees.

## e-UBL21 project rule

The repository root `run-all-tests.sh` is the authoritative full local regression gate.
Codex may run narrower tests during implementation, but must not modify that script
merely to make an issue pass unless the issue itself explicitly requires such a change.
