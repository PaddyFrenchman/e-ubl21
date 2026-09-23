# AWS Kubernetes Scalability and Observability Architecture

**Project:** UBL-21 Validator  
**Document:** `WAS-KUBERNETES-SCALIBILITY.md`  
**Purpose:** Define the target AWS/Kubernetes architecture, deployment constraints, observability model, and autoscaling strategy required to operate the UBL-21 / EN 16931 / Peppol validation platform as an enterprise-grade managed service.  
**Status:** Architecture and implementation roadmap  
**Checked:** 21 August 2026

> **Filename note:** the requested filename is intentionally preserved as `WAS-KUBERNETES-SCALIBILITY.md`. The architecture described in this document targets **AWS**.

---

# 1. Executive objective

The commercial objective is to operate the UBL validation platform as a managed service for large organizations.

The service must evolve from:

```text
web service
    |
    v
queue
    |
    v
worker
    |
    v
retrieve XML
    |
    v
UBL 2.1 validation
```

into an enterprise SaaS platform capable of:

- accepting high and bursty document volumes;
- serving multiple customers while preserving tenant isolation;
- validating UBL 2.1, EN 16931, Peppol BIS, and later national profiles;
- scaling workers independently from public API traffic;
- absorbing spikes through durable queues rather than rejecting traffic;
- scaling Kubernetes pods from meaningful business/workload signals;
- scaling EC2 capacity underneath Kubernetes automatically;
- surviving node and Availability Zone failures;
- providing measurable service-level objectives;
- exposing enough telemetry to explain every scaling decision;
- controlling cost per validated document;
- supporting enterprise security, auditing, retention, and compliance requirements;
- performing safe rolling upgrades of validation engines and standards artefacts.

The target platform should use **Amazon EKS** as the Kubernetes control plane and AWS managed services around it where they reduce operational risk.

---

# 2. Core architectural principle

The existing asynchronous queue design is the correct foundation for scalability.

Validation workloads are naturally queueable:

```text
request
   |
   v
accept quickly
   |
   v
durable queue
   |
   v
workers consume at available capacity
```

This decouples:

```text
customer request rate
```

from:

```text
validation execution rate
```

That is important because semantic validation with Java/Saxon/XSLT can be significantly more CPU- and memory-intensive than ordinary REST processing.

The platform should therefore retain two independent scaling planes:

```text
ONLINE PLANE
API / authentication / job submission / result retrieval

BACKGROUND COMPUTE PLANE
UBL / EN16931 / Peppol validation workers
```

Do not scale them together.

---

# 3. Target high-level architecture

```text
                           Internet / Enterprise clients
                                      |
                                      v
                               Route 53 DNS
                                      |
                                      v
                                  AWS WAF
                                      |
                                      v
                             ALB / API ingress
                                      |
                                      v
                +---------------- Amazon EKS ----------------+
                |                                            |
                |           API Deployment                   |
                |       +--------------------+               |
                |       | auth / validation |               |
                |       | request handlers  |               |
                |       +----------+---------+               |
                |                  |                         |
                +------------------|-------------------------+
                                   |
                 +-----------------+------------------+
                 |                                    |
                 v                                    v
            Amazon S3                         Metadata database
       immutable XML/object store             Aurora PostgreSQL
                 |                                    |
                 +-----------------+------------------+
                                   |
                                   v
                              Amazon SQS
                          validation queue(s)
                                   |
                                   v
                     +-------------+-------------+
                     |                           |
             KEDA watches queue             DLQ / retry
                     |
                     v
                EKS worker pods
                     |
           +---------+----------+
           |                    |
           v                    v
       xmllint/libxml2       Saxon runtime
       UBL 2.1 XSD           EN16931 / Peppol
           |                    |
           +---------+----------+
                     |
                     v
              normalized result
                     |
          +----------+-----------+
          |                      |
          v                      v
      Aurora metadata       S3 artefacts
      status/issues         raw SVRL/reports
          |
          v
     API result retrieval
```

Observability overlays every layer:

```text
OpenTelemetry / Prometheus metrics
             |
             v
Amazon Managed Service for Prometheus
             |
             v
Amazon Managed Grafana

logs
  |
  v
CloudWatch Logs

traces
  |
  v
OpenTelemetry -> X-Ray / selected tracing backend

AWS service metrics
  |
  +--> SQS
  +--> ALB
  +--> EKS
  +--> RDS/Aurora
  +--> S3
  +--> Karpenter
```

---

# 4. Recommended AWS services

## 4.1 Amazon EKS

Use Amazon EKS for:

- public/internal API workloads;
- validation workers;
- schedulers/controllers;
- OpenTelemetry collectors where required;
- KEDA;
- Karpenter;
- Kubernetes-level service isolation.

Do not place the durable queue or primary object store inside Kubernetes.

Kubernetes workloads should be disposable.

---

## 4.2 Amazon SQS

Use SQS as the primary validation work queue unless there is a strong requirement for a different message broker.

SQS fits this workload because:

- validation jobs are independent;
- strict global ordering is normally unnecessary;
- durable asynchronous buffering is valuable;
- queue depth is directly observable;
- KEDA supports scaling from SQS queue length;
- SQS removes broker-cluster administration.

Suggested queues:

```text
ubl-validation-standard
ubl-validation-priority
ubl-validation-dlq
```

Later, separate queues may be introduced by workload class:

```text
ubl-xsd
en16931
peppol
bulk-validation
```

Do not create one queue per customer unless there is a contractual isolation reason. That creates unnecessary operational cardinality.

---

## 4.3 Amazon S3

Store submitted XML in S3 rather than embedding large XML documents in queue messages.

Queue messages should contain references:

```json
{
  "job_id": "...",
  "tenant_id": "...",
  "object_key": "...",
  "validation_profile": "en16931",
  "profile_version": "1.3.16"
}
```

Advantages:

- queue messages remain small;
- XML retention can be controlled independently;
- object versioning can be enabled;
- lifecycle policies can archive/delete content;
- server-side encryption is straightforward;
- validation jobs can be replayed from immutable input;
- raw SVRL and result artefacts can also be stored.

Recommended object model:

```text
s3://validation-data/
  tenants/
    <tenant-id>/
      documents/
        <document-id>/
          input.xml
          results/
            ubl21.json
            en16931.json
            en16931.svrl.xml
            peppol.json
```

Do not expose bucket/object keys directly as customer authorization boundaries. Authorization belongs in the application and IAM policies.

---

## 4.4 Aurora PostgreSQL

Use Aurora PostgreSQL or RDS PostgreSQL for:

- job metadata;
- tenant configuration;
- validation status;
- normalized validation issues;
- profile/version metadata;
- audit references;
- billing/usage records;
- API idempotency keys;
- document lookup indexes.

Do not store every XML payload as a large database blob unless a customer contract requires it.

Use S3 for document bodies and PostgreSQL for relational metadata.

---

## 4.5 ElastiCache, optional

Redis/Valkey through ElastiCache may later be useful for:

- short-lived API caching;
- rate-limit counters;
- idempotency acceleration;
- profile metadata cache;
- distributed locks where genuinely required.

It should not become the authoritative queue or authoritative validation store if SQS and Aurora already fill those roles.

---

# 5. Kubernetes workload decomposition

The cluster should contain separate Deployments for distinct scaling characteristics.

Recommended:

```text
api
validation-worker-ubl
validation-worker-semantic
```

At first it may be practical to use:

```text
api
validation-worker
```

with the worker containing:

```text
xmllint
Java runtime
Saxon
UBL artefacts
EN16931 artefacts
Peppol artefacts
```

As utilization data accumulates, split workers only when there is a measurable reason.

A likely future split:

```text
xsd-worker
    CPU light/moderate
    short-lived jobs

semantic-worker
    Java/Saxon
    higher memory
    more CPU
    longer processing

bulk-worker
    low-priority
    cheaper capacity / Spot-friendly
```

---

