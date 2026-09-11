# Regression contracts

This directory defines the version 1 semantic contract shared by corpus
generators and regression runners. The JSON schemas are the authority for the
machine-readable shape; this document fixes naming and normalization rules
that JSON Schema cannot express by itself.

## Files

- `schemas/fixture-manifest.schema.json` validates a complete corpus manifest.
- `schemas/normalized-result.schema.json` validates one line of the
  `results.jsonl` output.
- `testdata/*.example.json` are reviewable examples validated by the Go test
  suite.
- `ValidateFixtureManifest` applies the fixture schema, rejects duplicate IDs,
  and enforces lexicographically sorted rule IDs.
- `ValidateNormalizedResult` applies the normalized-result schema and enforces
  lexicographically sorted expected and actual rule IDs.

Both contracts use JSON Schema draft 2020-12 and start at `schemaVersion: 1`.
Changing the meaning of an existing field requires a new schema version.
Additive changes must still be reviewed because version 1 rejects unknown
properties.

EN 16931 fixtures also carry `rulesets.en16931.version` and the SHA-256 of
the exact pinned validation stylesheet. The version mirrors
`standards.en16931`; the digest makes a ruleset selection auditable even if an
upstream asset is ever replaced without changing its release label.

## Normalized regression runner

Run a complete manifest through the authenticated public API and durable
worker/result path with:

```bash
EUBL21_REGRESSION_TOKEN='<signed bearer token>' \
go run . regression \
  --base-url http://127.0.0.1:8080 \
  --manifest createsamples/samples-en16931/manifest.json \
  --profile all \
  --output regression-results/latest
```

The command verifies manifest and fixture hashes, uploads fixtures in stable ID
order with their requested validation profile, polls to a durable terminal
status, removes IDs/timestamps/durations and other volatile fields, and writes
`results.jsonl`, `summary.json`, and `junit.xml`. A separate volatile
`run-map.jsonl` correlates each fixture ID with its upload ID, document type,
profile, and terminal status for diagnostics. It returns non-zero after
writing the artifacts if any expected final outcome, stage outcome, or required
rule ID differs. Use the UBL manifest path to run the structural corpus.

## Validation-split equivalence gate

The reviewed monolithic-worker result sets live in
`baselines/validation-split/`. Both standard regression lifecycles compare each
canonical corpus with its baseline after producing `results.jsonl`, and write
`equivalence.json`. The comparison is stricter than the manifest's required
rule-ID subset: candidate and baseline actual rule-ID sets must be exactly
equal, as must fixture identities, hashes, profiles, final outcomes, and
ordered stage outcomes.

To compare an independently produced split-stage result set:

```bash
make regression-equivalence \
  BASELINE=regression/baselines/validation-split/en16931.results.jsonl \
  CANDIDATE=path/to/split-stage/results.jsonl \
  OUTPUT=path/to/split-stage/equivalence.json
```

Do not replace a baseline merely because a candidate differs. See the baseline
directory README for the review policy and the separate mandatory asynchronous
queue/restart/scale-to-zero scenarios.

For any UBL, EN 16931, or Peppol version change, follow the canonical
[`standards upgrade workflow`](../docs/STANDARDS-UPGRADE-WORKFLOW.md). It
defines how to preserve the accepted result set, stage candidate corpora,
produce normalized old/new evidence, classify every changed fixture, and only
then promote deliberate manifest and baseline updates.

## Diagnostic evidence collector

Collect bounded diagnostics into the same artifact directory after every run,
including failed runs:

```bash
go run . regression collect-evidence \
  --manifest createsamples/samples-en16931/manifest.json \
  --output regression-results/latest \
  --log api=/tmp/e-ubl21-api.log \
  --log worker=/tmp/e-ubl21-worker.log \
  --metrics api=http://127.0.0.1:8080/metrics \
  --metrics worker=http://127.0.0.1:9091/metrics \
  --nats-state \
  --nats-url nats://127.0.0.1:4222 \
  --queue-namespace eubl21.dev
```

For a deployed run, add `--kubernetes`, `--kubernetes-namespace NAME`, and
the deployment's `--kubernetes-selector`. The collector uses `kubectl` to
capture events, a sanitized pod/container snapshot, and the last 2,000 log
lines per selected container, including previous-container logs after a
restart. It also records Kubernetes and Minikube version commands in
`environment.json`.

The diagnostic files are separate from normalized correctness results:

