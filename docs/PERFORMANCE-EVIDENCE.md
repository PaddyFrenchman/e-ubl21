# Release and nightly performance evidence

Performance evidence is deliberately separate from deterministic semantic
correctness. Pull requests and ordinary pushes continue to use the correctness
and conformity workflows; they do not acquire timing assertions. The
`Performance and stress evidence` workflow runs nightly, on demand, and when a
GitHub release is published.

## Evidence boundary

The run exercises the existing durable capacity matrix with at least 100
observations per case. It covers small, invalid, 1 MiB, 1,000-line, and
2,950-line near-limit documents where applicable, along with concurrency and
orchestrator history-size cases. Each stage records:

- controlled-backlog queue-wait p50/p95/p99;
- service p50/p95/p99 across authorization, Object Store read, validator
  execution, durable result publication, and acknowledgement;
- end-to-end p50/p95/p99 as queue wait plus service time for each document;
- throughput, aggregate CPU per document/transition/scan, Go allocation volume,
  input bytes, and semantic line count;
- process-tree peak RSS for the host matrix; and
- cgroup peak memory and CPU throttling for the optional production-equivalent
  2 CPU / 2 GiB constrained semantic-stage matrix.

The queue measurements intentionally describe draining an already-durable
backlog. They are not presented as live-arrival broker latency or as an SLO.

## Retention and comparison

Every run uploads the raw host log, the constrained log when selected,
machine-readable `evidence.json`, the prior evidence used for comparison when
available, and a Markdown summary for 90 days.
A successful run is cached as the baseline for the next run on the same runner
OS. The first run without compatible prior evidence seeds that history.

The summary flags throughput reductions or latency/resource increases of 25%
or more by default. It also reports material improvements. These comparisons
are advisory because hosted-runner hardware and contention vary; unexplained
changes must be investigated against the raw environment and benchmark data.
They never change a document's semantic correctness result and do not fail the
correctness workflow.

Only these conditions fail the performance workflow:

1. setup, fixture, validator, or benchmark execution fails;
2. an explicitly configured absolute resource ceiling is exceeded; or
3. the measurement required to enforce a configured ceiling is absent.

The checked-in workflow sets a 2 GiB process-tree RSS ceiling and the deployed
2 GiB cgroup memory ceiling. A missing RSS/cgroup measurement is therefore an
evidence failure rather than a silent guardrail bypass. Timing deltas alone
never fail a run.

## Local and on-demand use

Collect host evidence locally:

```sh
CAPACITY_SAMPLES=100 ARTIFACT_DIR=performance-results/local \
  make performance-evidence
```

Add the constrained matrix and compare with a prior retained result:

```sh
CAPACITY_SAMPLES=100 RUN_CGROUP=1 \
PERFORMANCE_BASELINE=performance-results/previous/evidence.json \
ARTIFACT_DIR=performance-results/current \
  make performance-evidence
```

Use the workflow-dispatch inputs to increase the sample count or omit the
constrained matrix for a quicker diagnostic. Release and scheduled runs always
run both matrices. Historical sizing decisions remain documented in
`docs/STAGE-CAPACITY.md`; workflow evidence is the repeatable evolution record.
