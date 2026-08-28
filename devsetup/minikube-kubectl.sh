#!/usr/bin/env bash
set -euo pipefail

profile="${EUBL21_MINIKUBE_PROFILE:-e-ubl21-regression}"
if [[ -z "${MINIKUBE_ROOTLESS:-}" ]] && command -v docker >/dev/null 2>&1; then
    docker_version="$(docker --version 2>&1)"
    if [[ "${docker_version,,}" == *podman* ]]; then
        export MINIKUBE_ROOTLESS=true
    fi
fi
exec minikube -p "$profile" kubectl -- "$@"
