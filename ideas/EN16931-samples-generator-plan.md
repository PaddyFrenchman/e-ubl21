# EN 16931 Samples Generator — Implementation Plan

**Project:** UBL-21 Validator
**Document:** `EN16931-samples-generator-plan.md`
**Purpose:** Step-by-step plan to extend the existing UBL sample generator (`createsamples/`) so it can also produce an **EN 16931 semantically-compliant** positive/negative test corpus for Invoice and CreditNote, as required by the test-corpus strategy in [MAIN-EN16931-UBL-validation.md](/home/a/e-ubl21/ideas/MAIN-EN16931-UBL-validation.md) (§46–48).
**Status:** Proposed implementation roadmap
**Prepared:** 22 August 2026

---

## 1. Relationship to existing documents

Two documents already exist and must not be duplicated, only extended:

- [.github/create-samples.md](/home/a/e-ubl21/.github/create-samples.md) — the original "UBL 2.1 Sample Generator Subproject" spec. It explicitly scopes itself to **XSD validity only**:

  > "The initial version guarantees XSD validity, not complete business-process validity. Schematron rules, PEPPOL rules, EN 16931 ... are follow-on concerns." (§2)

- [ideas/MAIN-EN16931-UBL-validation.md](/home/a/e-ubl21/ideas/MAIN-EN16931-UBL-validation.md) — the EN 16931 Schematron validation roadmap. Its §46 "Test corpus strategy" names the generated UBL samples as one of three required test-corpus sources, but only for proving that `UBL-valid` does **not** imply `EN16931-valid` (§46.2). It does not yet define how a *positive* EN16931 corpus (documents that should pass) or a *targeted* negative corpus (documents that should fail one named rule) get built.

This document is the missing piece: it defines **Phase EN-0 … EN-6**, a follow-on to the existing generator's Phase SG-0…SG-7 (see `.github/create-samples.md` §14), that adds a second, semantic-aware generation mode limited to `Invoice` and `CreditNote`.

---

## 2. What was verified against the official EN 16931 documentation

Fetched directly from the pinned release referenced in the MAIN doc (`https://github.com/ConnectingEurope/eInvoicing-EN16931`, tag `validation-1.3.16`):

- The repository is the official CEN/TC 434 validation artefact source; it explicitly does **not** contain CIUS rules (e.g. Peppol BIS) — confirmed via its `README.md`.
- `ubl/examples/` contains reference fixtures: `guide-example1.xml`, `guide-example2.xml`, `BIS3_Invoice_positive.XML`, `BIS3_Invoice_negativ.XML`, plus a large real-world Italian example. The `BIS3_*` pair is a Peppol BIS fixture (out of scope here beyond structural inspiration); `guide-example1.xml`/`guide-example2.xml` are baseline-EN16931 UBL invoices and are the most directly reusable references.
- Inspecting `guide-example1.xml` confirms the concrete shape a generator must produce, not just what the XSD allows:
  - `cbc:CustomizationID` **must** be the literal `urn:cen.eu:en16931:2017` (this is BT-24, checked by `BR-01`-family rules) — the current generic generator instead emits a placeholder identifier (`ID-001`), which is XSD-valid but immediately fails EN 16931.
  - Party blocks require specific nested aggregates to be present together: `cac:PartyTaxScheme`/`cac:TaxScheme` (seller VAT identification), `cac:PartyLegalEntity/cbc:RegistrationName`, and a legal postal address with a `cac:Country/cbc:IdentificationCode`.
  - `cac:TaxTotal` / `cac:TaxSubtotal` / `cac:TaxCategory` amounts and the `cac:LegalMonetaryTotal` block are all numerically tied together (line extension → tax-exclusive → tax-inclusive → payable), and the `cac:TaxCategory/cbc:Percent` must match the tax subtotal amount arithmetically.
  - Generic-generator placeholder values (`CODE-001`, `USD` regardless of context, unrelated `TaxAmount`/`LegalMonetaryTotal` figures) satisfy the XSD but would fail `BR-CL-*` (code list) and `BR-CO-*` (arithmetic) rules — this is a concrete, verified instance of the exact problem §3 of the MAIN doc describes in the abstract.
