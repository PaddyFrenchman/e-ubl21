# Local Minikube regression deployment

This Kustomize base deploys the current distributed runtime into the dedicated
`e-ubl21-regression` namespace:

- one ephemeral, namespace-isolated NATS JetStream deployment;
- one authenticated API deployment;
- one validation-worker deployment using the pinned EN 16931 runtime in the
  candidate application image.

The base targets a single-node local cluster and uses host networking with
non-conflicting NATS/API/worker ports. This avoids broken ClusterIP forwarding
in rootless Podman Minikube while preserving separate Kubernetes Deployments,
namespace isolation, probes, events, logs, and rollout behavior. It is not a
production networking model and must not be reused as one.

Run the complete lifecycle from the repository root:

```bash
make regression-minikube
```

The command builds and loads `localhost/e-ubl21:regression`, creates a short-lived local
JWT signing secret, waits for all rollouts and health endpoints, invokes the
same `e-ubl21 regression` semantic runner used outside Kubernetes, collects
the #40 evidence bundle, and deletes the namespace. It leaves the Minikube
profile running for faster subsequent runs.

Artifacts are written to a timestamped directory below
`regression-results/minikube/`. Override the defaults when needed:

```bash
make regression-minikube \
  MANIFEST=createsamples/samples/manifest.json \
  PROFILE=ubl21 \
  ARTIFACT_DIR=regression-results/minikube/ubl21
```

Prerequisites are `minikube`, Docker, Go, and `curl`. A standalone `kubectl`
is not required: `devsetup/minikube-kubectl.sh` uses Minikube's pinned kubectl.
Set `KEEP_DEPLOYMENT=1` to retain the namespace for interactive debugging, or
run `make regression-minikube-clean` after an interrupted/retained run.

Useful overrides include `MINIKUBE_PROFILE`, `MINIKUBE_DRIVER`, `IMAGE`,
`API_LOCAL_PORT`, `WORKER_LOCAL_PORT`, `NATS_LOCAL_PORT`, and
`SOURCE_TIMEOUT`. The deployment is intentionally local/test-only; its HMAC
secret, ephemeral NATS storage, and port-forwards are not production defaults.
The script automatically selects rootless Podman when the installed `docker`
command is the Podman compatibility wrapper; otherwise it defaults to Docker.
