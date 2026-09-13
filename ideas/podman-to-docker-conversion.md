# Podman to Docker conversion

## Tracking

GitHub issue: #133 — `PRIORITY — Convert local development runtime from Podman compatibility to Docker Engine`

Priority: high, scheduled **immediately after #121** unless the current Podman environment becomes a hard blocker for #121.

Preparation branch: `issue/133-podman-to-docker-conversion`

Conversion helper:

```text
devsetup/podman-to-docker-conversion.sh
```

The helper defaults to dry-run. The machine-changing phase requires `--apply`.

## Why this issue exists

The first real-runtime work for #121 exposed an environment limitation rather than a repository defect. On the current Linux Mint development machine, the `docker` command is provided through Podman compatibility, while the external Compose provider expects a Docker-compatible API socket. A temporary user-scoped Podman API socket can unblock the current work, but this adds an avoidable compatibility layer to the normal developer workflow.

The preferred local development arrangement is:

```text
docker          -> genuine Docker Engine CLI
docker compose  -> official Docker Compose plugin
podman          -> optional independent Podman CLI
```

Podman itself does not need to be removed. The important change is that `docker` must stop resolving to Podman compatibility.

## Why the conversion is scheduled after #121

Changing the workstation container runtime while #121 is still being implemented would mix two independent variables:

1. the new YugabyteDB local-runtime implementation;
2. the developer-machine container-runtime conversion.

Completing #121 first gives us a concrete, tested lifecycle and exposes the exact Compose/runtime assumptions that the repository actually needs. The Docker conversion can then be validated against that known lifecycle. #121 therefore becomes a post-conversion acceptance test.

Exception: if the current Podman setup becomes a hard blocker that prevents #121 from completing, #133 may be promoted and handled first.

# Migration stages

## Stage 0 — finish and freeze #121 behavior

**No package removal or Docker installation occurs in this stage.**

Before the normal #133 cutover:

- complete #121 under the current Podman workaround if possible;
- retain its exact-candidate test/regression evidence;
- record the final YugabyteDB Compose lifecycle and prerequisites;
- identify any temporary Podman API socket instructions introduced solely to unblock #121.

If #121 cannot finish because Podman compatibility is itself the blocker, record that fact and move directly to Stage 1 under the exception defined above.

## Stage 1 — inventory and rollback capture

Run the helper without `--apply`:

```bash
devsetup/podman-to-docker-conversion.sh
```

It records:

- the current `docker` command/provider;
- Podman and Compose versions;
- installed Podman/Docker/containerd/runc/crun packages;
- Podman socket and Docker service status;
- Minikube version, configuration and profile state;
- `DOCKER_HOST`, `CONTAINER_HOST`, `MINIKUBE_ROOTLESS` and `MINIKUBE_DRIVER` environment overrides;
- likely persistent shell aliases/exports.

Identify any non-disposable Podman images, containers or volumes before proceeding. The default assumption for e-ubl21 development infrastructure is that these assets are disposable and will be recreated.

## Stage 2 — remove the old Minikube profile while Podman still works

**This must happen before Podman compatibility packages/sockets are removed.**

The `e-ubl21-regression` profile is driver-specific. An existing profile created with the Podman driver must not be reused as a Docker-driver cluster.

The conversion helper deletes the dedicated profile before changing packages:

```bash
minikube delete -p e-ubl21-regression
```

It also clears stale global Minikube `driver` and `rootless` properties rather than replacing them with a new global default. Repository scripts already choose the appropriate driver explicitly/automatically.

This deletion affects only the disposable local Minikube cluster. It does not modify repository Kubernetes manifests.

## Stage 3 — actual Podman-to-Docker package cutover

**This is the stage where installation/removal must be performed.**

Run:

```bash
devsetup/podman-to-docker-conversion.sh --apply
```

The helper performs the following guarded sequence:

1. optionally disables the temporary user-scoped `podman.socket`;
2. preserves the `podman` package itself;
3. installs `crun` first if needed so Podman remains independent when the distro `runc` package is removed;
4. simulates APT removal and aborts if unrelated packages would be removed;
5. removes Docker-conflicting/compatibility packages when installed:
   - `docker.io`;
   - `docker-compose`;
   - `docker-compose-v2`;
   - `docker-doc`;
   - `docker-buildx`;
   - `podman-docker`;
   - distro `containerd`;
   - distro `runc`;
