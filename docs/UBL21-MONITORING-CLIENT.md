# UBL21 monitoring client

Status: proposed design, documentation only. Updated: 2026-10-07.

## Objective

Build a dedicated, open-source, customizable UI that shows what UBL21 is doing:
active components and containers, pod health, logs, processing metrics, and
when and why workloads scale. HPA and KEDA are already represented in the
repository architecture. This document does not claim the client is implemented
or that a running cluster has been inspected.

## Recommended foundation

Use a **Headlamp plugin** as the initial client, reusing Kubernetes resource
inspection and adding UBL21-specific topology, metrics and scaling views.
Use Prometheus for metric history and a durable event store for scaling evidence.
Use the existing log and tracing backends where available; choose missing
backends during implementation rather than introducing duplicate telemetry stacks.

Headlamp provides the extensible Kubernetes UI; the application-level map,
historical scaling explanations and business panels require custom development.
Coroot remains an optional complementary tool for observed service traffic and
performance investigation.

The earlier [Kubernetes UI evaluation](kubernetes-UI.md) considers Meshery for
visual infrastructure design and administration. This proposal addresses the
more focused monitoring-client requirement. It does not replace that evaluation
or authorize visual deployment changes.

## Scope of the first version

The client is read-only. It must provide:

- An interactive map of UBL21 workloads, services, pods and containers.
- Component health, desired/current/ready replicas, rollout status and restarts.
- Container logs with time, container and severity filters where supported.
- CPU, memory, throttling, processing throughput, latency and queue indicators.
- A scaling timeline correlating autoscaler inputs with requested and ready capacity.
- Explicit stale, unavailable and unauthorized states rather than misleading zeroes.

Selecting a logical component opens its workload, pods, metrics, logs and
scaling history. Namespace, environment and time-range filters must remain
consistent across panels.

## Component map

Discover deployed components through Kubernetes labels, selectors and ownership
references. Map logical UBL21 components to the actual deployment resources:
API, orchestrator, reconciler, XSD worker, EN 16931 worker, Peppol worker,
NATS/JetStream, database and object storage where deployed.

Keep fixed-replica components distinct from autoscaled workloads. Discover
names and mappings from the manifests and live resources; do not hard-code
the documentation's list as proof that every component is running.

Distinguish Kubernetes ownership/selection relationships, configured pipeline
dependencies, and observed traffic. A configured dependency must not appear
as measured traffic without telemetry evidence.

## Data sources and management-plane architecture

| Source | Client use |
| --- | --- |
| Kubernetes API | Workloads, pods, readiness, restarts, events, HPA spec/status and conditions |
| KEDA resources and telemetry | ScaledObject configuration/status, trigger activity, errors and associated generated HPA |
| Prometheus | Historical resource metrics, application metrics, autoscaler metric observations and replica series |
| Existing log backend / pod logs | Live debugging and retained log history |
| Existing tracing backend | Optional request-level investigation |
| Durable scaling recorder | Timestamped resource transitions, configuration snapshots and collected decision evidence |

The UI runs outside the validation request path. Its failure must not affect
validation, queue consumption or autoscaling.

Use Headlamp's authenticated Kubernetes access for supported resource reads.
If a backend adapter is needed for metrics, retained events or multiple data
sources, expose authenticated, bounded read APIs. Do not distribute privileged
backend credentials to browser code.

## Existing autoscaling baseline

The following values were read from
[autoscaling.yaml](../deploy/kubernetes/autoscaling.yaml) on the development
branch when this document was created. The UI must display actual live values,
including overrides and generated HPA policies.

| Workload | Controller configuration | Replica range | Metric targets |
| --- | --- | --- | --- |
| API | HPA | 2–10 | CPU utilization 70%; memory utilization 80% |
| Orchestrator | KEDA | 0–10 | XSD result lag 16; EN and Peppol result lag 8; CPU utilization 70% |
| XSD worker | KEDA | 0–20 | Request consumer lag 8; CPU utilization 70% |
| EN 16931 worker | KEDA | 0–30 | Request consumer lag 4; CPU utilization 70% |
| Peppol worker | KEDA | 0–30 | Request consumer lag 4; CPU utilization 70% |

KEDA polling is configured at 10 seconds. Cooldown is 120 seconds for
orchestrator/XSD and 180 seconds for EN/Peppol. API scale-down stabilization
is configured at 300 seconds.

KEDA manages an associated HPA for ordinary replica scaling; do not present
them as independent controllers competing for the same workload. Show
activation/scale-to-zero separately from HPA scaling while active.

## Scaling timeline and explanations

For each workload, place the following on one synchronized timeline:

- Actual input values and configured targets for every scaling metric.
- Desired, current and ready replicas.
- HPA conditions and rescale events.
- KEDA activation, trigger errors, fallback state where configured, and cooldown.
- Replica bounds, stabilization and scaling-rate policies.
- Pod scheduling, startup and readiness transitions.

