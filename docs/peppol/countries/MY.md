# Malaysia — Peppol implementation resource

Extraction date: 2026-10-02. [Rule selection contract](../COUNTRY-OVERLAY-RULES.md).

Select the declared supported profile and its release; country alone does not select a validator. Preserve the exact contexts and scoped dependencies in the linked technical extraction.

## BIS Billing 3.0.21

Always include [all 63 common assertions](../extracted/COMMON-BIS-RULES.md) and [global definitions](../extracted/BIS-DEPENDENCIES.md) when BIS Billing is the declared profile. These include country-dependent exceptions and identifier checks. Do not universally apply BIS rules to PINT specialisations.

## Jurisdiction-specific packages

### my/pint-my 1.3.1

Official package: [https://docs.peppol.eu/poac/my/pint-my/resources.zip](https://docs.peppol.eu/poac/my/pint-my/resources.zip). ZIP SHA-256: `bcfa311ffcb082f535c911d12a6b8537115b55e630c71fa1ccf47272271dd3d8`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 28 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/9d2a3ca98fcdfbfd.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/4e66ec0e7659e82c.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 28 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/9d2a3ca98fcdfbfd.md).

### my/pint-my-sb 1.3.1

Official package: [https://docs.peppol.eu/poac/my/pint-my-sb/resources.zip](https://docs.peppol.eu/poac/my/pint-my-sb/resources.zip). ZIP SHA-256: `a071cddd2c3ab3e9f9a81fa430b06d52c0ef35625d092cd19059939e8730d80f`.

- `trn-creditnote/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/1911a29636864a68.md).
- `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`: 28 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/0e7c3b5f2f943198.md).
- `trn-invoice/schematron/PINT-UBL-validation-preprocessed.sch`: 170 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/1911a29636864a68.md).
- `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`: 28 assertions, 0 reports; [complete expressions and dependencies](../extracted/pint/0e7c3b5f2f943198.md).

## Test and implementation requirements

Each extracted rule defines planned valid/invalid/boundary/non-applicable fixture identifiers. CreateSamples must qualify these by country, profile, release, transaction and rule. Fixture mutations must satisfy or negate the exact XPath while controlling other rules. Manifest expected assertion IDs and severity; run stored fixtures through regression without regeneration. Planned identifiers are not generated samples or proof of support.

Source code lists, schemas and official examples are inventoried with hashes in [the package manifest](../extracted/PACKAGE-MANIFEST.md). Acquire the exact ZIP with the provided downloader; verify the checksum before use. Reuse official Schematron/XSLT rather than independently rewriting validation logic.
