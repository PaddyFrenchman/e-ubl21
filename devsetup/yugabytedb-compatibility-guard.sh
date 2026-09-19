#!/usr/bin/env bash
set -Eeuo pipefail

# This guard deliberately examines only current production deployment inputs.
# Historical migration documentation and the guarded legacy-retirement scripts
# retain PostgreSQL terminology as evidence of the supported upgrade path.
ROOT_DIR="${EUBL21_COMPATIBILITY_GUARD_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
readonly ROOT_DIR

production_inputs=(
    "Dockerfile"
    "deploy/local/yugabytedb-compose.yml"
    "deploy/minikube"
    "deploy/kubernetes"
    "deploy/on-prem/yugabytedb-values.yaml"
)
prohibited_references=(
    "CloudNativePG"
    "e-ubl21-postgres"
    "POSTGRES_STORAGE_SIZE"
)

for input in "${production_inputs[@]}"; do
    [[ -e "$ROOT_DIR/$input" ]] || {
        echo "ERROR: compatibility guard input is missing: $input" >&2
        exit 2
    }
done

failed=0
for prohibited in "${prohibited_references[@]}"; do
    if matches="$(rg --line-number --fixed-strings -- "$prohibited" "${production_inputs[@]/#/$ROOT_DIR/}" 2>/dev/null)" && [[ -n "$matches" ]]; then
        echo "ERROR: retired PostgreSQL deployment reference '$prohibited' is prohibited:" >&2
        echo "$matches" >&2
        failed=1
    fi
done

if (( failed != 0 )); then
    exit 1
fi

echo "YugabyteDB compatibility guard passed."
