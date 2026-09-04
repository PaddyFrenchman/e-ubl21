# TODO-PEPPOL — Peppol BIS Billing conformity validation roadmap

**Project:** e-ubl21  
**Status:** Planned  
**Created:** 2026-08-26  
**Baseline:** current server already validates UBL 2.1 and EN 16931 for `Invoice` and `CreditNote`  
**TODO namespace:** `TODO-PEPPOL-###`

The normative product decisions for `TODO-PEPPOL-001` are frozen in
[`docs/PEPPOL-VALIDATION-CONTRACT.md`](docs/PEPPOL-VALIDATION-CONTRACT.md).
Later roadmap items must implement that contract without silently broadening
the meaning of Peppol-valid.

## 1. Objective

Extend the existing e-ubl21 validation server so it can determine whether a UBL `Invoice` or `CreditNote` conforms to the applicable **Peppol BIS Billing 3.0** rules in addition to the validation already performed against:

1. UBL 2.1 XML Schema; and
2. EN 16931 semantic/business rules.

The target validation chain is:

```text
submitted UBL XML
      |
      v
UBL 2.1 XSD
      |
      +-- fail --> standards-invalid
      |
      v
EN 16931
      |
      +-- fail --> standards-invalid
      |
      v
Peppol BIS Billing 3.0
      |
      +-- fail --> standards-invalid
      |
      v
Peppol-conformant payload
```

A document requested to be validated as Peppol **must never be reported valid unless every applicable layer has passed**.

The existing EN 16931 implementation remains the semantic foundation. The Peppol layer is an additional profile-specific layer, not a replacement for UBL XSD or EN 16931.

---

## 2. Current official Peppol baseline to pin

At the creation of this roadmap, the current mandatory Peppol BIS Billing release is:

- **Peppol BIS Billing 3.0 version 3.0.21**;
- published **20 May 2026**;
- mandatory from **17 August 2026**;
- associated UBL/CII validation artefacts: **1.3.16**, published 10 April 2026;
- code-list publication date: **6 March 2026**.

The release introduced/recognizes two Billing process profiles for Invoice/CreditNote payloads:

### Profile 01 — Billing

`CustomizationID`:

```text
urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0
```

`ProfileID`:

```text
urn:fdc:peppol.eu:2017:poacc:billing:01:1.0
```

### Profile 02 — Billing with Response

Uses the same `CustomizationID` and the following `ProfileID`:

```text
urn:peppol:bis:billing_with_response
```

The authoritative release source must be OpenPeppol documentation/download artefacts, not an unpinned branch.

Official reference points:

- `https://docs.peppol.eu/poacc/billing/3.0/`
- `https://docs.peppol.eu/poacc/billing/3.0/release-notes/`
- OpenPeppol BIS Billing source repository: `OpenPEPPOL/peppol-bis-invoice-3`

The implementation must pin immutable bytes and digests before the Peppol validator becomes part of production validity.

The pinned source identity, checksums, licensing constraints, and reproducible
download procedure are maintained under [`standard/peppol/`](standard/peppol/).
The corresponding EN 16931 provenance is under
[`standard/en16931/`](standard/en16931/).

---

## 3. Scope

### In scope for the first Peppol milestone

- UBL 2.1 `Invoice`;
- UBL 2.1 `CreditNote`;
- Peppol BIS Billing 3.0;
- current mandatory release and its exact Peppol validation artefacts;
- Profile 01 Billing;
- Profile 02 Billing with Response;
- Peppol rules, common identifier rules, code lists, and country-specific rules included in the pinned Peppol validation artefact;
- structured Peppol validation results through the existing asynchronous server API;
- durable retry/result/DLQ/replay behavior identical in principle to the EN 16931 stage;
- offline runtime and reproducible release/version reporting.

### Explicitly outside this first payload-validation milestone

- sending documents over the Peppol network;
- SMP/SML recipient discovery;
- AS4/access-point operation;
- Peppol certificate handling;
- invoice response transport;
- national e-invoicing regimes not represented by the selected Peppol Billing rules;
- Peppol Self-Billing BIS;
- non-billing Peppol BIS families such as Ordering and Despatch;
- CII input syntax;
- service-provider accreditation/certification.

An optional SBDH/enveloping validation milestone is defined separately near the end of this roadmap.

---

