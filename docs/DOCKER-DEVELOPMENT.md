# Docker Engine development runtime

Local container workflows use a genuine Docker Engine, the Docker CLI, Buildx,
and the official Docker Compose v2 plugin. Podman may remain installed and be
used through the `podman` command, but `docker` must not be the `podman-docker`
compatibility wrapper.

## Linux Mint setup

The repository installer follows Docker's current Ubuntu apt-repository
procedure. Linux Mint 22.3 has an Ubuntu Noble base; Docker notes that Ubuntu
derivatives are not officially supported, although this package path works in
practice. Review Docker's [Ubuntu installation
guide](https://docs.docker.com/engine/install/ubuntu/) before running it if the
host distribution or base release has changed.

```bash
sudo ./devsetup/install-docker-engine.sh
```

The installer:

- records command ownership, relevant package versions, and service state in a
  mode-`0600` `/var/tmp/eubl21-docker-conversion-before-*.log` file;
- removes the conflicting `podman-docker`, legacy Compose, distro Docker,
  `containerd`, and `runc` packages only when installed;
- installs Docker Engine, CLI, containerd, Buildx, and Compose from Docker's
  Noble repository and enables their system services;
- adds the invoking developer to the `docker` group;
- leaves Podman, Podman containers, images, volumes, and Minikube profiles in
  place.

Do not run `apt autoremove` as part of this conversion: on a host where Podman
was installed as an automatic dependency, apt may offer to remove it.

Membership in the `docker` group gives root-level control of the host through
the Docker daemon. This is the selected access policy for the development
workstation. Log out and back in after installation so the new membership is
active. See Docker's [Linux post-installation
guide](https://docs.docker.com/engine/install/linux-postinstall/) for the
security implications and rootless alternatives.

## Verification

After starting a new login session, run:

```bash
docker --version
docker version
docker compose version
podman --version || true
make docker-verify
make database-verify
```

The verification script uses disposable names and checks an image pull,
container execution, a named network and volume, a bind mount, a health check,
and Compose `up`, `ps`, `logs`, `restart`, `down`, and volume cleanup. The
database target then checks the complete persistent YugabyteDB lifecycle.

Repository commands reject a missing daemon, the Podman compatibility wrapper,
and legacy/external Compose providers with a concise setup pointer.

## Rollback

The pre-conversion log contains the exact prior package versions and service
state. To restore command compatibility while preserving both engines' stored
data, first stop and remove the Docker packages and repository:

```bash
sudo systemctl disable --now docker.service containerd.service
sudo apt-get purge docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras
sudo rm /etc/apt/sources.list.d/docker.sources /etc/apt/keyrings/docker.asc
sudo apt-get update
```

Then reinstall the prior packages recorded in the inventory (the original
Mint setup used the following) and remove Docker socket membership if desired:

```bash
sudo apt-get install podman podman-docker docker-compose runc
sudo gpasswd --delete "$USER" docker
```

Do not delete `/var/lib/docker` or the user's Podman storage during rollback;
package removal does not require deleting either engine's containers, images,
or volumes.
