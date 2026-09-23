# Podman to Docker conversion

## Tracking

GitHub issue: #133 — `PRIORITY — Convert local development runtime from Podman compatibility to Docker Engine`

Priority: high, but scheduled **immediately after #121** unless the current Podman environment becomes a hard blocker for #121.

## Why this issue exists

The first real-runtime work for #121 exposed an environment limitation rather than a repository defect. On the current Linux Mint development machine, the `docker` command is provided through Podman compatibility, while the external Compose provider expects a Docker-compatible API socket. A temporary user-scoped Podman API socket can unblock the current work, but this adds an avoidable compatibility layer to the normal developer workflow.

The preferred local development arrangement is therefore:

```text
docker          -> genuine Docker Engine CLI
docker compose  -> official Docker Compose plugin
podman          -> optional independent Podman CLI
```

Podman does not need to be removed unless a later reason requires it. The important change is that `docker` must stop resolving to the Podman compatibility layer.

## Why the conversion is scheduled after #121

Changing the workstation container runtime while #121 is still being implemented would mix two independent variables:

1. the new YugabyteDB local-runtime implementation;
2. the developer-machine container-runtime conversion.

Completing #121 first gives us a concrete, tested lifecycle and exposes the exact Compose/runtime assumptions that the repository actually needs. The Docker conversion can then be validated against that known lifecycle.

This ordering also makes #121 a useful post-conversion acceptance test.

Exception: if the current Podman setup becomes a hard blocker that prevents #121 from completing, #133 may be promoted and handled first.

## Required inventory before conversion

Record the current state before changing packages or services:

```bash
which docker
readlink -f "$(command -v docker)"
docker --version || true
podman --version || true
docker compose version || true
podman compose version || true

dpkg -l | grep -E 'podman|docker|containerd|compose|runc' || true
systemctl --user status podman.socket || true
systemctl status docker || true

env | grep -E 'DOCKER_HOST|CONTAINER_HOST|PODMAN' || true
```

Also identify:

- shell aliases/functions named `docker`;
- `podman-docker` or equivalent compatibility packages;
- external Compose providers;
- user-scoped Podman API socket configuration;
- repository scripts that assume `/var/run/docker.sock`;
- Minikube driver/runtime assumptions;
- local images/volumes that are actually valuable rather than disposable.

## Conversion strategy

### 1. Preserve evidence and rollback information

Before changing the machine:

- record installed package versions;
- record current Podman configuration;
- identify non-disposable local volumes/data;
- record any Podman socket/user-service overrides created for #121;
- retain the commands required to restore the old setup if necessary.

### 2. Remove only the Docker-to-Podman compatibility layer

The conversion should normally remove or disable the component that causes `docker` to invoke Podman, such as `podman-docker` or a shell alias.

Do **not** remove Podman itself merely to make Docker work. Keeping both tools available independently is acceptable and useful.

### 3. Install genuine Docker tooling

Install and configure the supported Docker stack for the machine's Linux Mint / Ubuntu base:

- Docker Engine;
- Docker CLI;
- containerd as required by the Docker packaging;
- Docker Buildx plugin;
- official Docker Compose plugin.

Use the current official Docker installation guidance at implementation time rather than freezing potentially stale package-repository commands in this planning document.

### 4. Configure service and access policy

Verify the Docker daemon starts reliably and choose an explicit developer access model.

Possible models include:

- invoking Docker through `sudo`;
- granting the development user access through the `docker` group.

The selected approach must be documented, including the security implications of Docker daemon access.

### 5. Verify command ownership

After conversion:

```bash
which docker
docker --version
docker version
docker compose version
podman --version || true
```

`docker version` must show communication with a genuine Docker Engine daemon. `docker compose` must be the official Docker Compose plugin rather than an external Podman provider.

## Functional verification

Before testing e-ubl21, verify basic Docker behavior:

- pull an image;
- run and remove a container;
- create/use/remove a named volume;
- create/use/remove a network;
- exercise a bind mount;
- run a simple health check;
- run a small Compose project through `up`, `ps`, `logs`, `restart`, `down`, and volume cleanup.

## Repository verification

After Docker itself is healthy, rerun the complete #121 local YugabyteDB lifecycle using the repository wrapper rather than ad-hoc commands.

Required lifecycle:

```text
start YugabyteDB
wait for YSQL readiness
bootstrap eubl21 database/user
run smoke query
stop
restart with persistent data
clean persistent state
```

Then run the verification required by the repository and by #121, including the applicable unit/integration tests and local regression gate.

The Docker migration is not accepted merely because `docker run hello-world` works. The real acceptance criterion is that the e-ubl21 local runtime works normally without the temporary Podman API socket workaround.

## Repository changes allowed by #133

The issue may update developer tooling/documentation where necessary to make prerequisites and errors clear, including:

- concise detection of missing Docker Engine/Compose prerequisites;
- clear distinction between genuine Docker and Podman compatibility;
- removal of temporary Podman-socket instructions from the normal path after Docker succeeds;
- optional notes describing how Podman can still be used independently.

Avoid Docker-specific application coupling. The application and production architecture should remain container-runtime neutral unless another issue explicitly changes that contract.

## Non-goals

This issue does not automatically:

- change production Kubernetes/container runtime;
- replace Minikube architecture unless required for compatibility;
- migrate disposable Podman containers/images/volumes into Docker;
- remove Podman from the workstation;
- introduce Docker-specific APIs into application code.

## Acceptance criteria

The issue is complete when all of the following are true:

- `docker` resolves to genuine Docker tooling rather than Podman compatibility;
- Docker Engine is running through a documented, reproducible setup;
- `docker compose` uses the official Compose plugin;
- Podman may remain installed independently without hijacking the `docker` command;
- the complete #121 YugabyteDB local lifecycle passes under Docker;
- relevant repository tests and regression gates pass;
- normal contributor instructions no longer depend on a temporary user-scoped Podman API socket;
- prerequisite failures are concise and actionable rather than exposing provider/Python tracebacks;
- rollback notes exist.

## Execution order

```text
#121  Complete reproducible YugabyteDB local runtime
  |
  v
#133  Convert local workstation from Podman compatibility to Docker Engine
  |
  v
rerun #121 lifecycle under Docker
  |
  v
#122  Continue YugabyteDB migration work
```

This issue follows the repository workflow in `ISSUES.MD`: implementation on its own issue branch, PR review, explicit closure authorization, and one squash commit on `development` when completed.