## 4. Architectural rules

1. **XSD first, EN 16931 second, Peppol third.**
2. Peppol validation is never executed after an XSD or EN standards failure.
3. Peppol rules must come from pinned official OpenPeppol validation artefacts. Do not rewrite Peppol business rules manually in Go.
4. Reuse the existing Saxon/SVRL architecture where practical.
5. A Peppol standards failure is `invalid`; a missing/corrupt validator runtime is an operational failure and remains retryable.
6. No runtime network access is permitted to obtain rules or code lists.
7. Every validation layer must expose its own version and structured issues.
8. `valid` is profile-dependent. A document can be EN 16931-valid while still being Peppol-invalid.
9. Backwards compatibility matters. Existing callers that only request the current UBL/EN behavior must not silently be forced into Peppol validity unless the contract explicitly says so.
10. A caller must be able to **force** Peppol validation. Automatic inference alone is not sufficient because a document with a missing or incorrect Peppol `CustomizationID` must fail a Peppol validation request rather than quietly falling back to EN-only validation.

---

## 5. Proposed profile-selection contract

Introduce a validation-profile concept independent of document type.

Suggested logical profiles:

```text
ubl21
ubl21-en16931
peppol-bis-billing
```

For backward compatibility, the existing upload endpoint may default to `auto` behavior:

```text
auto
  |
  +-- non Invoice/CreditNote ------------> UBL XSD only
  |
  +-- Invoice/CreditNote without Peppol
  |   customization ---------------------> UBL XSD + EN 16931
  |
  +-- Invoice/CreditNote declaring
      supported Peppol customization ----> UBL XSD + EN 16931 + Peppol
```

But an explicit request such as:

```text
validationProfile=peppol-bis-billing
```

must force all three stages and must fail Peppol validation when required Peppol identifiers/profile values are absent or wrong.

The exact transport may be a query parameter, request field, header, or a future generic validation endpoint. The contract should be frozen before implementation.

---

# TODO-PEPPOL-001 — Freeze Peppol validation scope and contract

## Goal

Define exactly what e-ubl21 means by `Peppol valid` before writing the adapter.

## Work

- Freeze the first supported BIS as Peppol BIS Billing 3.0.
- Freeze supported syntax to UBL 2.1 Invoice/CreditNote.
- Freeze the pinned release to the current mandatory release at implementation time.
- Define Profile 01 and Profile 02 behavior.
- Define explicit versus automatic Peppol profile selection.
- Define what `valid`, `invalid`, and `processing_failed` mean when Peppol is requested.
- Define whether warnings affect final validity. Default: only Peppol fatal/error assertions make the Peppol stage invalid; warnings remain visible but do not fail the stage unless the official artefact semantics say otherwise.
- Define Peppol version fields returned to clients.

## Acceptance criteria

There is one documented truth table mapping document type + requested profile + layer outcomes to final status.

---

# TODO-PEPPOL-002 — Pin the official Peppol release and provenance

## Goal

Make the exact Peppol standard implementation reproducible.

## Work

- Select the immutable official OpenPeppol release/tag/archive corresponding to Peppol BIS Billing 3.0.21 or the then-current mandatory release.
- Record:
  - BIS release number;
  - publication date;
  - mandatory-from date;
  - source URL/tag/archive;
  - Peppol UBL Schematron filename;
  - SHA-256 of every validation source artefact;
  - code-list versions/publication date;
  - licence/provenance information.
- Never pin `master`, `main`, `upcoming`, or another moving branch as the production source of truth.
- Add metadata alongside existing `validation-assets` rather than scattering version strings through Docker/scripts/server code.

## Acceptance criteria

A clean checkout can identify exactly which Peppol release bytes the server validates against.

---

# TODO-PEPPOL-003 — Package Peppol Schematron, code-list and example assets

## Goal

Bring the official Peppol Billing rule sources into the same controlled asset lifecycle as EN 16931.

## Work

Pin at minimum:

- `PEPPOL-EN16931-UBL.sch` or the official equivalent for the selected release;
- any official auxiliary Schematron includes/resources required by it;
- relevant code-list sources used to build/verify the rules;
- official positive example documents;
- provenance metadata and digests.

Do not duplicate the existing EN 16931 rule artefact unless Peppol packaging requires it. The current server already owns the EN stage separately.