- Rule family behavior (per the MAIN doc §13, cross-checked against the public EN 16931 rule set naming convention): `BR-S/Z/E/AE/G/IC/O-*` are **conditional on the VAT category code** used in each `cac:TaxCategory/cbc:ID` — e.g. category `AE` (reverse charge) requires a VAT exemption reason and typically a zero rule-level VAT amount, category `E`/`G`/`IC` similarly require an exemption reason (`cbc:TaxExemptionReasonCode`/`cbc:TaxExemptionReason`), while `S` (standard-rated) requires a non-zero rate and the seller's VAT identifier. A single generic sample cannot exercise all of these; the corpus needs **one variant per VAT category family** to give the EN16931 stage (§13/§46) real coverage.
- `BR-CL-*` code-list rules mean the generator cannot use arbitrary placeholder codes (`CODE-001`, `USD` for a Dutch/German seller, etc.) — it needs a small, curated, checked-in table of **known-legal code values** (currency, country, VAT category, invoice type code, unit code), analogous to the enumeration-first policy `createsamples/values.go` already uses for XSD enumerations, but sourced from the EN 16931 code lists, not just XSD restrictions.

Conclusion confirmed from documentation, not assumption: the current `createsamples` generator (structural, reflection-driven over the XSD graph) is architecturally the wrong tool to directly produce EN16931-valid documents, because EN 16931 compliance depends on **cross-field arithmetic and conditional business-term combinations** that do not exist in the XSD content model at all. A second, semantic-model-driven generator is required. The existing generator remains valuable exactly as the MAIN doc's §46.2 already says: as a source of "UBL-valid but not EN16931-valid" negative fixtures.

---

## 3. Design principle

Keep two clearly separated generation strategies, matching the two clearly separated validation stages the MAIN doc already defines (UBL XSD vs. EN 16931 Schematron):

```text
createsamples (existing)                 createsamples/en16931 (new)
--------------------------                --------------------------
schema-graph driven                       business-term (BT/BG) driven
covers all 65 doc types                   Invoice + CreditNote only
guarantees XSD validity                   guarantees XSD validity AND
                                           EN 16931 semantic validity
placeholder scalar values                 code-list-correct, arithmetically
                                           consistent scalar values
used as EN16931 "expected to fail"        used as EN16931 "expected to pass"
fixtures (MAIN doc §46.2)                 fixtures, plus a mutation-based
                                           targeted-negative corpus (§46.3)
```

The new generator is **not** a rewrite of `createsamples`; it is an additional mode that reuses what already fits (namespace constants, XML rendering, CLI conventions) and adds a semantic layer on top.

---

## 4. Proposed package layout

Follow the existing package convention (`.github/create-samples.md` §15) rather than inventing a new one:

```text
createsamples/
├── generate.go        (existing — structural/XSD generator, unchanged)
├── values.go           (existing — XSD scalar values, unchanged)
├── render.go           (existing — XML serialization, reused as-is)
├── commands.go         (existing — gains a new subcommand, see §9)
│
└── en16931/                       (new package)
    ├── model.go        — Go structs for the Invoice/CreditNote business-term
    │                     model (BG/BT groups actually required by EN 16931)
    ├── codelists.go     — curated, versioned tables of legal code values
    │                     (currency, country, VAT category, invoice type,
    │                     unit code) pinned to the EN 16931 artefact version
    ├── calculate.go     — monetary/tax arithmetic (BR-CO-* family): line
    │                     totals → tax subtotals → document totals, with
    │                     correct decimal rounding (BR-DEC-*)
    ├── variants.go      — the VAT-category variant matrix (S/Z/E/AE/G/IC/O)
    │                     and the mutation catalogue for the negative corpus
    ├── build.go         — assembles a populated model into the shared
    │                     xmlNode tree (createsamples.xmlNode) for reuse of
    │                     render.go
    └── manifest.go       — writes the machine-readable fixture manifest
                          described in §8 below
```

Rationale for a separate package rather than extending `generate.go` in place: the structural generator's `genState`/`expandElement` machinery is schema-shape-driven and deliberately has no concept of "this scalar must equal the sum of those other three scalars" — bolting semantic constraints onto it would fight its design instead of using it. A dedicated business-object model is simpler to keep correct and easier to review against the standard.