```text
environment.json
manifest-used.json
collection.json
run-map.jsonl
logs/*.log
metrics/*.prom
kubernetes/events.json
kubernetes/pods.json
nats/state.json
```

`run-map.jsonl` is produced by the runner rather than the collector and is not
part of deterministic semantic comparison. Logs are capped at 4 MiB per
source, metrics at 2 MiB per endpoint, and command
output at 4 MiB. Remote sources and commands have a configurable per-source
deadline (`--source-timeout`, 15 seconds by default). Bearer credentials and
Invoice/CreditNote XML blocks are redacted before logs are written. Kubernetes
pod data is reduced to names, images, readiness, restart counts, and image IDs,
so environment values and
Secrets are not copied. NATS evidence contains only selected namespaced stream
and consumer configuration/counters; it never reads messages or raw upload
objects. Requested-source failures are retained in `collection.json`, other
sources are still attempted, and the command exits non-zero after collection.

`environment.json` records the commit and dirty state, manifest generator
commits, UBL/EN/Saxon/XML Resolver versions and digests, Go/Java/xmllint
versions, optional image digest, optional Kubernetes/Minikube/NATS versions,
and test start/end timestamps. Supply `--started-at` and `--image-digest` (or
`EUBL21_IMAGE_DIGEST`) when the surrounding runner knows them.

## Deterministic UBL 2.1 positive corpus

The structural generator produces one manifest-driven, XSD-valid positive
fixture for every one of the 65 UBL 2.1 main document schemas:

```bash
go run ./createsamples generate \
  --profile ubl21 \
  --set positive \
  --xsd XSD \
  --output createsamples/samples
```

Generation resolves the latest full Git commit that changed generator source,
its manifest contract, or UBL XSD inputs for immutable provenance. It generates
the complete corpus twice and rejects any byte difference, computes each
fixture's SHA-256, validates the canonical manifest, and independently validates
every XML document with `xmllint` before replacing the output tree. Automatic
provenance rejects uncommitted changes to those inputs. Tests and generated
artifacts do not change that provenance version. The canonical manifest is
written as `<output>/manifest.json`. Use
`--generator-version git:<40-or-64-lowercase-hex-commit>` when generation runs
outside a Git checkout or when deliberately overriding the clean-input guard.

Generated output contains `.e-ubl21-generated-corpus`, an ownership marker.
Replacement is staged in a temporary sibling directory and is permitted only
when an existing output directory contains the valid marker; an arbitrary
directory is never recursively cleared.

## Deterministic UBL 2.1 targeted-negative corpus

The canonical checked-in UBL corpus contains both sets and is regenerated with:

```bash
go run ./createsamples generate \
  --profile ubl21 \
  --set all \
  --xsd XSD \
  --output createsamples/samples
```

Peppol BIS Billing candidate positives are generated separately:

```bash
go run ./createsamples generate \
  --profile peppol-bis-billing \
  --set positive \
  --xsd XSD \
  --output createsamples/samples-peppol
```

This corpus contains deterministic Invoice and CreditNote variants for Profile
01 and Profile 02. Each fixture pins the Peppol BIS version, EN and Peppol
runtime artefact digests, endpoint/party identifier schemes, and the expected
XSD -> EN 16931 -> Peppol stage sequence. Peppol negative mutations are not
generated by this target; they belong to issue #38.

Use `--set negative` to generate only the six targeted Invoice mutations. Each
starts from the same compact XSD-valid baseline and introduces one catalogued
defect:

| Mutation ID | Deliberate defect |
| --- | --- |
| `missing-required-element` | Removes the required top-level `cbc:ID`. |
| `invalid-datatype` | Uses a non-boolean `cbc:CopyIndicator`. |
| `invalid-namespace` | Places the required ID in an unrecognized namespace. |
| `wrong-element-order` | Places `cbc:IssueDate` before `cbc:ID`. |
| `invalid-enumeration` | Uses a value outside the imported XAdES qualifier enumeration. |
| `malformed-xml` | Removes the closing `Invoice` tag. |

Every negative fixture records the mutation ID and exact reason in its manifest
entry, expects only `ubl21-xsd = standards-invalid`, and is independently
required to fail with the diagnostic associated with that mutation. Tests also
run the tracked fixtures through the current validation pipeline and prove that
the EN 16931 validator is never invoked after the structural failure.

## Deterministic EN 16931 corpus

The EN 16931 semantic generator is available through the same CLI and writes a
separate profile corpus:

