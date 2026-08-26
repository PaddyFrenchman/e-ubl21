# Regression contracts

This directory defines the version 1 semantic contract shared by corpus
generators and regression runners. The JSON schemas are the authority for the
machine-readable shape; this document fixes naming and normalization rules
that JSON Schema cannot express by itself.

## Files

- `schemas/fixture-manifest.schema.json` validates a complete corpus manifest.
- `schemas/normalized-result.schema.json` validates one line of the future
  `results.jsonl` output.
- `testdata/*.example.json` are reviewable examples validated by the Go test
  suite.
- `ValidateFixtureManifest` applies the fixture schema and rejects duplicate
  IDs across manifest entries, which JSON Schema cannot express by property.

Both contracts use JSON Schema draft 2020-12 and start at `schemaVersion: 1`.
Changing the meaning of an existing field requires a new schema version.
Additive changes must still be reviewed because version 1 rejects unknown
properties.

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
outcomes, ordered stages, and per-stage rule IDs are equal. JSON Schema checks
the shape of a result; the runner introduced by TODO-REGRESSION-008 is
responsible for that cross-field equality comparison.
