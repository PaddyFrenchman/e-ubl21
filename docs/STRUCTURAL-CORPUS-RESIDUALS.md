# UBL 2.1 structural corpus coverage status

## Status

This document records structural coverage of the locally generated UBL 2.1
corpus. The multi-gigabyte structural corpus is intentionally ignored by Git.
The exhaustive, machine-readable report is retained locally at
`createsamples/samples-structural/coverage.json`; its readable rendering is
`createsamples/samples-structural/coverage.md`. Both are regenerated with the
corpus and take precedence over the counts in this document.

The corpus generated from commit `5840d2c2` contains 24,713 fixtures. The
coverage report passes with zero uncovered targets, zero missing required
cardinalities, zero cardinality residuals, and zero unparseable fixtures. Every
nonempty target category in the report is at 100% coverage. The formerly
uncovered XMLDSig/XAdES, global component, and UDT/CCTS target groups are now
covered by generated fixtures. No schema targets have been excluded.

| Target kind | Remaining |
|---|---:|
| attributes | 0 |
| choice alternatives | 0 |
| choice sites | 0 |
| complex types | 0 |
| elements | 0 |
| inheritance edges | 0 |
| optional attributes | 0 |
| optional elements | 0 |
| repeatable elements missing required cardinalities | 0 |
| required attributes | 0 |
| required elements | 0 |
| sequence members | 0 |
| wildcards | 0 |

The generated report's distinct uncovered-target map is empty. Category counts
overlap, so the report's distinct target IDs remain the source of truth.

## Review classification

### Previously uncovered target groups

The following groups were addressed in the regenerated corpus:

- The remaining XMLDSig signature-object and signature-property branches and
  attributes, plus their associated wildcard positions.
- The remaining XAdES 1.3.2 and 1.4.1 qualification graph: certificate and
  revocation references and values, policy qualifiers, signer roles,
  counter-signatures, timestamp/reference forms, validation data, and their
  IDs/URIs/encodings.
- Global component declarations not reachable from a main document root.
- UDT/CCTS types and attributes, including graphic, picture, sound, video,
  date-time, and value forms not observed in the current corpus.
- Remaining UBL global component declarations and their required-element
  targets. The dependency map identifies which are reachable from main-document
  roots and which need review as standalone or wildcard-boundary constructs.

The generator has standalone CAC-root fixtures for the seven uncovered global
declarations (`Branch`, `Communication`, `Duty`, `MonetaryTotal`, `Payment`,
`Regulation`, and `ServiceFrequency`). It now also adds wrapper-validated type
roots for the eight otherwise unreachable CCTS/UDT types, a standalone UBL
`SignatureInformation` root, standalone XAdES 1.4.1 roots and targeted XAdES
choice branches. Strict wildcard fixtures use
declared elements from the root schema's import closure. The matcher now
credits simple-typed global roots, named attribute types, and the correct
choice branch when alternatives contain the same element name. Focused tests
cover these additions. The refreshed coverage report confirms they now have
fixture witnesses.

### Bounded by design; approval requested

The generator intentionally has these safety bounds. They are not unsupported
schema constructs, but they limit what a fixture proves:

- General cardinality witnesses exercise unbounded particles through five
  siblings. Dedicated large-collection fixtures separately exercise Invoice
  lines at 10, 50, 100, and 500; CreditNote lines at 10, 50, 100, and 250; and
  Order lines at 10, 50, 100, and 250. These are test-data bounds, not runtime
  limits, and do not enumerate every possible collection size.
- Recursive fixtures use explicit depth-2 and depth-3 generation variants;
  arbitrary recursion depth is not enumerated. The report's `maximumDepth`
  measures repeated occurrences of the same complex type along an ancestor
  path. Repeated siblings may cover a recursive schema edge but do not increase
  that depth.
- Wildcard fixtures use declared foreign elements where strict processing
  requires them; they do not attempt arbitrary external schemas.
- XMLDSig/XAdES fixtures prove UBL/XSD structural acceptance only. They do not
  assert cryptographic signature validity, certificate trust, revocation, or
  timestamp semantics.

## Approval record

The repository user accepted the safety bounds and semantic limits listed
above on 2026-10-15 for issue #167. This acceptance covers structural XSD
qualification only; it does not assert cryptographic signature validity,
certificate trust, revocation, timestamp semantics, arbitrary recursion depth,
or exhaustive repeat counts.

The generated report has no uncovered schema targets, cardinality gaps, or
cardinality residuals. Together with the accepted bounds above, the structural
corpus is qualified for issue #167.
