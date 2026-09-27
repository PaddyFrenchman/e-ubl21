#!/usr/bin/env bash
set -Eeuo pipefail

# Scan the repository rather than a hand-picked manifest list so a newly added
# deployment script cannot silently reintroduce a retired dependency. The
# exclusions below are historical records, migration/retirement procedures, or
# their dedicated test harnesses; they are not active clean-install inputs.
ROOT_DIR="${EUBL21_COMPATIBILITY_GUARD_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
readonly ROOT_DIR

historical_exclusions=(
    "--glob=!docs/**"
    "--glob=!ideas/**"
    "--glob=!ISSUES.MD"
    "--glob=!TODO*"
    "--glob=!README.md"
    "--glob=!deployment-target.md"
    "--glob=!deploy/on-prem/deployment-on-prem.md"
    "--glob=!e-invoice/**"
    "--glob=!run-issue-124-tests.sh"
    "--glob=!deploy/on-prem/scripts/upgrade-postgresql-to-yugabytedb.sh"
    "--glob=!devsetup/test-on-prem-yugabytedb-upgrade.sh"
    "--glob=!devsetup/test-on-prem-yugabytedb-upgrade-integration.sh"
    "--glob=!devsetup/test-on-prem-yugabytedb.sh"
    "--glob=!devsetup/yugabytedb-compatibility-guard.sh"
)
prohibited_references=(
    "CloudNativePG"
    "e-ubl21-postgres"
    "POSTGRES_STORAGE_SIZE"
)

failed=0
for prohibited in "${prohibited_references[@]}"; do
    matches="$({
        rg --line-number --fixed-strings "${historical_exclusions[@]}" -- "$prohibited" "$ROOT_DIR" || true
    } | rg -v \
        -e 'deploy/on-prem/scripts/install-on-prem\.sh:[0-9]+:.*POSTGRES_STORAGE_SIZE' \
        -e 'deploy/on-prem/scripts/install-on-prem\.sh:[0-9]+:.*settings are retired' \
        -e 'deploy/on-prem/scripts/install-on-prem\.sh:[0-9]+:.*get cluster\.postgresql\.cnpg\.io e-ubl21-postgres' \
        -e 'deploy/on-prem/scripts/install-on-prem\.sh:[0-9]+:.*Legacy e-ubl21-postgres exists; refusing a fresh-install conversion' || true)"
    if [[ -n "$matches" ]]; then
        echo "ERROR: retired PostgreSQL deployment reference '$prohibited' is prohibited:" >&2
        echo "$matches" >&2
        failed=1
    fi
done

if (( failed != 0 )); then
    exit 1
fi

echo "YugabyteDB compatibility guard passed."
