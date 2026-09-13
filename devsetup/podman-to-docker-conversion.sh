#!/usr/bin/env bash
set -Eeuo pipefail

# e-ubl21 issue #133 helper.
#
# Purpose:
#   Convert a Linux Mint / Ubuntu-derived development workstation from a
#   Podman-provided Docker compatibility command to genuine Docker Engine,
#   while keeping Podman itself available as an independent CLI.
#
# Safety model:
#   - default is dry-run;
#   - --apply is required for any mutation;
#   - the dedicated e-ubl21 Minikube profile is deleted before the driver
#     changes, because Minikube profiles are driver-specific;
#   - Podman itself is preserved;
#   - unexpected apt package removals abort the conversion;
#   - the script never edits shell startup files automatically.
#
# Intended ordering:
#   Complete #121 first, then run this script for #133. If #121 is genuinely
#   blocked by the Podman compatibility layer, #133 may be pulled forward.

PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
APPLY=0
ASSUME_YES=0
ADD_DOCKER_GROUP=1
KEEP_PODMAN_SOCKET=0
DOCKER_VERSION_STRING="${DOCKER_VERSION_STRING:-}"

usage() {
    cat <<'USAGE'
Usage:
  devsetup/podman-to-docker-conversion.sh [options]

Options:
  --apply                 Perform the conversion. Without this flag the script
                          only inventories the host and prints the planned work.
  --yes                   Do not ask for final interactive confirmation.
  --profile NAME          Minikube profile to remove/recreate later.
                          Default: e-ubl21-regression
  --no-docker-group       Do not add the invoking user to the docker group.
                          Repository scripts will then require another access
                          mechanism because they invoke docker without sudo.
  --keep-podman-socket    Leave the user-scoped podman.socket enabled.
  -h, --help              Show this help.

Environment:
  DOCKER_VERSION_STRING   Optional exact docker-ce/docker-ce-cli apt version.
                          When unset, install the current stable version from
                          Docker's official Ubuntu repository.

Examples:
  # Inventory only
  ./devsetup/podman-to-docker-conversion.sh

  # Perform the cutover after #121 is complete
  ./devsetup/podman-to-docker-conversion.sh --apply
USAGE
}

while (($#)); do
    case "$1" in
        --apply)
            APPLY=1
            ;;
        --yes)
            ASSUME_YES=1
            ;;
        --profile)
            shift
            [[ $# -gt 0 ]] || { echo "ERROR: --profile requires a value" >&2; exit 2; }
            PROFILE="$1"
            ;;
        --no-docker-group)
            ADD_DOCKER_GROUP=0
            ;;
        --keep-podman-socket)
            KEEP_PODMAN_SOCKET=1
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown option: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
    shift
done

if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    if [[ -z "${SUDO_USER:-}" || "${SUDO_USER}" == "root" ]]; then
        echo "ERROR: run this script as the normal development user, not as root." >&2
        echo "The script will invoke sudo only for system-level steps." >&2
        exit 2
    fi
    TARGET_USER="$SUDO_USER"
else
    TARGET_USER="${USER:-$(id -un)}"
fi

TARGET_HOME="$(getent passwd "$TARGET_USER" | cut -d: -f6)"
[[ -n "$TARGET_HOME" ]] || { echo "ERROR: could not determine home directory for $TARGET_USER" >&2; exit 2; }

STATE_ROOT="${XDG_STATE_HOME:-$TARGET_HOME/.local/state}/e-ubl21"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_DIR="$STATE_ROOT/podman-to-docker-$STAMP"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/conversion.log"
exec > >(tee -a "$LOG_FILE") 2>&1

say() {
    printf '\n==> %s\n' "$*"
}

quote_cmd() {
    local out=""
    local arg
    for arg in "$@"; do
        printf -v out '%s %q' "$out" "$arg"
    done
    printf '%s\n' "${out# }"
}

run_mutation() {
    printf '+ '
    quote_cmd "$@"
    if (( APPLY )); then
        "$@"
    fi
}

run_root_mutation() {
    printf '+ sudo '
    quote_cmd "$@"
    if (( APPLY )); then
        sudo "$@"
    fi
}

pkg_installed() {
    dpkg-query -W -f='${db:Status-Abbrev}' "$1" 2>/dev/null | grep -q '^ii'
}

record_cmd() {
    local label=$1
    shift
    {
        echo
        echo "--- $label ---"
        "$@" 2>&1 || true
    } | tee -a "$LOG_DIR/inventory.txt"
}

