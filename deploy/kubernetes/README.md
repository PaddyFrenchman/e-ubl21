# Independent Kubernetes validation workloads

This Kustomize base is the operational topology for issue #59. It deploys the
API, orchestrator, XSD worker, and EN 16931 worker as separate Deployments from
one immutable application image. Peppol will add another worker Deployment
without changing these roles.

The base assumes that the namespace can resolve `nats` on ports 4222 (client)
and 8222 (monitoring), KEDA 2.20 or compatible is installed, the
[Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
is installed for the
[KEDA CPU/memory scaling signals](https://keda.sh/docs/2.20/scalers/cpu/), an
OpenTelemetry collector is
reachable as `otel-collector:4318`, and an `e-ubl21-auth` Secret contains
`issuer`, `audience`, and `hmac-secret`. Metrics Server is not present by
default on every managed Kubernetes platform; install it using the deployment
platform's supported method and verify the resource metrics API before rollout:

```bash
kubectl get --raw /apis/metrics.k8s.io/v1beta1/nodes
```

Patch the external endpoints and placeholder image before applying:

```bash
kubectl apply -f deploy/kubernetes/namespace.yaml
kubectl -n e-ubl21 create secret generic e-ubl21-auth \
  --from-literal=issuer=https://issuer.example \
  --from-literal=audience=e-ubl21 \
  --from-literal=hmac-secret='replace-with-a-secret'

(cd deploy/kubernetes && \
  kustomize edit set image e-ubl21=registry.example/e-ubl21@sha256:...)
kubectl apply -k deploy/kubernetes
kubectl -n e-ubl21 wait --for=condition=complete job/e-ubl21-provision --timeout=5m
```

The provisioning Job owns the execution/evidence buckets and stage-specific
JetStream streams and consumers. The orchestrator and stage workers only
verify or open those resources and never change their topology. The API keeps
its existing idempotent upload-infrastructure bootstrap until the migration in
issue #61. To apply a changed stage `AckWait` or fleet-wide `MaxAckPending`,
delete the completed Job and apply the base again:

```bash
kubectl -n e-ubl21 delete job e-ubl21-provision --ignore-not-found
kubectl apply -k deploy/kubernetes
kubectl -n e-ubl21 wait --for=condition=complete job/e-ubl21-provision --timeout=5m
```

## Independent capacity controls

| Role | Replicas/scaler | Concurrency | Prefetch | Timeout | AckWait | Initial resources |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| API | HPA 2–10 | HTTP server | n/a | HTTP limits | n/a | 100m CPU / 128Mi |
| Orchestrator | KEDA 1–10 | one loop per result stream | 8 per stage | reconciliation 10s | stage result consumer | 100m CPU / 128Mi |
| XSD | KEDA 0–20 | 2 | 2 | 30s | 90s | 250m CPU / 256Mi |
| EN 16931 | KEDA 0–30 | 1 | 1 | 45s | 120s | 500m CPU / 768Mi |

The XSD and EN `ScaledObject` resources watch different JetStream streams and
durable consumers. Increasing EN backlog therefore changes only EN replicas.
CPU is a secondary signal. API scaling uses CPU/memory because it has no stage
queue, while orchestrator scaling watches both result queues.

All autoscaler-managed Deployments deliberately omit `spec.replicas`, so
reapplying the manifests cannot reset a replica count currently owned by HPA or
KEDA. The scale-to-zero worker triggers explicitly use an activation lag of
zero: because KEDA activation is a strict greater-than comparison, one queued
request is sufficient to activate a worker.

KEDA's [NATS JetStream scaler](https://keda.sh/docs/2.20/scalers/nats-jetstream/)
provides durable consumer lag even while a worker is at zero replicas. The
stream and consumer values are derived from
`EUBL21_QUEUE_NAMESPACE=eubl21.prod`; an overlay that changes the namespace
must patch those names too. The NATS monitoring endpoint should be restricted
to the cluster network.

## Capacity calculation

Measure mean service time from
`e_ubl21_stage_service_duration_seconds` separately for each stage. For stage
arrival rate `lambda`, per-pod concurrency `c`, mean service time `s` seconds,
and safety factor `f`:

```text
per-pod throughput = c / s
replicas = ceil(lambda / (c / s) * f)
```

Start with `f = 1.25` and revisit it using p95 queue age, redelivery, CPU, and
memory observations. Set JetStream `lagThreshold` near the number of requests
one replica can finish during the desired drain interval. Queue depth is the
live primary scaler; measured service time determines that threshold and the
maximum replica count. Oldest-request age remains an operator alarm and a
reason to lower the lag target or raise the maximum—it must not be hidden by a
healthy CPU signal.

Validator Deployments deliberately have no PodDisruptionBudget because they
can scale to zero and unacknowledged work is durable. API and orchestrator use
PDBs to preserve request admission and plan advancement during voluntary node
maintenance.

This base defines the post-split workloads but does not authorize switching an
existing installation from the combined worker. In particular, applying it is
not a substitute for reconciling legacy pending uploads. The stop/drain,
activation, legacy reconciliation, and rollback barrier remains issue #61.
