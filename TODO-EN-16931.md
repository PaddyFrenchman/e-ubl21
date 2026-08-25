# TODO-EN-16931 — Server-side EN 16931 semantic validation

**Project:** e-ubl21 / UBL-21 e-invoice validation  
**Status:** Planned  
**Created:** 2026-08-25  
**Scope:** Add mandatory EN 16931 semantic/business-rule validation to the existing server validation pipeline for UBL `Invoice` and `CreditNote` documents.

> This document defines EN 16931-specific implementation steps. The identifiers `TODO-EN-16931-001`, `TODO-EN-16931-002`, etc. are sub-TODOs of this roadmap and do not replace or renumber the repository-wide permanent `TODO-###` identifiers in `TODO.md`.

---

## 1. Objective

The server currently validates submitted UBL 2.1 documents against the authoritative UBL 2.1 XML Schemas. For invoices, structural XSD validity is necessary but insufficient: an XML document can be a perfectly valid UBL `Invoice` while violating EN 16931 business rules, arithmetic rules, VAT rules, code lists, cardinality restrictions, or semantic requirements.

The target behavior is therefore:

```text
submitted XML
    |
    v
root / namespace recognition
    |
    v
UBL 2.1 XSD validation
    |
    +---- FAIL --------------------> invalid
    |
    v
EN 16931 semantic validation
(Invoice / CreditNote only)
    |
    +---- fatal EN rule failure ---> invalid
    |
    v
EN 16931 compliant
    |
    v
valid
```

For `Invoice` and `CreditNote`, **the top-level server result must be `valid` only when both of these conditions are true:**

1. the document conforms to the appropriate UBL 2.1 XSD; and
2. the document conforms to the EN 16931 UBL semantic/business rules.

A UBL-valid invoice that fails EN 16931 must therefore be reported as `invalid`, with the EN 16931 rule failures exposed separately from XSD diagnostics.

Other UBL 2.1 document types continue to receive UBL 2.1 XSD validation only unless another semantic profile is explicitly added in the future.

---

## 2. Current baseline

The implementation must extend the existing runtime rather than build a parallel validator.

Current server behavior already provides:

- lightweight root and namespace recognition at ingress;
- immutable raw XML in JetStream Object Store;
- compact queued work descriptors;
- asynchronous worker execution;
- authoritative embedded UBL 2.1 XSDs;
- streamed Object Store content into `xmllint`;
- `xmllint --nonet` XSD validation with timeout and bounded diagnostics;
- durable upload metadata and result publication;
- idempotent terminal result handling;
- operational retries, `processing_failed`, DLQ and replay;
- completion reconciliation independent of work-message lifetime;
- raw-content retention and safe Object Store maintenance.

The repository also already contains EN 16931 groundwork:

- `ideas/MAIN-EN16931-UBL-validation.md` describes the intended Schematron/XSLT architecture;
- `devsetup/download_en16931_validation_assets.sh` pins the official ConnectingEurope validation artefact release `validation-1.3.16` and verifies its checksum;
- `validation-assets/en16931/1.3.16/` records the pinned validation asset metadata;
- `devsetup/download_saxon_he.sh` pins Saxon-HE `12.7` and xmlresolver `6.0.17`;
- `createsamples/en16931/` contains EN 16931-aware Invoice fixtures/variants including VAT categories `S`, `Z`, `E`, `AE`, `G`, `K`, and `O`.

As of 2026-08-25 the official ConnectingEurope repository identifies `validation-1.3.16` as the latest UBL/CII EN 16931 validation artefact release. The implementation must remain version-pinned and must never download validation rules at request time or worker startup.

---

## 3. Architectural rules

The implementation must obey the following rules throughout all steps below.