# 6. Multi-tenancy

Large-organization customers will expect strong logical isolation.

Every job must carry a stable:

```text
tenant_id
```

The tenant ID must be propagated through:

```text
API request
queue message
object metadata/path
database rows
worker context
logs
traces
metrics where cardinality permits
billing records
audit records
```

However, **do not put tenant_id into high-volume Prometheus metric labels** if there can be hundreds or thousands of tenants.

Prometheus metric cardinality must remain bounded.

Instead:

```text
metrics:
    global/service dimensions

logs/traces:
    tenant_id allowed where policy permits

billing/usage database:
    exact per-tenant accounting
```

---

# 7. Enterprise isolation tiers

The platform can support several commercial isolation levels.

## Tier A: shared SaaS

```text
shared EKS cluster
shared worker pools
shared SQS queues
shared Aurora cluster
tenant_id row isolation
S3 tenant prefixes
```

Best cost efficiency.

## Tier B: logically isolated enterprise

```text
shared cluster
dedicated namespace
dedicated service account / IAM role
optional dedicated queue
optional dedicated worker Deployment
tenant-specific encryption key if contracted
```

Useful for large regulated customers.

## Tier C: dedicated deployment

```text
dedicated AWS account
dedicated VPC
dedicated EKS cluster
dedicated database
dedicated queues
dedicated observability workspace
```

Offer only when the commercial contract pays for the additional operational footprint.

The application architecture should not require Tier C for ordinary security.

---

# 8. AWS account structure

For serious enterprise operation, avoid running everything in one AWS account.

Recommended minimum:

```text
AWS Organization
|
+-- security / audit
|
+-- shared services
|
+-- non-production
|    +-- dev
|    +-- test
|    +-- staging
|
+-- production
|
+-- optional dedicated-customer accounts
```

Separate production from development credentials, networks, logs, and deployment permissions.

---

# 9. Network architecture

Production EKS worker nodes should run in private subnets.

Recommended VPC:

```text
VPC
|
+-- public subnets
|     +-- internet-facing ALB
|     +-- NAT only if required
|
+-- private application subnets, AZ-A
|
+-- private application subnets, AZ-B
|
+-- private application subnets, AZ-C
|
+-- isolated database subnets
      +-- Aurora
```

Use at least **three Availability Zones** where the selected AWS Region permits it.

Prefer VPC endpoints for heavily used AWS services where economically justified, particularly:

```text
S3
ECR
CloudWatch
STS
SQS
Secrets Manager
```

This reduces dependence on NAT gateways and can reduce data path exposure/cost.

---

# 10. Ingress

Use:

```text
Route 53
   |
AWS WAF
   |
Application Load Balancer
   |
Kubernetes ingress / service
   |
API pods
```

WAF policies should cover:

- basic managed protections;
- request size limits;
- rate-based controls;
- known malicious patterns;
- tenant/API-plan limits enforced additionally by the application.

Do not use WAF as the sole application authorization mechanism.

---

# 11. Authentication and enterprise integration

The API should support OAuth 2.0 / OpenID Connect.

Large organizations may require:

```text
client credentials
JWT
enterprise IdP integration
machine-to-machine access
scoped service accounts
private connectivity
```

Keep identity outside validation logic.

Authorization should resolve:

```text
principal
    |
    v
tenant
    |
    v
allowed operations / API scopes
```

---

# 12. AWS workload identity

Use **EKS Pod Identity** or the approved AWS service-account IAM mechanism.

Each Kubernetes service account should receive only the AWS permissions required by its workload.

Examples:

```text
api-service-account
    SQS:SendMessage
    S3:Put/Get specific prefixes
    database secret access

worker-service-account
    SQS:Receive/Delete/ChangeMessageVisibility
    S3:Get/Put validation prefixes
    result persistence access

observability-service-account
    telemetry-specific permissions
```

Never put static AWS access keys in Kubernetes Secrets.

AWS EKS Pod Identity:

- https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html

---

# 13. Secret management

Use AWS Secrets Manager and/or SSM Parameter Store for:

```text
database credentials
OAuth client secrets
external service credentials
license keys if any
```

Kubernetes should receive secrets through a controlled integration.

Do not commit:

```text
passwords
private keys
tokens
AWS credentials
```

to container images or Git repositories.

---

# 14. Encryption

Default requirements:

```text
TLS externally
TLS internally where required
S3 SSE-KMS
database encryption at rest
EBS encryption
encrypted backups
Secrets Manager encryption
```

Large customers may request customer-specific KMS keys.

Plan the data model so that per-tenant encryption keys can be introduced without redesigning storage.

---

# 15. High availability

The API tier should run multiple replicas.

Minimum production target:

```text
API min replicas >= 3
```

spread across Availability Zones.

Workers should also run across multiple zones.

Use topology spread constraints so a single node or AZ failure cannot remove all replicas.

Kubernetes topology spread documentation:

- https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/

Use PodDisruptionBudgets for critical replicated services so planned node maintenance cannot evict too many pods simultaneously:

- https://kubernetes.io/docs/concepts/workloads/pods/disruptions/

---

# 16. Pod scheduling constraints

For production deployments:

```text
topologySpreadConstraints
podAntiAffinity where justified
PodDisruptionBudget
readinessProbe
livenessProbe
startupProbe
resource requests
resource limits
graceful termination
```

For API pods:

```text
spread across zones
spread across nodes
avoid all replicas on one node
```

For workers:

```text
spread across zones
but optimize cost and packing
```

Do not apply extremely restrictive anti-affinity rules to large worker fleets; they can make pods unschedulable and force unnecessary nodes.

---

# 17. Graceful worker shutdown

Scaling down a worker must not lose or duplicate work unnecessarily.

Worker lifecycle:

```text
SIGTERM
   |
   v
stop receiving new queue messages
   |
   v
finish current validation if within grace period
   |
   v
persist result
   |
   v
delete SQS message
   |
   v
exit
```

Set:

```text
terminationGracePeriodSeconds
```

long enough for the upper bound of normal validation.

If the worker cannot finish:

```text
do not delete SQS message
```

The visibility timeout must allow another worker to retry later.

---

# 18. SQS visibility timeout

The SQS visibility timeout must be greater than normal job execution time.

Measure:

```text
validation_duration_seconds
```

Then choose a timeout safely above high-percentile duration.

Conceptually:

```text
visibility_timeout >
P99 validation duration
+
shutdown margin
+
network/storage margin
```

For unusually long jobs, workers can extend message visibility while processing.

A message must only be deleted after the authoritative result has been persisted.

---

# 19. Dead-letter queue

Use a DLQ.

A job should reach the DLQ only for repeated execution/infrastructure failures.

It should **not** go to a DLQ merely because the XML is non-compliant.

Examples:

```text
UBL invalid
    => successful processing result: INVALID
    => delete queue message

EN16931 invalid
    => successful processing result: INVALID
    => delete queue message

worker repeatedly crashes
    => retry
    => eventually DLQ

persistent object retrieval failure
    => retry / DLQ according to classification
```

DLQ depth must be monitored and alerted.

---

# 20. Scaling architecture

There are three independent autoscaling layers.

```text
Layer 1: application pods
    HPA / KEDA

Layer 2: Kubernetes nodes
    Karpenter

Layer 3: managed AWS services
    SQS / S3 naturally elastic
    Aurora scaled/configured independently
```

Do not expect one autoscaler to solve all three.

---

# 21. API pod scaling

The API workload is request-driven.

Use Kubernetes **Horizontal Pod Autoscaler (HPA)**.

Potential signals:

```text
CPU utilization
memory pressure
requests per second
P95/P99 request latency
in-flight requests
```

Start with:

```text
CPU
+
request rate or latency
```

Do not scale an API solely from memory unless the memory behavior is well understood.

The Kubernetes HPA supports resource metrics and custom/external metrics.

Official documentation:

- https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/

AWS EKS also recommends custom/external metrics where CPU and memory do not reflect the actual application constraint.

