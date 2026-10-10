#!/usr/bin/env bash
# Regenerate the two versioned UBL corpora only through an explicit review and
# promotion boundary.  The generated output is first held in a private staging
# directory so ordinary regression never mutates reviewed fixtures.
set -euo pipefail

case "${1:-}" in
  "") replace=false ;;
  --replace) replace=true ;;
  *) echo "usage: $0 [--replace]" >&2; exit 2 ;;
esac

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

for command in git go xmllint diff; do
  command -v "$command" >/dev/null || { echo "ERROR: $command is required" >&2; exit 2; }
done

# The XSD tree is the pinned, authoritative input. Refuse a regeneration from
# an uncommitted schema change unless the user deliberately commits/reviews it.
provenance_inputs=(
  ':(glob)createsamples/**/*.go'
  ':(exclude,glob)createsamples/**/*_test.go'
  'validation-assets/peppol.go'
  'regression/manifest.go'
  'regression/schemas/fixture-manifest.schema.json'
  ':(glob)XSD/**/*.xsd'
)
# Compare against HEAD, rather than the index, so staged modifications are
# rejected too. git status includes untracked provenance inputs.
if ! git diff --quiet HEAD -- "${provenance_inputs[@]}" || \
   [ -n "$(git status --porcelain --untracked-files=all -- "${provenance_inputs[@]}")" ]; then
  echo "ERROR: generator, manifest-contract, or XSD inputs have staged, unstaged, or untracked changes; commit them before regenerating" >&2
  exit 2
fi

stage=$(mktemp -d "${TMPDIR:-/tmp}/e-ubl21-corpus.XXXXXX")
cleanup_stage() {
  status=$?
  if [ "$status" -eq 0 ]; then
    rm -rf "$stage"
  else
    echo "Retaining failed corpus staging directory for diagnosis: $stage" >&2
  fi
  exit "$status"
}
trap cleanup_stage EXIT
go_cache="$stage/go-cache"
# Generated fixtures are deliberately excluded. Otherwise, committing a
# regenerated manifest changes the computed version embedded in that manifest
# and makes the next no-write regeneration differ solely due to itself.
version="git:$(git log -1 --format=%H -- \
  ':(glob)createsamples/**/*.go' \
  ':(exclude,glob)createsamples/**/*_test.go' \
  regression/manifest.go \
  regression/schemas/fixture-manifest.schema.json \
  ':(glob)XSD/**/*.xsd')"
run_generator() {
  resume_args=()
  if [ -n "${RESUME_FROM:-}" ]; then
    resume_args=(--resume-from "$RESUME_FROM")
  fi
  GOCACHE="$go_cache" go run ./createsamples generate --xsd XSD --generator-version "$version" "${resume_args[@]}" "$@"
}

# A resumed structural run is a diagnostic pass: it deliberately skips the
# earlier corpus phases and cannot promote a complete replacement corpus.
if [ -n "${RESUME_FROM:-}" ]; then
  run_generator --profile ubl21-structural --set positive --output "$stage/samples-structural"
  exit 0
fi

run_generator --profile ubl21 --set all --output "$stage/samples"
run_generator --profile ubl21-structural --set positive --output "$stage/samples-structural"
# Cardinality is a hard structural-coverage gate. The report is written before
# failure so the missing parent-instance states remain reviewable.
GOCACHE="$go_cache" go run ./createsamples coverage --xsd XSD --samples "$stage/samples-structural" --json "$stage/coverage.json" --markdown "$stage/coverage.md"

status=0
for corpus in samples samples-structural; do
  echo "\n== review: createsamples/$corpus =="
  if ! diff -ruN "createsamples/$corpus" "$stage/$corpus"; then
    status=1
  fi
done

if [ "$replace" = true ]; then
  # The generator's own ownership-marker and staged-replacement guard performs
  # the only tracked-output write; this is intentionally a second generation.
  run_generator --profile ubl21 --set all --output createsamples/samples
  run_generator --profile ubl21-structural --set positive --output createsamples/samples-structural
  GOCACHE="$go_cache" go run ./createsamples coverage --xsd XSD --samples createsamples/samples-structural --json "$stage/coverage.json" --markdown "$stage/coverage.md"
  echo "Promoted regenerated corpus. Review with: git diff -- createsamples/samples createsamples/samples-structural"
  exit 0
fi

if [ "$status" -ne 0 ]; then
  echo "Generated corpus differs from tracked output. Review the diff above; rerun with --replace only to promote it." >&2
  exit 1
fi
echo "Tracked UBL corpora and structural coverage reports are current."
