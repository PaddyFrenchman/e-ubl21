# UBL structural corpus API selection evidence

## Local regression run

The local regression run completed at `2026-10-16T09:58:32Z` with all four
suites passing. Its report is
`regression-results/local/20261016T090958Z/report.md`.

| Structural corpus measure | Count |
|---|---:|
| Total generated structural fixtures | 24,713 |
| Selected for authenticated API regression | 22,136 |
| Excluded because the root is not a UBL main document | 2,577 |
| Selected fixtures submitted and terminally evaluated | 22,136 |
| Selected fixtures passed | 22,136 |

The API selection includes every structural fixture whose `documentType`
appears in the canonical main-document corpus. The exclusions are standalone
common-component, XMLDSig/XAdES, and schema-type roots, which the upload API
does not accept as UBL main documents. Selection is based on API support, not
on validation results.

## Independent XSD validation of excluded fixtures

The structural corpus was regenerated successfully before this regression
run. Its manifest identifies generator version
`git:ffcbb964a79b3ac4eb696b9b345f4feae05fb896`. During generation,
`createsamples` independently validates each fixture against its
fixture-specific XSD with `xmllint --nonet --noout --schema`; generation fails
and does not install the corpus if a valid fixture fails XSD validation. The
excluded 2,577 fixtures were therefore independently XSD-validated during
corpus generation. Their manifest entries also expect the `ubl21-xsd` stage
to pass.

The generated corpus is locally ignored by Git. The regression report generator
now emits the selected and excluded counts, the excluded fixture IDs, and a
machine-readable `selection-evidence.json` alongside future regression
artifacts.