1. **XSD first, EN 16931 second.** Do not run EN 16931 rules on an XSD-invalid invoice.
2. **EN 16931 is mandatory for Invoice and CreditNote.** It is not an optional client switch in this milestone.
3. **Do not reimplement EN 16931 rules in Go.** Execute the official compiled UBL validation XSLT.
4. **Use Saxon for EN 16931.** `xmllint` remains the UBL XSD engine; Saxon executes the EN 16931 XSLT.
5. **Parse SVRL into a stable Go domain model.** Do not expose raw process stderr as the semantic API.
6. **Keep semantic invalidity separate from operational failure.** A BR-* failure means `invalid`; a missing Saxon binary, timeout, corrupt XSLT, Object Store failure, or execution crash is retryable operational failure and can ultimately become `processing_failed`.
7. **Preserve durable/idempotent processing.** The new stage must not weaken TODO-009/TODO-011 result uniqueness, replay safety, or reconciliation.
8. **Pin standards and tool versions.** Upgrades must be deliberate, reviewed, tested changes.
9. **No network dependency during validation.** Production validation must use local immutable assets only.
10. **Expose validation layers explicitly.** Clients must be able to distinguish UBL XSD validity from EN 16931 validity and inspect EN rule IDs.

---

# Implementation steps

## TODO-EN-16931-001 — Freeze the validation contract and supported document scope ✅ DONE

### Goal

Define exactly what the server means by `valid` once EN 16931 support is enabled.

### Work

- Declare `Invoice` and `CreditNote` as the EN 16931-enabled UBL document types.
- Define the mandatory pipeline:

```text
Invoice/CreditNote = UBL 2.1 XSD + EN 16931
other UBL types    = UBL 2.1 XSD only
```

- Define short-circuit behavior: EN 16931 does not run after an XSD failure.
- Define overall status semantics:
  - `valid`: every required validation stage passed;
  - `invalid`: at least one required standards-validation stage failed;
  - `processing_failed`: validation could not be completed because of operational/infrastructure failure.
- Decide severity policy for SVRL. Initial policy should be:
  - `fatal`/error failed assertions make EN 16931 invalid;
  - warnings are retained but do not make the document invalid;
  - unknown severities are retained verbatim and handled conservatively by a documented policy.
- Document that Peppol BIS Billing / national CIUS validation is outside this TODO and remains a future layer after EN 16931.

### Acceptance criteria

The server contract unambiguously distinguishes structural validity, EN 16931 semantic validity, and operational processing failure.

Implemented by `server/validation_contract.go`, its executable policy tests,
and `docs/EN16931-VALIDATION-CONTRACT.md`. Unknown/empty SVRL severities use
the conservative initial policy: retain the original value and invalidate the
EN 16931 stage rather than risk a false compliant result.

---

## TODO-EN-16931-002 — Formalize and verify pinned EN 16931 runtime assets ✅ DONE

### Goal

Make the official EN 16931 rules a reproducible, immutable runtime dependency.

### Work

- Keep the official UBL XSLT pinned to `validation-1.3.16` for the initial implementation.
- Verify the existing recorded checksum in `devsetup/download_en16931_validation_assets.sh`.
- Add a small Go/build-time or CI verification step that checks the runtime artefact against the expected checksum before packaging.
- Record in one source-level constant/config object:
  - profile name: `en16931`;
  - profile version: `1.3.16`;
  - XSLT path/name;
  - source repository/release identifier.
- Do not fetch GitHub, Maven, or any standards site at runtime.
- Document the upgrade procedure: new release, checksum review, fixture suite, compatibility review, explicit version bump.

### Acceptance criteria

A build can prove which EN 16931 rule version it contains, and two builds using the same source revision use the same validation artefact.

Implemented by `validation-assets/assets.go`, the offline
`tools/verify-validation-assets` command, and the pre-packaging verification in
the `Dockerfile`. The controlled upgrade procedure is documented in
`docs/EN16931-ASSET-UPGRADE.md`.

---

## TODO-EN-16931-003 — Package and validate the Saxon runtime ✅ DONE

### Goal