```bash
go run ./createsamples generate \
  --profile en16931 \
  --set all \
  --xsd XSD \
  --output createsamples/samples-en16931
```

The corpus covers generated Invoice variants for VAT categories S, Z, E, AE,
G, K, and O, a generated standard-rated CreditNote, and the two byte-pinned
official upstream guide examples. Every entry expects ordered UBL XSD and EN
16931 passes, records ruleset version `1.3.16` and the packaged stylesheet
digest, and is independently checked against its UBL schema before output is
installed. Official examples use the common `official-import` provenance
contract rather than generator provenance.

The same tracked corpus also contains 22 targeted-negative Invoices generated
from compliant VAT-family baselines. Each negative changes one business
condition, remains UBL 2.1 XSD-valid, and records its stable mutation ID,
human-readable description, and expected official EN 16931 rule IDs. Generate
only this subset with `--set negative`. The real Saxon acceptance gate reads
the tracked manifest and requires every declared rule ID to appear as an
invalidating failed assertion from the pinned `1.3.16` ruleset.

## Stable names

Fixture IDs are lowercase dotted identifiers. They identify the semantic test
case and must not contain a filesystem location, standards release, generated
timestamp, or random value. A manifest must not contain duplicate fixture IDs.

Examples:

```text
ubl21.invoice.minimal
en16931.invoice.br-co-15
peppol.invoice.invalid-endpoint-scheme
```

Version 1 profile and required-stage sequences are fixed:

| Profile | Required stages, in order |
| --- | --- |
| `ubl21` | `ubl21-xsd` |
| `ubl21-en16931` | `ubl21-xsd`, `en16931` |
| `peppol-bis-billing` | `ubl21-xsd`, `en16931`, `peppol-bis-billing` |

Expected stage outcomes are `passed` and `standards-invalid`. An EN 16931 or
Peppol `standards-invalid` expectation must name at least one official rule ID.
XSD diagnostics do not have an equivalent portable rule-ID contract, so XSD
negative fixtures do not require one.

Rule IDs belong to the stage that emitted them. Arrays must be deduplicated and
sorted lexicographically before serialization. They preserve exact spelling
from the pinned official rule set; consumers must not case-fold or rewrite
them.

Final fixture expectations are `valid` or `invalid`. Actual normalized results
also permit `operational-failure`, which must never be translated to
`invalid`. An operationally failed stage has no standards rule IDs.

The first accepted Peppol release baseline is stored in
`baselines/peppol/`. Its metadata binds normalized results to the complete
corpus, reviewed reference evidence, and pinned release digests. Run
`make test-peppol-compliance` to verify that baseline before exercising the
same documents through the real public asynchronous pipeline. A baseline
change is a reviewed standards-release event, never an automatic update from a
candidate run.

## Fixture provenance

Every fixture records one of two provenance forms:

- `generated` identifies the generator and its immutable full Git commit as
  `git:<40-or-64-lowercase-hex-digits>`.
- `official-import` identifies the publisher, exact asset/archive URL and
  release, SHA-256 of the bytes obtained from that source, archive member (or
  `null` for a direct asset), extraction/transformation description, and
  license/provenance note.

`sha256` on the fixture itself always hashes the exact XML file named by
`file`. `sourceSha256` records the imported upstream asset and can differ when
an archive contains the fixture or when a documented extraction is applied.
Generated targeted-negative fixtures additionally carry a `mutation` object
with a stable lowercase-hyphenated ID and a human-reviewable description of the
single deliberate defect.

## Semantic normalization

Only fields admitted by `normalized-result.schema.json` participate in the
deterministic correctness result. The following values are deliberately
excluded and belong in diagnostic artifacts such as `environment.json`, logs,
metrics, or traces:

- upload, validation, request, message, and correlation IDs;
- timestamps and durations;
- pod, node, container, and worker names;
- NATS or JetStream stream/consumer sequence values and delivery counts;
- trace and span IDs;
- log ordering and raw validator output.

The ordered stage outcomes and their sorted rule IDs are semantic. Human error
messages, locations, XPath tests, and raw SVRL/XML are diagnostic and are not
part of the normalized equality comparison.

The runner sets `result` to `pass` only when the expected and actual final
outcomes and ordered stage outcomes are equal. Every expected rule ID must be
present in its actual stage; additional invalidating rule IDs are retained in
`results.jsonl` without failing a targeted-negative fixture because one
deliberate defect can trigger a legitimate rule cascade. JSON Schema checks
the result shape, while the runner enforces this cross-field comparison.