---

## 5. Business-term data model (`en16931/model.go`)

Model only what EN 16931 baseline actually requires plus what its conditional rule families need, not the full UBL content model:

```text
InvoiceDocument
├── CustomizationID          (fixed: "urn:cen.eu:en16931:2017")
├── ID, IssueDate, DueDate, InvoiceTypeCode, DocumentCurrencyCode
├── Seller  (name, address+country, VAT scheme id, legal registration)
├── Buyer   (name, address+country, optional VAT scheme id)
├── PaymentMeans (code, optional payee account)
├── Lines[] (id, quantity+unitCode, netPrice, netAmount, item name,
│            VAT category id + rate)
├── TaxBreakdown[] (per VAT category: taxable amount, tax amount, rate,
│            optional exemption reason code/text)
└── Totals (lineExtension, taxExclusive, taxAmount, taxInclusive, payable)
```

`CreditNoteDocument` mirrors this with `CreditNoteTypeCode` and `CreditNoteLine`.

This is intentionally a much smaller, flatter model than the full UBL schema graph — it only encodes the fields EN 16931 actually constrains, plus enough structural context (nesting, attributes) to render valid UBL XML around them.

---

## 6. Arithmetic and rounding engine (`en16931/calculate.go`)

Implement as pure functions, unit-testable without any XML involved:

1. Per line: `netAmount = round2(quantity * netPrice)`.
2. `LineExtensionAmount = sum(line.netAmount)` (mirrors `BR-CO-10`).
3. Group lines by VAT category+rate into `TaxSubtotal` entries; each subtotal's `TaxAmount = round2(taxableAmount * rate / 100)` (mirrors `BR-CO-17`).
4. `TaxExclusiveAmount = LineExtensionAmount` (no allowances/charges in the baseline model; document as an explicit v1 simplification — see §11 "Out of scope for v1").
5. `TaxAmount (document level) = sum(subtotal.TaxAmount)`.
6. `TaxInclusiveAmount = TaxExclusiveAmount + TaxAmount` (mirrors `BR-CO-15`).
7. `PayableAmount = TaxInclusiveAmount` (no prepaid/rounding amount in v1; document as a v1 simplification).
8. All monetary values rounded/formatted to exactly 2 decimals (mirrors the `BR-DEC-*` family) using a single shared `round2` helper — never ad hoc `fmt.Sprintf` per call site, to avoid rounding drift between totals and subtotals.

Golden rule for the whole engine: **derive every downstream figure from the line items programmatically; never hand-author two independently-typed numbers that are supposed to reconcile.** This is exactly the failure mode the MAIN doc's §3 worked example demonstrates.

---

## 7. Code lists and VAT-category variants (`en16931/codelists.go`, `en16931/variants.go`)

