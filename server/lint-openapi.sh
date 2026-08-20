#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPEC_PATH="${1:-${ROOT_DIR}/openapi.yaml}"
BIN_DIR="${GOBIN:-${ROOT_DIR}/.bin}"
mkdir -p "${BIN_DIR}"

if ! command -v vacuum >/dev/null 2>&1; then
  echo "vacuum not found; installing to ${BIN_DIR}"
  GOBIN="${BIN_DIR}" go install github.com/daveshanley/vacuum@v0.30.0
fi

export PATH="${BIN_DIR}:${PATH}"

if [ ! -f "${SPEC_PATH}" ]; then
  echo "OpenAPI spec not found: ${SPEC_PATH}" >&2
  exit 1
fi

vacuum lint "${SPEC_PATH}"
