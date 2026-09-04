#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for command_name in go java curl sha256sum unzip; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the Peppol Schematron compilation gate." >&2
        exit 1
    fi
done

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
SCHXSLT_DIR="${ROOT_DIR}/devsetup/tools/schxslt"
ACCEPTED_XSLT="${ROOT_DIR}/validation-assets/peppol/3.0.21/runtime/PEPPOL-EN16931-UBL-validation.xslt"

if [[ ! -f "${SAXON_DIR}/saxon-he-12.7.jar" || ! -f "${SAXON_DIR}/xmlresolver-6.0.17.jar" ]]; then
    "${ROOT_DIR}/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi
if [[ ! -f "${SCHXSLT_DIR}/schxslt-1.10.1/2.0/pipeline-for-svrl.xsl" ]]; then
    "${ROOT_DIR}/devsetup/download_schxslt.sh" "$SCHXSLT_DIR"
fi

go run ./tools/verify-validation-assets \
    -saxon-dir "$SAXON_DIR" \
    -schxslt-dir "$SCHXSLT_DIR"
go run ./tools/compile-peppol-schematron \
    -saxon-dir "$SAXON_DIR" \
    -schxslt-dir "$SCHXSLT_DIR" \
    -check "$ACCEPTED_XSLT"
go test ./validation-assets