Explain requested capacity separately from delivered capacity. For example:
"Desired replicas changed from 2 to 5; only 3 pods are ready; 2 are pending."

Scaling explanations must distinguish **controller-reported evidence** from
**inferred correlation**. Several metrics may be involved; missing metrics,
tolerance, stabilization, rate limits and replica bounds can alter the outcome.
Do not claim that a nearby metric spike definitively caused a scale event.

An illustrative backlog explanation is not a replica-calculation contract:
the metric's type and scaler semantics must be checked before deriving a
per-metric recommendation. Show all inputs and identify uncertainty when the
controller does not expose a complete decision.

## Durable scaling evidence

Kubernetes events and status snapshots are not a durable, complete decision
audit. Add a management-plane recorder that watches relevant HPA, ScaledObject,
workload and pod changes, and retains:

- Cluster, namespace, workload UID and controller identity.
- Observation time and source timestamp where available.
- Previous/new desired replicas and observed/ready capacity.
- Controller reason/message, conditions and related Kubernetes events.
- Metric values with their observation timestamps and configured target types.
- Configuration generation/resource version and relevant policies.
- Attribution level: controller-reported, inferred, or unknown.
- Collection gaps, watch reconnects and unavailable telemetry.

Deduplicate events, resume watches safely, and reconcile after disconnects.
Do not claim an exact metric sample at decision time when only a later scrape
was observed. Capture exposed controller logs or audit evidence if a stronger
audit requirement emerges; even then document completeness limits.
Choose retention and storage with the existing observability architecture.

## UBL21 metrics and diagnostic panels

Reuse [the per-stage telemetry contract](STAGE-TELEMETRY.md):

| Panel | Existing metric |
| --- | --- |
| Throughput and outcomes | rate of `e_ubl21_validation_stage_requests_total` |
| Service latency | `e_ubl21_validation_stage_duration_seconds` histogram |
| Queue wait / request age | `e_ubl21_validation_stage_queue_wait_seconds`, `e_ubl21_validation_stage_request_age_seconds` histograms |
| In-flight work | `e_ubl21_validation_stage_inflight` |
| Stream backlog / DLQ | `e_ubl21_validation_stage_queue_depth`, `e_ubl21_validation_stage_dlq_depth` |
| Retries / redeliveries / operational errors | Existing per-stage counters |
| Broker publication latency | Existing stage and orchestrator publication histograms |

The application queue-depth gauge counts retained stream messages, including
unacknowledged/in-flight work. It is **not equivalent to consumer pending lag**
used by the KEDA trigger. Label both explicitly and collect the actual scaler
input for scaling explanations.

Preserve bounded metric labels. Document, upload, validation, rule and tenant
identifiers belong in authorized logs/traces, not Prometheus labels.
Business-invalid documents must be distinguished from operational failures.

See [stage topology](VALIDATION-STAGE-TOPOLOGY.md),
[capacity baseline](STAGE-CAPACITY.md) and
[distributed tracing](DISTRIBUTED-TRACING.md) for existing contracts.

## Access and operational constraints

Use least-privilege get/list/watch and explicitly authorized pod-log access.
Do not grant deployment mutation, exec, secret-value access or scaling controls
for the first version. Authenticate users and preserve cluster/namespace
authorization across Kubernetes, metrics, logs and historical evidence.

Protect invoice and customer data. Bound queries, log streams and watch
connections. Display timestamps and telemetry freshness. Verify pinned upstream
versions, plugin APIs and licenses before packaging a distributable client.

## Implementation sequence

1. Inventory live labels, resources, telemetry endpoints, generated HPAs and RBAC.
2. Prototype a Headlamp plugin with the UBL21 component map and pod inspection.
3. Integrate Prometheus and existing log access using actual metric contracts.
4. Add HPA/KEDA status and synchronized metric/replica history.
5. Implement durable scaling evidence and qualified explanations.
6. Validate under controlled traffic and pod failures, then package deployment guidance.

## Acceptance criteria

- The map follows real workload/pod changes and supports component drill-down.
- An authorized operator can inspect pod health, logs and stage metrics.
- A controlled scale-up shows relevant inputs, targets and replica transitions.
- Multi-metric scaling, replica limits and stabilization are represented honestly.
- Pending/unready pods explain the gap between requested and available capacity.
- KEDA activation and scale-to-zero are distinguishable from active HPA scaling.
- Historical scaling evidence survives UI restarts within configured retention.
- Telemetry gaps and unsupported attribution are visible.
- Read-only permissions prevent workload changes and secret access.
- Disabling the entire monitoring client leaves UBL21 validation operational.

## Upstream references

- [Headlamp](https://headlamp.dev/docs/)
- [Headlamp plugin development](https://headlamp.dev/docs/latest/development/plugins/)
- [Kubernetes HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [KEDA documentation](https://keda.sh/docs/)
- [Prometheus](https://prometheus.io/docs/introduction/overview/)
- [Coroot](https://docs.coroot.com/)

These references guide implementation; version-specific behavior must be
verified against the versions selected for UBL21.