---

# 22. Worker pod scaling

The validation worker is **queue-driven**, not primarily request-driven.

Use **KEDA** with the SQS scaler.

KEDA SQS documentation:

- https://keda.sh/docs/2.20/scalers/aws-sqs/

Primary metric:

```text
ApproximateNumberOfMessagesVisible
```

Potentially include in-flight work depending on the scaler configuration and desired behavior.

The first scaling model should be:

```text
desired workers
    ~= queue backlog / target backlog per worker
```

KEDA exposes a `queueLength` target; a target such as:

```text
5 messages per worker
```

means roughly:

```text
100 messages
/
5 messages per worker
=
20 desired workers
```

This is a starting point, not a universal production number.

---

# 23. Queue age is more important than queue depth alone

Two queues can both contain 1,000 messages:

```text
Queue A:
1000 messages
arrival rate falling
oldest message = 5 seconds

Queue B:
1000 messages
arrival rate rising
oldest message = 8 minutes
```

They have very different customer impact.

Therefore observe:

```text
queue depth
oldest message age
arrival rate
completion rate
```

The most important operational question is:

> How long will a newly submitted job wait before processing?

That is a service-level signal.

---

# 24. Backlog-drain scaling model

A more useful planning formula is:

```text
desired_workers =
ceil(
    queue_depth
    * average_job_duration
    / target_queue_drain_time
)
```

Example:

```text
queue depth             = 10,000
mean validation time    = 0.5 s
target drain time       = 60 s

desired workers =
10,000 * 0.5 / 60
= 83.3

=> approximately 84 concurrent workers
```

This assumes approximately one job at a time per worker and should be corrected using measured concurrency and overhead.

For capacity planning use conservative latency such as P90/P95 rather than an optimistic mean.

---

# 25. Throughput model

Measure:

```text
jobs_completed_total
```

and derive:

```text
jobs_per_second
```

Per-worker throughput:

```text
worker_throughput =
completed jobs / worker-seconds
```

Then:

```text
required_workers =
incoming_jobs_per_second
/
sustainable_jobs_per_second_per_worker
```

Apply a headroom factor.

Example:

```text
arrival rate = 200 jobs/s
worker sustainable throughput = 4 jobs/s
headroom = 30%

base requirement = 50 workers
with headroom ≈ 65 workers
```

This is the capacity model behind autoscaling.

---

# 26. Scale on service-level risk, not CPU alone

A validation worker can have:

```text
low CPU
large queue
```

for example if:

- storage access is slow;
- Java startup dominates;
- workers are blocked on I/O;
- per-worker concurrency is too low.

Therefore worker scaling should use:

```text
PRIMARY
queue backlog
queue age

SECONDARY
CPU
memory
job execution time
worker saturation
```

CPU is supporting evidence.

Queue pressure is the business demand signal.

---

# 27. Recommended worker autoscaling signals

Use the following observables.

## Demand signals

```text
SQS ApproximateNumberOfMessagesVisible
SQS ApproximateAgeOfOldestMessage
incoming jobs/sec
```

## Supply signals

```text
worker replicas
workers ready
jobs completed/sec
jobs active
```

## Saturation signals

```text
worker CPU
worker memory
JVM heap
GC pause
job duration P95/P99
ephemeral storage
Saxon process concurrency
```

## Reliability signals

```text
job errors/sec
timeouts/sec
pod restarts
OOMKills
DLQ depth
SQS redelivery count
```

Scale based primarily on demand and SLA risk; use saturation and reliability to constrain the scale policy.

---

# 28. KEDA worker policy

Recommended conceptual policy:

```text
minReplicaCount: 2
maxReplicaCount: capacity-tested ceiling

SQS trigger:
    queueLength: target backlog per worker

scale up:
    aggressive

scale down:
    conservative
```

In production, avoid scaling to zero for the standard enterprise queue unless cold-start latency is contractually acceptable.

For low-priority/batch worker pools, scale-to-zero may be useful.

---

# 29. Scale-up behavior

Scale up quickly when:

```text
queue age increases
queue depth increases rapidly
arrival rate exceeds completion rate
```

Goal:

```text
prevent backlog from becoming customer-visible latency
```

Do not throttle scale-up too aggressively.

However AWS warns against huge Kubernetes control-plane bursts where cluster size changes by large percentages immediately.

Use Karpenter and HPA/KEDA limits that grow capacity progressively but quickly.

EKS control-plane scaling guidance:

- https://docs.aws.amazon.com/eks/latest/best-practices/scale-control-plane.html

---

# 30. Scale-down behavior

Scale down much more slowly than scale up.

Reasons:

- avoid thrashing;
- allow queue bursts to clear;
- preserve warm JVM/Saxon capacity;
- avoid repeated EC2 launch/termination;
- prevent messages being repeatedly abandoned during worker termination.

Kubernetes HPA supports a scale-down stabilization window.

Default HPA scale-down stabilization is typically 300 seconds unless changed.

Recommended initial worker policy:

```text
scale-up stabilization: 0
scale-down stabilization: 5-15 minutes
```

Tune from actual traffic.

HPA behavior reference:

- https://kubernetes.io/docs/reference/kubernetes-api/autoscaling/horizontal-pod-autoscaler-v2/

---

# 31. Warm capacity

Enterprise customers care about tail latency.

Maintain a minimum warm pool.

Example:

```text
normal API replicas: >= 3
normal validation workers: >= 2 or 3
```

For predictable daily peaks, increase minimum capacity before the peak rather than relying only on reactive scaling.

Historical telemetry should eventually support scheduled or predictive pre-scaling.

---

# 32. Node autoscaling with Karpenter

Use **Karpenter** for EKS node provisioning where possible.

Karpenter observes pods that cannot be scheduled and provisions EC2 nodes that satisfy their requirements.

AWS recommends Karpenter for changing capacity needs and highlights its ability to choose from broad compatible instance types.

References:

- https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html
- https://docs.aws.amazon.com/eks/latest/best-practices/scale-data-plane.html

Conceptually:

```text
SQS backlog rises
    |
    v
KEDA increases worker replicas
    |
    v
pods become Pending
    |
    v
Karpenter sees resource demand
    |
    v
Karpenter provisions EC2 nodes
    |
    v
pods schedule
    |
    v
queue drains
```

This is the desired scaling cascade.

---

# 33. Node pools

Use at least two logical capacity pools.

```text
system / critical
    On-Demand

application / worker
    On-Demand + optional Spot
```

System components should not depend entirely on Spot.

Potential worker pools:

```text
api-pool
    On-Demand
    stable

worker-on-demand
    baseline capacity

worker-spot
    burst/bulk capacity
```

Do not run 100% of enterprise validation capacity on Spot.

---

# 34. Spot strategy

Spot Instances can substantially reduce burst compute cost for queue workers because queue jobs are naturally retryable.

Safe uses:

```text
bulk jobs
overflow capacity
replay jobs
non-urgent validation
part of standard worker fleet
```

Maintain sufficient On-Demand baseline to meet SLA under Spot shortage/interruption.

Worker termination handling must be correct before Spot is introduced.

---

# 35. Instance selection

Do not pin the fleet to a single EC2 instance type.

Karpenter can choose from multiple compatible instance families/types.

This improves:

- capacity availability;
- resilience to instance shortages;
- cost efficiency.

Validation workers are likely CPU-oriented with meaningful memory requirements.

Benchmark likely candidate families rather than guessing.

Measure:

```text
jobs/sec per vCPU
jobs/sec per €
memory/job
startup latency
```

Then express broad Karpenter constraints rather than a single SKU.

---

# 36. Resource requests and limits

Autoscaling depends on correct Kubernetes resource requests.

Every container must have realistic:

```text
cpu.requests
memory.requests
```

Without them:

- scheduler packing is poor;
- Karpenter decisions are poor;
- HPA CPU utilization becomes misleading;
- cost attribution becomes inaccurate.

