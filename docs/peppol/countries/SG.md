# Singapore: Peppol country resource
Country code: SG
Checked: 2026-10-02
Status: verified official-profile overview; detailed normative rule extraction is incomplete.
Implementation status: this resource does not establish that these profiles are implemented in UBL21.

## Official specifications
[Official country portal](https://docs.peppol.eu/poac/sg/)
Published families: PINT SG Billing 1.4.1; separate SG BIS Billing 3.0.
[OpenPeppol release directory](https://peppol.org/documentation/technical-documentation/post-award-documentation/)

## Country-specific findings
PINT SG and the separately published SG BIS Billing are distinct specification families. Resolve the intended profile explicitly; never treat their names as interchangeable.

## Instructions for Codex
Use this file as an entry point. Retrieve the exact versioned official profile before implementing requirements. Do not infer tax rates, field cardinality, identifier formats, effective dates or CustomizationID/ProfileID values from country names or generic EU Billing rules.
Follow each portal's Billing/Self-Billing/TDD links as applicable, then collect semantic model, syntax mapping, rules, code lists, validation assets and official examples. Record their immutable provenance and licence.
Existing UBL21 support for EU BIS Billing 3.0.21 does not automatically implement this jurisdiction profile.

## Required detailed annex
Record every applicable rule's ID, applicability, severity, test condition and implementation/test mapping. Separate common PINT rules from jurisdiction rules. Document unresolved facts; this annex is pending.
Confirm accepted invoice/credit-note scenarios, business roles, currencies, rounding, identifiers, taxes, corrections and domestic/cross-border constraints from official assets.
Assess national requirements beyond the Peppol payload separately using the relevant Authority's publications.

## Createsamples requirements
Add explicit SG and versioned-profile selection to the common engine.
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
The invoice validation catalogue separates shared PINT rules from the SG aligned rules. The SG rule page is labelled 1.4.0 while the profile landing page is 1.4.1; record both and resolve the release package before assigning a ruleset identity. Receiver capability affects whether jurisdiction-specific rules are applied. Do not make supplier country the sole selector. Complete SG rule expressions remain to be extracted.

### Direct implementation resources
- [Profile](https://docs.peppol.eu/poac/sg/pint-sg/)
- [Invoice business rules](https://docs.peppol.eu/poac/sg/pint-sg/trn-invoice/businessrule/)
- [Shared/jurisdiction Schematron catalogue](https://docs.peppol.eu/poac/sg/pint-sg/trn-invoice/rule/)
- [Jurisdiction aligned rule index](https://docs.peppol.eu/poac/sg/pint-sg/trn-invoice/rule/PINT-jurisdiction-aligned-rules/)

Generation must include missing/wrong profile identifiers, shared-rule failures and jurisdiction-only failures, plus receiving-capability applicability cases. Preserve the exact official expressions in separately acquired original assets; prose anchors are explanatory.
