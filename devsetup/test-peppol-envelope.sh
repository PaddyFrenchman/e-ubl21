#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

for command_name in java curl sha256sum unzip; do
    command -v "$command_name" >/dev/null 2>&1 || { echo "ERROR: $command_name is required" >&2; exit 1; }
done

SAXON_DIR="$ROOT_DIR/devsetup/tools/saxon"
SCHXSLT_DIR="$ROOT_DIR/devsetup/tools/schxslt"
if [[ ! -f "$SAXON_DIR/saxon-he-12.7.jar" || ! -f "$SAXON_DIR/xmlresolver-6.0.17.jar" ]]; then
    "$ROOT_DIR/devsetup/download_saxon_he.sh" "$SAXON_DIR"
fi
if [[ ! -f "$SCHXSLT_DIR/schxslt-1.10.1/2.0/pipeline-for-svrl.xsl" ]]; then
    "$ROOT_DIR/devsetup/download_schxslt.sh" "$SCHXSLT_DIR"
fi

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
classpath="$SAXON_DIR/saxon-he-12.7.jar:$SAXON_DIR/xmlresolver-6.0.17.jar"
stylesheet="$work_dir/envelope.xslt"
java -cp "$classpath" net.sf.saxon.Transform -config:validation-assets/saxon/saxon-config.xml -ext:off \
    -s:validation-assets/peppol-envelope/3.0.21/rules/PeppolBIS-Envelope-Rules.sch \
    -xsl:"$SCHXSLT_DIR/schxslt-1.10.1/2.0/pipeline-for-svrl.xsl" \
    '?schxslt.compile.metadata=false()' -o:"$stylesheet"

run_fixture() {
    local fixture="$1"
    local expected_rule="$2"
    local output="$work_dir/$fixture.svrl"
    java -cp "$classpath" net.sf.saxon.Transform -config:validation-assets/saxon/saxon-config.xml -ext:off \
        -s:"server/testdata/peppol-envelope/$fixture.xml" -xsl:"$stylesheet" -o:"$output"
    if [[ -z "$expected_rule" ]]; then
        ! rg -q '<svrl:failed-assert' "$output" || { echo "$fixture unexpectedly failed" >&2; exit 1; }
    else
        rg -q "id=\"$expected_rule\"" "$output" || { echo "$fixture did not emit $expected_rule" >&2; exit 1; }
    fi
}

run_fixture valid-billing ''
run_fixture valid-creditnote ''
run_fixture valid-self-billing ''
run_fixture invalid-payload-valid-envelope ''
run_fixture wrong-sender BR-SBDH-2
run_fixture wrong-receiver BR-SBDH-1
echo "Peppol SBDH envelope fixtures passed with the pinned official rules."
