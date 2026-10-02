# Country-rule implementation and sample generation plan

## Objective

Turn the extracted official validation expressions into reproducible CreateSamples fixtures and qualified profile support. Read [country resources](countries/README.md), [verification](extracted/VERIFICATION.md) and [source manifest](extracted/PACKAGE-MANIFEST.md) first.

Executable-rule extraction is separate from fixture generation, implementation and regression qualification. This plan does not claim those stages are complete.

## Per-rule documentation contract

For every assertion/report, record profile, release, document type, source member/hash, exact rule ID, severity, context, test and scoped dependencies. Add a plain-language explanation of the triggering conditions and requirement, with business terms and affected UBL paths.

Prepare a concrete recipe with:
- A source-valid baseline fixture and its identity.
- Exact XML nodes/attributes to add, remove or change.
- Prerequisites that make the context applicable.
- Values that satisfy the test and values that violate it.
- Boundary values, identifier/checksum cases and missing/empty/whitespace distinctions where relevant.
- A non-applicable variation and expected absence of the target assertion.
- Expected collateral assertion IDs if isolation is impossible.
- Expected warnings separately from fatal errors.

Preserve XPath 2.0 semantics, official rule ordering and country/profile selection variables. A generic instruction to “make the rule fail” is not a completed mutation recipe.

## Worked recipe: DE-R-001

Select supported BIS Billing 3.0.21 Invoice or CreditNote. Set supplier and buyer postal country to DE, matching the pinned German country booleans. Start with a document accepted by the complete validation pipeline.

Valid mutation: retain a PaymentMeans group and all other required payment fields. Invalid mutation: remove the entire PaymentMeans group. Expect DE-R-001 fatal; record any additional payment assertions.

Non-applicable variations: change only supplier postal country to IE, then in a separate fixture only buyer postal country to IE. Expect DE-R-001 to be absent. Other rules may still reject the document, so do not infer global validity from absence of this assertion.

## Worked recipe: DK-R-002

Select Invoice or CreditNote and populate country inputs so the pinned DKSupplierCountry resolves to DK; use its exact extracted definition. Begin with a fully valid document with a valid seller legal registration identifier.

Valid mutation: retain populated seller PartyLegalEntity/CompanyID. Invalid variations: remove CompanyID; provide an empty element; provide whitespace only. Expect DK-R-002 fatal; empty-element checks may also fail.

Non-applicable variation: change the source inputs so DKSupplierCountry no longer resolves to DK. Expect absence of DK-R-002, while evaluating the remaining pipeline normally.

## Development steps and acceptance criteria

1. Profile registry: map declared CustomizationID, ProfileID and document type to exact supported releases/assets. Record unsupported combinations explicitly.
2. CreateSamples selection: add country/profile/release/transaction selection without replacing the main engine.
3. Baseline generation: create deterministic source-valid baselines for every supported combination and validate against official assets.
4. Rule mutations: implement reviewed concrete recipes, including boundary and non-applicable cases.
5. Manifest extension: fixture path/hash, profile/release/country inputs, source rule/member/hash, expected validity, assertion IDs and warning severities.
6. Regression ingestion: consume the stored corpus without regenerating it. Verify expected assertions and their absence where inapplicable.
7. Failure evidence: report fixture path, inputs, source version, expected versus actual IDs/severities and offending XML location for Codex.
8. Qualification matrix: separately mark extracted, documented, fixture-generated and regression-qualified. Claim profile support only with evidence.

All fixtures should be deterministic and regeneration straightforward. Respect repository size constraints; choose representative small documents for per-rule tests and handle structural stress corpora under their existing storage policy.

## Remaining documentation work

The linked extraction preserves executable expressions and dependencies. It does not yet provide a reviewed plain-language explanation and concrete XML mutation recipe for every rule. The two worked recipes above establish the required format; the remaining recipes must be derived and verified individually.

This document is the implementation plan, not evidence that CreateSamples enhancements or country regressions have been implemented.
