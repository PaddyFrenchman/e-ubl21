# Australia — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Australian identifier assertion PEPPOL-COMMON-R050 is in the common extraction.

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### aunz/pint-aunz 1.1.3

Official package: [https://docs.peppol.eu/poac/aunz/pint-aunz/resources.zip](https://docs.peppol.eu/poac/aunz/pint-aunz/resources.zip). ZIP SHA-256: `277206b650f22689b6f71de38e7e92619de315625910efb3fd89a4d1e039ea5d`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 74 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/5016cb5aec794518.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 74 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/5016cb5aec794518.md).

### aunz/pint-aunz-sb 1.1.3

Official package: [https://docs.peppol.eu/poac/aunz/pint-aunz-sb/resources.zip](https://docs.peppol.eu/poac/aunz/pint-aunz-sb/resources.zip). ZIP SHA-256: `60e481d13da1cd7d1692333c2e30e6743cd0d401c290fb072e6626e600cea1df`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/b9c890c4ae5a15c3.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 74 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/710948f8d333b83f.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/b9c890c4ae5a15c3.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 74 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/710948f8d333b83f.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
