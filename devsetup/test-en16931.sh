#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_command() {
    local command_name="$1"
    local install_hint="$2"
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "ERROR: ${command_name} is required for the mandatory EN 16931 integration suite." >&2
        echo "Install ${install_hint}, then rerun: make test-en16931" >&2
        exit 1
    fi
}

require_command go "Go"
require_command java "a Java runtime (OpenJDK 21 recommended)"
require_command xmllint "libxml2-utils (Debian/Ubuntu) or libxml2 (Alpine/macOS)"

SAXON_DIR="${ROOT_DIR}/devsetup/tools/saxon"
SAXON_JAR="${SAXON_DIR}/saxon-he-12.7.jar"
RESOLVER_JAR="${SAXON_DIR}/xmlresolver-6.0.17.jar"

if [[ ! -f "$SAXON_JAR" || ! -f "$RESOLVER_JAR" ]]; then
    require_command curl "curl"
    require_command sha256sum "GNU coreutils"
    echo "Pinned Saxon dependencies are missing; downloading checksum-verified copies."
    "${ROOT_DIR}/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi

echo "Verifying pinned EN 16931, Saxon, XML Resolver, and Saxon configuration assets."
go run ./tools/verify-validation-assets -saxon-dir "$SAXON_DIR"

echo "Running the mandatory EN 16931 corpora and final end-to-end compliance gate."
EUBL21_REQUIRE_EN16931_TEST_RUNTIME=1 \
    go test ./server \
        -run '^(TestPositiveEN16931FixturesPassCompleteValidationPipeline|TestTargetedEN16931NegativeCorpus|TestXSDInvalidControlSkipsEN16931|TestEN16931PublicComplianceScenarios|TestEN16931OperationalFailureReplayEndToEnd|TestEN16931CrashRestartEndToEnd|TestWorkerClaimsOneReplayAfterMessageIDDeduplicationWindow)$' \
        -count=1 -v
