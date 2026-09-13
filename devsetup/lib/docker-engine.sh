#!/usr/bin/env bash

# Shared, side-effect-free checks for repository commands that require the
# genuine Docker CLI and Engine. The caller is expected to enable strict mode.

docker_setup_hint="See docs/DOCKER-DEVELOPMENT.md or run: sudo ./devsetup/install-docker-engine.sh"

require_docker_engine() {
    local client_version server_platform

    if ! command -v docker >/dev/null 2>&1; then
        echo "ERROR: Docker Engine is required, but the docker CLI was not found. $docker_setup_hint" >&2
        return 1
    fi

    if ! client_version="$(docker --version 2>&1)"; then
        echo "ERROR: the docker CLI could not report its version. $docker_setup_hint" >&2
        return 1
    fi
    if [[ "${client_version,,}" == *podman* ]]; then
        echo "ERROR: docker resolves to Podman compatibility, not Docker Engine. $docker_setup_hint" >&2
        return 1
    fi

    if ! server_platform="$(docker version --format '{{.Server.Platform.Name}}' 2>/dev/null)"; then
        echo "ERROR: the Docker Engine daemon is not reachable. Start docker.service and verify Docker socket access; $docker_setup_hint" >&2
        return 1
    fi
    if [[ "${server_platform,,}" != *"docker engine"* ]]; then
        echo "ERROR: docker is connected to '${server_platform:-an unidentified server}', not a genuine Docker Engine. $docker_setup_hint" >&2
        return 1
    fi
}

require_docker_compose() {
    local compose_version

    require_docker_engine || return
    if ! compose_version="$(docker compose version 2>&1)"; then
        echo "ERROR: the official Docker Compose v2 plugin is required. $docker_setup_hint" >&2
        return 1
    fi
    if [[ "${compose_version,,}" != *"docker compose version"* ]]; then
        echo "ERROR: docker compose is not the official Docker Compose v2 plugin. $docker_setup_hint" >&2
        return 1
    fi
}
