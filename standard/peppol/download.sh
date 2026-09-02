#!/bin/sh

set -eu

BIS_VERSION=3.0.21
SOURCE_COMMIT=806866bd2bd91d7e9623b68f08164e8fbe9e67a0
ARCHIVE_NAME=peppol-bis-invoice-3-$SOURCE_COMMIT.tar.gz
ARCHIVE_SHA256=a843cd3808c6e95565eded1a535a5a40a88ad6f92e009d2b1816a4818a82299e
ARCHIVE_URL=https://codeload.github.com/OpenPEPPOL/peppol-bis-invoice-3/tar.gz/$SOURCE_COMMIT

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
MANIFEST=$SCRIPT_DIR/sha256.txt
OFFLINE_INDEX=$SCRIPT_DIR/offline-index.html
DOWNLOAD_ROOT=$SCRIPT_DIR/downloaded
DESTINATION=$DOWNLOAD_ROOT/$BIS_VERSION
LOCK_DIR=$DOWNLOAD_ROOT/.download-$BIS_VERSION.lock

fail() {
    printf 'Peppol %s download failed: %s\n' "$BIS_VERSION" "$*" >&2
    exit 1
}

command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"
command -v tar >/dev/null 2>&1 || fail "tar is required"
command -v cmp >/dev/null 2>&1 || fail "cmp is required"
command -v awk >/dev/null 2>&1 || fail "awk is required"
command -v cp >/dev/null 2>&1 || fail "cp is required"
[ -r "$MANIFEST" ] || fail "checksum manifest is missing: $MANIFEST"
[ -r "$OFFLINE_INDEX" ] || fail "offline source index is missing: $OFFLINE_INDEX"

verify_download() {
    verify_root=$1
    (
        cd "$verify_root"
        sha256sum --check "$MANIFEST"
    ) || fail "checksum mismatch under $verify_root"

    cmp -s "$OFFLINE_INDEX" "$verify_root/index.html" ||
        fail "offline source index differs from the version-controlled index"

    release_notes=$verify_root/source/guide/release-notes/v3.0.21.adoc
    grep -Fq '= Version 3.0.21' "$release_notes" ||
        fail "release notes do not identify version 3.0.21"
    grep -Fq 'Date of publication: 2026-05-20' "$release_notes" ||
        fail "release notes have an unexpected publication date"
    grep -Fq 'Mandatory to use from: 2026-08-17' "$release_notes" ||
        fail "release notes have an unexpected mandatory date"
    grep -Fq 'Validation artefacts UBL and CII 1.3.16 publication date: 2026-04-10' \
        "$release_notes" ||
        fail "release notes have an unexpected validation artefact identity"
    grep -Fq 'Last update: 2026 May release 3.0.21.' \
        "$verify_root/source/rules/sch/PEPPOL-EN16931-UBL.sch" ||
        fail "Peppol Schematron has an unexpected release identity"

    for required_path in \
        source/README.adoc \
        source/project.xml \
        source/guide/bis/main.adoc \
        source/guide/compliance/main.adoc \
        source/guide/transaction-spec \
        source/guide/syntax \
        source/guide/profile \
        source/guide/cius \
        source/guide/national-rules \
        source/structure/syntax/ubl-invoice.xml \
        source/structure/syntax/ubl-creditnote.xml \
        source/rules/examples \
        source/rules/national-examples \
        source/mapping \
        source/stylesheet \
        source/tools
    do
        [ -e "$verify_root/$required_path" ] ||
            fail "complete source tree is missing $required_path"
    done
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
mkdir -p "$STAGING/archive" "$STAGING/source"

download() {
    source_url=$1
    destination_file=$2
    printf 'Downloading %s\n' "$source_url"
    curl --fail --location --proto '=https' --tlsv1.2 --retry 3 \
        --output "$STAGING/$destination_file" "$source_url" ||
        fail "could not download $source_url"
}

download "$ARCHIVE_URL" "archive/$ARCHIVE_NAME"

actual_archive_sha256=$(sha256sum "$STAGING/archive/$ARCHIVE_NAME" | awk '{print $1}')
[ "$actual_archive_sha256" = "$ARCHIVE_SHA256" ] ||
    fail "archive checksum mismatch: expected $ARCHIVE_SHA256, got $actual_archive_sha256"

tar -xzf "$STAGING/archive/$ARCHIVE_NAME" \
    --strip-components=1 -C "$STAGING/source" ||
    fail "could not extract the pinned source archive"
cp "$OFFLINE_INDEX" "$STAGING/index.html"

verify_download "$STAGING"
[ ! -e "$DESTINATION" ] || fail "destination appeared during download: $DESTINATION"
mv "$STAGING" "$DESTINATION"
trap - EXIT HUP INT TERM
rmdir "$LOCK_DIR"

printf 'Downloaded and verified Peppol BIS Billing %s at %s\n' \
    "$BIS_VERSION" "$DESTINATION"