- Curate a small, explicitly-sourced table per code list, each entry commented with which EN 16931/UBL code list it came from (currency = ISO 4217, country = ISO 3166-1 alpha-2, VAT category = UNCL 5305 subset used by EN 16931, invoice type code = UNTDID 1001 subset, unit code = UN/ECE Recommendation 20 subset). Values only need to be legal, not exhaustive.
- Pin this table's provenance to the same EN 16931 artefact version already pinned in `validation-assets/en16931/<version>/` per the MAIN doc §30, so a future artefact upgrade (§48) prompts a review of whether the code table also needs updating.
- Define one **positive variant per VAT category rule family** relevant to UBL invoicing, each toggling exactly the fields that family's rules examine:

  | Variant | VAT category (`cac:TaxCategory/cbc:ID`) | Extra required fields |
  |---|---|---|
  | Standard-rated | `S` | Seller VAT id, rate > 0 |
  | Zero-rated | `Z` | Rate = 0 |
  | Exempt | `E` | `TaxExemptionReasonCode`/`TaxExemptionReason` |
  | Reverse charge | `AE` | Buyer VAT id, exemption reason, rule-level VAT amount = 0 |
  | Export outside EU | `G` | Exemption reason |
  | Intra-community supply | `K` | Seller & buyer EU VAT ids, exemption reason |
  | Not subject to VAT | `O` | Exemption reason, no VAT breakdown amount |

  **Correction (verified during Phase EN-2 against the official
  `EN16931-UBL-codes.sch` in the pinned `validation-1.3.16` release):** the
  actual `cac:TaxCategory/cbc:ID` code for "Intra-community supply" is `K`,
  not `IC` — confirmed both by the `BR-CL-17`/`BR-CL-18` accepted-value list
  (`AE L M E S Z G O K B`) and by the compiled `EN16931-UBL-validation.xslt`'s
  `BR-IC-01` rule, whose test expression checks `cbc:ID = 'K'`. `IC` is only
  the name of the `BR-IC-*` rule *family* (MAIN doc §13); confusingly, `IC`
  also appears as a substring inside the unrelated VATEX exemption-reason
  code `VATEX-EU-IC`, which is why the mislabeling was easy to make. See
  `createsamples/en16931/codelists.go`'s `VATCategories` table for the
  verified value and its provenance.

  Each variant is a small, targeted document, not one document trying to exercise every family simultaneously — this keeps each failure attributable to one rule family during debugging.

  **Further corrections (verified during Phase EN-3, step 14, by actually
  running every variant through the official Saxon + `EN16931-UBL-
  validation.xslt` pipeline and fixing every real failure it reported —
  not by re-reading the rule text alone):** the table above understated
  several families' requirements.

  | Rule | Finding |
  |---|---|
  | `BR-Z-02` | The **Zero-rated** family also requires the Seller VAT identifier, not only `S` — the table's "Rate = 0" entry for `Z` was incomplete. |
  | `BR-E-02` | The **Exempt** family also requires the Seller VAT identifier. |
  | `BR-S-10`/`BR-Z-10` | `S` and `Z` must **not** carry a VAT exemption reason at all (not just "not required" — actively forbidden). |
  | `BR-O-05` | The **Not subject to VAT** invoice line must omit `cbc:Percent` on `cac:ClassifiedTaxCategory` entirely — a present-but-zero rate still fails. See `build.go`'s `buildLineNode`. |
  | `BR-CO-26` | Since the `O` variant cannot carry a Seller VAT identifier, the Seller needs *some* other identifier (legal registration identifier used here) so a buyer can still identify the supplier. |
  | `BR-IC-11` | The **Intra-community supply** (`K`) variant needs an Actual delivery date (BT-72, `cac:Delivery/cbc:ActualDeliveryDate`) or an Invoicing period — this v1 model only supports the former. |
  | `BR-IC-12` | The `K` variant separately needs a Deliver-to country code (BT-80, `cac:Delivery/cac:DeliveryLocation/cac:Address/cac:Country/cbc:IdentificationCode`) — a second, independent `cac:Delivery` requirement from `BR-IC-11`. |

  See `createsamples/en16931/variants.go` for the corrected, fully-verified
  implementation of all seven variants (all confirmed zero `failed-assert`
  and zero `fatal`-flagged entries against the official pipeline).

---

## 8. Negative corpus via mutation (mirrors MAIN doc §46.3)

Reuse the same idea already accepted in `.github/create-samples.md` §14 Phase SG-7 ("Negative corpus") and MAIN doc §46.3/§47, applied at the semantic layer:

1. Start from one known-good baseline model (the `S` variant).
2. Apply exactly one mutation per generated file, e.g.:
   - delete `ID` → expect a mandatory-element rule (`BR-02`-family)
   - delete `IssueDate` → expect `BR-03`-family
   - change `DocumentCurrencyCode` to a non-ISO-4217 value → expect a `BR-CL-*` code-list rule
   - change `LegalMonetaryTotal/TaxInclusiveAmount` without changing `TaxAmount` → expect `BR-CO-15`
   - change a `TaxSubtotal/TaxAmount` inconsistent with its `Percent`/taxable amount → expect `BR-CO-17`
   - drop the required exemption reason on the `E`/`AE`/`G`/`IC` variants → expect the matching `BR-E/AE/G/IC` rule