6. configures Docker's official Ubuntu APT repository using the Linux Mint machine's `UBUNTU_CODENAME`;
7. installs:
   - `docker-ce`;
   - `docker-ce-cli`;
   - `containerd.io`;
   - `docker-buildx-plugin`;
   - `docker-compose-plugin`;
8. enables and starts Docker Engine;
9. by default adds the development user to the `docker` group so the repository's unsudoed `docker` commands work;
10. verifies genuine Docker Engine and the official Compose plugin;
11. verifies that Podman remains separately available when installed.

The helper deliberately does not edit `.bashrc`, `.profile` or equivalent startup files. Any obsolete `docker` alias, Podman `DOCKER_HOST`, or persistent rootless/driver environment export is reported for manual cleanup.

### Docker-group security note

The repository currently invokes Docker without `sudo`, including the regression scripts. The simplest compatible workstation setup is therefore membership in the `docker` group. That grants root-level control through the Docker daemon and must be treated accordingly.

The helper supports `--no-docker-group` if another access model is deliberately selected, but repository commands must then be adapted or invoked from an authorized Docker-enabled shell.

## Stage 4 — login/session refresh and Docker verification

After Stage 3, fully log out and log back in so new group membership is reflected in the desktop session.

Verify:

```bash
docker --version
docker version
docker info
docker compose version
podman --version
```

`docker version` must talk to Docker Engine and must not be redirected to a Podman API socket.

Any persistent `DOCKER_HOST`/`CONTAINER_HOST` or shell alias discovered in Stage 1 must be removed before this stage can pass.

## Stage 5 — recreate Minikube with Docker

The first post-conversion Minikube run must create a fresh profile using the Docker driver:

```bash
minikube start -p e-ubl21-regression --driver=docker
```

Normally `devsetup/regression-minikube.sh` will do this automatically. It currently inspects `docker --version`: when the command identifies Podman it selects `podman`; otherwise it selects `docker`.

Do not set `MINIKUBE_ROOTLESS=true` for the normal rootful Docker Engine configuration. Any stale persistent Minikube rootless setting must remain unset.

## Stage 6 — rerun the complete #121 lifecycle under Docker

Run the repository's #121 lifecycle, not an ad-hoc Docker container:

```text
start YugabyteDB
wait for YSQL readiness
bootstrap eubl21 database/user
run smoke query
stop
restart with persistent data
clean persistent state
```

This is the principal functional proof that Docker replaces the temporary Podman compatibility workaround without changing the intended local database contract.

## Stage 7 — run the authoritative local regression gate

Run:

```bash
./run-regression-tests.sh
```

The complete gate must pass under Docker before #133 can be considered complete.

The first run after conversion may take longer because Docker has a fresh image store and Minikube profile. That is expected and is not a semantic regression.

## Stage 8 — remove temporary workaround documentation

Only after Stages 6 and 7 pass:

- remove temporary Podman-socket instructions from the normal #121 path;
- retain any useful optional Podman notes separately;
- update prerequisite diagnostics so missing Docker Engine/Compose produces concise actionable errors;
- record the verified Docker/Compose/Minikube versions in issue evidence.

# Minikube and Kubernetes impact

## Minikube: expected impact

There **is** a Minikube transition, but it is contained and predictable.

The current regression script already supports both runtimes:

```text
Podman-compatible docker command -> MINIKUBE_DRIVER=podman
real Docker command              -> MINIKUBE_DRIVER=docker
```

Therefore the script logic should naturally switch to Docker after conversion.

Foreseen Minikube difficulties:

1. **Old profile cannot be carried across drivers.** Delete and recreate `e-ubl21-regression` as described above.
2. **Stale rootless configuration can break Docker-driver startup.** Clear persistent `rootless=true` and stale environment exports.
3. **Fresh image cache.** Docker and Podman use separate image storage, so the first Docker regression run rebuilds/pulls images again.
4. **Docker socket permissions.** Minikube and the regression scripts must be able to use Docker without `sudo` under the chosen developer-access model.
5. **Firewall/network behavior.** Docker manages host packet-filtering rules. A host with custom UFW/nftables policy should be checked after installation.
6. **Disk usage temporarily increases.** Keeping both Podman and Docker means two independent image/container stores can coexist until unused Podman data is deliberately pruned.

Minikube's Docker driver is a normal supported Linux path and is preferable here to relying on its experimental Podman driver.

## Kubernetes repository manifests: no required semantic change

The switch is a **host development-runtime change**, not a Kubernetes API or workload-model change.

