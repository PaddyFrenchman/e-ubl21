# Japan — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### jp/pint-jp 1.1.3

Official package: [https://docs.peppol.eu/poac/jp/pint-jp/resources.zip](https://docs.peppol.eu/poac/jp/pint-jp/resources.zip). ZIP SHA-256: `2d0bb36c14106649f8c14c3e2377c0bd027fb434bd908afeb06900ee620aa570`.

- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 43 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/222df76439f04fcf.md).

### jp/pint-jp-ntr 1.1.3

Official package: [https://docs.peppol.eu/poac/jp/pint-jp-ntr/resources.zip](https://docs.peppol.eu/poac/jp/pint-jp-ntr/resources.zip). ZIP SHA-256: `53b57443e45c442db6ed36ee03b528d87114b37c82d59b919c6950697affe33d`.

- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/ba8eb7c171a1ea1f.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 41 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/dc6ddee5a65d421b.md).

### jp/pint-jp-sb 1.1.3

Official package: [https://docs.peppol.eu/poac/jp/pint-jp-sb/resources.zip](https://docs.peppol.eu/poac/jp/pint-jp-sb/resources.zip). ZIP SHA-256: `b681345d9b5bb44e2e7c75d45f211b19e01340556f589fd39263b4cc8d4d1d6a`.

- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/b7bd42db327e4a04.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 42 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/61ed9c95702c210e.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
