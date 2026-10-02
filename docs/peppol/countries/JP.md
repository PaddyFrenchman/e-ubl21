# Japan: Peppol country resource
Country code: JP
Checked: 2026-10-02
Status: verified official-profile overview; detailed normative rule extraction is incomplete.
Implementation status: this resource does not establish that these profiles are implemented in UBL21.

## Official specifications
[Official country portal](https://docs.peppol.eu/poac/jp/)
Published families: JP PINT Standard Invoice, Non-tax Registered Businesses and Self-Billing 1.1.3.
[OpenPeppol release directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/)

## Country-specific findings
The official portal distinguishes standard invoices from invoices for non-tax-registered businesses. Preserve that distinction in profile resolution and sample generation.

## Instructions for Codex
Use this file as an entry point. Retrieve the exact versioned official profile before implementing requirements. Do not infer tax rates, field cardinality, identifier formats, effective dates or CustomizationID/ProfileID values from country names or generic EU Billing rules.
Follow each portal's Billing/Self-Billing/TDD links as applicable, then collect semantic model, syntax mapping, rules, code lists, validation assets and official examples. Record their immutable provenance and licence.
Existing UBL21 support for EU BIS Billing 3.0.21 does not automatically implement this jurisdiction profile.

## Required detailed annex
Record every applicable rule's ID, applicability, severity, test condition and implementation/test mapping. Separate common PINT rules from jurisdiction rules. Document unresolved facts; this annex is pending.
Confirm accepted invoice/credit-note scenarios, business roles, currencies, rounding, identifiers, taxes, corrections and domestic/cross-border constraints from official assets.
Assess national requirements beyond the Peppol payload separately using the relevant Authority's publications.

## Createsamples requirements
Add explicit JP and versioned-profile selection to the common engine.
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
The JP invoice rule index contains 43 jurisdiction-aligned entries. Verified anchors: ALIGNED-IBR-JP-04 requires a seller tax identifier; ALIGNED-IBR-JP-05 constrains a supplied tax-accounting currency to JPY; ALIGNED-IBR-JP-06 prohibits fractional JPY tax-breakdown amounts. ALIGNED-IBRP-CL-02-JP selects invoice type 380. Test these independently of shared PINT rules. Exact XPath, severity and source hashes remain to be extracted; these anchors do not constitute complete rule coverage.

### Direct implementation resources
- [Profile](https://docs.peppol.eu/poac/jp/pint-jp/)
- [Invoice business rules](https://docs.peppol.eu/poac/jp/pint-jp/trn-invoice/businessrule/)
- [Shared/jurisdiction Schematron catalogue](https://docs.peppol.eu/poac/jp/pint-jp/trn-invoice/rule/)
- [Jurisdiction aligned rule index](https://docs.peppol.eu/poac/jp/pint-jp/trn-invoice/rule/PINT-jurisdiction-aligned-rules/)

Generation must include missing/wrong profile identifiers, shared-rule failures and jurisdiction-only failures, plus receiving-capability applicability cases. Preserve the exact official expressions in separately acquired original assets; prose anchors are explanatory.
