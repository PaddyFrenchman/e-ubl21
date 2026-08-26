#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="${1:-e-ubl21:en16931-check}"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker (or a Docker-compatible CLI) is required." >&2
    exit 1
fi
if ! command -v timeout >/dev/null 2>&1; then
    echo "ERROR: GNU timeout is required." >&2
    exit 1
fi

docker build -t "$IMAGE_NAME" "$ROOT_DIR"

docker run --rm --network none --entrypoint /bin/sh "$IMAGE_NAME" -c \
    'test -x /usr/bin/xmllint && test -r /opt/e-ubl21/THIRD_PARTY_NOTICES.md'

OUTPUT_FILE="$(mktemp)"
trap 'rm -f "$OUTPUT_FILE"' EXIT

status=0
timeout 60s docker run --rm --network none "$IMAGE_NAME" worker \
    --nats-url nats://127.0.0.1:4222 \
    --worker-metrics-addr "" >"$OUTPUT_FILE" 2>&1 || status=$?

if [[ "$status" -eq 0 || "$status" -eq 124 ]]; then
    cat "$OUTPUT_FILE" >&2
    echo "ERROR: offline worker smoke did not reach the expected NATS connection failure." >&2
    exit 1
fi
if ! grep -Fq 'validation runtime verified' "$OUTPUT_FILE"; then
    cat "$OUTPUT_FILE" >&2
    echo "ERROR: packaged validation runtime did not complete its offline startup check." >&2
    exit 1
fi
grep -F 'validation runtime verified' "$OUTPUT_FILE"
echo "Offline packaged-runtime check passed; the later NATS-boundary failure is intentional."