Provide a production-grade XSLT processor for the official EN 16931 stylesheet.

### Work

- Reuse the existing pinned Saxon-HE `12.7` and xmlresolver `6.0.17` choices unless testing finds an incompatibility.
- Extend the worker runtime image with a pinned Java runtime and the required Saxon/xmlresolver JARs.
- Keep Saxon and the EN 16931 XSLT local to the container/image.
- Add worker-startup validation analogous to `ensureXMLLint()`:
  - Java executable available;
  - Saxon classes load;
  - EN 16931 XSLT exists and is readable;
  - optional one-time smoke transform proves the packaged combination works.
- Fail worker startup clearly if EN 16931 is configured as mandatory but its runtime cannot be initialized.
- Ensure the transform does not resolve arbitrary network resources.

### Acceptance criteria

The production worker image can execute the pinned EN 16931 XSLT without network access and fails fast on a broken Saxon/XSLT installation.

Implemented by `server/saxon_runtime.go`, the pinned metadata and restricted
configuration in `validation-assets`, and the worker/container configuration.
The mandatory worker CLI verifies Java, both JAR digests, configuration and
XSLT digests, then executes a real SVRL smoke transform before connecting to
NATS. Direct library callers may explicitly leave EN 16931 non-mandatory while
embedding the worker during the staged rollout.

---

## TODO-EN-16931-004 — Introduce a layered validation result model ✅ DONE

### Goal

Represent XSD and EN 16931 results independently while preserving one overall document status.

### Work

Introduce stable models along these lines:

```go
type ValidationIssue struct {
    Layer      string `json:"layer"`
    Rule       string `json:"rule,omitempty"`
    Severity   string `json:"severity"`
    Location   string `json:"location,omitempty"`
    Test       string `json:"test,omitempty"`
    Message    string `json:"message"`
    Diagnostic string `json:"diagnostic,omitempty"`
}

type ValidationStageResult struct {
    Layer       string            `json:"layer"`
    Version     string            `json:"version,omitempty"`
    Valid       bool              `json:"valid"`
    Issues      []ValidationIssue `json:"issues,omitempty"`
    ValidatedAt string            `json:"validatedAt"`
    DurationMS  int64             `json:"durationMs,omitempty"`
}
```

The exact shape may change during implementation, but it must support at least:

- `ubl21-xsd` stage result;
- `en16931` stage result;
- EN rule ID;
- severity;
- XPath/location where supplied;
- failed test expression where supplied;
- human-readable message;
- validator/profile version.

Extend `UploadRecord` and `ValidationResult` without putting raw SVRL or raw XML into JetStream KV/work messages.

Preserve backwards-compatible top-level `status` semantics where practical, while changing the meaning of `valid` for Invoice/CreditNote to mean all mandatory stages passed.

### Acceptance criteria

The persistent/result model can answer independently:

```text
Did UBL XSD pass?
Did EN 16931 pass?
Which EN 16931 rules failed?
Which profile version produced the result?
```

Implemented by the stable public models in `queue/results.go`. Both
`UploadRecord` and `ValidationResult` retain ordered `validationStages`; each
stage carries its layer, validator/profile version, validity, timing and
structured issues. The current worker now persists and publishes the UBL 2.1
XSD stage, while the EN 16931 parser/adapter steps will populate the semantic
stage. The status endpoint and both OpenAPI representations expose the same
model without raw XML or raw SVRL.

---

## TODO-EN-16931-005 — Implement and test the SVRL parser ✅ DONE

### Goal

Convert official Schematron Validation Report Language output into the stable Go result model.

### Work

- Add a dedicated package/module for SVRL decoding.
- Parse at minimum:
  - `svrl:failed-assert`;
  - `svrl:successful-report` where relevant;
  - `id`;
  - `flag`/severity;
  - `location`;
  - `test`;
  - `svrl:text`;
  - diagnostic references where present.
