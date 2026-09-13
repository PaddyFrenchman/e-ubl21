#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/devsetup/lib/docker-engine.sh"

require_docker_compose

work_dir="$(mktemp -d)"
suffix="${UID}-$$"
volume_name="eubl21-docker-check-volume-$suffix"
network_name="eubl21-docker-check-network-$suffix"
project_name="eubl21-docker-check-$suffix"

cleanup() {
    docker compose --project-name "$project_name" --file "$work_dir/compose.yml" down --volumes --remove-orphans >/dev/null 2>&1 || true
    docker volume rm --force "$volume_name" >/dev/null 2>&1 || true
    docker network rm "$network_name" >/dev/null 2>&1 || true
    rm -rf "$work_dir"
}
trap cleanup EXIT

docker pull alpine:3.21
docker volume create "$volume_name" >/dev/null
docker network create "$network_name" >/dev/null
docker run --rm --network "$network_name" --volume "$volume_name:/data" alpine:3.21 \
    sh -c 'printf volume-ok >/data/result'
docker run --rm --volume "$volume_name:/data:ro" alpine:3.21 grep -Fxq volume-ok /data/result

printf 'bind-ok\n' >"$work_dir/bind-source"
docker run --rm --volume "$work_dir:/check:ro" alpine:3.21 grep -Fxq bind-ok /check/bind-source

cat >"$work_dir/compose.yml" <<'EOF'
services:
  check:
    image: alpine:3.21
    command: ["sh", "-c", "touch /state/ready && exec sleep 300"]
    healthcheck:
      test: ["CMD", "test", "-f", "/state/ready"]
      interval: 1s
      timeout: 1s
      retries: 10
    volumes:
      - state:/state
volumes:
  state:
EOF

docker compose --project-name "$project_name" --file "$work_dir/compose.yml" up --detach --wait
docker compose --project-name "$project_name" --file "$work_dir/compose.yml" ps
docker compose --project-name "$project_name" --file "$work_dir/compose.yml" logs check >/dev/null
docker compose --project-name "$project_name" --file "$work_dir/compose.yml" restart check
docker compose --project-name "$project_name" --file "$work_dir/compose.yml" down --volumes

echo "Docker Engine image, container, network, volume, bind-mount, health-check, and Compose lifecycle verification passed."
