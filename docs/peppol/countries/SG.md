# Singapore — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### sg/pint-sg 1.4.1

Official package: [https://docs.peppol.eu/poac/sg/pint-sg/resources.zip](https://docs.peppol.eu/poac/sg/pint-sg/resources.zip). ZIP SHA-256: `584353b4fed58474d3ec7e32cc99d24eb8f50c812ac7d4a4ae72e6e0566b2670`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 92 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/3f93031bd2e578bd.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 92 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/3f93031bd2e578bd.md).

### sg/sg-bis 3.0

Official package: [https://www.peppolguide.sg/billing/files/SG-BIS-Billing3-Schematron.zip](https://www.peppolguide.sg/billing/files/SG-BIS-Billing3-Schematron.zip). ZIP SHA-256: `77c7cbf5e686240e364cb88789d3902ff87202444f9a12fe64c35157edb23e51`.

- `rules/sch/SG-Billing3-UBL.sch`: 61 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/1cba06ecc9e33844.md).
- `rules/sch/SG-Subset-CEN-EN16931-UBL.sch`: 825 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/de82b7220a10d34b.md).
- `rules/sch/SG-Subset-PEPPOL-EN16931-UBL.sch`: 45 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/b598042371b5952c.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