say "Preflight"
for command in bash apt-get dpkg dpkg-query curl grep sed awk systemctl getent; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "ERROR: required command not found: $command" >&2
        exit 2
    }
done

# shellcheck disable=SC1091
source /etc/os-release
UBUNTU_SUITE="${UBUNTU_CODENAME:-}"
if [[ -z "$UBUNTU_SUITE" ]]; then
    echo "ERROR: /etc/os-release does not expose UBUNTU_CODENAME." >&2
    echo "This helper intentionally targets Ubuntu-based Linux Mint." >&2
    echo "For LMDE/Debian, use Docker's Debian repository procedure instead." >&2
    exit 2
fi

case " ${ID:-} ${ID_LIKE:-} " in
    *ubuntu*|*linuxmint*) ;;
    *)
        echo "ERROR: this host does not look like an Ubuntu-derived distribution." >&2
        echo "Detected ID=${ID:-unknown}, ID_LIKE=${ID_LIKE:-unknown}." >&2
        exit 2
        ;;
esac

ARCH="$(dpkg --print-architecture)"
echo "Target user:        $TARGET_USER"
echo "Distribution:       ${PRETTY_NAME:-unknown}"
echo "Ubuntu suite:       $UBUNTU_SUITE"
echo "Architecture:       $ARCH"
echo "Minikube profile:   $PROFILE"
echo "Mode:               $([[ $APPLY -eq 1 ]] && echo APPLY || echo DRY-RUN)"
echo "Evidence directory: $LOG_DIR"

say "Stage 1 - inventory current runtime"
: > "$LOG_DIR/inventory.txt"
record_cmd "docker command" bash -lc 'type -a docker || true; command -v docker || true; readlink -f "$(command -v docker)" 2>/dev/null || true'
record_cmd "docker version" bash -lc 'docker --version || true; docker version || true; docker info || true'
record_cmd "compose" bash -lc 'docker compose version || true; command -v docker-compose || true; docker-compose version || true'
record_cmd "podman" bash -lc 'podman --version || true; podman info || true; podman compose version || true'
record_cmd "packages" bash -lc "dpkg -l | grep -E 'podman|docker|containerd|compose|runc|crun' || true"
record_cmd "podman user socket" systemctl --user status podman.socket
record_cmd "docker service" systemctl status docker
record_cmd "minikube version" bash -lc 'minikube version --short || true'
record_cmd "minikube config" bash -lc 'minikube config view || true'
record_cmd "minikube status" bash -lc "minikube status -p '$PROFILE' || true"
record_cmd "container environment" bash -lc 'env | grep -E "^(DOCKER_HOST|CONTAINER_HOST|MINIKUBE_ROOTLESS|MINIKUBE_DRIVER)=" || true'

say "Checking persistent shell overrides"
SHELL_FILES=(
    "$TARGET_HOME/.bashrc"
    "$TARGET_HOME/.profile"
    "$TARGET_HOME/.bash_profile"
    "$TARGET_HOME/.zshrc"
)
FOUND_SHELL_OVERRIDE=0
for file in "${SHELL_FILES[@]}"; do
    [[ -f "$file" ]] || continue
    if grep -nE '(^|[[:space:]])alias[[:space:]]+docker=|(^|[[:space:]])export[[:space:]]+(DOCKER_HOST|CONTAINER_HOST|MINIKUBE_ROOTLESS|MINIKUBE_DRIVER)=' "$file"; then
        FOUND_SHELL_OVERRIDE=1
        echo "WARNING: container-runtime override found in $file" >&2
    fi
done
if (( FOUND_SHELL_OVERRIDE )); then
    echo "WARNING: the script will not edit shell startup files automatically." >&2
    echo "Remove obsolete Docker-to-Podman aliases/DOCKER_HOST settings before final acceptance." >&2
fi

if (( ! APPLY )); then
    say "Dry-run conversion plan"
    cat <<EOF
1. Delete the disposable Minikube profile '$PROFILE' while the current driver still works.
2. Unset stale persistent Minikube 'driver' and 'rootless' settings.
3. Disable the temporary user podman.socket unless --keep-podman-socket is used.
4. Keep Podman itself installed; install crun first if needed to keep Podman independent of runc.
5. Remove conflicting distro/compatibility packages required by Docker's official Ubuntu packages.
6. Add Docker's official Ubuntu apt repository for '$UBUNTU_SUITE'.
7. Install Docker Engine, CLI, containerd.io, Buildx and the official Compose plugin.
8. Enable Docker and optionally grant '$TARGET_USER' docker-group access.
9. Verify genuine Docker Engine + Compose; Podman remains separately available.
10. Log out/in, recreate Minikube with the Docker driver, rerun #121, then run ./run-regression-tests.sh.
EOF
    echo
    echo "No system changes were made. Re-run with --apply when the #133 cutover window begins."
    exit 0
