# Peppol validation extension development plan

Date: 17 October 2026. Status: planned; no implementation or new passing test claim.

## Goal and boundaries

Extend existing validation incrementally. First qualify SBDH and Ireland/UK scenarios; reconcile structural evidence in parallel. Invoice Response is a subsequent phase. Production deployment, Access Point accreditation, routing and response exchange are excluded. Self-billing and other profiles need a separate future scope decision.

Reuse #82 as the envelope umbrella and #167/#160 for structural qualification. Existing #38/#76/#77 cover original corpus work: new issues extend missing evidence/scenarios rather than restart those deliverables. Read docs/peppol/COUNTRY-RULE-IMPLEMENTATION-PLAN.md and docs/PEPPOL-VALIDATION-CONTRACT.md before implementation.

## Sequenced steps

### ENV-01: Audit existing SBDH implementation and remaining qualification gaps

Dependencies: none; can start now.

Inspect #82 implementation, pinned envelope artefacts, fixtures and local evidence. Record implemented versus unqualified behaviour; pin the exact tested commit. Acceptance: a checked gap matrix identifies remaining changes without reimplementing passing functionality.

### ENV-02: Qualify SBDH sender and receiver identifier matching

Dependencies: ENV-01.

Add only missing deterministic pass/fail/boundary cases for sender and receiver schemes/values against payload endpoints. Derive matching semantics from pinned official rules; cover mismatches, missing and ambiguous identifiers. Acceptance: exact expected envelope diagnostics and independent payload verdicts are asserted.

### ENV-03: Qualify SBDH document and process identifiers

Dependencies: ENV-01.

Cover Invoice/CreditNote, profiles 01/02, document type/version and process matching, including unsupported combinations. Acceptance: realistic baseline messages pass and deliberate mismatches fail with traceable rule evidence; no invented official rule IDs.

### ENV-04: Qualify envelope pipeline and publish issue 82 closure evidence

Dependencies: ENV-02, ENV-03.

Exercise public asynchronous envelope submission, payload/envelope verdict separation, short-circuiting, operational failure and duplicate/replay behaviour. Reuse existing tests. Acceptance: required local gate fails on missing dependencies, evidence includes commands, commit and asset hashes, and remaining #82 limitations are explicit. Propose closure only after all #82 criteria pass.

### MARKET-01: Inventory authoritative Ireland and UK invoice requirements

Dependencies: none; can start now.

Create a dated official-source applicability matrix for IE, GB and Northern Ireland transaction scenarios. Separate pinned BIS/EN rules, legal requirements and optional customer policy. Do not invent national Peppol overlays. Acceptance: each candidate requirement has source/version, applicability, existing coverage and a documented gap or no-gap decision.

### MARKET-02: Specify reviewed Ireland and UK fixture recipes

Dependencies: MARKET-01.

Use the existing country-rule implementation plan. Define a small reviewed scenario matrix for invoices/credits, EUR/GBP, VAT categories, cross-border/NI applicability, identifiers, payments, allowances and rounding. Acceptance: each recipe has baseline, exact mutation, expected stage/rule/severity and non-applicable controls; legal cases without machine rules are distinguished.

### MARKET-03: Generate minimal deterministic Ireland fixtures

Dependencies: MARKET-02.

Extend CreateSamples rather than creating another generator. Implement the approved IE recipes with hashes/provenance and explicit missing-data diagnostics. Acceptance: reviewed positive fixtures pass applicable pinned stages; negatives fail at the intended stage with declared collateral rules; regeneration is deterministic.

### MARKET-04: Generate minimal deterministic GB and Northern Ireland fixtures

Dependencies: MARKET-02.

Implement reviewed GB/NI recipes in CreateSamples. Derive VAT/identifier treatment from transaction applicability, never a country label alone. Acceptance: manifest records jurisdiction/transaction inputs, expected stages/rules and provenance; deterministic pass/fail/non-applicable cases cover the approved scope.

### MARKET-05: Decide and implement only evidenced market-rule gaps

Dependencies: MARKET-03, MARKET-04.

Compare observed coverage with MARKET-01. If no additional executable rule is justified, deliver a documented no-code decision. Otherwise implement only sourced gaps as separately identified/versioned checks with opt-in/applicability and regression tests; preserve official rule IDs and distinguish product diagnostics. Acceptance: no unsupported claim of national conformity or silently stricter BIS validity.

### MARKET-06: Publish reproducible Ireland and UK qualification evidence

Dependencies: MARKET-05.

Run stored fixtures through the real runtime without regeneration, verify expected assertions/severities and absences, and publish supported/unsupported scenario matrix plus exact commit/assets/commands. Link #167 structural evidence separately; market passes cannot close #167. Acceptance: missing runtime fails the gate and completion claims match measured scope.

### CORPUS-01: Reconcile outstanding issue 167 evidence and residuals

Dependencies: none; can start now.

Work under existing #167/#160; do not replace the structural qualification effort. Audit latest reports, fixture/submitted/terminal counts, deterministic regeneration, all expected-valid XSD results and uncovered constructs against #167 acceptance. Acceptance: one evidence checklist links exact commit/artefacts and identifies remaining failures or approved residuals; local passes remain recorded without premature closure.

### RESP-01: Define versioned Invoice Response validation contract

Dependencies: ENV-04, MARKET-06.

