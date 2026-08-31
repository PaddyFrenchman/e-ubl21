# Split-stage capacity baseline

This document records the initial capacity evidence for the independent XSD,
EN 16931, and orchestrator deployments introduced by the validation split. It
is a sizing baseline, not a service-level objective. Repeat the measurements
on the intended Kubernetes node type whenever the Go or Java runtime, pinned
validation assets, XML limits, persistence topology, or pod resources change.

Peppol is not implemented yet. Its worker must be added to this same benchmark
and this decision record before the Peppol deployment receives production
resource or scaling defaults.

## Reproduce the measurements

Run the fixed-count matrix:

```sh
BENCHTIME=12x make benchmark-stages
```

The runner verifies the pinned Saxon and EN 16931 assets and prints the host,
CPU count, memory, Go, Java, and libxml versions. It then measures the real
split-worker boundaries:

- XSD: durable request, authorization, Object Store read, `xmllint`, durable
  result publication, and acknowledgement;
- EN 16931: the same durable boundary with one Saxon JVM per document and
  prerequisite XSD evidence already present;
- orchestrator: durable start/dispatch, XSD-to-EN advance, operational retry,
  and terminal projection transitions using production JetStream KV, evidence,
  outbox, and compare-and-swap paths;
- reconciler: steady-state dispatch and final-result full-history scans at 100
  and 1,000 validation executions;
- Java/Saxon process cost: launch-only Java startup and full stylesheet
  compilation, transform, bounded SVRL capture, and parse.

The benchmark refuses more than 24 iterations per case because every iteration
creates durable state. Fixed counts make concurrency comparisons use the same
document count. Twelve observations provide a useful local p50/p95/p99 shape;
production telemetry over a much larger sample remains authoritative.

For a separate process-tree memory diagnostic, run:

```sh
MEASURE_TREE_RSS=1 BENCHTIME=12x make benchmark-stages
```

Sampling with `ps` perturbs latency, so do not use that run for timing
comparisons. Go's `B/op` is allocation traffic per operation, not live heap.
The process-tree figure includes the benchmark process, embedded NATS server,
worker, and concurrent Java children; pod working-set and throttling metrics on
the target platform remain required before production.

The queue percentiles are controlled-backlog drain observations: all requests
are made durable before the timer begins. They demonstrate the effect of
service time and concurrency on an existing backlog, not broker publication
latency under a live arrival stream.

## Baseline recorded 2026-08-31

Environment: Linux x86-64, Intel i7-11800H, 16 logical CPUs, 15.3 GiB RAM,
Go 1.26.2, OpenJDK 21.0.11, Saxon-HE 12.7, XML Resolver 6.0.17, and libxml
2.9.14. The table records twelve documents per case.

| Stage/workload | Concurrency | Throughput (items/s) | Service p50 / p95 / p99 | CPU per operation | Go allocated bytes/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| XSD, valid 112 KiB Order | 1 | 14.43 | 68 / 79 / 79 ms | 72 ms | 430 KiB |
| XSD, valid small Invoice | 1 | 14.98 | 67 / 69 / 69 ms | 69 ms | 269 KiB |
| XSD, valid small Invoice | 2 | 24.71 | 80 / 84 / 84 ms | 82 ms | 279 KiB |
| XSD, valid small Invoice | 4 | 32.32 | 124 / 126 / 126 ms | 124 ms | 288 KiB |
| XSD, valid 1 MiB Invoice | 2 | 23.25 | 85 / 94 / 94 ms | 93 ms | 3.57 MiB |
| XSD, valid 1,000-line Invoice | 2 | 19.88 | 99 / 110 / 110 ms | 106 ms | 2.63 MiB |
| XSD, malformed Invoice | 2 | 24.32 | 82 / 84 / 84 ms | 83 ms | 244 KiB |
| EN, valid small Invoice | 1 | 0.699 | 1,443 / 1,522 / 1,522 ms | 5,513 ms | 383 KiB |
| EN, valid small Invoice | 2 | 0.985 | 1,988 / 2,207 / 2,207 ms | 7,952 ms | 386 KiB |
| EN, valid small Invoice | 4 | 0.925 | 4,295 / 4,501 / 4,501 ms | 12,955 ms | 388 KiB |
| EN, valid 1,000-line Invoice | 1 | 0.471 | 2,138 / 2,392 / 2,392 ms | 9,961 ms | 19.54 MiB |
| EN, semantically invalid Invoice | 1 | 0.713 | 1,373 / 1,514 / 1,514 ms | 6,064 ms | 438 KiB |
| Orchestrator, start/dispatch | 1 | 1,671 | 0.44 / 1.06 / 1.06 ms | 1.09 ms | 139 KiB |
| Orchestrator, XSD-to-EN | 1 | 1,800 | 0.43 / 0.92 / 0.92 ms | 0.71 ms | 168 KiB |
| Orchestrator, operational retry | 1 | 2,517 | 0.31 / 1.12 / 1.12 ms | 0.50 ms | 130 KiB |
| Orchestrator, terminal projection | 1 | 1,418 | 0.65 / 1.05 / 1.05 ms | 0.88 ms | 218 KiB |
| Orchestrator, terminal projection | 4 | 2,504 | 1.43 / 1.93 / 1.93 ms | 1.02 ms | 218 KiB |
| Reconciler, dispatch scan, 1,000 entries | 1 | 22,226 | 44.8 / 48.1 / 48.1 ms | 66.8 ms | 15.26 MiB |
| Reconciler, final-result scan, 1,000 entries | 1 | 22,527 | 43.6 / 48.7 / 48.7 ms | 67.0 ms | 15.37 MiB |

