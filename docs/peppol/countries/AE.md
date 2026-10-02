# United Arab Emirates — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### ae/ae-tdd 1.0.4

Official package: [https://docs.peppol.eu/poac/ae/ae-tdd/resources.zip](https://docs.peppol.eu/poac/ae/ae-tdd/resources.zip). ZIP SHA-256: `56cb018a5ac0500f5e6e9841788c8b62d894942ff820ab5449d90fdeb9981649`.

- `trn-tdd/schematron/peppol-ae-tdd.sch`: 58 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/eeea49282cc1d83b.md).

### ae/pint-ae 1.0.4

Official package: [https://docs.peppol.eu/poac/ae/pint-ae/resources.zip](https://docs.peppol.eu/poac/ae/pint-ae/resources.zip). ZIP SHA-256: `1e8b0bd595c672ac9d6fc886ddd0eb8027cb39a1d26bebdcb1ada37b7eee491f`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/c073dd645db0401a.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 132 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/d0e767f246fa8e90.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/c073dd645db0401a.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 132 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/d0e767f246fa8e90.md).

### ae/pint-ae-sb 1.0.4

Official package: [https://docs.peppol.eu/poac/ae/pint-ae-sb/resources.zip](https://docs.peppol.eu/poac/ae/pint-ae-sb/resources.zip). ZIP SHA-256: `7e6e58f120132dd2d2a6acf21b4a9318f0d07f2685c30ff282fae0e2a6613d15`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/5dd815399dfd0c22.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 132 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/d0e767f246fa8e90.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/5dd815399dfd0c22.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 132 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/d0e767f246fa8e90.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
