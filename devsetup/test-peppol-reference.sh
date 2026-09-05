#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for command_name in go java curl sha256sum unzip; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the mandatory Peppol reference cross-check." >&2
        exit 1
    fi
done

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
SCHXSLT_DIR="${ROOT_DIR}/devsetup/tools/schxslt"

if [[ ! -f "${SAXON_DIR}/saxon-he-12.7.jar" || ! -f "${SAXON_DIR}/xmlresolver-6.0.17.jar" ]]; then
    "${ROOT_DIR}/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi
if [[ ! -f "${SCHXSLT_DIR}/schxslt-1.10.1/2.0/pipeline-for-svrl.xsl" ]]; then
    "${ROOT_DIR}/devsetup/download_schxslt.sh" "$SCHXSLT_DIR"
fi

go run ./tools/check-peppol-reference \
    -saxon-dir "$SAXON_DIR" \
    -schxslt-dir "$SCHXSLT_DIR"

go test ./server \
    -run '^TestPinnedPeppolValidatorMatchesReferenceManifest$' \
    -count=1