Do not choose values from intuition.

Derive them from load tests.

---

# 37. VPA in recommendation mode

Use Vertical Pod Autoscaler initially in recommendation/audit mode to right-size workloads.

Do not immediately allow automatic production mutation if pod recreation could violate service expectations.

AWS recommends using VPA recommendations as part of resource right-sizing.

Reference:

- https://docs.aws.amazon.com/eks/latest/best-practices/cost-opt-compute.html

---

# 38. Observability stack

Recommended production telemetry:

```text
Metrics
    Amazon Managed Service for Prometheus

Dashboards
    Amazon Managed Grafana

Logs
    CloudWatch Logs

Traces
    OpenTelemetry
    -> AWS X-Ray or selected compatible backend

AWS infrastructure metrics
    CloudWatch

Kubernetes infrastructure
    kube-state-metrics
    node exporter / managed collector
```

Amazon Managed Service for Prometheus is Prometheus-compatible and scales ingestion/storage/querying as a managed service.

References:

- https://docs.aws.amazon.com/prometheus/
- https://docs.aws.amazon.com/eks/latest/userguide/prometheus.html

---

# 39. OpenTelemetry

Instrument the Go API and validation worker with OpenTelemetry.

A trace should cross:

```text
HTTP request
    |
    v
job creation
    |
    v
SQS publish
    |
    v
queue wait
    |
    v
worker receive
    |
    v
S3 fetch
    |
    v
XSD validation
    |
    v
EN16931 validation
    |
    v
Peppol validation
    |
    v
result persistence
```

Because the processing is asynchronous, explicitly propagate trace context through queue message metadata.

Do not depend on thread-local request context surviving the queue.

---

# 40. Trace attributes

Useful bounded attributes:

```text
validation.profile
validation.profile_version
document.type
job.result
validation.layer
aws.region
k8s.namespace
service.name
```

Potentially sensitive/high-cardinality fields such as:

```text
tenant_id
invoice_id
document_id
```

should be governed carefully.

They may be useful in traces/logs but should not become unbounded metric labels.

---

# 41. Metrics taxonomy

Use four groups:

```text
RED for APIs
    Rate
    Errors
    Duration

USE for resources
    Utilization
    Saturation
    Errors

Queue/workflow metrics
    backlog
    age
    throughput
    retries

Business/SLA metrics
    documents validated
    compliance result counts
    end-to-end latency
```

---

# 42. API observables

Measure:

```text
http_requests_total
http_request_duration_seconds
http_requests_in_flight
http_response_size_bytes
http_request_size_bytes
http_errors_total
rate_limit_rejections_total
auth_failures_total
```

Important views:

```text
RPS
P50 latency
P95 latency
P99 latency
5xx rate
4xx rate
```

Scale API pods when:

```text
sustained RPS per pod rises
or
latency increases with saturation
or
CPU crosses target
```

---

# 43. Queue observables

Mandatory:

```text
queue_depth_visible
queue_messages_in_flight
queue_oldest_message_age_seconds
queue_messages_sent_total
queue_messages_received_total
queue_messages_deleted_total
queue_dlq_depth
```

Derived:

```text
arrival_rate
completion_rate
backlog_growth_rate
estimated_drain_time
```

Estimated drain time:

```text
estimated_drain_seconds =
queue_depth / current_completion_rate
```

when completion rate is non-zero.

This should become a first-class dashboard panel.

---

# 44. Worker observables

Measure per worker deployment:

```text
worker_jobs_started_total
worker_jobs_completed_total
worker_jobs_active
worker_job_duration_seconds
worker_job_failures_total
worker_timeouts_total
worker_retries_total

process_cpu_seconds
container_cpu_usage
container_memory_working_set
container_restarts
OOMKills

Saxon invocation duration
Saxon process failures
SVRL bytes
XML input bytes
```

Histogram dimensions should remain bounded.

---

# 45. Validation-layer observables

Measure independently:

```text
ubl_xsd_duration_seconds
en16931_duration_seconds
peppol_duration_seconds

ubl_xsd_invalid_total
en16931_invalid_total
peppol_invalid_total

validation_engine_errors_total
```

This will reveal whether semantic validation is the dominant cost.

---

# 46. Rule failure metrics

Useful but dangerous due to cardinality.

A bounded metric can use the standards rule ID:

```text
validation_rule_failures_total{
    profile="en16931",
    rule="BR-CO-13"
}
```

Rule IDs are finite and controlled, so cardinality is manageable.

Do not add:

```text
tenant
document_id
invoice_number
```

to this metric.

Use logs/database queries for those dimensions.

---

# 47. Java/Saxon observables

When Saxon runs in a JVM process, observe:

```text
process startup duration
validation execution duration
maximum resident memory
exit code
timeouts
```

If the architecture later uses long-lived JVM workers, also expose:

```text
heap used
heap max
GC pause duration
GC count
threads
class loading
```

A per-job JVM subprocess has different scaling behavior from a warm persistent JVM.

Benchmark both before changing architecture.

---

# 48. Database observables

Monitor:

```text
database connections
connection pool saturation
CPU
memory
read latency
write latency
transaction latency
lock waits
deadlocks
replication lag where applicable
free storage
IOPS
```

API/worker scaling can overload the database even when EKS has abundant compute.

Therefore worker `maxReplicaCount` must respect downstream database capacity.

---

# 49. S3 observables

Monitor:

```text
GetObject latency/errors
PutObject latency/errors
request rate
4xx
5xx
data transferred
storage growth
```

Document storage can become a major cost driver even when compute cost is controlled.

Implement lifecycle rules for:

```text
raw XML
raw SVRL
old validation artefacts
audit retention
```

according to contract.

---

# 50. Kubernetes observables

At cluster level:

```text
pod count
pending pods
unschedulable pods
node count
node CPU
node memory
node filesystem pressure
pod restarts
OOMKills
evictions
scheduler latency
API server throttling
DaemonSet health
CoreDNS health
CNI/IP availability
```

For scaling, `Pending` and `Unschedulable` pods are particularly important because they indicate that pod autoscaling has requested more capacity than the node layer currently provides.

---

# 51. Karpenter observables

Monitor:

```text
pending unschedulable pods
node provisioning latency
node registration latency
node count
provisioning failures
instance capacity errors
node consolidation events
node terminations
Spot interruption handling
```

Key question:

```text
How long from "KEDA wants more workers"
to "workers are Ready"?
```

Call this:

```text
capacity_activation_latency
```

It directly affects burst SLA.

---

# 52. Scaling decision dashboard

Create one dashboard specifically for autoscaling.

Top row:

```text
incoming jobs/sec
completed jobs/sec
queue depth
oldest message age
```

Second row:

```text
desired worker replicas
actual worker replicas
ready worker replicas
pending worker pods
```

Third row:

```text
node count
Karpenter provisioning
node CPU/memory
```

Fourth row:

```text
P50/P95/P99 validation duration
timeouts
error rate
DLQ
```

Fifth row:

```text
estimated queue drain time
cost/document
documents/sec
```

An operator should be able to explain a scale event from one screen.

---

# 53. Scale-up conditions

Worker scale-up should occur when one or more of these conditions are sustained:

```text
queue depth > target backlog per worker
oldest queue age approaches SLO budget
arrival rate > completion rate
estimated drain time exceeds target
```

Secondary evidence:

```text
CPU high
worker active saturation high
P95 job duration rising
```

Recommended conceptual policy:

```text
if queue pressure rises:
    increase pods

if pods cannot schedule:
    Karpenter adds nodes
```

---

# 54. Scale-down conditions

Scale down only when all are healthy:

```text
queue depth low
oldest message age near zero
arrival rate <= completion capacity
worker utilization low
no recent backlog spike
no pending jobs requiring warm capacity
```

Use a stabilization period.

Example initial policy:

```text
no aggressive scale-down for 10 minutes
```

Then tune based on cost and workload periodicity.