fi

say "Final confirmation"
echo "This operation changes host container-runtime packages and deletes only the"
echo "dedicated disposable Minikube profile '$PROFILE'. Podman itself is preserved."
echo
if (( ! ASSUME_YES )); then
    read -r -p "Proceed with issue #133 runtime cutover? [y/N] " answer
    case "$answer" in
        y|Y|yes|YES) ;;
        *) echo "Cancelled."; exit 1 ;;
    esac
fi

sudo -v

say "Stage 2 - remove the old Minikube profile before changing drivers"
if command -v minikube >/dev/null 2>&1; then
    PROFILE_DIR="$TARGET_HOME/.minikube/profiles/$PROFILE"
    if [[ -d "$PROFILE_DIR" ]]; then
        echo "Deleting existing driver-specific Minikube profile: $PROFILE"
        if ! minikube delete -p "$PROFILE"; then
            echo "ERROR: failed to delete the existing Minikube profile." >&2
            echo "Restore the current Podman socket/runtime if necessary, delete the profile, then retry." >&2
            exit 1
        fi
    else
        echo "Minikube profile '$PROFILE' is not present; nothing to delete."
    fi

    # A persisted Podman rootless setting breaks an ordinary rootful Docker
    # driver. The repository passes its driver explicitly, so stale global
    # settings should be cleared rather than replaced with new global defaults.
    minikube config unset rootless >/dev/null 2>&1 || true
    minikube config unset driver >/dev/null 2>&1 || true
fi

say "Stage 3 - retire temporary Podman Docker-API compatibility"
if (( ! KEEP_PODMAN_SOCKET )); then
    systemctl --user disable --now podman.socket >/dev/null 2>&1 || true
    echo "User podman.socket disabled if it existed. Podman CLI remains installed."
else
    echo "Keeping user podman.socket enabled by request."
fi

# Variables inherited by this process must not redirect the new Docker CLI to
# a Podman API socket. Shell startup files are only reported, never edited.
unset DOCKER_HOST CONTAINER_HOST MINIKUBE_ROOTLESS MINIKUBE_DRIVER || true

say "Stage 4 - prepare Podman to survive Docker's runc package replacement"
if pkg_installed podman && ! pkg_installed crun; then
    echo "Podman is installed but crun is not. Installing crun before removing the distro runc package."
    run_root_mutation apt-get update
    run_root_mutation apt-get install -y crun
fi

say "Stage 5 - remove packages that conflict with official Docker Engine"
CONFLICTING_PACKAGES=(
    docker.io
    docker-compose
    docker-compose-v2
    docker-doc
    docker-buildx
    podman-docker
    containerd
    runc
)
INSTALLED_CONFLICTS=()
for package in "${CONFLICTING_PACKAGES[@]}"; do
    if pkg_installed "$package"; then
        INSTALLED_CONFLICTS+=("$package")
    fi
done

