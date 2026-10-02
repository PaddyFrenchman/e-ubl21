# Peppol official specs
Checked: 2026-10-02
Purpose: consolidated official-source catalogue and acquisition guide for UBL21, Createsamples and routing.
Status: reference document; not an official OpenPeppol publication or a verbatim reproduction of the complete specification collection.

## How to use this document
Peppol is a family of business, transport and governance specifications, not one specification file. This Markdown document gathers entry points and explains their implementation roles. Full normative texts and executable assets remain authoritative at the linked sources. It does not claim that all linked documents have been downloaded, read or implemented.

For complete offline source acquisition, preserve original files and their licences, then index them here. Do not replace Schematron, XSD, WSDL or cryptographic provenance with prose.

## Official source entry points
| Area | Authoritative source | Purpose |
| --- | --- | --- |
| Business documents and jurisdiction profiles | [Post-award directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/) | Release catalogue for invoice and related business specifications |
| Billing | [BIS Billing portal](https://docs.peppol.eu/poacc/billing/3.0/) | Semantics, syntax, validation downloads and examples |
| Transport | [eDelivery directory](https://peppol.org/documentation/technical-documentation/edelivery-documentation/) | Network specifications and associated assets |
| Transport publications | [eDelivery technical portal](https://docs.peppol.eu/edelivery/) | Detailed specifications linked by the directory |
| Governance | [Peppol website documentation menu](https://peppol.org/) | Agreements, operational procedures and Authority-specific requirements |
| Accreditation | [Support/Testbed](https://peppol.org/support/) | Conformance and onboarding guidance |

## Billing source package
The Billing portal provides:
- Business specification and compliance guidance.
- UBL Invoice/CreditNote syntax mappings.
- EN16931 and Peppol rule views.
- Envelope validation rules.
- Identifier, currency, country, unit, tax, payment and other code lists.
- Downloadable Schematron and example documents.

[National-rule methodology](https://docs.peppol.eu/poacc/billing/3.0/national-rules/) explains applicability. Country support also requires separate national-profile and tax-authority research; the Billing package alone is not worldwide legal coverage.

## Existing repository acquisition mechanism
The repository already includes [standard/peppol/download.sh](../standard/peppol/download.sh).
Inspection confirms these pinned identifiers:
- BIS release: 3.0.21.
- Official source commit: 806866bd2bd91d7e9623b68f08164e8fbe9e67a0.
- Archive SHA-256: a843cd3808c6e95565eded1a535a5a40a88ad6f92e009d2b1816a4818a82299e.
- Source: OpenPEPPOL/peppol-bis-invoice-3.
- Destination: standard/peppol/downloaded/3.0.21.

Run from the repository root:
```sh
sh standard/peppol/download.sh
```

The inspected script checks hashes and release identity and requires the source tree to include guides, syntax/profile material, national rules, examples and mappings. This document does not claim the command was run during its preparation. That mechanism covers the pinned Billing source tree, not every Peppol specification family.

## Jurisdiction-specific invoice sources
Follow the [official directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/) for current release identity:
| Family/jurisdiction | Official portal |
| --- | --- |
| International PINT | https://docs.peppol.eu/poac/pint/ |
| European Union | https://docs.peppol.eu/poac/eu/ |
| Australia/New Zealand | https://docs.peppol.eu/poac/aunz |
| Japan | https://docs.peppol.eu/poac/jp/ |
| Malaysia | https://docs.peppol.eu/poac/my/ |
| Singapore | https://docs.peppol.eu/poac/sg/ |
| Oman | https://docs.peppol.eu/poac/om/ |
| United Arab Emirates | https://docs.peppol.eu/poac/ae/ |

This list reflects the inspected directory and is not a claim that every country has a Peppol specialisation. Record missing jurisdictions in the worldwide registry and obtain their requirements from official national sources.

## Other business-document families
The same directory links Self-Billing and other BIS transactions. Assess these independently of ordinary Billing: a supported invoice does not establish support for orders, despatch, catalogues or responses.
Logistics, pre-award and continuous-transaction-control material are separate categories available through OpenPeppol's documentation menu. Catalogue applicable documents before claiming a complete all-family collection.

## Network source inventory
The inspected eDelivery directory identifies:
| Specification | Listed version |
| --- | --- |
| AS4 profile | 2.0.3 |
| SMP | 1.4.0 |
| SML | 1.3.0 |
| Business message envelope | 2.0.2 |
| Identifier policy | 4.4.0 |
| Network policy | 1.0.0 |
| Message Level Status | 1.1.0 |
| eDelivery code lists | 9.7 |

It also lists transport-security policy, statistics reporting, Directory assets, schemas/WSDLs, migrations and onboarding guidance. Listed versions are a dated snapshot, not automatically the versions implemented in our repository.

## Full collection and provenance workflow
For each selected specification:
1. Resolve its official publication and immutable release.
2. Record normative/informative status, jurisdiction, document scope, publication and effective dates.
3. Review licence and redistribution conditions.
4. Download the original specification and all required validation/code-list/schema/example dependencies.
5. Record URLs, source commit/archive identity and SHA-256 for each asset.
6. Store original bytes in a controlled source directory or artefact store.
7. Generate an offline index and an acquisition report listing missing or failed downloads.
8. Bind validator and Createsamples configuration to the same release.
9. Verify with official examples, negative fixtures and reference checks.
10. Review upgrades rather than tracking moving latest/upcoming URLs at runtime.

A complete collection claim requires a defined release snapshot and inventory plus successful acquisition and verification of every in-scope item. This Markdown catalogue alone does not meet that gate.

## Project ownership
- UBL21 consumes applicable validation artefacts.
- Createsamples consumes matching jurisdiction/profile constraints to generate reproducible fixtures.
- Router consumes identifiers, discovery requirements and provider contracts.
- A self-operated AP additionally implements network specifications and fulfils governance/accreditation requirements.

## Related development documents
- [Country support plan](country-support.md)
- [Provider routing](PEPPOL-ROUTE.md)
- [Peppol accreditation](PEPPOL-CERTIFICATION.md)
- [ISO and Peppol certification requirements](ISO-27001-CERTIFICATION.md)
- [Current validation contract](PEPPOL-VALIDATION-CONTRACT.md)

## Jurisdiction-specific specification catalogue
Snapshot: official OpenPeppol post-award directory checked 2026-10-02. The rows below include all jurisdiction-specific invoice, self-billing and tax-data entries in that inspected directory. They are references to specifications, not a reproduction of their complete normative content.

| Jurisdiction | Specification family | Listed release | Official source |
| --- | --- | --- | --- |
| EU | BIS Billing | 3.0 / repository pin 3.0.21 | https://docs.peppol.eu/poacc/billing/3.0/ |
| EU | BIS Self-Billing | 3.0.1 | https://docs.peppol.eu/poacc/self-billing/3.0/v3.0.1/ |
| EU | PINT Billing | 1.1.1 | https://docs.peppol.eu/poac/eu/ |
| Australia/New Zealand | PINT Billing; Self-Billing | 1.1.3 | https://docs.peppol.eu/poac/aunz |
| Japan | PINT Billing; Self-Billing | 1.1.3 | https://docs.peppol.eu/poac/jp/ |
| Malaysia | PINT Billing; Self-Billing | 1.3.1 | https://docs.peppol.eu/poac/my/ |
| Singapore | SG BIS Billing | 3.0 | https://www.peppolguide.sg/billing/ |
| Singapore | PINT Billing | 1.4.1 | https://docs.peppol.eu/poac/sg/ |
| Oman | PINT Billing; Self-Billing; Tax Data Document | 1.0.1 | https://docs.peppol.eu/poac/om/ |
| UAE | PINT Billing; Self-Billing; Tax Data Document | 1.0.4 | https://docs.peppol.eu/poac/ae/ |

The international baseline PINT release listed in the directory is 1.1.3. It is not interchangeable with every jurisdiction specialisation.

## Country-specific rules inside BIS Billing
Country-specific BIS rules are additional to the jurisdiction profiles above. Their applicability can depend on supplier country and other document context.
- [Official national-rule methodology](https://docs.peppol.eu/poacc/billing/3.0/national-rules/)
- [Complete official Billing rule catalogue](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/)
- The pinned official Schematron and national examples acquired through standard/peppol/download.sh.

A complete per-country rule inventory must be extracted from the pinned source, retaining every rule ID, context/applicability expression, assertion, severity and release. Do not infer the complete country list from selected examples or from the PINT jurisdiction table.

## Required country annexes
For every profile and every country represented in the ruleset, the eventual collected documentation must include:
1. Scope, version and effective dates.
2. Specification/customisation and process identifiers.
3. Document syntax, cardinality and semantic constraints.
4. Country-specific rule catalogue and applicability.
5. Code lists and identifier schemes.
6. Official schemas/Schematron and their dependency manifest.
7. Official examples, generated test coverage and unresolved gaps.
8. Transport, registration and reporting requirements with sources.
9. Licence, original-source links and checksums.

Current completion status: the source catalogue is gathered; full country annex extraction and acquisition of every original jurisdiction package are not yet completed. Those tasks belong in the worldwide support plan. Countries without a Peppol-specific profile still require official national requirements research; no country-specific Peppol specification should be invented for them.