---

# 55. Scale safety boundaries

Every autoscaler needs a ceiling.

Worker ceiling must reflect:

```text
AWS account limits
database connection budget
S3/SQS behavior
cluster/IP capacity
license constraints if any
cost guardrails
downstream API limits
```

Example:

```text
max workers = min(
    tested EKS capacity,
    DB-safe concurrency,
    cost ceiling,
    tenant/SLA concurrency policy
)
```

Do not set `maxReplicaCount: 10000` merely because Kubernetes permits a large number.

---

# 56. Database connection budget

Suppose:

```text
database max safe connections = 500
API reserves                  = 100
operations/admin reserve      = 50
available for workers         = 350
worker pool max connections   = 2
```

Then:

```text
worker max based on DB
= 350 / 2
= 175 workers
```

That becomes one input to KEDA's `maxReplicaCount`.

Better still, workers should keep database interactions short and use bounded pools.

---

# 57. Per-tenant fairness

A single customer must not consume the entire worker fleet.

Introduce fairness controls as volume grows.

Possible strategies:

```text
per-tenant admission rate
per-tenant queued-job quota
weighted priority
priority queues
worker concurrency quotas
enterprise reserved capacity
```

Avoid a design where one bulk upload pushes every other customer behind 10 million jobs.

---

# 58. Priority model

Recommended eventual service classes:

```text
interactive
standard
bulk
```

Possible queues:

```text
validation-interactive
validation-standard
validation-bulk
```

Each has:

```text
different worker minimum
different queue age SLO
different max capacity
different Spot percentage
```

This maps naturally to commercial plans.

---

# 59. Service-level objectives

Define SLOs around customer experience, not infrastructure.

Possible initial SLOs:

```text
API availability
job acceptance latency
queue wait latency
end-to-end validation latency
result retrieval availability
```

Example, for illustration only:

```text
99.9% API availability
99% job submissions accepted < 500 ms
99% standard validations completed < 10 s
99.9% jobs not lost
```

Actual numbers must be derived from load testing and commercial commitments.

Do not sign customer SLAs before the SLO has been observed successfully under production-like load.

---

# 60. Error budget

For each SLO, define an error budget.

Example:

```text
SLO = 99.9% successful service time
```

The error budget governs:

```text
release velocity
risk tolerance
incident response
capacity work
```

If the budget is being consumed rapidly, scaling and reliability work takes priority over feature releases.

---

# 61. Autoscaling SLO

Define a separate internal scaling objective:

```text
When queue age breaches warning threshold,
capacity must increase sufficiently within N seconds.
```

Observe:

```text
queue_pressure_detected_at
worker_scale_requested_at
pod_created_at
node_ready_at
worker_ready_at
queue_recovery_at
```

This gives a measurable autoscaling feedback loop.

---

# 62. Observability-driven tuning cycle

Autoscaling configuration should not be static.

Use this loop:

```text
observe
   |
   v
identify saturation point
   |
   v
adjust requests / scaling target
   |
   v
load test
   |
   v
deploy
   |
   v
compare SLO + cost
   |
   +------> repeat
```

Every scaling change should answer:

```text
Did latency improve?
Did error rate improve?
Did cost/document worsen?
Did cluster churn increase?
```

---

# 63. Cost observables

For a commercial service, observability must include economics.

Track:

```text
AWS cost/day
EKS compute cost
database cost
S3 storage cost
CloudWatch/telemetry cost
data transfer cost
cost per 1,000 validations
cost per profile
```

Derived:

```text
compute_seconds / document
GB-hours / document
cost / document
```

EN 16931 and Peppol validation may have different costs from UBL XSD-only validation.

This should inform pricing.

---

# 64. FinOps metric

A useful business metric:

```text
validation_cost_per_1000_documents
```

Break down by:

```text
ubl21
en16931
peppol
```

A technically impressive autoscaler that doubles cost per document is not a complete success.

---

# 65. Telemetry retention and cost

Observability can become expensive.

Control:

```text
metric cardinality
trace sampling
log verbosity
retention
raw XML logging
high-volume debug logs
```

Recommended:

```text
metrics:
    retain aggregate operational data

traces:
    sample normally
    increase sampling for errors/slow requests

logs:
    structured
    no full XML by default
```

Customer documents may contain sensitive commercial information.

---

# 66. OpenTelemetry collection architecture

Recommended:

```text
Go services
    |
    | OTLP
    v
OpenTelemetry Collector
    |
    +--> metrics -> Amazon Managed Service for Prometheus
    |
    +--> traces -> AWS X-Ray / selected backend
    |
    +--> logs -> CloudWatch where appropriate
```

AWS Observability Accelerator supports managed Prometheus collectors and an OpenTelemetry-based path for metrics, logs, and traces.

Reference:

- https://docs.aws.amazon.com/prometheus/latest/userguide/obs_accelerator.html

---

# 67. Amazon Managed Grafana

Use Amazon Managed Grafana or an equivalently governed Grafana deployment for:

```text
operations dashboards
capacity dashboards
SLO dashboards
tenant-independent business metrics
cost/throughput views
```

AWS provides EKS monitoring integrations with Amazon Managed Service for Prometheus and Managed Grafana.

Reference:

- https://docs.aws.amazon.com/grafana/latest/userguide/solution-eks.html

---

# 68. Alerting policy

Do not page on every metric fluctuation.

Alerts should identify actionable conditions.

## Page immediately

```text
API unavailable
queue oldest age above critical SLO threshold
no ready workers
DLQ growing rapidly
database unavailable
persistent validation-engine failure
error rate above critical threshold
```

## Ticket / warning

```text
worker CPU sustained high
cost/document rising
node utilization poor
queue age approaching threshold
Karpenter provisioning slow
database pool saturation warning
```

---

# 69. Example alert hierarchy

```text
WARNING:
queue_oldest_age > 30s for 5m

CRITICAL:
queue_oldest_age > 120s for 5m

WARNING:
DLQ > 0

CRITICAL:
DLQ increasing continuously

WARNING:
pending worker pods > 0 for 2m

CRITICAL:
pending worker pods > 0 for 10m
```

Thresholds must be aligned with contracted SLOs.

---

# 70. Deployment constraints

Production deployments must preserve capacity and job correctness.

Required constraints:

```text
rolling deployments
PodDisruptionBudgets
readiness gates
multi-AZ replicas
graceful worker termination
version-pinned validation artefacts
backward-compatible queue message schema
database migration compatibility
no loss of in-flight SQS work
```

---

# 71. Immutable releases

A production worker image should identify exactly:

```text
application version
UBL version
EN16931 version
Peppol version
Java version
Saxon version
base image
```

Never use:

```text
latest
```

for production images or validation standards.

AWS also recommends pinning known-good AMIs rather than using an automatically moving latest AMI for production Karpenter nodes.

Reference:

- https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html

---

# 72. Validation artefacts and deployment

Validation standards are part of the release.

Example image labels:

```text
app.version=1.8.0
ubl.version=2.1
en16931.version=1.3.16
peppol.version=3.0.21
saxon.version=<pinned>
```

Changing:

```text
EN16931 1.3.16 -> 1.3.17
```

should be treated like an application behavior change.

Run regression tests before rollout.

---

# 73. Blue/green or canary validation upgrades

A new standards version can legitimately produce different validation outcomes.

Therefore use controlled rollout:

```text
old worker profile version
       |
       +---- majority traffic

new worker profile version
       |
       +---- canary subset
```

Compare:

```text
valid/invalid delta
rule failure delta
duration delta
memory delta
```

Do not silently replace the validation rule set for every customer.

---

# 74. Explicit profile version selection

Large customers may require reproducibility.

Eventually support:

```json
{
  "profile": "en16931",
  "version": "1.3.16"
}
```

Policy can define:

```text
default current version
supported legacy versions
retirement date
```

That enables customers to reproduce historical validation.

---

# 75. Kubernetes namespaces