if (( ${#INSTALLED_CONFLICTS[@]} > 0 )); then
    echo "Installed conflicting packages: ${INSTALLED_CONFLICTS[*]}"
    APT_SIMULATION="$LOG_DIR/apt-remove-simulation.txt"
    apt-get -s remove "${INSTALLED_CONFLICTS[@]}" | tee "$APT_SIMULATION"

    mapfile -t PLANNED_REMOVALS < <(awk '/^Remv / {print $2}' "$APT_SIMULATION")
    for package in "${PLANNED_REMOVALS[@]}"; do
        allowed=0
        for expected in "${CONFLICTING_PACKAGES[@]}"; do
            if [[ "$package" == "$expected" ]]; then
                allowed=1
                break
            fi
        done
        if (( ! allowed )); then
            echo "ERROR: apt plans to remove unexpected package '$package'." >&2
            echo "Aborting before package removal. Inspect $APT_SIMULATION." >&2
            exit 1
        fi
    done

    run_root_mutation apt-get remove -y "${INSTALLED_CONFLICTS[@]}"
else
    echo "No conflicting apt packages are installed."
fi

say "Stage 6 - configure Docker's official Ubuntu apt repository"
run_root_mutation apt-get update
run_root_mutation apt-get install -y ca-certificates curl

RELEASE_URL="https://download.docker.com/linux/ubuntu/dists/$UBUNTU_SUITE/Release"
if ! curl -fsSL "$RELEASE_URL" -o "$LOG_DIR/docker-repo-release"; then
    echo "ERROR: Docker's Ubuntu repository has no Release metadata for '$UBUNTU_SUITE'." >&2
    echo "Do not substitute the Linux Mint codename; verify the Ubuntu base release." >&2
    exit 1
fi

TMP_KEY="$(mktemp)"
trap 'rm -f "$TMP_KEY"' EXIT
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o "$TMP_KEY"
run_root_mutation install -m 0755 -d /etc/apt/keyrings
run_root_mutation install -m 0644 "$TMP_KEY" /etc/apt/keyrings/docker.asc

DOCKER_SOURCES="$LOG_DIR/docker.sources"
cat > "$DOCKER_SOURCES" <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $UBUNTU_SUITE
Components: stable
Architectures: $ARCH
Signed-By: /etc/apt/keyrings/docker.asc
EOF
run_root_mutation install -m 0644 "$DOCKER_SOURCES" /etc/apt/sources.list.d/docker.sources
run_root_mutation apt-get update

say "Stage 7 - install genuine Docker Engine and Compose"
if [[ -n "$DOCKER_VERSION_STRING" ]]; then
    run_root_mutation apt-get install -y \
        "docker-ce=$DOCKER_VERSION_STRING" \
        "docker-ce-cli=$DOCKER_VERSION_STRING" \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin
else
    run_root_mutation apt-get install -y \
        docker-ce \
        docker-ce-cli \
        containerd.io \
        docker-buildx-plugin \
        docker-compose-plugin
fi
run_root_mutation systemctl enable --now docker

say "Stage 8 - configure developer access"
if (( ADD_DOCKER_GROUP )); then
    echo "WARNING: membership of the docker group grants root-level control through the Docker daemon."
    run_root_mutation groupadd -f docker
    run_root_mutation usermod -aG docker "$TARGET_USER"
    echo "User '$TARGET_USER' added to docker group. A full logout/login is required for the desktop session to inherit it."
else
    echo "Docker group membership was skipped. Repository scripts invoke docker without sudo, so configure another compatible access model before regression."
fi

say "Stage 9 - verify the installed runtime"
hash -r || true
DOCKER_PATH="$(command -v docker || true)"
if [[ -z "$DOCKER_PATH" ]]; then
    echo "ERROR: docker command not found after installation." >&2
    exit 1
fi

echo "docker path: $DOCKER_PATH"
DOCKER_CLIENT_VERSION="$(env -u DOCKER_HOST -u CONTAINER_HOST docker --version 2>&1 || true)"
echo "$DOCKER_CLIENT_VERSION"
if [[ "${DOCKER_CLIENT_VERSION,,}" == *podman* ]]; then
    echo "ERROR: docker still resolves to Podman compatibility." >&2
    echo "Check shell aliases, /usr/local/bin, and package ownership before continuing." >&2
    exit 1
fi

sudo env -u DOCKER_HOST -u CONTAINER_HOST docker version
sudo env -u DOCKER_HOST -u CONTAINER_HOST docker compose version
sudo env -u DOCKER_HOST -u CONTAINER_HOST docker run --rm hello-world

if command -v podman >/dev/null 2>&1; then
    echo
    echo "Independent Podman remains available:"
    podman --version || true
fi

if (( ADD_DOCKER_GROUP )) && command -v sg >/dev/null 2>&1; then
    echo
    echo "Verifying Docker socket access using the newly assigned docker group:"
    sudo -u "$TARGET_USER" -H sg docker -c 'env -u DOCKER_HOST -u CONTAINER_HOST docker info >/dev/null && docker compose version'
fi

say "Conversion package phase complete"
cat <<EOF
Evidence: $LOG_DIR

Required post-cutover stages:
  1. Remove any obsolete docker alias, DOCKER_HOST/CONTAINER_HOST, or
     MINIKUBE_ROOTLESS/MINIKUBE_DRIVER export reported from shell startup files.
  2. Log out of the desktop session and log back in so docker-group membership
     is applied (unless you deliberately chose another access model).
  3. Verify:
       docker version
       docker compose version
       docker info
  4. Recreate the disposable cluster with the Docker driver:
       minikube start -p '$PROFILE' --driver=docker
     The repository normally does this automatically through
     devsetup/regression-minikube.sh.
  5. Rerun the complete #121 YugabyteDB lifecycle under genuine Docker.
  6. Run the authoritative local gate:
       ./run-regression-tests.sh

Do not migrate Podman images/containers/volumes unless a specific non-disposable
asset has been identified. Docker and Podman use separate local storage.
EOF