Follow-up phase only. Inspect official Invoice Response specification and reusable runtime; select supported release, syntax, identifiers, code lists and evidence contract. Acceptance: normative scope separates response payload validation from invoice validation and response exchange; unsupported combinations are explicit. No response transport implementation.

### RESP-02: Pin Invoice Response validation assets and fixtures

Dependencies: RESP-01.

Acquire permitted official release assets, provenance/licence/hashes and positive/negative fixtures from RESP-01. Acceptance: reproducible offline asset verification and expected diagnostics; missing executable official assets are an explicit blocker, not replaced with invented conformity rules.

### RESP-03: Add isolated Invoice Response validator and regression gate

Dependencies: RESP-02.

Reuse bounded runner/evidence mechanisms; wire an explicit versioned response validation plan and documented API selection. Acceptance: valid/invalid/runtime-failure cases, official rule evidence and profile isolation pass; no change to existing Billing verdicts and no implication of response delivery.

## Working rules

One bounded reviewable change per issue; split further if inspection reveals multiple independent fixes. Audit existing behaviour before changing it. Every implementation issue needs targeted tests, exact revision/asset identity and source-linked diagnostics. Keep national legal advice/applicability distinct from machine-enforced Peppol rules. Local passing evidence does not establish network interoperability or certification. Do not close #82 or #167 automatically when a child issue is complete.

## GitHub implementation checklist

- [ ] [#175: ENV-01 — Audit existing SBDH implementation and remaining qualification gaps](https://github.com/PaddyFrenchman/e-ubl21/issues/175)
- [ ] [#176: ENV-02 — Qualify SBDH sender and receiver identifier matching](https://github.com/PaddyFrenchman/e-ubl21/issues/176)
- [ ] [#177: ENV-03 — Qualify SBDH document and process identifiers](https://github.com/PaddyFrenchman/e-ubl21/issues/177)
- [ ] [#178: ENV-04 — Qualify envelope pipeline and publish issue 82 closure evidence](https://github.com/PaddyFrenchman/e-ubl21/issues/178)
- [ ] [#179: MARKET-01 — Inventory authoritative Ireland and UK invoice requirements](https://github.com/PaddyFrenchman/e-ubl21/issues/179)
- [ ] [#180: MARKET-02 — Specify reviewed Ireland and UK fixture recipes](https://github.com/PaddyFrenchman/e-ubl21/issues/180)
- [ ] [#181: MARKET-03 — Generate minimal deterministic Ireland fixtures](https://github.com/PaddyFrenchman/e-ubl21/issues/181)
- [ ] [#182: MARKET-04 — Generate minimal deterministic GB and Northern Ireland fixtures](https://github.com/PaddyFrenchman/e-ubl21/issues/182)
- [ ] [#183: MARKET-05 — Decide and implement only evidenced market-rule gaps](https://github.com/PaddyFrenchman/e-ubl21/issues/183)
- [ ] [#184: MARKET-06 — Publish reproducible Ireland and UK qualification evidence](https://github.com/PaddyFrenchman/e-ubl21/issues/184)
- [ ] [#185: CORPUS-01 — Reconcile outstanding issue 167 evidence and residuals](https://github.com/PaddyFrenchman/e-ubl21/issues/185)
- [ ] [#186: RESP-01 — Define versioned Invoice Response validation contract](https://github.com/PaddyFrenchman/e-ubl21/issues/186)
- [ ] [#187: RESP-02 — Pin Invoice Response validation assets and fixtures](https://github.com/PaddyFrenchman/e-ubl21/issues/187)
- [ ] [#188: RESP-03 — Add isolated Invoice Response validator and regression gate](https://github.com/PaddyFrenchman/e-ubl21/issues/188)

Start with #175 (envelope audit), #179 (market requirements) and #185 (structural evidence). Invoice Response starts only after #178 and #184.

## Focused enhancement corpus — required development foundation

User requirement: independently generate and test the new features without processing the large structural corpus.

1. [#189](https://github.com/PaddyFrenchman/e-ubl21/issues/189): select named feature suites and recipe/fixture/rule filters before generation; dry-run and fail on unknown/empty selection.
2. [#190](https://github.com/PaddyFrenchman/e-ubl21/issues/190): write only selected fixtures and an isolated deterministic manifest, with safe output ownership and reviewed size budgets. Depends on #189.
3. [#191](https://github.com/PaddyFrenchman/e-ubl21/issues/191): add manifest-only focused test targets, real prerequisite validators, exact counts, timing evidence and reproduction commands. Depends on #190.

Proposed suite names (not implemented CLI flags): peppol-envelope, peppol-ie, peppol-gb-ni, peppol-response (later), combined peppol-enhancements. Generate only necessary positive/negative/boundary/non-applicable scenarios and explicit prerequisite controls. The runner reads only the selected manifest; it does not regenerate fixtures or walk the full repository. Unknown suites, unimplemented suites, zero matches and missing dependencies fail clearly.

Existing audit/requirements work #175, #179 and #185 can begin immediately. Build selection/manifest/runner infrastructure before repeated feature qualification. #181 and #182 use #190 for generation. #178 and #184 use #191 for focused execution/evidence. #187 and #188 extend the same infrastructure for responses later. Infrastructure can be proven initially with existing representative Peppol fixtures so there is no circular dependency on future scenario implementation.

Keep full regression separately invokable for broad qualification; a focused pass does not satisfy #167. No performance result or implemented CLI is claimed by this plan.