Recommended:

```text
platform-system
observability
api
validation
```

Optionally:

```text
customer-dedicated-<id>
```

for contracted isolated workloads.

Use NetworkPolicies where the CNI/networking implementation supports the required controls.

---

# 76. Resource quotas

Apply namespace quotas so accidental deployments cannot consume the entire cluster.

Control:

```text
CPU
memory
pods
ephemeral storage
```

Also use LimitRanges or admission policy to require resource requests.

---

# 77. Deployment pipeline

Recommended:

```text
Git
 |
 v
CI
 |
 +--> unit tests
 +--> security scan
 +--> SBOM
 +--> image build
 +--> integration validation corpus
 +--> load smoke test
 |
 v
ECR
 |
 v
staging EKS
 |
 +--> automated tests
 +--> observability verification
 |
 v
production approval
 |
 v
progressive deployment
```

Use Infrastructure as Code.

Recommended candidates:

```text
Terraform
AWS CDK
```

Choose one primary tool to avoid fragmented infrastructure ownership.

---

# 78. GitOps, optional but recommended

For Kubernetes manifests, a GitOps model using Argo CD or Flux can provide:

```text
declarative desired state
audit trail
drift detection
controlled promotion
rollback
```

Large enterprise customers value repeatable operational processes as much as application features.

---

# 79. Container supply-chain constraints

Production images should be:

```text
minimal
non-root where practical
scanned
signed if supply-chain policy requires
immutable
SBOM-generated
```

Pin:

```text
base image digest
Java version
Saxon artefact
xmllint/libxml2 version
```

Avoid runtime package installation.

---

# 80. Load testing strategy

Scalability cannot be designed only on diagrams.

Build a representative workload corpus.

Dimensions:

```text
XML size
document complexity
UBL-only
EN16931
Peppol
valid documents
invalid documents
many failed assertions
```

Test arrival patterns:

```text
steady state
2x burst
10x burst
sudden batch upload
diurnal peak
customer bulk import
```

---

# 81. Scale test stages

## Stage 1: single worker benchmark

Determine:

```text
jobs/sec
P95 execution time
CPU/job
memory/job
```

## Stage 2: one node

Increase workers until node saturation.

Determine:

```text
optimal pods/node
```

## Stage 3: multi-node fixed size

Confirm near-linear horizontal scaling.

## Stage 4: KEDA

Introduce queue-driven pod scaling.

## Stage 5: Karpenter

Allow node autoscaling.

## Stage 6: failure injection

Terminate:

```text
pod
node
AZ-access path where practical
```

## Stage 7: sustained load

Run long enough to expose:

```text
memory leaks
GC effects
DB pool exhaustion
log/metric costs
disk accumulation
```

---

# 82. Scalability acceptance criteria

A load test should pass only if:

```text
queue age stays inside target
API remains inside latency SLO
error rate remains acceptable
no message loss
no uncontrolled duplicate results
DB remains within safe operating envelope
worker scaling stabilizes
node scaling stabilizes
cost/document remains acceptable
```

Raw throughput alone is not enough.

---

# 83. Idempotency

Queue processing must be idempotent.

SQS and distributed systems can produce duplicate delivery.

Use:

```text
job_id
```

as an idempotency key.

Persist job state transactionally enough that reprocessing cannot create incorrect duplicate business state.

A worker seeing an already-completed job should safely acknowledge or reconcile it.

---

# 84. State machine

Recommended job states:

```text
accepted
queued
processing
completed
failed-infrastructure
dead-lettered
cancelled
```

Compliance is separate:

```text
valid
invalid
not-applicable
unknown
```

Do not make `"invalid invoice"` equivalent to `"failed job"`.

---

# 85. Timeouts

Define separate timeouts:

```text
API timeout
S3 retrieval timeout
XSD validation timeout
EN16931 timeout
Peppol timeout
DB write timeout
overall job timeout
```

A single global timeout makes diagnosis difficult.

Record which stage timed out.

---

# 86. Backpressure

When downstream systems saturate, the queue is the backpressure mechanism.

Do not keep scaling workers indefinitely if:

```text
database is saturated
S3 errors rise
memory exhaustion occurs
```

Implement a maximum worker ceiling and alert when:

```text
queue pressure high
AND
worker count == max
```

That is a clear capacity incident.

---

# 87. Circuit-breaking downstream pressure

If Aurora becomes unhealthy:

```text
reduce or halt new worker consumption
rather than multiply database pressure
```

Possible mechanism:

```text
worker health gate
dynamic concurrency reduction
KEDA pause/manual cap
```

Do not respond to database latency by spawning thousands more workers.

---

# 88. Autoscaling feedback-loop risks

Bad autoscaling loops can amplify incidents.

Example:

```text
DB slows
    |
job duration rises
    |
queue rises
    |
autoscaler adds workers
    |
DB receives more connections
    |
DB slows further
```

Therefore scaling logic must observe downstream saturation.

Create alert:

```text
queue_pressure_high
AND
database_saturation_high
```

Response may be:

```text
capacity incident
not worker scale-up
```

---

# 89. Capacity envelope

Maintain a documented capacity envelope:

```text
max tested API RPS
max tested validations/sec
max tested worker replicas
max tested nodes
max tested DB connections
max tested queue growth
max tested XML size
```

Update it after every major performance release.

Sales commitments should stay inside a known capacity envelope or trigger a planned capacity project.

---

# 90. Regional deployment

Start with a single AWS Region deployed across multiple Availability Zones.

Do not begin with active-active multi-region unless a customer or SLA requires it.

Single-region Multi-AZ is substantially simpler and is enough for many enterprise services.

Architecture:

```text
Region
|
+-- AZ A
+-- AZ B
+-- AZ C
```

---

# 91. Disaster recovery

Even if the first production version is single-region, design backups for regional recovery.

Requirements:

```text
Aurora automated backups / snapshots
S3 versioning and replication policy if required
ECR replication if required
IaC capable of rebuilding cluster
validation artefacts stored reproducibly
Secrets recovery
DNS failover plan
```

Define:

```text
RPO
RTO
```

before making disaster-recovery promises.

---

# 92. Multi-region future

If required later:

```text
Route 53 / Global Accelerator
          |
    +-----+-----+
    |           |
Region A      Region B
EKS           EKS
SQS           SQS
S3            S3
DB strategy   DB strategy
```

Multi-region queue/database consistency is non-trivial.

Do not assume EKS replication alone creates a multi-region application.

---

# 93. Enterprise private connectivity

Some large organizations will want to avoid public Internet APIs.

Future option:

```text
AWS PrivateLink
private ALB/NLB patterns
VPN
Direct Connect
```

Keep API addressing and identity architecture compatible with private endpoints.

---

# 94. Auditability

Persist audit events for:

```text
job accepted
document retrieved
validation started
validation completed
profile/version used
result retrieved
administrative change
profile configuration change
retention deletion
```

Use immutable/append-oriented audit storage where regulatory requirements justify it.

Do not put full XML content in general audit logs.

---

# 95. Data retention

Make retention configurable per customer/plan.

Potential modes:

```text
ephemeral validation
    delete XML after result

short-term
    retain 7/30/90 days

long-term archive
    contract-specific
```

S3 lifecycle rules should enforce policy.

Metadata deletion must coordinate with object deletion.

---

# 96. Observability and privacy

Telemetry must never accidentally become a shadow document store.

Prohibit by default:

```text
full XML in logs
invoice free text in metric labels
customer names in Prometheus labels
VAT IDs in metric labels
bank details in traces
```

Record identifiers only where operationally required and appropriately protected.

---

# 97. Initial production observability stack

Recommended minimum for first enterprise production:

```text
Amazon Managed Service for Prometheus
Amazon Managed Grafana
CloudWatch Logs
CloudWatch AWS-service metrics
OpenTelemetry in Go services
kube-state-metrics
Karpenter metrics
KEDA metrics
```

