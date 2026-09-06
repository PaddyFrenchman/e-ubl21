#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_command() {
    local command_name="$1"
    local install_hint="$2"
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the mandatory Peppol compliance gate." >&2
        echo "Install ${install_hint}, then rerun: make test-peppol-compliance" >&2
        exit 1
    fi
}

require_command go "Go"
require_command java "a Java runtime (OpenJDK 21 recommended)"
require_command xmllint "libxml2-utils (Debian/Ubuntu) or libxml2 (Alpine/macOS)"

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
if [[ ! -f "${SAXON_DIR}/saxon-he-12.7.jar" || ! -f "${SAXON_DIR}/xmlresolver-6.0.17.jar" ]]; then
    require_command curl "curl"
    require_command sha256sum "GNU coreutils"
    "${ROOT_DIR}/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi

echo "Verifying the accepted normalized Peppol release baseline."
go test ./regression \
    -run '^TestPeppolReleaseBaselineMatchesPinnedCorpusAndReference$' \
    -count=1 -v

echo "Running the final public asynchronous Peppol compliance gate."
EUBL21_REQUIRE_EN16931_TEST_RUNTIME=1 \
    go test ./server \
        -run '^(TestPeppolPublicComplianceScenarios|TestPeppolOperationalFailureReplayEndToEnd|TestPeppolCrashRestartAndDuplicateEndToEnd)$' \
        -count=1 -v
