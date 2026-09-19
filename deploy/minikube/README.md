# Local Minikube regression deployment

This Kustomize base deploys the current distributed runtime into the dedicated
`e-ubl21-regression` namespace:

- one ephemeral, namespace-isolated NATS JetStream deployment;
- one pinned single-node YugabyteDB deployment with authenticated YSQL and
  namespace-owned persistent volumes;
- one authenticated API deployment;
- one validation-worker deployment using the pinned EN 16931 runtime in the
  candidate application image.

The base targets a single-node local cluster and uses host networking with
non-conflicting NATS/API/worker ports. This avoids broken ClusterIP forwarding
in rootless Podman Minikube while preserving separate Kubernetes Deployments,
namespace isolation, probes, events, logs, and rollout behavior. It is not a
production networking model and must not be reused as one. The application
Deployments use the `Recreate` strategy because two generations cannot reserve
the same host ports concurrently on the single node.

Run the complete lifecycle from the repository root:

```bash
make regression-minikube
```

The command builds and loads `localhost/e-ubl21:regression`, creates a
short-lived local JWT signing secret, installs the checksum-pinned YugabyteDB
chart, bootstraps the `eubl21` identity/database, applies embedded schema
migrations, runs a live YSQL/schema smoke check, restarts the tserver, and
repeats migration and smoke verification before starting the application. It
then forces the API and worker Deployments to restart with the newly loaded
image, waits for all rollouts and health endpoints, invokes the same `e-ubl21
regression` semantic runner used outside Kubernetes, collects the #40 evidence
bundle, and deletes the namespace. Namespace deletion also removes the
YugabyteDB PVCs so each sequential corpus starts from zero state. On a
post-deployment infrastructure failure it first writes best-effort Kubernetes
events, pod state, and logs to `failure-evidence/` below the artifact directory.
It leaves the Minikube profile running for faster subsequent runs.

Artifacts are written to a timestamped directory below
`regression-results/minikube/`. Override the defaults when needed:

```bash
make regression-minikube \
  MANIFEST=createsamples/samples/manifest.json \
  PROFILE=ubl21 \
  ARTIFACT_DIR=regression-results/minikube/ubl21
```

Each run writes `database/summary.json` with the live YugabyteDB version,
bootstrap/migration result, application schema version, smoke result, and
tserver restart result. Bootstrap, migration, and smoke logs are retained in
the same directory. The comprehensive `./run-regression-tests.sh` report embeds
these summaries and includes Minikube database startup failures in its normal
failure-log tails.

Prerequisites are `minikube`, genuine Docker Engine, Go, Helm, `curl`,
`sha256sum`, `setsid`, and GNU `timeout`
(normally provided by `util-linux`). Docker setup and verification are in
[`docs/DOCKER-DEVELOPMENT.md`](../../docs/DOCKER-DEVELOPMENT.md). A standalone `kubectl` is not required:
`devsetup/minikube-kubectl.sh` uses Minikube's pinned kubectl.
Set `KEEP_DEPLOYMENT=1` to retain the namespace for interactive debugging, or
run `make regression-minikube-clean` after an interrupted/retained run.

Useful overrides include `MINIKUBE_PROFILE`, `MINIKUBE_DRIVER`, `IMAGE`,
`API_LOCAL_PORT`, `WORKER_LOCAL_PORT`, `NATS_LOCAL_PORT`, and
`SOURCE_TIMEOUT`. The deployment is intentionally local/test-only; its HMAC
secret, fixed disposable database credential, reduced YugabyteDB resources,
ephemeral NATS storage, and port-forwards are not production defaults.
The script defaults to the Docker driver. Independent Podman testing remains
available by explicitly setting `MINIKUBE_DRIVER=podman`; Podman must be invoked
as itself and must not replace the `docker` command.

## GitHub Actions

`.github/workflows/regression-minikube.yml` runs this same
`make regression-minikube` lifecycle for pull requests and pushes to `main`.
UBL 2.1 and EN 16931 use separate matrix jobs so both deployed corpora run in
parallel without sharing a cluster or namespace. Each job installs a
checksum-pinned Minikube binary, starts a disposable Docker-driver cluster,
uploads semantic and diagnostic artifacts on success or failure, and always
deletes the complete Minikube profile afterward. Failed fixture IDs are also
written to the GitHub job summary.