- Preserve official EN 16931 rule identifiers such as `BR-*`, `BR-CO-*`, `BR-DEC-*`, `BR-S-*`, `UBL-SR-*`, `UBL-CR-*`.
- Normalize whitespace without destroying useful diagnostic text.
- Bound parsed/output diagnostic volume so a malicious or pathological stylesheet result cannot create unbounded KV/API payloads.
- Treat malformed/unparseable SVRL as an operational validator failure, not as an EN 16931 business-rule failure.
- Add focused unit tests with minimal SVRL fixtures for pass, fatal failure, warning, multiple failures, diagnostics, and malformed output.

### Acceptance criteria

Given deterministic SVRL, the parser produces deterministic typed validation issues and never confuses parser/runtime errors with standards non-compliance.

Implemented by the dedicated `svrl` package. It retains source order and
official rule/severity/location/test/message/diagnostic fields, normalizes
mixed-content whitespace, and fails closed on malformed XML, unexpected roots,
oversized input, excess issues, or excess normalized diagnostic text. Errors
return no partial semantic report so the validator adapter can classify them
strictly as operational failures.

---

## TODO-EN-16931-006 — Implement the EN 16931 validator adapter

### Goal

Create one server-side function/component that executes the pinned XSLT and returns a typed EN 16931 stage result.

### Work

Provide an interface similar to:

```go
type EN16931Validator interface {
    Validate(ctx context.Context, xml io.Reader) (ValidationStageResult, error)
}
```

Requirements:

- use `exec.CommandContext` or an equally bounded process API;
- apply a configurable hard timeout;
- stream XML input to Saxon rather than writing an arbitrary unbounded temporary XML file;
- capture SVRL in a bounded manner or via a bounded temporary/output mechanism suitable for the expected report size;
- bound stderr diagnostics separately;
- classify:
  - successful transform + fatal failed assertions => semantic invalid result, no operational error;
  - successful transform + no fatal assertions => semantic valid result;
  - timeout, process crash, missing runtime, unreadable XSLT, malformed SVRL => operational error;
- include EN 16931 artefact version in every stage result;
- avoid network retrieval.

If Saxon command-line constraints make direct stdin-to-SVRL streaming awkward, a tightly bounded temporary file is acceptable for the validator output/input bridge, but this must be measured and documented. The design preference remains streaming and bounded resources.

### Acceptance criteria

One isolated validator component can classify known compliant and non-compliant UBL invoices using the official XSLT and returns structured issues rather than shell text.

---

## TODO-EN-16931-007 — Build the multi-stage validation orchestrator

### Goal

Move the worker from a single XSD check to an explicit validation pipeline.

### Work

Introduce an internal orchestrator conceptually equivalent to:

```text
validate(documentType, content)
    |
    +--> UBL 2.1 XSD
    |      |
    |      +--> invalid => stop, standards-invalid
    |
    +--> if Invoice/CreditNote:
    |      EN 16931
    |      |
    |      +--> invalid => standards-invalid
    |
    +--> all required stages valid
```

The orchestrator should own stage ordering and policy rather than scattering EN 16931 conditionals through `worker.handle()`.

Important resource issue: the same immutable XML must feed two validators when XSD passes. Since the current Object Store stream is consumed by the XSD validator, choose one safe strategy and document it:

1. reopen the immutable Object Store object for the EN 16931 stage after XSD passes; or
2. introduce a bounded tee/spool abstraction if benchmarks prove reopening undesirable.

Prefer reopening the Object Store object first because it preserves bounded memory and keeps the implementation simple.

### Acceptance criteria

The worker has one clearly testable validation pipeline with deterministic stage ordering and no duplicate full-document heap buffer.

---

## TODO-EN-16931-008 — Integrate EN 16931 into durable worker state transitions

### Goal

Make semantic validation participate correctly in retries, dead-letter handling, replay, result uniqueness, and reconciliation.

### Work