No change is expected for production/on-prem Kubernetes manifests merely because Docker replaces Podman on this workstation. Kubernetes resources such as:

- Deployments;
- Services;
- Secrets;
- ConfigMaps;
- probes;
- namespaces;
- Kustomize configuration;

are interpreted by Kubernetes, not by the workstation's Docker/Podman CLI.

### Minikube-specific `hostNetwork` workaround

The current `deploy/minikube/` manifests use `hostNetwork: true` and `ClusterFirstWithHostNet` because rootless Podman Minikube previously had broken ClusterIP forwarding in this environment.

Those fields are valid with Docker and can remain during #133. Removing them during the runtime migration would change two things at once and make regression diagnosis harder.

After #133 is accepted, a separate cleanup may test whether ordinary ClusterIP networking works reliably with the Docker driver and whether the Minikube-only `hostNetwork` workaround can be removed. That cleanup is optional and should not block the conversion.

### Local kubeconfig

Deleting/recreating the Minikube profile will update the user's local `~/.kube/config` context/certificates for that profile. This is expected local state. It does **not** change repository Kubernetes YAML.

# Regression-test impact

## Expected code/test behavior

The semantic regression corpus and expected validation results should be unaffected. The container runtime is infrastructure below the validation contract.

The root `run-regression-tests.sh` currently:

- requires a `docker` command;
- calls `docker info` as a prerequisite check;
- records `docker --version` in its report;
- runs two Minikube suites through `make regression-minikube`.

`devsetup/regression-minikube.sh`:

- detects whether `docker --version` is Podman compatibility or Docker;
- selects the corresponding Minikube driver;
- builds the candidate image with `docker build`;
- transfers it into Minikube with `docker save | minikube image load -`;
- later inspects the Docker image ID for evidence.

With genuine Docker Engine these commands remain valid. In fact, the current `docker info` prerequisite becomes simpler because it no longer depends on a temporary Podman API socket.

## Required regression treatment

The conversion **does affect the execution environment**, so a complete post-conversion regression run is mandatory even though no semantic change is expected.

Expected differences in evidence are infrastructural only:

- Docker version/provider changes;
- Minikube driver changes from Podman to Docker;
- image IDs/digests may change because images are rebuilt in a different local store;
- first-run timing may be slower due to fresh pulls/build/cache;
- Kubernetes node/container-runtime metadata may differ.

These must not change normalized UBL/EN/Peppol semantic outcomes.

A failed semantic fixture after the conversion is a blocker and must not be dismissed as a runtime migration side effect.

# Rollback principle

If Docker installation or post-cutover verification fails:

1. do not continue into #122;
2. retain the conversion evidence/log;
3. restore any required Podman Docker-compatibility package/socket only as necessary to recover the previous developer environment;
4. recreate the Minikube profile with the previous driver if regression work must resume before the Docker problem is fixed;
5. do not delete valuable Podman data during rollback investigation.

# Acceptance criteria

The issue is complete when all of the following are true:

- `docker` resolves to genuine Docker tooling rather than Podman compatibility;
- Docker Engine is running through a documented, reproducible setup;
- `docker compose` uses the official Compose plugin;
- Podman may remain installed independently without hijacking the `docker` command;
- the old Podman-driver Minikube profile has been replaced by a Docker-driver profile;
- stale rootless/Podman Docker-host configuration has been removed;
- the complete #121 YugabyteDB local lifecycle passes under Docker;
- `./run-regression-tests.sh` passes under Docker;
- normal contributor instructions no longer depend on a temporary user-scoped Podman API socket;
- prerequisite failures are concise and actionable rather than exposing provider/Python tracebacks;
- rollback evidence/instructions exist.

## Execution order

```text
#121  Complete reproducible YugabyteDB local runtime
  |
  v
#133 Stage 1/2  Inventory + delete old Podman-driver Minikube profile
  |
  v
#133 Stage 3/4  Remove compatibility packages + install/verify Docker
  |
  v
#133 Stage 5    Recreate Minikube using Docker driver
  |
  v
#133 Stage 6    Rerun #121 lifecycle under Docker
  |
  v
#133 Stage 7    Full ./run-regression-tests.sh
  |
  v
#133 Stage 8    Remove temporary workaround docs
  |
  v
#122  Continue YugabyteDB migration work
```

This issue follows the repository workflow in `ISSUES.MD`: implementation on its own issue branch, PR review, explicit closure authorization, and one squash commit on `development` when completed.
