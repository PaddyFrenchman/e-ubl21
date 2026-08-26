# EN 16931 worker capacity

This document records a reproducible baseline for the real production
validation path. It is capacity evidence, not a universal service-level
objective: rerun the benchmark on the intended Kubernetes node type whenever
the Java runtime, Saxon, stylesheet, XML limits, or node resources change.

## Reproduce the measurements

Run:

```sh
BENCHTIME=4x make benchmark-en16931
```

The target verifies the pinned Saxon 12.7, XML Resolver 6.0.17, restricted
Saxon configuration, and EN 16931 1.3.16 stylesheet before benchmarking. It
also prints the OS, CPU count, memory, Go, Java, and libxml versions. A fixed
iteration count is intentional: duration-based benchmark calibration can give
different concurrency cases different document counts.

For a separate aggregate-memory diagnostic, run:

```sh
MEASURE_TREE_RSS=1 BENCHTIME=4x make benchmark-en16931
```

Process-tree sampling invokes `ps` repeatedly and can perturb latency, so do
not use that run for latency comparisons. `capacity_max_single_process_rss_kib`
comes from GNU `time`; `capacity_peak_process_tree_rss_kib` is the sampled sum
for the benchmark command, embedded NATS server, worker, and concurrent Java
children. Neither number is a substitute for pod working-set measurements on
the deployment platform.

## Baseline recorded 2026-08-26

Environment: Linux x86-64, Intel i7-11800H, 16 logical CPUs, 15.3 GiB RAM,
Go 1.26.2, OpenJDK 21.0.11, Saxon-HE 12.7, XML Resolver 6.0.17, and libxml
2.9.14. Results below use four documents per case and include Object Store,
JetStream, KV, both validation stages where applicable, permanent result
publication, clearing `ResultPending`, and message acknowledgement.

| Workload | Workers / prefetch | Wall ms/document | Documents/second | Go allocated bytes/op |
|---|---:|---:|---:|---:|
| XSD-only Order, 112 KiB | 1 / 1 | 68 | 14.78 | 1.41 MiB |
| EN-valid Invoice, small | 1 / 1 | 1,441 | 0.694 | 2.07 MiB |
| EN-invalid Invoice, small | 1 / 1 | 1,479 | 0.676 | 2.36 MiB |
| EN-valid Invoice, 1 MiB comment padding | 1 / 1 | 1,521 | 0.658 | 11.77 MiB |
| EN-valid Invoice, 9 MiB comment padding | 1 / 1 | 1,699 | 0.589 | 82.49 MiB |
| EN-valid Invoice, 100 real lines | 1 / 1 | 1,479 | 0.676 | 4.34 MiB |
| EN-valid Invoice, 1,000 real lines | 1 / 1 | 2,077 | 0.481 | 22.22 MiB |
| EN-valid Invoice, 2,950 real lines and 9.94 MiB | 1 / 1 | 2,844 | 0.352 | 135.54 MiB |
| EN-valid Invoice, small | 1 / 4 | 1,406 | 0.711 | 2.11 MiB |
| EN-valid Invoice, small | 2 / 1 | 964 | 1.037 | 1.76 MiB |
| EN-valid Invoice, small | 2 / 2 | 978 | 1.023 | 1.66 MiB |
| EN-valid Invoice, small | 4 / 4 | 1,159 | 0.863 | 1.71 MiB |

`B/op` is total Go allocation traffic, not peak live heap. The opt-in
`BenchmarkWorkerDeepBacklogMemory` remains available for descriptor-backlog
heap testing. A separate four-document process-tree sampling run of the
complete corrected matrix, including four simultaneous Java children and the
embedded NATS server, observed approximately 937 MiB peak aggregate RSS and
384 MiB maximum for one process.

The two comment-padded cases are deliberately byte-volume tests: they measure
Object Store transfer, XML input volume, and copying, but not proportional
Schematron node traversal. The 100-line, 1,000-line, and 2,950-line cases use
real `InvoiceLine`, `Item`, `Price`, VAT category, and reconciled monetary
nodes. The near-limit case distributes its remaining bytes across legitimate
item names rather than comments, so it combines high semantic cardinality with
9.94 MiB input volume.

The 2,950-line case is the highest tested passing point under the production
4 MiB SVRL cap. Equivalent 2,990-, 3,000-, and 4,000-line generated invoices
crossed that cap and correctly became operational failures. This is a measured
resource boundary, not evidence that every structurally valid document below
the HTTP byte limit can complete semantic validation. Reassess the SVRL bound
and memory budget together if product requirements demand higher cardinality.

The isolated process benchmark measured:

| Operation | Time/op | Go allocations/op |
|---|---:|---:|
| OpenJDK `java -version` launch baseline | 26 ms | 44 |
| Saxon launch + stylesheet compile + transform + SVRL parse | 1,452 ms | 828 |

Run-to-run variation is expected on shared hardware. The conclusions depend
on the order of magnitude and scaling shape, not a single precise value.

## Decisions and capacity assumptions

- Keep one Saxon JVM per document for now. Bare JVM launch was under 2% of the
  measured complete transform. The current model also gives simple hard
  cancellation and per-document isolation. Stylesheet compilation and
  transformation dominate; evaluate a persistent JVM only if production
  profiling shows the required throughput cannot be reached by replicas, and
  retain the existing `EN16931Validator` interface and result semantics.
- Keep the defaults at `worker-concurrency=1` and `worker-prefetch=1`.
  Increasing prefetch alone produced no throughput gain. Two concurrent Saxon
  processes improved throughput on this host, but four added no reliable gain
  and raised simultaneous memory demand. Prefer horizontal replicas before
  increasing per-pod concurrency.
- Keep the 30-second overall/stage budgets and 60-second `AckWait`. The measured
  near-limit document completed far inside those bounds, while progress
  heartbeats protect queued work. Treat sustained approach to a timeout as a
  capacity incident, not a reason to increase `AckWait` without investigation.
- For initial Kubernetes testing with concurrency 1, start at a 1 vCPU / 512
  MiB request and a 2 vCPU / 1 GiB limit. For concurrency 2, start at 2 vCPU /
  768 MiB requested and a 4 vCPU / 1.5 GiB limit. These are test starting
  points, not guarantees; validate pod working set, throttling, and near-limit
  documents on the actual node/runtime before production rollout.
- Keep `MaxAckPending` at least the fleet-wide sum of `concurrency + prefetch`.
  Autoscale primarily on durable queue depth and EN stage duration. With the
  conservative single-worker baseline of roughly 0.69 invoices/second, plan no
  more than about 0.45 invoices/second (27/minute) per replica at 65% target
  utilization, then calculate replicas as
  `ceil(peak_invoice_rate / 0.45)`. Recalculate from deployment measurements.

EN-invalid invoices cost essentially the same as compliant invoices, so
capacity planning must not assume early semantic rejection. XSD-only document
types are much cheaper and may be isolated with subject-filtered worker pools
if billing traffic requires independent scaling.
