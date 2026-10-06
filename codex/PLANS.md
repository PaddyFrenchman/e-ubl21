# Codex Execution Plans

For a large issue, Codex should create an internal execution plan before editing.

The plan should contain:

1. **Issue summary** — the observable behavior requested.
2. **Affected areas** — packages, services, schemas, deployment files, docs, tests.
3. **Current behavior** — what the repository does today.
4. **Target behavior** — what must be true when the issue is complete.
5. **Implementation steps** — small ordered changes.
6. **Validation** — focused tests plus the expected regression impact.
7. **Risks** — compatibility, persistence, concurrency, migration, API, or deployment risks.

The plan is a working aid, not a substitute for implementation.

For small issues, do not manufacture unnecessary planning ceremony. Inspect, implement,
test, and report.

For large or ambiguous issues, prefer resolving ambiguity from repository evidence and
the GitHub issue itself. Do not broaden the scope without evidence.