The isolated process cases measured 27 ms for `java -version` startup and
1,532 ms for JVM launch, stylesheet compilation, transform, bounded SVRL
capture, and parse. Bare JVM startup is therefore under 2% of the full
operation on this host; compilation and transformation dominate.

The complete timing run observed 449 MiB maximum RSS for one process. A
separate four-document sampled run observed 977 MiB peak aggregate RSS for the
benchmark command, embedded NATS server, worker, and up to four simultaneous
Java children. The full raw benchmark output should be retained with release
evidence. Values in this committed table are rounded because their purpose is
initial sizing and comparison, not a machine-independent timing promise.

CPU values are aggregate user and system time for the Go process and terminated
validator children. They can exceed elapsed service time because the JVM and
concurrent subprocesses use multiple cores.

## Initial deployment decisions

### XSD worker

- Keep concurrency and prefetch at `2`. On the baseline host, small-invoice
  throughput increased substantially from concurrency 1 to 2 and continued to
  rise at 4, but service latency and CPU per document also rose. Concurrency 2
  is the conservative balance for a 250m CPU request / 2 CPU limit pod.
- Start with 250m CPU / 256 MiB requested and 2 CPU / 1 GiB limited. The 1 MiB
  and 1,000-line workloads explicitly cover byte volume and semantic
  cardinality; validate near-limit working set on the target node.
- Keep the KEDA lag threshold at 8. This is deliberately latency-oriented: it
  activates on the first queued request and adds roughly one replica per eight
  queued requests, well below measured multi-second drain capacity.

### EN 16931 worker

- Keep concurrency and prefetch at `1`. Two concurrent JVMs increased aggregate
  throughput by about 41%, not 100%, while increasing per-document latency and
  simultaneous CPU/memory pressure; four reduced throughput. Prefer horizontal
  replicas for isolation and predictable memory.
- Request 1 CPU / 768 MiB and limit at 2 CPU / 2 GiB. The higher CPU request is
  intentional: a single Saxon transform is multi-threaded enough that a 500m
  request would make CPU throttling likely. Memory limits must still be checked
  with pod working-set measurements and near-limit documents.
- Keep the KEDA lag threshold at 4. At the measured single-worker rate this is
  several seconds of work, close to one 10-second scaler polling interval while
  remaining responsive to low-volume traffic.
- Valid and semantically invalid invoices have comparable cost; do not size on
  an assumption that invalid traffic exits cheaply. The 1,000-line case captures
  the material effect of semantic complexity.

### Orchestrator

- Retain 100m CPU / 128 MiB requested and 1 CPU / 512 MiB limited for the
  autoscaled consumer. Even terminal projection is orders of magnitude faster
  than EN validation; durable broker/KV latency dominates the local result.
- Retain result prefetch 8 and the current per-stream KEDA lag thresholds of 16
  for XSD results and 8 for EN results. These are low-latency thresholds rather
  than throughput ceilings and leave ample measured headroom.
- Keep the reconciler fixed at one replica. The benchmark measures its current
  full-history scan cost separately; scaling those scans horizontally would
  violate their current ownership model and multiply broker/KV reads.
- Request 250m CPU / 128 MiB for the reconciler, with the existing 1 CPU /
  512 MiB limit. Each pair of 1,000-entry scans consumed about 134 ms of CPU and
  32 MiB of Go allocation traffic. At the 10-second interval, keep each scan's
  p95 below 25% of that interval. Revisit the architecture near 20,000 retained
  executions or earlier if p95 exceeds 2.5 seconds; replace full-history scans
  with a durable pending-work index rather than adding reconciler replicas.

## Capacity calculation and production checks

For arrival rate `lambda`, concurrency `c`, measured mean service time `s`, and
safety factor `f`:

```text
per-pod throughput = c / s
replicas = ceil(lambda / (c / s) * f)
```

Start with `f = 1.25`, target no more than 70% sustained utilization, and
recalculate with target-node measurements. Before production, run at least a
30-minute workload with representative valid, invalid, large, and line-heavy
documents and inspect pod working set, CPU throttling, queue p95 age, service
p95/p99, redelivery, and drain time. Queue age approaching the stage timeout is
a capacity incident; it is not a reason to weaken validation or silently raise
timeouts.
