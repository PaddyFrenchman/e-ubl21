# Malaysia: Peppol country resource
Country code: MY
Checked: 2026-10-02
Status: verified official-profile overview; detailed normative rule extraction is incomplete.
Implementation status: this resource does not establish that these profiles are implemented in UBL21.

## Official specifications
[Official country portal](https://docs.peppol.eu/poac/my/)
Published families: PINT MY Billing and Self-Billing 1.3.1.
[OpenPeppol release directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/)

## Country-specific findings
The published specialisation incorporates Malaysian tax-authority requirements. Its self-billing transactions share the Billing data model. This does not prove implementation of separate authority submission workflows.

## Instructions for Codex
Use this file as an entry point. Retrieve the exact versioned official profile before implementing requirements. Do not infer tax rates, field cardinality, identifier formats, effective dates or CustomizationID/ProfileID values from country names or generic EU Billing rules.
Follow each portal's Billing/Self-Billing/TDD links as applicable, then collect semantic model, syntax mapping, rules, code lists, validation assets and official examples. Record their immutable provenance and licence.
Existing UBL21 support for EU BIS Billing 3.0.21 does not automatically implement this jurisdiction profile.

## Required detailed annex
Record every applicable rule's ID, applicability, severity, test condition and implementation/test mapping. Separate common PINT rules from jurisdiction rules. Document unresolved facts; this annex is pending.
Confirm accepted invoice/credit-note scenarios, business roles, currencies, rounding, identifiers, taxes, corrections and domestic/cross-border constraints from official assets.
Assess national requirements beyond the Peppol payload separately using the relevant Authority's publications.

## Createsamples requirements
Add explicit MY and versioned-profile selection to the common engine.
Generate valid minimal/rich documents and deliberate invalid mutations for each implemented national rule. Include supplier/buyer country applicability and cases where national rules must not fire.
Use distinct fixtures for each document/process family. Manifests must retain country/profile/ruleset identity, input hashes, expected stages, outcomes and diagnostics.
Missing rules/profile support blocks a country-qualified claim; generic UBL samples must not be relabelled compliant.

## Qualification gates
Pin original assets and hashes; verify official examples; compare validator results against authoritative rules; run the corpus through UBL21's public API.
Provider delivery and authority reporting require separate integration qualification.
Publish exact supported scenarios and limitations. Profile catalogue presence is not certification or national legal-compliance evidence.

## Related resources
- [Worldwide development plan](../../country-support.md)
- [Official sources catalogue](../../PEPPOL-OFFICIAL-SPECS.md)
- [Shared national-rule methodology](README.md)

## Detailed extraction: first verified increment
Verified profile prefix: urn:peppol:pint:billing-1@my-1 (ALIGNED-IBRP-001-MY). The process rule uses urn:peppol:bis:billing. VAT and AAL/TTX tax subtotals have different requirements: TTX identifies AAL subtotals, which must omit the tax rate. The official rule introduction distinguishes sender obligations from an overseas receiver's advertised capability. Do not impose the complete MY specialisation on a receiver supporting only shared PINT. Exact expressions and remaining rules still require extraction.

### Direct implementation resources
- [Profile](https://docs.peppol.eu/poac/my/pint-my/)
- [Invoice business rules](https://docs.peppol.eu/poac/my/pint-my/trn-invoice/businessrule/)
- [Shared/jurisdiction Schematron catalogue](https://docs.peppol.eu/poac/my/pint-my/trn-invoice/rule/)
- [Jurisdiction aligned rule index](https://docs.peppol.eu/poac/my/pint-my/trn-invoice/rule/PINT-jurisdiction-aligned-rules/)

Generation must include missing/wrong profile identifiers, shared-rule failures and jurisdiction-only failures, plus receiving-capability applicability cases. Preserve the exact official expressions in separately acquired original assets; prose anchors are explanatory.