- Run EN 16931 only after XSD succeeds.
- Classify an EN business-rule failure as terminal `invalid`, not retryable.
- Classify Saxon/runtime/SVRL failures as operational and send them through the existing `retryOperationalFailure` / `processing_failed` machinery.
- Ensure a worker crash between XSD completion and EN completion cannot publish a misleading final `valid` result.
- Do not make `StatusValid` durable until every mandatory validation stage has completed successfully.
- Persist enough intermediate state to recover safely if needed, but avoid unnecessary state-machine proliferation.
- Preserve TODO-009 permanent one-result-per-upload behavior.
- Preserve TODO-011 replay execution uniqueness.
- Ensure the completion reconciler republishes the complete layered result, not a partial result.
- Decide and test how old records created before this feature are interpreted during rolling upgrades.

### Acceptance criteria

No crash/retry/replay path can produce `valid` for an Invoice/CreditNote whose EN 16931 stage was never successfully completed.

---

## TODO-EN-16931-009 — Extend HTTP/OpenAPI result semantics

### Goal

Expose the layered standards result to API clients without forcing them to parse textual diagnostics.

### Work

- Extend the upload/status response with structured validation stages.
- Update both `server/openapi.go` and `server/openapi.yaml` together.
- Keep the generated and static OpenAPI documents equivalent and continue to pass the repository's strict OpenAPI lint rules.
- Document examples for:
  1. XSD-invalid Invoice;
  2. XSD-valid but EN 16931-invalid Invoice;
  3. fully compliant Invoice;
  4. non-Invoice UBL document with XSD-only validation;
  5. operational `processing_failed`.
- Make profile/version visible, e.g. `en16931` / `1.3.16`.
- Preserve rule IDs and severities in API output.
- Do not return raw SVRL by default. If raw SVRL is ever exposed, make it a separate explicit diagnostic feature.

Example target shape:

```json
{
  "id": "...",
  "document": "Invoice",
  "status": "invalid",
  "validation": {
    "ubl21": {
      "valid": true
    },
    "en16931": {
      "version": "1.3.16",
      "valid": false,
      "issues": [
        {
          "rule": "BR-CO-13",
          "severity": "fatal",
          "location": "...",
          "message": "..."
        }
      ]
    }
  }
}
```

### Acceptance criteria

A client can determine exactly why an invoice failed and which validation layer rejected it without parsing a single combined error string.

---

## TODO-EN-16931-010 — Add positive EN 16931 integration fixtures

### Goal

Prove that known compliant Invoice/CreditNote documents pass both validation layers.

### Work

- Reuse `createsamples/en16931/` as a fixture source where appropriate.
- Include all existing VAT-category Invoice variants: `S`, `Z`, `E`, `AE`, `G`, `K`, `O`.
- Include official ConnectingEurope EN 16931 example documents from the pinned release where licensing/repository policy permits.
- Add at least one valid UBL `CreditNote` fixture before declaring server EN 16931 support complete.
- For every positive fixture, assert:
  - UBL XSD valid;
  - EN 16931 valid;
  - overall status `valid`;
  - zero fatal EN issues;
  - correct EN validation version reported.

### Acceptance criteria

The integration suite demonstrates positive compliance across representative EN 16931 Invoice rules and both supported UBL billing document roots.

---

## TODO-EN-16931-011 — Add a rule-targeted negative corpus

### Goal

Prove the server detects semantic failures that XSD validation cannot detect.

### Work

Generate/mutate documents that remain UBL XSD-valid but fail specific EN 16931 rules. Cover at minimum:

- monetary total reconciliation (`BR-CO-*`);
- decimal precision (`BR-DEC-*`);
- invalid controlled codes (`BR-CL-*`);
- required seller/buyer identifiers;
- VAT category/rate consistency for `S`, `Z`, `E`, `AE`, `G`, `K`, `O`;
- exemption reason rules;
- payment means/account requirements;
- prohibited UBL constructs / syntax restrictions (`UBL-SR-*`, `UBL-CR-*`).

