#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
profile="${EUBL21_MINIKUBE_PROFILE:-e-ubl21-regression}"
if [[ "${MINIKUBE_DRIVER:-docker}" == podman ]]; then
    export MINIKUBE_ROOTLESS="${MINIKUBE_ROOTLESS:-true}"
else
    source "$ROOT_DIR/devsetup/lib/docker-engine.sh"
    require_docker_engine
fi
exec minikube -p "$profile" kubectl -- "$@"
