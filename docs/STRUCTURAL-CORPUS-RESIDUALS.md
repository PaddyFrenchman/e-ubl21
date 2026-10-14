# UBL 2.1 structural corpus residual coverage

## Status

This document records the residual structural coverage of the checked-in UBL
2.1 corpus. It is a review and approval record, not a claim of complete schema
coverage. The exhaustive, machine-readable list is
`createsamples/samples-structural/coverage.json`; its readable rendering is
`createsamples/samples-structural/coverage.md`. Both are regenerated with the
corpus and take precedence over the counts in this document.

The current report analyzes 24,692 fixtures. It records no missing required
cardinalities (`cardinalityGaps: 0`, `cardinalityResiduals: 0`) and zero
unparseable fixtures. It still lists 1,771 distinct uncovered schema targets
across 3,476 category entries. These remaining targets are not approved for
exclusion; the generated report is the detailed register.

| Target kind | Remaining |
|---|---:|
| attributes | 10 |
| choice alternatives | 4 |
| choice sites | 2 |
| complex types | 32 |
| elements | 1,690 |
| inheritance edges | 30 |
| optional attributes | 6 |
| optional elements | 5 |
| recursive edges | 2 |
| repeatable elements missing required cardinalities | 0 |
| required attributes | 4 |
| required elements | 1,685 |
| sequence members | 3 |
| wildcards | 3 |

Category counts overlap; the report contains 1,771 distinct uncovered targets.

## Review classification

### Requires future fixture work

These targets are schema-reachable and are not approved as unsupported:

- The remaining XMLDSig signature-object and signature-property branches and
  attributes, plus their associated wildcard positions.
- The remaining XAdES 1.3.2 and 1.4.1 qualification graph: certificate and
  revocation references and values, policy qualifiers, signer roles,
  counter-signatures, timestamp/reference forms, validation data, and their
  IDs/URIs/encodings.
- Uncovered wildcards, recursive edges, choice branches, attributes, and
  required or optional document elements identified in the generated register.
- UBL document content not present in the representative root fixtures,
  including the 1,715 required-element targets and 1,787 total element targets.

These are ordinary coverage gaps. They must remain visible in the generated
register and should be addressed by adding XSD-valid fixtures, not by excluding
them from accounting.

### Bounded by design; approval requested

The generator intentionally has these safety bounds. They are not unsupported
schema constructs, but they limit what a fixture proves:

- Unbounded repeatable particles are exercised only through five siblings;
  this is a test-data safety cap, not a runtime limit.
- Recursive structures are finite and use the generator's recorded maximum
  depth; arbitrary recursion depth is not enumerated.
- Wildcard fixtures use declared foreign elements where strict processing
  requires them; they do not attempt arbitrary external schemas.
- XMLDSig/XAdES fixtures prove UBL/XSD structural acceptance only. They do not
  assert cryptographic signature validity, certificate trust, revocation, or
  timestamp semantics.

## Approval record

The approval recorded on 2026-10-02 was withdrawn by the repository user on
2026-10-02. No coverage boundary is approved.

Every schema-reachable construct and each documented boundary behavior remains
an open coverage obligation. The generated `coverage.json`/`coverage.md`
register remains the source of truth for the residual target list. In
particular, cardinality enforcement remains a qualification gate. The current
generated report passes that gate with no missing required cardinalities. The
other uncovered targets remain open until reviewed and explicitly
dispositioned.
