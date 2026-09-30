#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

MINIKUBE_PROFILE="${MINIKUBE_PROFILE:-e-ubl21-regression}"
MINIKUBE_DRIVER="${MINIKUBE_DRIVER:-docker}"
MINIKUBE_KUBERNETES_VERSION="${MINIKUBE_KUBERNETES_VERSION:-v1.35.1}"

section() {
  printf '\n============================================================\n'
  printf ' %s\n' "$1"
  printf '============================================================\n'
}

cleanup() {
  if command -v minikube >/dev/null 2>&1; then
    minikube delete --profile "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

section "Environment"
git status --short
go version
docker version
xmllint --version 2>&1 | head -n 1
java -version

section "Go unit and package tests"
go test ./... -count=1

section "Generate and validate UBL 2.1 structural corpus"
go run ./createsamples generate \
  --profile ubl21-structural \
  --set positive \
  --xsd XSD \
  --output createsamples/samples-structural

section "Measure structural schema and cardinality coverage"
make schema-coverage

section "Validation chaos, crash, and replay gate"
make test-validation-chaos

section "Deterministic correctness regression"
make regression-correctness

section "YugabyteDB integration and migrations"
make database-test

section "YugabyteDB production compatibility guard"
make yugabytedb-compatibility-guard

section "Kubernetes deployment tests"
go test ./deploy/... -count=1

section "Pinned on-prem YugabyteDB rendering"
make on-prem-database-test

section "Peppol Schematron reproducibility"
make test-peppol-schematron

section "Peppol SBDH envelope fixtures"
make test-peppol-envelope

section "Positive Peppol conformity corpus"
make test-peppol-positive

section "Negative Peppol conformity corpus"
make test-peppol-negative

section "Peppol reference cross-check"
make test-peppol-reference

section "Peppol compliance gate"
make test-peppol-compliance

section "EN 16931 compliance gate"
make test-en16931-compliance

section "Production image validation"
make test-en16931-image IMAGE=e-ubl21:ci

section "Packaged Peppol worker validation"
SKIP_IMAGE_BUILD=1 make test-peppol-image IMAGE=e-ubl21:ci

section "Start disposable Minikube cluster"
if ! command -v minikube >/dev/null 2>&1; then
  echo "ERROR: minikube is required for the local Kubernetes regression gate." >&2
  echo "Install Minikube v1.39.0 to match GitHub CI." >&2
  exit 1
fi

minikube delete --profile "$MINIKUBE_PROFILE" >/dev/null 2>&1 || true
minikube start \
  --profile "$MINIKUBE_PROFILE" \
  --driver "$MINIKUBE_DRIVER" \
  --container-runtime docker \
  --kubernetes-version "$MINIKUBE_KUBERNETES_VERSION" \
  --cpus 2 \
  --memory 6144mb

section "Minikube regression: UBL 2.1"
make regression-minikube \
  MANIFEST=createsamples/samples/manifest.json \
  PROFILE=all \
  ARTIFACT_DIR=regression-results/minikube/ubl21

section "Minikube regression: EN 16931"
make regression-minikube \
  MANIFEST=createsamples/samples-en16931/manifest.json \
  PROFILE=all \
  ARTIFACT_DIR=regression-results/minikube/en16931

section "All local CI-equivalent tests passed"