## Acceptance criteria

The asset verifier fails on any byte change, missing file, wrong version, or provenance mismatch.

---

# TODO-PEPPOL-004 — Define and implement deterministic Schematron compilation

## Goal

Convert the official Peppol ISO Schematron rules into an executable form compatible with the existing Saxon runtime.

## Preferred architecture

```text
pinned official .sch
       |
       | build-time only
       v
pinned Schematron compiler/toolchain
       |
       v
compiled XSLT
       |
       +--> checksum
       +--> package in production image
       +--> execute with existing Saxon runtime
```

## Work

- Determine whether the selected official release provides an authoritative executable XSLT. If yes, prefer and pin it.
- Otherwise pin the ISO Schematron compilation toolchain and compile the official `.sch` deterministically during build/release preparation.
- Require XSLT 2.0/3.0 capability as required by the Peppol Schematron.
- Store/check the compiled XSLT digest.
- Do not compile Schematron from the network during worker startup.
- Add a smoke transform that produces valid SVRL.
- Verify that compilation preserves rule IDs, flag/role severity, diagnostics, and namespace bindings.

## Acceptance criteria

The same pinned Schematron and compiler inputs produce the accepted runtime artefact, and worker startup can prove it executes offline.

---

# TODO-PEPPOL-005 — Extend the layered validation result model

## Goal

Represent Peppol independently from UBL and EN 16931.

## Work

Add a stable layer such as:

```text
peppol-bis-billing
```

A Peppol stage result should contain at least:

- `layer`;
- BIS version, e.g. `3.0.21`;
- profile identifier;
- validity;
- issues;
- validation timestamp;
- duration.

Reuse the existing structured issue fields:

- kind/origin;
- rule ID;
- severity;
- location;
- test;
- message;
- diagnostic.

Do not infer issue origin from severity.

## Acceptance criteria

A result can unambiguously show:

```text
UBL 2.1      passed
EN 16931     passed
Peppol       failed PEPPOL-...
```

without flattening all errors into one list.

---

# TODO-PEPPOL-006 — Implement Peppol profile applicability and identifier checks

## Goal

Prevent silent under-validation and ensure the correct Billing profile is being tested.

## Work

- Parse `cbc:CustomizationID` and `cbc:ProfileID` from Invoice/CreditNote safely.
- Recognize the supported Peppol Billing customization identifier exactly after the normalization allowed by the specification.
- Recognize Profile 01 and Profile 02 identifiers.
- In explicit Peppol mode, missing/unsupported identifiers must produce a Peppol standards failure, not EN-only success.
- In automatic mode, document the inference rule precisely.
- Keep profile detection separate from rule execution so future Peppol BIS profiles can plug into the same registry.

## Acceptance criteria

A caller requesting Peppol validation cannot obtain `valid` by omitting or corrupting the Peppol identifiers.

---

# TODO-PEPPOL-007 — Implement the Peppol validator adapter

## Goal

Execute the pinned Peppol UBL validation artefact and return a structured Peppol stage result.

## Work

- Define a `PeppolValidator` interface analogous to `EN16931Validator`.
- Reuse or generalize the existing secure Saxon runner instead of duplicating process-control code.
- Stream XML into Saxon.
- Capture bounded SVRL and stderr.
- Parse SVRL through the existing bounded parser/model where compatible.
- Preserve Peppol rule IDs such as `PEPPOL-EN16931-*`, `PEPPOL-COMMON-*`, and country-specific rule IDs.
- Classify malformed SVRL, runtime crashes, missing assets, timeouts and cancellation as operational failures.
- Keep successful reports/warnings visible without incorrectly making them fatal.
- Add real-runtime adapter tests.

## Acceptance criteria

Known Peppol-valid input passes, known Peppol-invalid input fails with the expected official rule IDs, and runtime failure is never misreported as standards invalidity.

---

# TODO-PEPPOL-008 — Integrate Peppol as the third validation stage

## Goal

Extend the current validation orchestrator without weakening its atomicity guarantees.

## Required sequence

```text
XSD
 |
 +-- invalid --> stop
 |
EN 16931
 |
 +-- invalid --> stop
 |
Peppol
 |
 +-- invalid --> final invalid
 |
final valid for requested Peppol profile
```

## Work