Add richer distributed tracing once request-to-queue trace propagation is implemented.

---

# 98. Minimum dashboards

Create these before go-live:

```text
1. Service Overview
2. API RED Dashboard
3. Queue and Worker Scaling
4. Validation Engine Performance
5. Kubernetes Capacity
6. Database Health
7. Customer/SLA Aggregate
8. Cost and Efficiency
```

No production launch should rely solely on ad-hoc queries.

---

# 99. Service Overview dashboard

Display:

```text
API availability
request rate
validation jobs/sec
P95 end-to-end latency
queue oldest age
queue depth
ready workers
invalid result rate
engine error rate
DLQ
```

This is the executive operations view.

---

# 100. Queue and Worker Scaling dashboard

Display together:

```text
queue depth
queue age
arrival rate
completion rate
desired workers
actual workers
ready workers
pending workers
node count
Karpenter launch latency
estimated drain time
```

This is the primary scalability dashboard.

---

# 101. Validation Engine dashboard

Display:

```text
UBL duration
EN16931 duration
Peppol duration
Saxon failures
XSD failures
SVRL size
validation duration by document size
memory/job
CPU/job
```

This guides worker sizing.

---

# 102. Cost dashboard

Display:

```text
documents/day
documents/profile
worker node cost
database cost
observability cost
storage cost
cost/1000 documents
```

Track whether scale optimization is financially useful.

---

# 103. Scaling policy evolution

Use four maturity levels.

## Level 1

```text
fixed replicas
manual nodes
```

Only for early development.

## Level 2

```text
HPA API
KEDA workers
fixed node pool
```

Good first scaling proof.

## Level 3

```text
HPA API
KEDA workers
Karpenter nodes
Prometheus/Grafana observability
```

Target production baseline.

## Level 4

```text
SLO-aware tuning
queue-age-informed policy
workload classes
Spot overflow
scheduled pre-scaling
tenant fairness
cost optimization
```

Target mature SaaS.

---

# 104. Implementation roadmap

The path from the current project to enterprise-scale AWS deployment should be incremental.

---

## Phase 0: instrument current worker before AWS migration

Add metrics now, even if the service is still local/simple.

Instrument:

```text
jobs accepted
jobs queued
jobs started
jobs completed
job duration
XSD duration
EN16931 duration
errors
XML size
```

Add OpenTelemetry-compatible trace IDs.

### Deliverable

A locally observable worker and API.

### Why first

Without a baseline, AWS autoscaling targets will be guesses.

---

## Phase 1: define job contract and idempotency

Stabilize queue payload:

```text
job_id
tenant_id
document reference
profile
profile version
created_at
trace context
priority/service class
```

Implement idempotent processing.

### Deliverable

Versioned job message schema.

---

## Phase 2: externalize document storage

Move raw XML to S3.

Queue only references.

### Deliverable

```text
API -> S3 -> queue -> worker -> S3/result DB
```

### Acceptance

Workers can be destroyed/recreated without local state loss.

---

## Phase 3: externalize durable metadata

Deploy PostgreSQL/Aurora-backed metadata persistence.

Store:

```text
job state
tenant
document metadata
results
versions
usage
```

### Acceptance

API and workers remain stateless.

---

## Phase 4: container hardening

Create production images for:

```text
api
worker
```

Worker image contains pinned:

```text
xmllint/libxml2
Java
Saxon
UBL XSD
EN16931
Peppol as introduced
```

### Acceptance

A job can run with no runtime download of standards artefacts.

---

## Phase 5: establish EKS non-production

Create via IaC:

```text
VPC
private subnets
EKS
ECR
SQS
S3
Aurora
IAM / Pod Identity
Secrets
```

Deploy fixed replicas first.

### Acceptance

Complete end-to-end validation succeeds in EKS.

---

## Phase 6: production observability foundation

Deploy:

```text
Amazon Managed Service for Prometheus
Amazon Managed Grafana
CloudWatch logs
OpenTelemetry collector/instrumentation
kube-state-metrics
```

Build core dashboards.

### Acceptance

Every job stage can be measured.

---

## Phase 7: HPA for API

Load test API.

Configure:

```text
min replicas
max replicas
CPU target
custom/request target if justified
scale-up policy
scale-down stabilization
```

### Acceptance

API latency remains inside target during burst test.

---

## Phase 8: KEDA for worker

Configure SQS-driven KEDA scaling.

Start with conservative:

```text
min workers
max workers
queueLength target
scale-down stabilization
```

### Acceptance

Worker replicas track backlog without oscillation.

---

## Phase 9: Karpenter

Introduce automatic node capacity.

Use:

```text
broad instance choices
multiple AZs
On-Demand baseline
```

### Acceptance

Pending worker pods cause node capacity to appear automatically.

Measure:

```text
capacity activation latency
```

---

## Phase 10: integrated autoscaling test

Generate a burst large enough that:

```text
SQS grows
KEDA adds pods
pods become pending
Karpenter adds nodes
workers become ready
queue drains
KEDA reduces pods
Karpenter consolidates nodes
```

Record every transition.

### Acceptance

No intervention required and SLO remains inside target.

---

## Phase 11: database and downstream guardrails

Establish:

```text
connection budgets
worker max replica limits
DB saturation alerts
circuit-breaker policy
```

### Acceptance

Scaling workers cannot accidentally collapse Aurora.

---

## Phase 12: multi-AZ availability

Add:

```text
topology spread
PDBs
three-AZ scheduling
failure testing
```

Terminate nodes deliberately.

### Acceptance

API remains available and queued jobs recover automatically.

---

## Phase 13: Spot burst capacity

After worker shutdown/idempotency are proven:

```text
introduce Spot worker capacity
```

Retain On-Demand baseline.

### Acceptance

Spot interruption does not lose work.

---

## Phase 14: tenant fairness and commercial service classes

Add:

```text
rate limits
tenant quotas
priority/bulk queues
reserved capacity if sold
```

### Acceptance

One tenant cannot destroy latency for another tenant.

---

## Phase 15: SLO-driven operation

Define:

```text
availability SLO
queue-age SLO
end-to-end validation SLO
error budget
```

Tie alerts to SLOs.

### Acceptance

Operations are based on customer impact rather than raw CPU graphs.

---

## Phase 16: cost optimization

Optimize only after reliability is proven.

Use telemetry to tune:

```text
requests/limits
pods/node
instance families
Spot ratio
warm worker minimum
scale-down delay
S3 retention
telemetry sampling
```

### Acceptance

Cost per document falls without SLO regression.

---

# 105. Recommended autoscaling matrix

| Component | Scaling mechanism | Primary signal | Secondary signals | Scale-down policy |
|---|---|---|---|---|
| API | HPA | RPS / CPU / latency | in-flight requests, memory | 5-10 min stabilization |
| Standard worker | KEDA | SQS queue depth | oldest age, throughput, CPU | 5-15 min stabilization |
| Bulk worker | KEDA | bulk queue | age, throughput | can scale to zero |
| EKS nodes | Karpenter | unschedulable pods | capacity availability, cost | consolidation after pod demand falls |
| Aurora | DB-native/configured | connections/CPU/IO | latency, lock waits | conservative |
| Observability | managed services | ingestion | cardinality/cost | managed / policy-driven |

---

# 106. Observable-to-action matrix

| Observable | Meaning | Likely action |
|---|---|---|
| Queue depth rising, age low | burst beginning | add workers |
| Queue depth rising, age rising | insufficient throughput | aggressive worker scale-up |
| Queue high, workers at max | capacity ceiling | alert/capacity incident |
| Workers desired > ready | scheduling/capacity problem | inspect Pending pods/Karpenter |
| Pending pods rising | insufficient node capacity | Karpenter/node investigation |
| Nodes launching slowly | AWS capacity/startup bottleneck | broaden instance types/pre-warm |
| CPU high, queue stable | per-worker saturation | scale workers / resize |
| Memory/OOM rising | bad requests or sizing | increase/request tune, investigate leak |
| DB pool saturated | downstream bottleneck | cap workers / DB scaling |
| Queue high + DB saturated | dangerous feedback loop | do not blindly add workers |
| P95 validation rising | complexity/resource contention | scale/benchmark |
| DLQ > 0 | repeated execution failure | investigate immediately |
| Cost/document rising | inefficiency | tune instance/pod/telemetry |
| Queue empty for long period | excess worker capacity | scale down |
| Node utilization low after pod scale-down | excess EC2 | Karpenter consolidation |

