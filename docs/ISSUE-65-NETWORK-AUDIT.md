# Issue #65 networking audit

Date: 2026-09-01

## Scope

This audit covers the networking contracts used by the Issue #65 split-deployment gate, the Kubernetes and Minikube manifests, Docker Compose, and the on-premises installer. It checks service discovery, selectors, listeners, ports, namespace boundaries, NATS limits, and deployment reproducibility.

## Original findings

1. The gate automatically selected rootless Podman but placed NATS behind a normal ClusterIP. In the `e-ubl21-i65` Minikube profile, a provisioner using `nats://nats:4222` timed out and never reached NATS, while the same provisioner using the NATS pod IP completed immediately. This isolated the failure to rootless ClusterIP forwarding rather than DNS, selectors, NATS, JetStream, or application code.
2. The gate's embedded NATS deployment did not apply the required `max_payload: 2097152` setting.
3. The on-premises NATS release did not expose its monitoring port through the Service even though KEDA connects to `nats:8222`.
4. The application configured the short OpenTelemetry name `otel-collector:4318`, while the installer creates the collector in the separate `observability` namespace without fixing the generated Service name.
5. The on-premises NATS release did not apply the required 2 MiB maximum payload.
6. Helm charts in the on-premises installer were unpinned, so generated resource and port contracts could drift between installations.
7. Docker Compose assigned a fixed `container_name` to the worker while documenting horizontal scaling with `--scale worker=3`.

Runtime validation after the network correction exposed two additional gate blockers:

8. The gate applied split runtimes while the durable deployment marker was still `legacy`, so every orchestrator and stage worker correctly failed closed.
9. The API's `/healthz` route is OAuth-protected, but its Kubernetes HTTP probes supplied no bearer token and received HTTP 401 indefinitely.
10. The acceptance runner polls multiple uploads and runs the equivalence corpus in parallel, exceeding the production API's default 5 RPS rate limit and turning otherwise successful scenarios into HTTP 429 failures.

## Corrections

- The gate runs only NATS on the Minikube node network and injects the discovered node IP into the disposable runtime ConfigMap. Its headless Service remains available for endpoint selection and diagnostics. This bypasses rootless Podman's broken DNS and ClusterIP forwarding while leaving all split application workloads on ordinary pod networking.
- The gate mounts an explicit NATS configuration with JetStream, monitoring on port 8222, and `max_payload: 2097152`. NATS readiness is checked through its monitoring endpoint.
- The on-premises NATS chart has an explicit resource name, exposes the monitoring Service port, and applies `config.merge.max_payload=2097152`.
- The OpenTelemetry chart has an explicit `otel-collector` resource name. Application workloads use the cross-namespace endpoint `otel-collector.observability.svc.cluster.local:4318`.
- Every Helm chart installed by the on-premises bootstrap has an overridable, pinned default version.
- The worker's fixed Compose container name was removed so Compose can allocate unique names for scaled replicas.
- The gate provisions first, performs the documented fresh-install `legacy` to `draining` to `split` activation, and only then starts split runtimes. Both migration command outputs are retained as artifacts.
- API startup, readiness, and liveness probes use the listening TCP socket. The gate still performs an authenticated `/healthz` request before running acceptance scenarios.
- Only the disposable gate API receives an explicit high regression rate limit; production admission defaults are unchanged.

## Contracts verified before correction

- Kubernetes workload labels agree with Service selectors and all `targetPort` values agree with application listeners.
- The API listens on 8080, the combined worker metrics endpoint on 9091, XSD metrics on 9092, EN 16931 metrics on 9093, and each independently selected orchestrator workload on 9094.
- Reusing port 9094 on different pods is valid because each Service selects only its intended workload.
- All split workloads consume the same runtime ConfigMap, NATS URL, and queue namespace.
- There are no repository NetworkPolicies that deny pod traffic.
- Docker Compose uses service DNS `nats:4222` and mounts the repository NATS configuration.
- The legacy single-node Minikube topology intentionally uses host networking and loopback to accommodate rootless Podman, and it rolled out successfully during the audit.
- The host routes do not overlap the Minikube node (`192.168.49.0/24`), pod (`10.244.0.0/16`), or Service (`10.96.0.0/12`) networks.

## Validation results

- `go test ./...` passed.
- The production Kubernetes and legacy Minikube Kustomize bases rendered successfully.
- Shell syntax, Compose configuration, and Git whitespace validation passed.
- NATS chart 2.14.6 rendered with Services named `nats` and `nats-headless`, client port 4222, monitoring port 8222, and `max_payload: 2097152`.
- OpenTelemetry Collector chart 0.172.0 rendered a Service named `otel-collector` in namespace `observability` with OTLP/HTTP port 4318.
- The corrected rootless Podman Minikube gate passed all 6 scenarios. Its frozen semantic-equivalence corpus passed 32 of 32 fixtures against the baseline.
- Migration artifacts show the fresh deployment entering `draining` and then `split`; reconciliation reported zero errors.
- Evidence is retained under `regression-results/issue-65/20260901T-network-fix-4` and the disposable `e-ubl21` namespace was removed.

A complete on-premises installation remains an environment-level release check because this workstation is not a supported K3s on-premises target. The two charts whose Service contracts changed were rendered locally from their pinned official releases.
