#!/bin/sh

set -eu

BIS_VERSION=3.0.21
SOURCE_COMMIT=806866bd2bd91d7e9623b68f08164e8fbe9e67a0
DOCS_BASE=https://docs.peppol.eu/poacc/billing/3.0
RAW_BASE=https://raw.githubusercontent.com/OpenPEPPOL/peppol-bis-invoice-3/$SOURCE_COMMIT

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
MANIFEST=$SCRIPT_DIR/sha256.txt
DOWNLOAD_ROOT=$SCRIPT_DIR/downloaded
DESTINATION=$DOWNLOAD_ROOT/$BIS_VERSION
LOCK_DIR=$DOWNLOAD_ROOT/.download-$BIS_VERSION.lock

fail() {
    printf 'Peppol %s download failed: %s\n' "$BIS_VERSION" "$*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"
[ -r "$MANIFEST" ] || fail "checksum manifest is missing: $MANIFEST"

verify_download() {
    verify_root=$1
    (
        cd "$verify_root"
        sha256sum --check "$MANIFEST"
    ) || fail "checksum mismatch under $verify_root"

    grep -Fq 'version 3.0.21' "$verify_root/docs/bis-index.html" ||
        fail "BIS documentation does not identify version 3.0.21"
    grep -Fq 'Version 3.0.21' "$verify_root/docs/release-notes-index.html" ||
        fail "release notes do not identify version 3.0.21"
    grep -Fq 'Last update: 2026 May release 3.0.21.' \
        "$verify_root/assets/PEPPOL-EN16931-UBL.sch" ||
        fail "Peppol Schematron has an unexpected release identity"
}

if [ -d "$DESTINATION" ]; then
    verify_download "$DESTINATION"
    printf 'Peppol BIS Billing %s is already downloaded and verified at %s\n' \
        "$BIS_VERSION" "$DESTINATION"
    exit 0
fi

mkdir -p "$DOWNLOAD_ROOT"
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
    fail "another download may be running (lock: $LOCK_DIR)"
fi

STAGING=$DOWNLOAD_ROOT/.download-$BIS_VERSION.$$
cleanup() {
    if [ -d "$STAGING" ]; then
        rm -rf -- "$STAGING"
    fi
    rmdir "$LOCK_DIR" 2>/dev/null || true
}
trap cleanup EXIT HUP INT TERM

[ ! -e "$STAGING" ] || fail "staging path already exists: $STAGING"
mkdir -p "$STAGING/assets/codelists" "$STAGING/docs"

download() {
    source_url=$1
    destination_file=$2
    printf 'Downloading %s\n' "$source_url"
    curl --fail --location --proto '=https' --tlsv1.2 --retry 3 \
        --output "$STAGING/$destination_file" "$source_url" ||
        fail "could not download $source_url"
}

download "$RAW_BASE/rules/sch/CEN-EN16931-UBL.sch" \
    assets/CEN-EN16931-UBL.sch
download "$RAW_BASE/rules/sch/PEPPOL-EN16931-UBL.sch" \
    assets/PEPPOL-EN16931-UBL.sch
for codelist in \
    ISO3166-1_Alpha2.xml \
    ISO4217_2015.xml \
    MimeCode.xml \
    SEPA.xml \
    UNCL1001-cn.xml \
    UNCL1001-inv.xml \
    UNCL1153.xml \
    UNCL2005.xml \
    UNCL4461.xml \
    UNCL5189.xml \
    UNCL5305.xml \
    UNCL7143.xml \
    UNCL7161.xml \
    UNECERec20-11e.xml \
    VATEX.xml \
    eas.xml \
    icd.xml
do
    download "$RAW_BASE/structure/codelist/$codelist" \
        "assets/codelists/$codelist"
done
download "$DOCS_BASE/bis/" docs/bis-index.html
download "$DOCS_BASE/" docs/index.html
download "$DOCS_BASE/release-notes/" docs/release-notes-index.html

verify_download "$STAGING"
[ ! -e "$DESTINATION" ] || fail "destination appeared during download: $DESTINATION"
mv "$STAGING" "$DESTINATION"
trap - EXIT HUP INT TERM
rmdir "$LOCK_DIR"

printf 'Downloaded and verified Peppol BIS Billing %s at %s\n' \
    "$BIS_VERSION" "$DESTINATION"
