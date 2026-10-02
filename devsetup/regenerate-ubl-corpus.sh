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
trap 'rm -rf "$stage"' EXIT
go_cache="$stage/go-cache"
version="git:$(git log -1 --format=%H -- createsamples regression/manifest.go regression/schemas/fixture-manifest.schema.json XSD)"
run_generator() {
  GOCACHE="$go_cache" go run ./createsamples generate --xsd XSD --generator-version "$version" "$@"
}

run_generator --profile ubl21 --set all --output "$stage/samples"
run_generator --profile ubl21-structural --set positive --output "$stage/samples-structural"
GOCACHE="$go_cache" go run ./createsamples coverage --xsd XSD --samples "$stage/samples-structural" --json "$stage/samples-structural/coverage.json" --markdown "$stage/samples-structural/coverage.md"

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
  GOCACHE="$go_cache" go run ./createsamples coverage --xsd XSD --samples createsamples/samples-structural --json createsamples/samples-structural/coverage.json --markdown createsamples/samples-structural/coverage.md
  echo "Promoted regenerated corpus. Review with: git diff -- createsamples/samples createsamples/samples-structural"
  exit 0
fi

if [ "$status" -ne 0 ]; then
  echo "Generated corpus differs from tracked output. Review the diff above; rerun with --replace only to promote it." >&2
  exit 1
fi
echo "Tracked UBL corpora and structural coverage reports are current."
