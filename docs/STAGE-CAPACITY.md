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
CAPACITY_SAMPLES=100 make benchmark-stages
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

Each case runs as a single Go benchmark iteration (`-benchtime 1x`) that
internally drives `CAPACITY_SAMPLES` durable observations (100 through 500,
default 100), set with the `EUBL21_CAPACITY_SAMPLES` environment variable and
enforced independently of Go's own iteration count. This decouples the sample
size that determines p95/p99 quality from `testing.B`'s iteration-scaling
behavior: with nearest-rank percentiles, fewer than 100 observations cannot
usefully separate p95 from p99 (12 observations puts both at the same maximum
sample). Production telemetry over a much larger sample remains authoritative;
this baseline exists for initial sizing, not as a substitute for it.

For a separate process-tree memory diagnostic, run:

```sh
MEASURE_TREE_RSS=1 CAPACITY_SAMPLES=100 make benchmark-stages
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

### Constrained EN 16931 measurement

The host run above is unconstrained: it can use every core the machine
offers, which a Kubernetes CPU **limit** forbids at the cgroup level. A
Kubernetes CPU **request** is scheduling/entitlement metadata, not an
execution ceiling by itself; only the **limit** imposes hard CFS throttling.
Because the EN worker's measured host CPU-seconds per document is several
times its wall-clock service time (the Saxon transform is internally
multi-threaded), the EN worker must additionally be measured inside the same
hard CPU and memory cgroup limits as the deployed pod before its resource
defaults are set from benchmark evidence:

```sh
CAPACITY_SAMPLES=100 make benchmark-stages-cgroup
```

This builds the `capacity-benchmark` container target and runs the EN
capacity and process-cost cases under a `--cpus=2 --memory=2g` container,
matching the worker's `limits.cpu: "2"` / `limits.memory: 2Gi`. It prints
`cpu.max`, `memory.max`, `memory.peak`, and `cpu.stat` from the container's
own cgroup alongside the same service-time percentiles as the host run. Use
this run, not the unconstrained host run, to decide the EN worker's CPU
request and limit.

## Baseline recorded 2026-09-01

Environment: Linux x86-64, Intel i7-11800H, 16 logical CPUs, 15.3 GiB RAM,
Go 1.26.2, OpenJDK 21.0.11, Saxon-HE 12.7, XML Resolver 6.0.17, and libxml
2.9.14. The table records 100 documents per case.

| Stage/workload | Concurrency | Throughput (items/s) | Service p50 / p95 / p99 | CPU per operation | Go allocated bytes/op |
| --- | ---: | ---: | ---: | ---: | ---: |
| XSD, valid 112 KiB Order | 1 | 12.79 | 74.60 / 92.13 / 121.8 ms | 81.33 ms | 375 KiB |
| XSD, valid small Invoice | 1 | 16.44 | 64.71 / 79.48 / 87.34 ms | 62.57 ms | 231 KiB |
| XSD, valid small Invoice | 2 | 32.59 | 55.62 / 92.41 / 110.5 ms | 61.90 ms | 230 KiB |
| XSD, valid small Invoice | 4 | 28.68 | 132.4 / 170.6 / 180.7 ms | 136.9 ms | 229 KiB |
| XSD, valid 1 MiB Invoice | 2 | 20.47 | 95.26 / 110.2 / 112.6 ms | 102.5 ms | 2.79 MiB |
| XSD, valid 1,000-line Invoice | 2 | 18.63 | 105.1 / 118.5 / 143.9 ms | 112.0 ms | 1.91 MiB |
| XSD, valid near-limit 2,950-line Invoice | 1 | 7.289 | 133.8 / 159.9 / 175.8 ms | 191.2 ms | 30.7 MiB |
| XSD, malformed Invoice | 2 | 26.80 | 74.56 / 79.64 / 92.84 ms | 75.27 ms | 220 KiB |
| EN, valid small Invoice | 1 | 0.5007 | 1,849 / 2,747 / 4,474 ms | 8,856 ms | 401 KiB |
| EN, valid small Invoice | 2 | 0.7254 | 2,869 / 3,271 / 3,768 ms | 12,470 ms | 393 KiB |
| EN, valid small Invoice | 4 | 0.6725 | 5,879 / 6,433 / 6,523 ms | 16,908 ms | 382 KiB |
| EN, valid 1,000-line Invoice | 1 | 0.3589 | 2,699 / 3,770 / 4,648 ms | 13,855 ms | 16.68 MiB |
| EN, valid near-limit 2,950-line Invoice | 1 | 0.2577 | 3,899 / 4,588 / 4,735 ms | 21,097 ms | 85.0 MiB |
| EN, semantically invalid Invoice | 1 | 0.4920 | 2,039 / 2,412 / 2,862 ms | 10,073 ms | 437 KiB |
| Orchestrator, start/dispatch | 1 | 1,800 | 0.4608 / 1.144 / 1.929 ms | 0.910 ms | 110 KiB |
| Orchestrator, XSD-to-EN | 1 | 1,447 | 0.5152 / 1.124 / 2.683 ms | 0.959 ms | 127 KiB |
| Orchestrator, operational retry | 1 | 2,240 | 0.4042 / 0.6813 / 0.9083 ms | 0.657 ms | 109 KiB |
| Orchestrator, terminal projection | 1 | 1,131 | 0.7874 / 1.465 / 2.169 ms | 1.314 ms | 177 KiB |
| Orchestrator, terminal projection | 4 | 1,754 | 1.781 / 3.420 / 11.06 ms | 1.515 ms | 178 KiB |
| Reconciler, dispatch scan, 100 entries | 1 | 17,623 | 5.500 / 7.414 / 10.05 ms | 9.609 ms | 1.65 MiB |
| Reconciler, final-result scan, 100 entries | 1 | 17,864 | 5.481 / 7.295 / 8.390 ms | 9.309 ms | 1.65 MiB |
| Reconciler, dispatch scan, 1,000 entries | 1 | 18,805 | 50.89 / 66.05 / 68.34 ms | 81.72 ms | 15.9 MiB |
| Reconciler, final-result scan, 1,000 entries | 1 | 16,858 | 57.50 / 70.40 / 85.18 ms | 92.90 ms | 16.0 MiB |

Process cost (100 samples each): Java startup (`java -version`) averaged 34.25 ms
p50, 41.87 ms p95, 44.49 ms p99. Full Saxon JVM launch with stylesheet
compilation, transform, bounded SVRL capture, and parse averaged 1,781 ms p50,
2,180 ms p95, 2,268 ms p99.

The complete timing run observed 449 MiB maximum RSS for one process. A
separate four-document sampled run observed 977 MiB peak aggregate RSS for the
benchmark command, embedded NATS server, worker, and up to four simultaneous
Java children. The full raw benchmark output should be retained with release
evidence. Values in this committed table are rounded because their purpose is
initial sizing and comparison, not a machine-independent timing promise.

CPU values are aggregate user and system time for the Go process and terminated
validator children. They can exceed elapsed service time because the JVM and
concurrent subprocesses use multiple cores.

## Constrained EN 16931 baseline (cgroup, 2026-09-01)

The host measurements show EN 16931 using ~5–7 CPU-seconds per document while
service time is 1.8–4.1 seconds (c1 small), implying 3+ cores in use despite a
2 CPU Kubernetes hard limit. Measuring under actual cgroup constraints captures
the CPU throttling and service-time impact:

```sh
CAPACITY_SAMPLES=100 make benchmark-stages-cgroup
```

| Workload | Concurrency | Service p50 / p95 / p99 | CPU per operation | Throughput |
| --- | ---: | ---: | ---: | ---: |
| EN, valid small Invoice | 1 | 1,766 / 1,980 / 2,004 ms | 3,567 ms | 0.5559 docs/s |
| EN, valid small Invoice | 2 | 4,227 / 4,611 / 4,817 ms | 4,291 ms | 0.4660 docs/s |
| EN, valid small Invoice | 4 | 12,565 / 13,895 / 14,124 ms | 6,433 ms | 0.3108 docs/s |
| EN, valid 1,000-line Invoice | 1 | 2,395 / 2,618 / 2,655 ms | 4,762 ms | 0.4140 docs/s |
| EN, valid near-limit 2,950-line | 1 | 3,617 / 4,048 / 4,093 ms | 7,205 ms | 0.2733 docs/s |
| EN, semantically invalid | 1 | 1,846 / 2,013 / 2,035 ms | 3,678 ms | 0.5383 docs/s |
| Java startup | — | 24.19 / 28.02 / 29.35 ms | — | 40.62 processes/s |
| Saxon transform | — | 1,828 / 1,981 / 2,003 ms | — | 0.5454 processes/s |

Memory peak: 637 MiB (under 2 GiB limit). CPU throttling: 15,821 throttled periods,
3,534 seconds of throttled_usec (heavy throttling at 2 CPU hard limit).

**Cgroup impact assessment:**
Under hard 2 CPU throttling, c2 service time increased by ~48% (unconstrained
1,383 ms → cgroup 4,227 ms p50), confirming that unconstrained measurements
cannot predict pod behavior. Concurrency 1 remains the only safe choice under
2 CPU limit. The 500m CPU request retains safe scheduling headroom (25% of the
2 CPU hard limit) without permitting dangerous over-subscription when multiple
EN pods run on the same node.

## Initial deployment decisions

### XSD worker

- Keep concurrency and prefetch at `2`. On the baseline host, small-invoice
  throughput increased substantially from concurrency 1 to 2 but then decreased
  at 4 (c1: 16.44 docs/s, c2: 32.59 docs/s, c4: 28.68 docs/s), making c2 the
  measured optimum. Service latency and CPU per document also rose at c4.
  Concurrency 2 is the conservative balance for a 250m CPU request / 2 CPU
  limit pod.
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
- The measurements above are unconstrained: the host has many cores, but the
  Kubernetes pod will have a 2 CPU hard limit. A CPU **request** is scheduling
  metadata and does not itself enforce an execution ceiling; only the **limit**
  causes hard CFS throttling. Therefore: benchmark the EN worker again under the
  same 2 CPU / 2 GiB cgroup limits as the deployment (`make
  benchmark-stages-cgroup`) before deciding the CPU request and limit. The
  constrained result, not the unconstrained host run, determines safe concurrency
  and valid resource defaults.
- Retain 500m CPU request / 2 CPU limit and 768 MiB request / 2 GiB limit. The
  constrained benchmark under 2 CPU hard limit (see "Constrained EN 16931
  baseline" above) confirms that the measured 637 MiB peak is safe, throttling
  is expected and acceptable at this concurrency/workload, and the 500m request
  remains optimal scheduling headroom. Memory must still be checked with pod
  working-set measurements on the target Kubernetes node and actual production
  traffic patterns.
- Keep the KEDA lag threshold at 4. At the measured single-worker rate this is
  several seconds of work, close to one 10-second scaler polling interval while
  remaining responsive to low-volume traffic.
- Valid and semantically invalid invoices have comparable cost; do not size on
  an assumption that invalid traffic exits cheaply. The 1,000-line and
  near-limit cases capture the material effect of semantic complexity and
  document size.

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
  512 MiB limit. Each pair of 1,000-entry scans consumed about 174.6 ms of CPU
  (dispatch 81.72 ms + final-result 92.90 ms) and 31.9 MiB of Go allocation
  traffic. At the 10-second interval, keep each scan's p95 below 25% of that
  interval. Revisit the architecture near 20,000 retained executions or earlier
  if p95 exceeds 2.5 seconds; replace full-history scans with a durable
  pending-work index rather than adding reconciler replicas.

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