Each fixture should record the expected rule ID(s). Prefer one intentional semantic defect per fixture where practical so failures remain diagnosable.

Also add an XSD-invalid control fixture and assert that EN 16931 is not executed for it.

### Acceptance criteria

The test corpus proves the new layer catches documents which the existing UBL XSD validator alone would incorrectly accept as `valid`.

---

## TODO-EN-16931-012 — Harden timeout, retry, cancellation and resource limits

### Goal

Give Saxon the same production hardening expectations already applied to `xmllint`.

### Work

- Add configurable EN 16931 validation timeout.
- Ensure worker `AckWait` / progress heartbeat behavior safely covers the combined XSD + EN runtime.
- Revisit the worker deadline model: one whole-pipeline timeout versus stage-specific deadlines. Prefer explicit stage-specific timeouts plus a bounded overall worker execution budget.
- Bound:
  - Saxon stderr;
  - SVRL size;
  - issue count returned/persisted;
  - diagnostic message length.
- Verify cancellation kills the Java/Saxon process.
- Test hung validator, killed process, malformed XSLT, malformed SVRL, missing JAR, missing Java, and missing XSLT.
- Confirm every operational failure follows existing retry/DLQ semantics.

### Acceptance criteria

A malformed or adversarial invoice cannot cause an unbounded Saxon process, unbounded SVRL memory growth, or a permanently wedged JetStream work item.

---

## TODO-EN-16931-013 — Add observability for the semantic stage

### Goal

Make production EN 16931 behavior measurable separately from XSD validation.

### Work

Add metrics such as:

```text
e_ubl21_validation_stage_total{stage="ubl21_xsd",outcome="valid|invalid|error"}
e_ubl21_validation_stage_total{stage="en16931",outcome="valid|invalid|error"}
e_ubl21_validation_stage_duration_seconds{stage="..."}
e_ubl21_en16931_issues_total{severity="fatal|warning|..."}
e_ubl21_en16931_runtime_failures_total{reason="timeout|saxon|svrl|asset|..."}
```

Do not use unbounded rule IDs as Prometheus labels. Rule IDs may be logged in structured logs and returned in API results, but metric labels must remain bounded.

Add structured logs containing upload ID, document type, stage, profile version, result, issue count, and duration without logging the complete invoice body.

### Acceptance criteria

Operators can distinguish XSD failures, genuine EN 16931 non-compliance, and EN validator infrastructure failures from metrics/logs alone.

---

## TODO-EN-16931-014 — Verify performance and worker capacity

### Goal

Measure the cost of adding a Java/Saxon stage and adjust worker defaults with evidence.

### Work

- Extend worker benchmarks to include:
  - XSD-only non-billing UBL document;
  - compliant Invoice: XSD + EN 16931;
  - EN-invalid Invoice: XSD + EN 16931;
  - representative small/medium/near-limit documents.
- Measure process startup cost, total validation latency, allocations, throughput, and concurrency behavior.
- Decide whether one Saxon JVM per document is acceptable.
- If process startup is material, evaluate a persistent Saxon worker/JVM only as a later optimization, while retaining exactly the same validator interface and validation semantics.
- Revisit default `worker-concurrency`, `prefetch`, `AckWait`, CPU/memory requests, and Kubernetes scaling guidance.

### Acceptance criteria

The project has measurements for the real two-stage invoice path and documented worker capacity assumptions.

---

## TODO-EN-16931-015 — Production packaging, documentation and upgrade policy

### Goal

Make EN 16931 validation reproducible in Docker, local development, CI and deployment documentation.

### Work

