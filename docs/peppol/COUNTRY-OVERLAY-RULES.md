# Peppol country overlays: identification and test mapping

This resource tells Codex how to identify, document and test country-specific validation rules. It supplements [country resources](countries/README.md) and [the country support plan](../country-support.md). It is not a complete extraction of all country rules.

## Authoritative sources

- [Official BIS Billing rule catalogue](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/): identifiers, requirements, severity and individual rule pages.
- [Official BIS Billing specification and downloads](https://docs.peppol.eu/poacc/billing/3.0/): semantic model, syntax, code lists and Schematron.
- [Official post-award directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/): separate jurisdiction-specific profiles, including PINT specialisations.
- Repository acquisition script: [standard/peppol/download.sh](../../standard/peppol/download.sh).

Country-prefixed identifiers such as DE-R and DK-R are useful discovery aids, but are not an exhaustive detector. Common rules may contain country-specific identifier checks. Inspect every assertion and its variable dependencies.

## Release identity

The development acquisition script pins BIS Billing 3.0.21, source commit `806866bd2bd91d7e9623b68f08164e8fbe9e67a0`, archive SHA-256 `a843cd3808c6e95565eded1a535a5a40a88ad6f92e009d2b1816a4818a82299e`, with EN16931 validation artefacts 1.3.16. Publication: 2026-05-20; mandatory use: 2026-08-17.

Run `sh standard/peppol/download.sh` to acquire and verify the source. The Schematron is under `standard/peppol/downloaded/3.0.21/source/rules/sch/PEPPOL-EN16931-UBL.sch`. This document does not claim that acquisition was executed when it was written.

Live documentation links can advance. Before coding, compare each extracted rule with the pinned source; record source path, assertion ID and checksum. Record jurisdiction-specific profile versions separately.

## Selecting and applying rules

1. Identify document type, CustomizationID and ProfileID. Resolve them against explicitly supported profiles and releases.
2. Execute the applicable UBL schema, EN16931 and selected Peppol/profile validation layers.
3. Let the official Schematron context and test determine applicability. Preserve namespace bindings, global/local variables, functions, inclusions and country determination expressions.
4. Do not replace those expressions with a generic seller postal-country switch. Some rules depend on buyer country, both parties, identifier schemes, tax identifiers, payment means or document type.
5. Separate pass, failure, warning and not-applicable outcomes. Preserve assertion IDs in diagnostics.
6. Unsupported profiles require an explicit unsupported result; do not silently select a generic Peppol profile.

Standard BIS Billing profile validation can be offered for documents from EU member states, including Ireland, where the declared profile is supported. Country-specific resources are an additional coverage inventory, not a geographic allowlist. A passing result establishes conformity with the selected profile and release; wider national tax/reporting obligations require separate assessment.

## Verified examples

The expressions below were checked against the official May 2026 individual rule pages on 2026-10-02. Variable definitions must still be resolved from the pinned Schematron before implementation.

| Rule | Applicability | Required check | Severity | Official source |
| --- | --- | --- | --- | --- |
| DE-R-001 | Invoice or CreditNote where both supplier and customer are identified as German by the rule variables | Payment instructions group is present | fatal | [DE-R-001](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/DE-R-001/) |
| DK-R-002 | Invoice or CreditNote where the rule's supplier-country variable is DK | Seller legal registration identifier has non-whitespace content | fatal | [DK-R-002](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/DK-R-002/) |

### DE-R-001

Context:

```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:

```xpath
cac:PaymentMeans
```

Sample mapping: applicable-valid with PaymentMeans; applicable-invalid without it; supplier DE/buyer outside DE and supplier outside DE/buyer DE cases confirming this assertion does not fire. Cover Invoice and CreditNote. Other rules may still reject those fixtures.

### DK-R-002

Context:

```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK'] | ubl-invoice:Invoice[$DKSupplierCountry = 'DK']
```

Test:

```xpath
(normalize-space(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID/text()) != '')
```

Sample mapping: applicable-valid with populated seller CompanyID; applicable-invalid with missing, empty and whitespace-only CompanyID; non-DK supplier case confirming this assertion does not fire. Cover Invoice and CreditNote. Passing this assertion alone does not establish that an identifier is otherwise valid.

## Required record for every extracted rule

Each country resource must record:

- Profile, release, document types, rule ID and severity.
- Official rule URL, pinned source file, checksum and assertion location.
- Exact context, test, namespace bindings, referenced variable/function definitions and include dependencies.
- Plain-language applicability and relevant business-term/UBL paths.
- Valid, invalid, boundary and non-applicable fixture IDs.
- Expected assertion IDs and severities; additional expected failures if a fixture cannot isolate one rule.
- Verification state: inventoried, extracted, source-verified, fixture-covered and regression-verified.

An overview or a link alone is not an implementation-ready record.

## CreateSamples and regression requirements

CreateSamples remains the main generation engine. Add country/profile/release selection and deterministic per-rule mutations. Store fixture identities, hashes, applicable countries/profile, source rule version, expected validity, assertion IDs and warnings in manifests.

Generate fixtures offline; regression consumes the stored corpus without regenerating it. Verify both expected failures and absence of the target assertion in non-applicable cases. Report document path, profile/release, country inputs, expected versus actual assertions and actionable failure evidence for Codex.

Reuse the existing official validator rather than rewriting XPath rules by hand. Additional development should address profile coverage, diagnostics and reproducible fixtures unless a verified source comparison establishes missing validation logic.

## Completion gate

This reference currently contains two verified examples and the extraction contract. Full country coverage is not complete. Mark a country/profile implementation-ready only after every applicable assertion and dependency is inventoried, pinned, documented and covered by generated fixtures and regression evidence.
