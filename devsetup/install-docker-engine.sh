#!/usr/bin/env bash
set -Eeuo pipefail

if (( EUID != 0 )); then
    echo "ERROR: run this installer as root: sudo ./devsetup/install-docker-engine.sh" >&2
    exit 2
fi

source /etc/os-release
ubuntu_codename="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
case "$ubuntu_codename" in
    noble|jammy|resolute) ;;
    *)
        echo "ERROR: unsupported Ubuntu base codename: ${ubuntu_codename:-unknown}" >&2
        exit 2
        ;;
esac

if [[ "${ID:-}" != ubuntu ]]; then
    echo "WARNING: Docker does not officially support Ubuntu derivatives; using the $ubuntu_codename Ubuntu repository for ${PRETTY_NAME:-this host}." >&2
fi

access_user="${DOCKER_ACCESS_USER:-${SUDO_USER:-}}"
if [[ -z "$access_user" || "$access_user" == root ]] || ! id "$access_user" >/dev/null 2>&1; then
    echo "ERROR: cannot determine the non-root developer account; set DOCKER_ACCESS_USER." >&2
    exit 2
fi

inventory_file="${DOCKER_CONVERSION_LOG:-/var/tmp/eubl21-docker-conversion-before-$(date -u +%Y%m%dT%H%M%SZ).log}"
{
    echo "Recorded: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "Developer: $access_user"
    echo "OS: ${PRETTY_NAME:-unknown} (Ubuntu base: $ubuntu_codename)"
    command -v docker || true
    readlink -f "$(command -v docker)" 2>/dev/null || true
    docker --version 2>&1 || true
    podman --version 2>&1 || true
    docker compose version 2>&1 || true
    podman ps --all --format 'container\t{{.Names}}\t{{.Status}}\t{{.Image}}' 2>&1 || true
    podman images --format 'image\t{{.Repository}}:{{.Tag}}\t{{.ID}}\t{{.Size}}' 2>&1 || true
    podman volume ls 2>&1 || true
    dpkg-query -W -f='${binary:Package}\t${Version}\n' \
        docker.io docker-compose docker-compose-v2 docker-doc docker-buildx \
        podman podman-docker containerd runc 2>/dev/null || true
    systemctl is-enabled docker 2>&1 || true
    systemctl is-active docker 2>&1 || true
    systemctl --user --machine="$access_user@" is-enabled podman.socket 2>&1 || true
    runuser --user "$access_user" -- env | grep -E '^(DOCKER_HOST|CONTAINER_HOST|PODMAN)' || true
} >"$inventory_file"
chmod 0600 "$inventory_file"
echo "Pre-conversion inventory: $inventory_file"

conflicts=()
for package in docker.io docker-compose docker-compose-v2 docker-doc docker-buildx podman-docker containerd runc; do
    if dpkg-query -W -f='${db:Status-Abbrev}' "$package" 2>/dev/null | grep -q '^ii'; then
        conflicts+=("$package")
    fi
done
if (( ${#conflicts[@]} > 0 )); then
    apt-get remove -y "${conflicts[@]}"
fi

apt-get update
apt-get install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl --fail --location --silent --show-error \
    https://download.docker.com/linux/ubuntu/gpg \
    --output /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

cat >/etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $ubuntu_codename
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker.service containerd.service
usermod -aG docker "$access_user"

docker version
docker compose version
runuser --user "$access_user" -- docker version >/dev/null
podman --version 2>/dev/null || true

echo
echo "Docker Engine installation passed. Podman and its storage were not removed."
echo "The docker group grants root-level privileges. Log out and back in before running Docker as $access_user."
echo "Then run: ./devsetup/verify-docker-engine.sh"