---

# 107. Suggested initial scaling targets

These are **starting hypotheses**, not production promises.

API:

```text
min replicas: 3
max replicas: based on load test
CPU target: 60-70%
scale-down stabilization: 5 minutes
```

Validation workers:

```text
min replicas: 2
queue target: begin testing around 2-10 queued jobs per worker
max replicas: derived from DB and cost capacity
scale-down stabilization: 10 minutes
```

Karpenter:

```text
three AZs
broad compatible instance families
On-Demand baseline
Spot only for explicitly allowed pools
```

Replace these values with measurements before production.

---

# 108. Scaling experiment log

Every tuning change should be recorded.

Example:

```text
Date:
Version:
Load pattern:
Queue target:
Worker min/max:
Pod CPU request:
Pod memory request:
Instance families:
Peak jobs/sec:
P95 latency:
Max queue age:
Max queue depth:
Peak workers:
Peak nodes:
Cost estimate:
Observed bottleneck:
Decision:
```

This becomes institutional capacity knowledge.

---

# 109. Commercial readiness checklist

Before selling to a large organization, be able to answer:

```text
What is your availability target?
Where is customer data stored?
How is it encrypted?
How are tenants isolated?
How long is data retained?
What are your RPO/RTO?
What happens if a node dies?
What happens if an AZ fails?
What happens if a worker crashes mid-validation?
Can validation be reproduced with the same standards version?
How are standards upgrades controlled?
How do you detect overload?
How quickly can you scale?
What is your maximum tested throughput?
How do you prevent one tenant consuming all capacity?
How are audit logs retained?
How are vulnerabilities patched?
```

The architecture in this document is designed to make those questions answerable.

---

# 110. Recommended production baseline

The target first serious enterprise deployment should be:

```text
AWS Region
|
+-- 3 Availability Zones
|
+-- EKS
|    |
|    +-- API Deployment
|    |     HPA
|    |     >= 3 minimum replicas
|    |
|    +-- Validation Worker Deployment
|          KEDA / SQS
|          warm minimum
|          bounded maximum
|
+-- Karpenter
|    |
|    +-- On-Demand baseline
|    +-- optional Spot burst
|
+-- SQS + DLQ
|
+-- S3 encrypted document/result storage
|
+-- Aurora PostgreSQL Multi-AZ metadata
|
+-- EKS Pod Identity / least privilege IAM
|
+-- AWS Secrets Manager
|
+-- Amazon Managed Service for Prometheus
|
+-- Amazon Managed Grafana
|
+-- CloudWatch Logs
|
+-- OpenTelemetry
```

This is a strong production baseline without prematurely creating a maze of microservices.

---

# 111. Architectural rule: scale the bottleneck you can observe

The platform should never scale because:

```text
"CPU is 70%, therefore add pods"
```

without understanding the workload.

The correct loop is:

```text
customer demand
      |
      v
queue pressure
      |
      v
worker demand
      |
      v
pod scaling
      |
      v
node demand
      |
      v
node scaling
      |
      v
throughput response
      |
      v
queue recovers
```

At every step, observe whether the expected response occurred.

If not, the bottleneck moved.

---

# 112. Final target

The mature service should behave as follows:

```text
1. Customer sends 100,000 invoices.

2. API accepts them rapidly and durably.

3. XML is stored once in S3.

4. Validation jobs enter SQS.

5. Queue backlog rises.

6. KEDA increases validation worker replicas.

7. Existing nodes fill.

8. Additional pods become unschedulable.

9. Karpenter provisions EC2 capacity across available AZs.

10. Workers become Ready.

11. Validation throughput rises.

12. Queue age remains within SLO.

13. Results are persisted and retrieved through the API.

14. Queue drains.

15. KEDA slowly reduces worker replicas.

16. Karpenter removes excess nodes.

17. Cost returns toward baseline.

18. Prometheus/Grafana/CloudWatch retain the evidence explaining every step.
```

That is the scalability objective.

The queue is the shock absorber.

KEDA converts business backlog into pod demand.

Karpenter converts pod demand into EC2 capacity.

Observability verifies whether the entire control loop is preserving customer SLOs at an acceptable cost.

---

# 113. Official references

## Amazon EKS scalability

EKS control-plane scaling:

- https://docs.aws.amazon.com/eks/latest/best-practices/scale-control-plane.html

EKS data-plane scaling:

- https://docs.aws.amazon.com/eks/latest/best-practices/scale-data-plane.html

Application scaling / HPA / external metrics:

- https://docs.aws.amazon.com/eks/latest/best-practices/application.html

Compute and cost optimization:

- https://docs.aws.amazon.com/eks/latest/best-practices/cost-opt-compute.html

Karpenter:

- https://docs.aws.amazon.com/eks/latest/best-practices/karpenter.html

Cluster autoscaling:

- https://docs.aws.amazon.com/eks/latest/best-practices/cluster-autoscaling.html

## Kubernetes autoscaling

Horizontal Pod Autoscaler:

- https://kubernetes.io/docs/concepts/workloads/autoscaling/horizontal-pod-autoscale/

HPA v2 behavior:

- https://kubernetes.io/docs/reference/kubernetes-api/autoscaling/horizontal-pod-autoscaler-v2/

Topology spread constraints:

- https://kubernetes.io/docs/concepts/scheduling-eviction/topology-spread-constraints/

Pod disruptions / PodDisruptionBudget:

- https://kubernetes.io/docs/concepts/workloads/pods/disruptions/

## KEDA

AWS SQS scaler:

- https://keda.sh/docs/2.20/scalers/aws-sqs/

## AWS identity

EKS Pod Identity:

- https://docs.aws.amazon.com/eks/latest/userguide/pod-identities.html

## Metrics and observability

Amazon Managed Service for Prometheus:

- https://docs.aws.amazon.com/prometheus/

EKS Prometheus monitoring:

- https://docs.aws.amazon.com/eks/latest/userguide/prometheus.html

AWS Observability Accelerator:

- https://docs.aws.amazon.com/prometheus/latest/userguide/obs_accelerator.html

Amazon Managed Grafana EKS monitoring:

- https://docs.aws.amazon.com/grafana/latest/userguide/solution-eks.html

CloudWatch Observability EKS add-on:

- https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/install-CloudWatch-Observability-EKS-addon.html

---

# 114. Immediate next steps for UBL-21

The next concrete work should be:

1. instrument the current API and queue worker with Prometheus/OpenTelemetry metrics;
2. establish exact per-stage duration metrics for UBL XSD and EN 16931/Saxon;
3. define the versioned queue job schema;
4. define idempotent job persistence;
5. externalize raw XML to an object-store abstraction compatible with S3;
6. establish PostgreSQL-backed metadata storage;
7. create production API and worker Docker images;
8. build a non-production EKS environment using IaC;
9. deploy SQS, S3 and Aurora integration;
10. deploy Amazon Managed Service for Prometheus and Grafana;
11. create the Queue and Worker Scaling dashboard;
12. benchmark one worker and establish jobs/sec and memory/job;
13. enable KEDA against SQS;
14. measure worker scaling;
15. enable Karpenter;
16. run a full burst test;
17. define the initial production SLO and worker maximum from measured data;
18. then begin enterprise pilot onboarding.

The most important sequencing rule is:

```text
instrument
    before
autoscaling
```

Without observability, autoscaling is guesswork.

With observability, scaling becomes an empirical control system.