- Run Peppol only for applicable Invoice/CreditNote requests.
- Run Peppol only after XSD and EN 16931 have passed.
- Use one overall document budget with bounded per-stage timeouts.
- Do not persist a partial final result while the Peppol stage is running.
- A Peppol operational error must enter the existing retry path.
- Preserve first-result uniqueness and replay semantics.

## Acceptance criteria

A Peppol-requested document cannot reach terminal `valid` with only XSD + EN evidence.

---

# TODO-PEPPOL-009 — Integrate durable worker, replay and migration semantics

## Goal

Make Peppol validation safe under retries, worker replacement, duplicate delivery and upgrades.

## Work

- Extend terminal-stage completeness checks to understand when Peppol evidence is required.
- Ensure legacy terminal Invoice/CreditNote records without Peppol evidence are not accidentally reinterpreted as Peppol-valid.
- Define migration behavior for deployments upgraded from EN-only to Peppol-aware validation.
- Preserve the existing controlled generation barrier where mixed worker generations could disagree on required stages.
- Test shutdown during Peppol execution.
- Test retry exhaustion and DLQ.
- Test successful replay after validator recovery.
- Test duplicate replay and exactly-one permanent result.

## Acceptance criteria

No crash/replay/upgrade path can publish or preserve a false Peppol-valid result.

---

# TODO-PEPPOL-010 — Extend HTTP/OpenAPI validation profile contract

## Goal

Allow clients to request and inspect Peppol validation explicitly.

## Work

- Add the chosen `validationProfile` request contract.
- Preserve backward compatibility for existing XML upload clients.
- Expose requested/resolved profile in status responses.
- Add Peppol stage schema and examples to OpenAPI.
- Add examples for:
  - Peppol-valid Invoice;
  - Peppol-valid CreditNote;
  - XSD-invalid Invoice;
  - EN-invalid Invoice;
  - Peppol-invalid Invoice;
  - unsupported Peppol ProfileID;
  - Peppol operational failure.
- Keep raw SVRL internal.

## Acceptance criteria

The public API can distinguish `EN 16931 valid` from `Peppol BIS Billing valid` without ambiguity.

---

# TODO-PEPPOL-011 — Build a positive Peppol conformity corpus

## Goal

Prove valid real-world Peppol Billing documents pass all three stages.

## Work

Include:

- official OpenPeppol Invoice examples;
- official CreditNote examples;
- Profile 01 examples;
- Profile 02 examples where officially available;
- representative VAT categories;
- representative payment means;
- identifier schemes;
- allowances/charges;
- attachments/references where allowed;
- country-specific examples for rules present in the pinned artefact.

Each fixture must assert:

```text
XSD valid
EN 16931 valid
Peppol valid
```

## Acceptance criteria

A mandatory real-runtime test target runs the positive corpus with missing dependencies treated as failure, not skip.

---

# TODO-PEPPOL-012 — Build a rule-targeted negative Peppol corpus

## Goal

Prove the new layer catches Peppol failures that XSD + EN 16931 alone would accept.

## Work

Create or mutate XSD-valid, EN-valid documents to trigger representative categories including:

- invalid/missing Peppol `CustomizationID`;
- invalid/missing `ProfileID`;
- invalid EndpointID/EAS schemes;
- invalid party identifier/ICD schemes;
- Peppol-specific code-list failures;
- payment-means restrictions;
- Peppol syntax/cardinality restrictions;
- currency consistency rules;
- line arithmetic/slack rules;
- `PEPPOL-COMMON-*` identifier validation;
- selected country-specific rules bundled in the release.

For the pinned release, the Peppol attachment MIME assertion uses the same
allowed set that EN 16931 rule `BR-CL-24` already rejects. An unsupported MIME
fixture must therefore remain an EN-invalid short-circuit control and must not
be counted as Peppol-targeted coverage; manufacturing a Peppol-only MIME case
would violate the required XSD-valid and EN-valid preconditions.

Each fixture must record the expected Peppol rule ID(s).

Prefer one deliberate semantic defect per fixture.

Also retain controls proving:

- XSD-invalid input never reaches EN/Peppol;
- EN-invalid input never reaches Peppol.

## Acceptance criteria

The corpus demonstrates that the Peppol stage adds real conformity coverage beyond the existing EN stage.

---

# TODO-PEPPOL-013 — Cross-check against official/reference validation behavior

