# Country support: worldwide development plan
Date: 2026-10-02
Status: proposed roadmap; worldwide support is the target, not a current product claim.

## Objective
Cover every country through an explicit, maintained inventory and a verifiable implementation plan. No country may be silently omitted or declared supported solely because generic UBL or Peppol Billing validation passes.

The inventory must include all ISO 3166-1 entries, with territories/jurisdictions identified separately where their requirements differ. Also record additional legally relevant jurisdictions not adequately represented by that inventory. Freeze and version the inventory source and date. Do not interpret country coverage as support only for countries currently using Peppol.

## Component boundaries
UBL21 owns document validation and structured evidence.
The invoice generator owns generation/conversion and jurisdiction-specific document production.
The router owns provider selection, discovery, transport and delivery evidence.
Tax-reporting/clearance connectors are separate services where required.
Shared jurisdiction documentation and contracts coordinate these projects; do not put every service implementation into UBL21.

## Current baseline
Verified source supports UBL 2.1 XSD, EN16931 and Peppol BIS Billing 3.0.21 / artefacts 1.3.16 for Invoice/CreditNote, including national rules contained in that pinned artefact. This is not blanket national legal compliance.
The reviewed corpus inventories contain 71 standard UBL, 32 EN16931, 546 structural and 30 Peppol fixtures. These counts do not establish country-by-country coverage or a fresh passing end-to-end run.

## Coverage dimensions
Track each dimension separately for every jurisdiction:
| Dimension | Required assessment |
| --- | --- |
| Syntax | UBL, CII, proprietary XML/JSON or other accepted invoice formats and versions |
| Business semantics | Mandatory fields, totals, taxes, rounding, identifiers, codes and conditional rules |
| National profile | CIUS, extensions, local Schematron/XSD or other official validation artefacts |
| Peppol | Applicable BIS/PINT specialisation, document/process IDs, release and national rules |
| Business scope | B2B, B2G, B2C, domestic/cross-border and issuer/recipient conditions |
| Workflow | Credit notes, corrections, cancellation, self-billing and response requirements |
| Delivery | Accepted networks, AP/provider support, registration and receipts |
| Tax reporting | Clearance, reporting, signatures, QR codes or authority acknowledgement where required |
| Operations | Retention, security, identity checks and applicable local service obligations |

A Peppol-valid document may still fail another national requirement. AS4 delivery is not tax clearance or business acceptance.

## Coverage registry
Create a machine-readable, version-controlled jurisdiction registry, accompanied by human-readable pages. Fields:
- Stable jurisdiction code/name and territory relationship.
- Business scope and applicability predicates.
- Authoritative sources, source language, review date and accountable owner.
- Legal effective dates, transitional arrangements and superseded versions.
- Accepted formats, profile IDs, ruleset versions, immutable assets and checksums.
- Peppol status and specific supported profiles.
- Provider/transport and tax connector availability.
- Fixture sets, qualification evidence and known gaps.
- Per-dimension support status and next work package.

Use statuses: unresearched, researched, planned, implemented, qualified, unsupported.
Record not_applicable only with evidence and scope. “No mandate found” is a dated research finding, not “no requirements.”
Qualified claims must name supported scenarios and release; do not use one undifferentiated country-supported flag.

## Phase 1: exhaustive inventory and research
1. Populate every inventory entry before implementing priority markets.
2. Research official tax authorities, government specifications and Peppol Authorities for each jurisdiction.
3. Record differing domestic, cross-border and sector/customer requirements.
4. Identify official validation assets and restrictions on reuse/distribution.
5. Record unresolved questions and specialist/local review needs.
Acceptance: every jurisdiction has a registry row; unknowns remain explicit; researched conclusions link authoritative evidence.

## Phase 2: common validation architecture
Implement a versioned jurisdiction/profile registry and ruleset loader. Keep profile identity separate from country codes: country alone cannot identify the correct format or legal workflow.
Resolve applicability from explicit requested profile, document identifiers and necessary supplier/buyer/business context. Reject contradictory or unsupported requests rather than silently falling back.
Freeze selected versions and applicability context with each validation execution.
Support additional syntaxes through isolated adapters; preserve original bytes and provenance for conversions.
Return structured stage/rule/severity/location evidence and the exact coverage claim.
Acceptance: unsupported profiles fail clearly; results distinguish implemented standards from unassessed national obligations.

