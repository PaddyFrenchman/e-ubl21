#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="1.10.1"
ARCHIVE_URL="https://codeberg.org/SchXslt/schxslt/releases/download/v${VERSION}/schxslt-${VERSION}-xslt-only.zip"
ARCHIVE_SHA256="e141fabd4804a0ea527945dc7253dd38f9ef8957d9b219b94e13d16859cff7f0"
DEST_ROOT="${1:-${ROOT_DIR}/devsetup/tools/schxslt}"
DEST_DIR="${DEST_ROOT}/schxslt-${VERSION}"

for command_name in curl sha256sum unzip; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required to download SchXslt." >&2
        exit 1
    fi
done

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
archive="${work_dir}/schxslt.zip"

curl --fail --location --silent --show-error -o "$archive" "$ARCHIVE_URL"
actual_sha256="$(sha256sum "$archive" | awk '{print $1}')"
if [[ "$actual_sha256" != "$ARCHIVE_SHA256" ]]; then
    echo "ERROR: checksum mismatch for SchXslt ${VERSION}." >&2
    echo "  expected: ${ARCHIVE_SHA256}" >&2
    echo "  actual:   ${actual_sha256}" >&2
    exit 1
fi

unzip -q "$archive" -d "${work_dir}/unpacked"
if [[ ! -d "${work_dir}/unpacked/schxslt-${VERSION}" ]]; then
    echo "ERROR: SchXslt archive did not contain schxslt-${VERSION}/." >&2
    exit 1
fi

mkdir -p "$DEST_ROOT"
rm -rf "$DEST_DIR"
mv "${work_dir}/unpacked/schxslt-${VERSION}" "$DEST_DIR"

echo "Downloaded checksum-verified SchXslt ${VERSION} to ${DEST_DIR}"
