#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_NAME="${1:-e-ubl21:peppol-check}"
SKIP_IMAGE_BUILD="${SKIP_IMAGE_BUILD:-0}"

if ! command -v docker >/dev/null 2>&1; then
    echo "ERROR: docker (or a Docker-compatible CLI) is required." >&2
    exit 1
fi
if ! command -v timeout >/dev/null 2>&1; then
    echo "ERROR: GNU timeout is required." >&2
    exit 1
fi
if [[ "$SKIP_IMAGE_BUILD" != "0" && "$SKIP_IMAGE_BUILD" != "1" ]]; then
    echo "ERROR: SKIP_IMAGE_BUILD must be 0 or 1." >&2
    exit 2
fi

if [[ "$SKIP_IMAGE_BUILD" == "0" ]]; then
    docker build -t "$IMAGE_NAME" "$ROOT_DIR"
fi

docker run --rm --network none --entrypoint /bin/sh "$IMAGE_NAME" -c '
    test -x /usr/bin/java
    test -r /opt/e-ubl21/saxon/saxon-he-12.7.jar
    test -r /opt/e-ubl21/saxon/xmlresolver-6.0.17.jar
    test -r /opt/e-ubl21/validation-assets/saxon/saxon-config.xml
    test -r /opt/e-ubl21/validation-assets/peppol/3.0.21/runtime/PEPPOL-EN16931-UBL-validation.xslt
    test -r /opt/e-ubl21/THIRD_PARTY_NOTICES.md
    grep -Fq "Peppol BIS Billing 3.0.21" /opt/e-ubl21/THIRD_PARTY_NOTICES.md
'

OUTPUT_FILE="$(mktemp)"
trap 'rm -f "$OUTPUT_FILE"' EXIT

status=0
timeout 60s docker run --rm --network none "$IMAGE_NAME" worker-peppol \
    --nats-url nats://127.0.0.1:4222 \
    --worker-peppol-metrics-addr "" >"$OUTPUT_FILE" 2>&1 || status=$?

if [[ "$status" -eq 0 || "$status" -eq 124 ]]; then
    sed -n '1,160p' "$OUTPUT_FILE" >&2
    echo "ERROR: offline worker-peppol smoke did not reach the expected NATS connection failure." >&2
    exit 1
fi
for expected in \
    'validation runtime verified' \
    'validationProfile=peppol-bis-billing' \
    'peppolBISVersion=3.0.21' \
    'peppolPublicationDate=2026-05-20' \
    'peppolMandatoryFrom=2026-08-17' \
    'peppolSourceCommit=806866bd2bd91d7e9623b68f08164e8fbe9e67a0' \
    'peppolSchematronSHA256=62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3' \
    'peppolCompiledXSLTSHA256=56bbd05ea26b4ffb859e0c15fcd2c1c8782870b530aae06a828ddb0bdf9b66aa' \
    'peppolCompilerVersion=1.10.1' \
    'peppolCompilerArchiveSHA256=e141fabd4804a0ea527945dc7253dd38f9ef8957d9b219b94e13d16859cff7f0' \
    'peppolCompilerMetadataSHA256=e52d9533c9e4c9bdf14959d546e0fe4a344bc49aecc67d1ee9731f4a30bba35b'
do
    if ! grep -Fq "$expected" "$OUTPUT_FILE"; then
        sed -n '1,160p' "$OUTPUT_FILE" >&2
        echo "ERROR: packaged Peppol runtime log is missing: ${expected}" >&2
        exit 1
    fi
done

grep -F 'validation runtime verified' "$OUTPUT_FILE"
echo "Offline packaged Peppol runtime check passed; the later NATS-boundary failure is intentional."