3. Write a manifest entry per mutated fixture recording: fixture path, mutation applied, and the **expected rule ID** — not merely "invalid". This directly implements the MAIN doc §46.3 requirement ("This is much stronger than merely asserting `invalid`") and feeds the §47 regression-test-rule mechanism (pin `en16931_version` + expected fatal count/rule id per fixture).

---

## 9. CLI integration (`createsamples/commands.go`)

Add one new subcommand, following the existing `inventory | generate | validate` pattern rather than overloading `generate`'s existing flags:

```bash
createsamples generate-en16931 \
    --output createsamples/samples/en16931 \
    --assets validation-assets/en16931/1.3.16
```

Behavior:

- Generates, per document type (`Invoice`, `CreditNote`):
  - `positive/<variant>.xml` for each VAT-category variant in §7;
  - `negative/<mutation-id>.xml` for each mutation in §8;
  - `manifest.json` recording, per fixture: document type, variant/mutation id, expected compliance (`valid`/`invalid`), and (for negatives) the expected rule ID.
- If `--assets` (the pinned `validation-assets/en16931/<version>/` directory from MAIN doc §30) is supplied and Saxon + the compiled XSLT are available, immediately runs the official EN16931 UBL XSLT against every generated fixture and cross-checks the SVRL result against the manifest before writing it, failing the command if any fixture disagrees with its own manifest entry. If `--assets` is omitted, it still generates the corpus but flags itself as "unverified" in the manifest, so CI can distinguish "generated" from "verified" runs.
- Never touches the existing `generate` output (`createsamples/samples/<Root>/...`); writes under its own `samples/en16931/` subtree so the existing structural corpus and its 65/65 XSD acceptance gate (`.github/create-samples.md` §13) are unaffected.

---

## 10. Phased implementation roadmap

### Phase EN-0: Ground truth and fixtures import
- Vendor the two baseline official examples (`guide-example1.xml`, `guide-example2.xml`) under e.g. `createsamples/en16931/testdata/official/`, with their EUPL 1.2 license notice preserved, for use as reference-shape fixtures in unit tests (not distributed as generator output).
- Confirm the pinned artefact version (`1.3.16`) and its `validation-assets/en16931/1.3.16/` layout already described in MAIN doc §30 exists or is created alongside this work.
- **Exit:** the two reference examples are checked in with license/provenance notes and are parseable by the project's existing XML tooling.

### Phase EN-1: Business-term model + arithmetic engine
- Implement `en16931/model.go` and `en16931/calculate.go` per §5–§6.
- Unit-test the arithmetic engine directly (no XML): given a line list, assert every derived total is internally consistent and correctly rounded.
- **Exit:** `go test ./createsamples/en16931/...` passes for the arithmetic engine in isolation.

