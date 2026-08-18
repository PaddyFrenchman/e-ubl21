#!/usr/bin/env bash
set -Eeuo pipefail
#chmod +x download_ubl_2_1_all_xsds.sh
#./download_ubl_2_1_all_xsds.sh
# Download the complete OASIS UBL 2.1 XSD distribution.
#
# Result:
#
# XSD/
# ├── common/
# │   └── ... 14 common XSD files
# └── maindoc/
#     └── ... 65 UBL document XSD files
#
# Usage:
#   chmod +x download_ubl_2_1_all_xsds.sh
#   ./download_ubl_2_1_all_xsds.sh
#
# Optional custom destination:
#   ./download_ubl_2_1_all_xsds.sh /path/to/XSD

BASE_URL="https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd"
DEST_DIR="${1:-XSD}"

EXPECTED_MAINDOC_COUNT=65
EXPECTED_COMMON_COUNT=14

if ! command -v wget >/dev/null 2>&1; then
    echo "ERROR: wget is required but was not found." >&2
    exit 1
fi

mkdir -p "$DEST_DIR/common" "$DEST_DIR/maindoc"

download_directory() {
    local directory="$1"

    echo
    echo "Downloading UBL 2.1 ${directory} schemas..."

    # The remote path is:
    #   /ubl/os-UBL-2.1/xsd/<directory>/<file>
    #
    # --cut-dirs=3 removes:
    #   ubl/os-UBL-2.1/xsd
    #
    # leaving:
    #   <directory>/<file>
    #
    # under DEST_DIR.
    wget \
        --recursive \
        --level=1 \
        --no-parent \
        --no-host-directories \
        --cut-dirs=3 \
        --directory-prefix="$DEST_DIR" \
        --accept="*.xsd" \
        --reject="*.gz" \
        --timestamping \
        --show-progress \
        "$BASE_URL/$directory/"
}

download_directory "common"
download_directory "maindoc"

echo
echo "Verifying download..."

common_count="$(
    find "$DEST_DIR/common" \
        -maxdepth 1 \
        -type f \
        -name '*.xsd' \
        | wc -l \
        | tr -d '[:space:]'
)"

maindoc_count="$(
    find "$DEST_DIR/maindoc" \
        -maxdepth 1 \
        -type f \
        -name '*.xsd' \
        | wc -l \
        | tr -d '[:space:]'
)"

if [[ "$common_count" -ne "$EXPECTED_COMMON_COUNT" ]]; then
    echo "ERROR: expected $EXPECTED_COMMON_COUNT common XSD files, found $common_count." >&2
    exit 1
fi

if [[ "$maindoc_count" -ne "$EXPECTED_MAINDOC_COUNT" ]]; then
    echo "ERROR: expected $EXPECTED_MAINDOC_COUNT maindoc XSD files, found $maindoc_count." >&2
    exit 1
fi

# Verify the six document types planned for the initial validator.
PRIORITY_SCHEMAS=(
    "UBL-Invoice-2.1.xsd"
    "UBL-CreditNote-2.1.xsd"
    "UBL-DebitNote-2.1.xsd"
    "UBL-SelfBilledInvoice-2.1.xsd"
    "UBL-SelfBilledCreditNote-2.1.xsd"
    "UBL-FreightInvoice-2.1.xsd"
)

for schema in "${PRIORITY_SCHEMAS[@]}"; do
    if [[ ! -f "$DEST_DIR/maindoc/$schema" ]]; then
        echo "ERROR: required schema is missing: $DEST_DIR/maindoc/$schema" >&2
        exit 1
    fi
done

echo
echo "UBL 2.1 XSD download complete."
echo
echo "Structure:"
echo "  $DEST_DIR/"
echo "  ├── common/   ($common_count XSD files)"
echo "  └── maindoc/  ($maindoc_count XSD files)"
echo
echo "Initial validator document schemas:"
for schema in "${PRIORITY_SCHEMAS[@]}"; do
    echo "  $DEST_DIR/maindoc/$schema"
done

echo
echo "Example Invoice validation:"
echo "  xmllint --noout --schema \"$DEST_DIR/maindoc/UBL-Invoice-2.1.xsd\" invoice.xml"
