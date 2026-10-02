# Oman — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### om/om-tdd 1.0.1

Official package: [https://docs.peppol.eu/poac/om/om-tdd/resources.zip](https://docs.peppol.eu/poac/om/om-tdd/resources.zip). ZIP SHA-256: `8750f70b4ab787ee6ead44103a5ab8030487408420d0ef5e943013c8fa904373`.

- `trn-tdd/schematron/peppol-om-tdd.sch`: 66 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/ffe8a198e3e60790.md).

### om/pint-om 1.0.1

Official package: [https://docs.peppol.eu/poac/om/pint-om/resources.zip](https://docs.peppol.eu/poac/om/pint-om/resources.zip). ZIP SHA-256: `023f57b9cc13ae641dab0c1726fc8167b86c125837875da98274cc6c0aaf8402`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 154 assertions, 1 reports; [complete expressions and dependencies](../extracted/pint/8652075b907b83dc.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 154 assertions, 1 reports; [complete expressions and dependencies](../extracted/pint/8652075b907b83dc.md).

### om/pint-om-sb 1.0.1

Official package: [https://docs.peppol.eu/poac/om/pint-om-sb/resources.zip](https://docs.peppol.eu/poac/om/pint-om-sb/resources.zip). ZIP SHA-256: `0f8095fecc15393d792cfddfd4cfc56b58e6fc3588b02b0e7c47df9de5fb532b`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/1f91639206c8377a.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 154 assertions, 1 reports; [complete expressions and dependencies](../extracted/pint/ab44bc25c0c52047.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/1f91639206c8377a.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 154 assertions, 1 reports; [complete expressions and dependencies](../extracted/pint/ab44bc25c0c52047.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