- Update `Dockerfile` with the pinned Java/Saxon runtime and pinned EN XSLT.
- Update `docker-compose.yml` if new configuration is required.
- Document environment/CLI settings, including timeouts and asset/version reporting.
- Update `README.md`, `SERVER-DATAFLOW-2026-08-24.md` (or its successor), and relevant deployment docs.
- Document EN 16931 artefact upgrade procedure and regression expectations.
- Record licenses/notices required by the EN validation artefacts and Saxon dependencies.
- Make a startup log identify:
  - UBL schema version;
  - EN 16931 validation artefact version;
  - Saxon version.

### Acceptance criteria

A clean deployment contains everything necessary to validate invoices offline and exposes exactly which standards/tool versions are running.

---

## TODO-EN-16931-016 — Final end-to-end compliance gate

### Goal

Prove that the public server behavior meets the requirement, not merely that individual components work.

### Required end-to-end scenarios

1. **Valid UBL + valid EN 16931 Invoice**
   - accepted and queued;
   - XSD passes;
   - EN 16931 passes;
   - final `status=valid`.

2. **Invalid UBL Invoice**
   - XSD fails;
   - EN 16931 is not run;
   - final `status=invalid`;
   - XSD diagnostics identify the failed layer.

3. **UBL-valid but EN 16931-invalid Invoice**
   - XSD passes;
   - EN 16931 runs and fails one or more rules;
   - final `status=invalid`;
   - API exposes rule IDs, severity and messages.

4. **Valid UBL + valid EN 16931 CreditNote**
   - both stages pass;
   - final `status=valid`.

5. **EN 16931 validator operational failure**
   - no false `invalid` or `valid` result;
   - existing retry semantics are used;
   - exhaustion produces `processing_failed` + DLQ;
   - replay succeeds after the runtime is restored.

6. **Crash/restart between XSD and EN completion**
   - no partial result is published as final;
   - the job resumes/retries safely;
   - exactly one permanent final validation result is produced.

7. **Concurrent/redelivered/replayed descriptor**
   - no duplicate final result;
   - existing replay execution uniqueness remains intact.

8. **Non-Invoice UBL document**
   - still receives normal UBL 2.1 XSD validation;
   - EN 16931 is explicitly marked not applicable/not run, rather than falsely passing.

### Final acceptance criteria

`TODO-EN-16931` is complete only when the following statement is true:

> For every submitted UBL `Invoice` or `CreditNote`, the e-ubl21 server reports `valid` only after the document has successfully passed both the correct UBL 2.1 XML Schema validation and the pinned EN 16931 UBL semantic/business-rule validation. Standards failures are returned as structured layer-specific diagnostics; validator infrastructure failures remain retryable operational failures and participate safely in the existing queue, dead-letter and replay mechanisms.

---

# Recommended implementation order

```text
EN-001  validation contract/scope
   |
EN-002  pinned EN assets
   |
EN-003  Saxon runtime
   |
EN-004  layered result model
   |
EN-005  SVRL parser
   |
EN-006  EN validator adapter
   |
EN-007  multi-stage orchestrator
   |
EN-008  durable worker integration
   |
EN-009  HTTP/OpenAPI contract
   |
   +--> EN-010 positive fixtures
   +--> EN-011 negative fixtures
   |
EN-012  failure/resource hardening
   |
EN-013  observability
   |
EN-014  performance/capacity
   |
EN-015  packaging/docs/upgrade policy
   |
EN-016  end-to-end compliance gate
```

The first deployable milestone is reached after EN-012 plus sufficient EN-010/EN-011 coverage. EN-013 through EN-016 are required before declaring the feature production-complete.

---

# Explicit non-goals

This TODO does **not** implement:

- Peppol BIS Billing rules;
- national CIUS rules;
- CII validation;
- EDIFACT validation;
- invoice generation as part of the server;
- manual Go translations of EN 16931 Schematron rules;
- runtime downloading of validation assets.

Those can be layered later without weakening the core pipeline:

```text
UBL 2.1 XSD
    |
EN 16931
    |
future Peppol / CIUS profile
```
