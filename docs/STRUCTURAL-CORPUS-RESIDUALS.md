# UBL 2.1 structural corpus residual coverage

## Status

This document records the residual structural coverage of the checked-in UBL
2.1 corpus. It is a review and approval record, not a claim of complete schema
coverage. The exhaustive, machine-readable list is
`createsamples/samples-structural/coverage.json`; its readable rendering is
`createsamples/samples-structural/coverage.md`. Both are regenerated with the
corpus and take precedence over the counts in this document.

The current report analyzes 578 fixtures. It records 3,925 uncovered targets:

| Target kind | Remaining |
|---|---:|
| attributes | 38 |
| choice alternatives | 21 |
| choice sites | 6 |
| complex types | 69 |
| elements | 1,787 |
| inheritance edges | 33 |
| optional attributes | 31 |
| optional elements | 72 |
| recursive edges | 22 |
| repeatable elements | 35 |
| required attributes | 7 |
| required elements | 1,715 |
| sequence members | 83 |
| wildcards | 6 |

## Review classification

### Requires future fixture work

These targets are schema-reachable and are not approved as unsupported:

- The remaining XMLDSig signature-object and signature-property branches and
  attributes, plus their associated wildcard positions.
- The remaining XAdES 1.3.2 and 1.4.1 qualification graph: certificate and
  revocation references and values, policy qualifiers, signer roles,
  counter-signatures, timestamp/reference forms, validation data, and their
  IDs/URIs/encodings.
- Uncovered wildcards, recursive edges, and cardinality combinations identified
  in the generated register.
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
register remains the source of truth for the residual target list.