## Phase 3: per-jurisdiction implementation packages
For each jurisdiction create documentation and proposed work items:
1. Requirements and applicability specification.
2. Pinned authoritative artefacts and licensing/provenance.
3. Validator/profile implementation.
4. Generator/converter changes in the appropriate component.
5. Createsamples corpus and manifest.
6. Public API/documentation and capability advertisement.
7. Provider routing or tax connector integration where needed.
8. Qualification and release evidence.
Apply the same completion criteria across all jurisdictions. Prioritisation affects order, never omission. No GitHub issues are created by this planning document.

## Phase 4: comprehensive test corpora
Createsamples remains the main sample-generation engine. Generate reproducible country/profile corpora outside ordinary regression runs; runners consume frozen manifests.
Preserve official unmodified examples separately with provenance where permitted. Clearly distinguish generated examples from official fixtures.

Include:
- Valid minimal and rich documents for each supported scenario.
- Missing/incorrect identifiers and required fields.
- Country-triggered and supplier/buyer applicability boundaries.
- Tax categories/rates, exemptions, currencies, rounding and total consistency.
- Domestic/cross-border differences and profile/version mismatches.
- Credits, corrections, cancellations and self-billing where applicable.
- Large invoices, zero/negative boundary cases where legal, and conditional rules.
- Routing discovery, provider rejection, receipt and tax-clearance cases in the appropriate integration suite.

For every implemented rule, trace it to applicable positive/negative tests or explicitly document why that coverage is not feasible. Representative tests must not be described as exhaustive coverage.
Keep large generated data out of Git when repository limits require it; retain reproducible commands, versioned manifests, hashes and a controlled artefact location.

## Phase 5: regression, interoperability and reporting
Ingest each jurisdiction corpus through the public validation API and assert stage outcomes and diagnostics.
On failure identify jurisdiction, profile/version, fixture ID, file/hash, expected/actual outcomes, failing rule IDs, correlation IDs and reproducible command. Produce a bounded diagnostic bundle suitable for engineering investigation.
Compare against official validators or independently compiled authoritative rules when available.
Run separate provider/authority sandbox tests for transport and clearance. Mock tests do not prove interoperability.
Acceptance: release evidence records exact corpus, rulesets, environment and results; unresolved qualification gaps prevent qualified status.

## Phase 6: rollout and ongoing maintenance
Release markets in batches based on customer demand, available official artefacts and connector feasibility, while maintaining the full worldwide backlog.
Publish the coverage matrix and limitations. Monitor official changes, assign owners and schedule reviews. Pin upgrades, run affected regression suites and document effective-date transitions.
Retain historical execution identity; upgrading rules must not reinterpret old validation results.

## Cross-project implementation sequence
1. Worldwide registry and researched applicability contracts.
2. Common profile/version resolution and explicit capability API.
3. Existing Peppol national-rule inventory and missing country fixture coverage.
4. Jurisdiction-specific CIUS/PINT and other accepted syntaxes.
5. Generation/conversion support.
6. Provider routing and jurisdiction-specific reporting connectors.
7. Qualification, release and maintenance for each jurisdiction.

## Definition of completion
Every inventory entry is accounted for. Each supported scenario has authoritative requirements, implemented necessary components, pinned artefacts, reproducible fixtures, passing qualification evidence and documented limitations.
Every remaining scenario has a named gap and development package. Worldwide implementation is complete only when all required scenarios meet these gates; unknown or unsupported entries cannot count as completed support.

## References
- [Peppol national-rule methodology](https://docs.peppol.eu/poacc/billing/3.0/national-rules/)
- [Peppol Billing rules](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/)
- [Peppol jurisdiction governance](https://peppol.org/learn-more/peppol-interoperability-framework/)
- [Existing validation contract](PEPPOL-VALIDATION-CONTRACT.md)
- [Provider routing plan](PEPPOL-ROUTE.md)
- [Access Point accreditation](PEPPOL-CERTIFICATION.md)
- [ISO and Peppol certification requirements](ISO-27001-CERTIFICATION.md)