### Phase EN-2: Code lists + one working positive fixture
- Implement `en16931/codelists.go` (§7) and `en16931/build.go` rendering one `S`-variant Invoice to XML via the existing `render.go`.
- Manually run the official Saxon + `EN16931-UBL-validation.xslt` (per MAIN doc §7/§8) against this one file and confirm zero fatals.
- **Exit:** one generated Invoice XML file is independently confirmed EN16931-valid by the official tooling (not just by the generator's own logic).

### Phase EN-3: Full VAT-category variant matrix
- Implement all seven variants from §7 for `Invoice`; extend to `CreditNote`.
- Run the official XSLT against every variant; fix the model/arithmetic/code-list logic until every variant is clean.
- **Exit:** every variant in §7 passes the official EN16931 UBL validation for both `Invoice` and `CreditNote`.

### Phase EN-4: Mutation-based negative corpus
- Implement `en16931/variants.go` mutation catalogue and manifest recording (§8).
- Run the official XSLT against every negative fixture; assert the *specific* expected rule ID appears in the SVRL output, not merely that the result is invalid.
- **Exit:** every negative fixture's manifest-declared rule ID is confirmed present in the actual SVRL output.

### Phase EN-5: CLI + CI wiring
- Add the `generate-en16931` subcommand (§9).
- Wire it into CI as a distinct job/step from the existing structural `generate`/`validate` steps, gated on the availability of the pinned Saxon + XSLT assets.
- **Exit:** a clean checkout can reproduce and self-verify the full positive+negative EN 16931 corpus from one command, consistent with the existing generator's Definition of Done style (`.github/create-samples.md` §17).

### Phase EN-6: Regression pinning and upgrade alignment
- Record `en16931_version` and expected fatal/rule outcomes per fixture, per MAIN doc §47.
- Document the upgrade procedure (MAIN doc §48) as it applies specifically to this corpus: on a new EN 16931 release, rerun Phase EN-2 through EN-4 before accepting the new artefact version, and diff any fixtures whose expected outcome changes.
- **Exit:** upgrading the pinned EN 16931 artefact version has a documented, repeatable procedure that this generator's own corpus exercises first, before the corpus is trusted as a regression baseline for the wider validator.

---

## 11. Out of scope for v1 (explicitly deferred, not forgotten)

- Allowances/charges at document or line level (`BR-CO-11/12/13` involve `AllowanceTotalAmount`/`ChargeTotalAmount`) — add once the base arithmetic engine is proven.
- Prepaid amounts and rounding amount (`BR-CO-16`) — same reasoning.
- CII syntax (the EN 16931 release also covers Cross Industry Invoice D16B) — this project is UBL-only per the MAIN doc's scope (§2).
- Peppol BIS Billing–specific rules — explicitly a later layer per MAIN doc §5/§42; the `BIS3_*` official example files are noted in §2 above only as structural inspiration, not as an EN16931-baseline target.
- Exhaustive code-list coverage (full ISO 4217/3166 tables) — only enough legal values to exercise each rule family are needed for this corpus; the runtime validator's own code-list checking is the official XSLT's responsibility, not this generator's.

---

## 12. Definition of done

1. `createsamples generate-en16931` produces, for both `Invoice` and `CreditNote`, one positive fixture per VAT-category variant (§7) and one negative fixture per mutation (§8), plus a `manifest.json`.
2. Every positive fixture is independently confirmed EN16931-valid (zero fatals) by the official Saxon + `EN16931-UBL-validation.xslt` pipeline described in the MAIN doc.
3. Every negative fixture is independently confirmed to trigger its manifest-declared rule ID by the same pipeline.
4. The existing structural generator (`createsamples generate`) and its 65/65 XSD acceptance gate are unmodified and unaffected.
5. The corpus is reproducible from a clean checkout using the pinned EN 16931 artefact version, with the upgrade procedure documented and exercised at least once.

---

## 13. Step-by-step execution checklist

This is the concrete, sequential task list to execute the plan above. It expands §10's phases into atomic, checkable steps; each step names the section it implements so context is never lost when picking the work back up.

### Setup

1. [x] Create the `createsamples/en16931/` package directory (empty `.go` files with package declarations only).
2. [x] Create `createsamples/en16931/testdata/official/` and vendor `guide-example1.xml` and `guide-example2.xml` from `https://github.com/ConnectingEurope/eInvoicing-EN16931/tree/validation-1.3.16/ubl/examples`, preserving the EUPL 1.2 license header found in each file (§2, Phase EN-0).
3. [x] Verify (or create) `validation-assets/en16931/1.3.16/` containing `EN16931-UBL-validation.xslt`, its `source-url.txt` and `sha256.txt`, per MAIN doc §30 (Phase EN-0).
4. [x] Confirm Saxon is available locally (dev machine or container) so the manual verification steps below are actually runnable (MAIN doc §8).

### Model and arithmetic (§5, §6, Phase EN-1)

5. [x] Write `en16931/model.go`: `InvoiceDocument`, `CreditNoteDocument`, `Party`, `Line`, `TaxSubtotal`, `Totals` structs per §5.
6. [x] Write `en16931/calculate.go`: `round2`, per-line net amount, line-extension sum, per-category tax subtotal grouping, tax-inclusive/payable totals per §6.
7. [x] Write table-driven unit tests for `calculate.go` covering: single line, multiple lines same VAT category, multiple lines mixed VAT categories, and a rounding-edge case (e.g. a rate that produces a third decimal digit).
8. [x] Run `go test ./createsamples/en16931/...` and confirm all arithmetic tests pass before writing any XML rendering code.

### Code lists and first fixture (§7, Phase EN-2)

9. [x] Write `en16931/codelists.go` with curated legal values for currency, country, VAT category, invoice type code, unit code — each entry commented with its source code list (ISO 4217 / ISO 3166-1 / UNCL 5305 / UNTDID 1001 / UN/ECE Rec 20).
10. [x] Write `en16931/build.go` to convert a populated `InvoiceDocument` (VAT category `S`) into the shared `xmlNode` tree, reusing `createsamples.renderXMLNode` for serialization.
11. [x] Generate one `S`-variant Invoice XML file to a scratch path and manually run:
    ```bash
    java net.sf.saxon.Transform -s:<file>.xml -xsl:validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt -o:svrl.xml
    ```
12. [x] Inspect `svrl.xml` for `<svrl:failed-assert>` elements; if any appear, fix the model/arithmetic/code-list values causing them and repeat step 11 until zero fatals remain.
13. [x] Commit the working `S`-variant generation code once confirmed clean.

### Full variant matrix (§7, Phase EN-3)

14. [x] Implement the `Z`, `E`, `AE`, `G`, `IC`, `O` VAT-category variants in `en16931/variants.go`, each toggling only the fields listed in the §7 table.
15. [ ] Extend `build.go`/`model.go` as needed to support `CreditNoteDocument` alongside `InvoiceDocument`.
16. [ ] For each of the 7 variants × 2 document types (14 fixtures total), repeat the generate → run Saxon → inspect SVRL → fix loop from steps 11–12.
17. [ ] Once all 14 fixtures are clean, wire them into a Go test that shells out to Saxon (or a fake/skippable stub when Saxon isn't installed) and asserts zero `failed-assert` elements per fixture.

### Negative corpus (§8, Phase EN-4)

18. [ ] Implement the mutation catalogue in `en16931/variants.go`: one function per mutation (delete ID, delete IssueDate, break currency code, break a total, break a tax subtotal, drop a required exemption reason).
19. [ ] For each mutation, generate the fixture, run it through Saxon, and record the actual failing rule ID(s) observed in the SVRL output.
20. [ ] Write `en16931/manifest.go` to persist, per fixture, the document type, variant/mutation id, expected compliance, and (for negatives) the expected rule ID confirmed in step 19.
21. [ ] Add a test that reruns Saxon against every negative fixture and asserts the manifest's expected rule ID is present in the SVRL output (not just that the result is invalid).

### CLI and CI (§9, Phase EN-5)

22. [ ] Add the `generate-en16931` subcommand to `createsamples/commands.go`, following the existing `flag.NewFlagSet` pattern used by `runGenerate`/`runValidate`.
23. [ ] Implement the `--assets` flag behavior: when supplied, run the Saxon+XSLT cross-check inline and fail the command on any fixture/manifest disagreement; when omitted, generate but mark the manifest `"verified": false`.
24. [ ] Ensure output is written under `createsamples/samples/en16931/` and never touches the existing `createsamples/samples/<Root>/` structural corpus.
25. [ ] Add a CI job/step invoking `generate-en16931` with `--assets` pointed at the pinned `validation-assets/en16931/1.3.16/` directory, gated so it only runs when Saxon + the pinned XSLT are provisioned.
26. [ ] Confirm a clean checkout reproduces the full positive+negative corpus and passes CI end to end.

### Regression pinning (§10 Phase EN-6)

27. [ ] Record the pinned `en16931_version` (`1.3.16`) and each fixture's expected fatal count / rule ID inside the manifest, per MAIN doc §47.
28. [ ] Write a short upgrade runbook (in this file or a companion doc) describing: download new artefact version → verify checksum → rerun steps 11–21 against the new XSLT → diff expected-vs-actual manifest entries → explicitly approve or reject the upgrade, per MAIN doc §48.
29. [ ] Exercise the runbook once (even against the same 1.3.16 version, as a dry run) to confirm it is actually executable end to end, not just documented.

### Final acceptance

30. [ ] Re-verify all five Definition of Done criteria in §12 are met.
31. [ ] Update this document's **Status** field from "Proposed implementation roadmap" to "Implemented" once all steps above are checked off.