## Goal

Reduce the risk of a locally compiled/executed Schematron behaving differently from the official Peppol artefacts.

## Work

- Compare the server against official Peppol example outcomes.
- Where practical, compare rule IDs/severity with an independent reference validator using the exact same Peppol release.
- Freeze a representative expected-rule manifest.
- Verify no rules disappear during Schematron compilation.
- Verify the number/namespaces of active patterns/asserts against the pinned source.
- Document acceptable differences such as diagnostic formatting versus semantic outcome differences, which are not acceptable.

## Acceptance criteria

There is evidence that the server executes the pinned Peppol rules faithfully, not merely an internally self-consistent approximation.

---

# TODO-PEPPOL-014 — Hardening, observability and capacity

## Goal

Give the third validation stage the same production guarantees as XSD and EN 16931.

## Work

- Add a distinct Peppol stage timeout beneath the overall validation budget.
- Preserve worker lifecycle cancellation.
- Bound Peppol SVRL, stderr, issue count and diagnostic fields.
- Add Prometheus stage outcome/duration metrics.
- Add bounded error-reason labels.
- Add structured logs with upload ID, document type, profile, stage, release version, outcome, issue count and duration.
- Never log invoice content.
- Benchmark:
  - EN-valid + Peppol-valid small Invoice;
  - Peppol-invalid Invoice;
  - large/high-line-count Peppol-valid Invoice;
  - concurrency 1/2/4;
  - complete durable publication latency.
- Revisit worker/pod capacity assumptions after adding the third Saxon transform.

## Acceptance criteria

The Peppol stage is bounded, observable and has measured production cost.

---

# TODO-PEPPOL-015 — Production packaging and upgrade policy

## Goal

Make Peppol validation reproducible and safely upgradeable.

## Work

- Package the Peppol runtime artefacts in the production image.
- Verify all Peppol assets by checksum before the worker connects to NATS.
- Keep validation offline.
- Extend startup version logging with:
  - Peppol BIS version;
  - Peppol release date/identifier;
  - Peppol Schematron/source SHA-256;
  - compiled XSLT SHA-256;
  - Schematron compiler version if compilation is used.
- Add Peppol third-party notices/licence/provenance.
- Add an image smoke test with `--network none`.
- Extend GitHub Actions mandatory validation gate.
- Document the Peppol release upgrade process.
- Because Peppol releases become mandatory on published transition dates, record both publication and mandatory-from dates and require a planned upgrade before the currently pinned release expires operationally.
- Never combine a Peppol ruleset upgrade with unrelated Java/Saxon changes unless required; keep regressions attributable.

## Acceptance criteria

A clean image contains everything required to perform Peppol Billing payload validation offline and logs the exact rule release being executed.

---

# TODO-PEPPOL-016 — Final public end-to-end Peppol compliance gate

## Goal

Prove the public server contract, not merely individual adapters.

## Required scenarios

1. **Valid Peppol Invoice**
   - upload through authenticated public API;
   - XSD passes;
   - EN 16931 passes;
   - Peppol passes;
   - final `status=valid`;
   - exactly one permanent result.

2. **Valid Peppol CreditNote**
   - same three-stage success.

3. **XSD-invalid Peppol request**
   - XSD fails;
   - no EN or Peppol stage.

4. **EN-invalid Peppol request**
   - XSD passes;
   - EN fails;
   - Peppol does not run.

5. **Peppol-invalid but EN-valid Invoice**
   - XSD passes;
   - EN passes;
   - Peppol fails;
   - API exposes Peppol rule ID, severity, message and layer.

6. **Missing/wrong Peppol identifiers under explicit Peppol request**
   - must be invalid, never silently downgraded to EN-only validation.

7. **Profile 01 valid**.

8. **Profile 02 valid**.

9. **Peppol validator operational failure**
   - retry;
   - exhaustion -> `processing_failed` + DLQ;
   - no false permanent standards result;
   - successful replay after runtime restoration.

10. **Worker interruption while Peppol is running**
    - no partial XSD/EN result becomes final;
    - no shutdown attempt consumed;
    - replacement worker completes safely.

11. **Duplicate/replayed descriptor**
    - one durable final result only.

12. **Non-Peppol EN-valid Invoice in EN-only/backward-compatible mode**
    - remains EN-valid without being falsely labelled Peppol-valid.

