#!/usr/bin/env bash
set -Eeuo pipefail
#chmod +x download_en16931_validation_assets.sh
#./download_en16931_validation_assets.sh
# Download the pinned official EN 16931 UBL validation XSLT.
#
# Source repository: ConnectingEurope/eInvoicing-EN16931
# Pinned release tag: validation-1.3.16
# License: EUPL 1.2
#
# See ideas/MAIN-EN16931-UBL-validation.md §30/§31 and
# ideas/EN16931-samples-generator-plan.md §2/Phase EN-0 for the rationale:
# the runtime must never download validation artefacts on startup, and an
# upgrade to a new EN16931 release must be a deliberate, reviewed step, never
# an unpinned moving branch.
#
# Result:
#
# validation-assets/en16931/1.3.16/
# ├── EN16931-UBL-validation.xslt
# ├── source-url.txt
# └── sha256.txt
#
# Usage:
#   chmod +x download_en16931_validation_assets.sh
#   ./download_en16931_validation_assets.sh
#
# Optional custom destination root (the version subdirectory is still
# appended automatically):
#   ./download_en16931_validation_assets.sh /path/to/validation-assets

TAG="validation-1.3.16"
VERSION="1.3.16"
REPO="ConnectingEurope/eInvoicing-EN16931"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${TAG}/ubl/xslt"
BLOB_URL="https://github.com/${REPO}/blob/${TAG}/ubl/xslt/EN16931-UBL-validation.xslt"
RELEASE_URL="https://github.com/${REPO}/releases/tag/${TAG}"

DEST_ROOT="${1:-validation-assets}"
DEST_DIR="${DEST_ROOT}/en16931/${VERSION}"

# Known-good checksum of the pinned artefact, recorded when this script was
# authored. If the upstream file ever changes under the same tag (it should
# not, since tags are immutable), this check will fail loudly instead of
# silently accepting a different file.
EXPECTED_SHA256="39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd"

if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required but was not found." >&2
    exit 1
fi

if ! command -v sha256sum >/dev/null 2>&1; then
    echo "ERROR: sha256sum is required but was not found." >&2
    exit 1
fi

mkdir -p "$DEST_DIR"

echo
echo "Downloading EN16931 UBL validation XSLT (release ${TAG})..."

curl -sSL \
    -o "$DEST_DIR/EN16931-UBL-validation.xslt" \
    "$RAW_BASE/EN16931-UBL-validation.xslt"

echo
echo "Verifying checksum..."

actual_sha256="$(sha256sum "$DEST_DIR/EN16931-UBL-validation.xslt" | awk '{print $1}')"

if [[ "$actual_sha256" != "$EXPECTED_SHA256" ]]; then
    echo "ERROR: checksum mismatch for EN16931-UBL-validation.xslt." >&2
    echo "  expected: $EXPECTED_SHA256" >&2
    echo "  actual:   $actual_sha256" >&2
    echo "Refusing to trust this artefact. Do not silently update EXPECTED_SHA256" >&2
    echo "without deliberately reviewing why the pinned tag's content changed." >&2
    exit 1
fi

cat > "$DEST_DIR/source-url.txt" <<EOF
${BLOB_URL}

Repository:   ${REPO}
Release tag:  ${TAG}
Raw source:   ${RAW_BASE}/EN16931-UBL-validation.xslt
Release page: ${RELEASE_URL}
License:      EUPL 1.2
Downloaded:   $(date -u +%Y-%m-%d)
EOF

echo "$actual_sha256  EN16931-UBL-validation.xslt" > "$DEST_DIR/sha256.txt"

echo
echo "EN16931 validation asset download complete."
echo
echo "Structure:"
echo "  $DEST_DIR/"
echo "  ├── EN16931-UBL-validation.xslt"
echo "  ├── source-url.txt"
echo "  └── sha256.txt"
echo
echo "Example validation (requires Saxon, see ideas/MAIN-EN16931-UBL-validation.md §8):"
echo "  java net.sf.saxon.Transform \\"
echo "      -s:invoice.xml \\"
echo "      -xsl:$DEST_DIR/EN16931-UBL-validation.xslt \\"
echo "      -o:en16931.svrl.xml"