## Final acceptance statement

`TODO-PEPPOL` payload validation is complete only when this statement is true:

> When a client requests Peppol BIS Billing validation, e-ubl21 reports `valid` only after the submitted UBL Invoice or CreditNote has passed the correct UBL 2.1 XSD, the pinned EN 16931 rule set, and the pinned current Peppol BIS Billing rule set for the resolved supported profile. Peppol standards failures are exposed as structured Peppol-layer diagnostics, while validator infrastructure failures remain retryable operational failures and participate safely in the existing durable queue, DLQ and replay mechanisms.

---

# TODO-PEPPOL-017 — Optional Peppol enveloping/SBDH conformity validation

## Goal

Extend beyond payload conformity to the optional Peppol Business Message Envelope/SBDH validation artefacts published with the Billing release.

This is deliberately separated because payload validity and transport-envelope validity are different concerns.

## Work

- Pin the official enveloping Schematron for the selected Peppol release.
- Define an additional validation layer such as `peppol-envelope`.
- Validate sender/receiver identifiers and document/process identifiers against the envelope and payload.
- Ensure SBDH identifiers agree with `CustomizationID`, `ProfileID`, document type and participant addressing.
- Add positive/negative SBDH corpora.
- Package and version the envelope artefact independently.

## Acceptance criteria

The server can optionally validate a complete Peppol business message envelope without conflating envelope failure with payload failure.

---

## 6. Recommended implementation order

```text
PEPPOL-001  freeze scope / contract
    |
PEPPOL-002  pin official release
    |
PEPPOL-003  package source artefacts
    |
PEPPOL-004  deterministic Schematron -> XSLT
    |
PEPPOL-005  layered result model
    |
PEPPOL-006  profile applicability
    |
PEPPOL-007  validator adapter
    |
PEPPOL-008  pipeline integration
    |
PEPPOL-009  durable/replay/migration semantics
    |
PEPPOL-010  HTTP/OpenAPI contract
    |
PEPPOL-011  positive corpus
    |
PEPPOL-012  targeted-negative corpus
    |
PEPPOL-013  independent/reference cross-check
    |
PEPPOL-014  hardening / observability / capacity
    |
PEPPOL-015  packaging / upgrade policy
    |
PEPPOL-016  final public compliance gate
    |
PEPPOL-017  optional SBDH/envelope conformity
```

---

## 7. Target server validation architecture after completion

```text
                         +--------------------+
Client XML ------------->| validation profile |
                         | resolver           |
                         +---------+----------+
                                   |
                                   v
                            UBL 2.1 XSD
                                   |
                             standards invalid
                                   |
                                   v
                               EN 16931
                                   |
                             standards invalid
                                   |
                                   v
                     Peppol BIS Billing 3.0
                                   |
                             standards invalid
                                   |
                                   v
                    optional envelope/SBDH
                                   |
                                   v
                         compliant result
```

The profile resolver should later become generic enough to support additional rule stacks, for example:

```text
UBL 2.1
EN 16931
Peppol BIS Billing
Peppol Self-Billing
country CIUS
national extensions
```

without hard-coding one giant validation function.

---

## 8. Relationship to the future generic JSON invoice API

The future generic JSON invoice generator described in `docs/SERVER-FUNCTIONALITY-AND-INVOICE-PLATFORM-ARCHITECTURE.md` should eventually use this same validation profile engine.

The generation flow should therefore be:

```text
minimal client JSON
      |
      v
canonical invoice model
      |
      v
profile/default resolver
      |
      v
UBL XML renderer
      |
      v
UBL XSD
      |
      v
EN 16931
      |
      v
Peppol BIS Billing
      |
      v
return validated XML / queue for delivery
```

Generation must never bypass validation. The generated document should be considered ready for Peppol delivery only after the Peppol profile gate succeeds.

---

## 9. Definition of success

The first Peppol product milestone is achieved when a client can submit a UBL Invoice or CreditNote, explicitly request `peppol-bis-billing`, and receive a durable structured result that proves:

```text
UBL 2.1             PASS
EN 16931            PASS
Peppol BIS Billing  PASS
```

or identifies the exact failing layer and official rule IDs when it does not pass.

Actual Peppol network transmission is a later product capability and is not required for this validation milestone.
