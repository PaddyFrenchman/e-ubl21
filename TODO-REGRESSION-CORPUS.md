# TODO-REGRESSION-CORPUS — Generated compliance corpus and regression evidence

**Project:** e-ubl21  
**Status:** Planned / prerequisite  
**Created:** 2026-08-26  
**TODO namespace:** `TODO-REGRESSION-###`  
**Priority:** This roadmap is a prerequisite for the validation-stage split and for Peppol implementation.

## 1. Objective

Turn the existing `createsamples` tool into the authoritative generator of deterministic regression corpora for all validation profiles supported by e-ubl21.

The generator must ultimately produce sample sets for:

1. **UBL 2.1 XSD conformity**;
2. **EN 16931 conformity**;
3. **Peppol BIS Billing conformity**.

For every profile it must generate both:

- positive samples that are expected to pass all required validation stages; and
- targeted negative samples that remain valid for all earlier stages but fail one or more explicitly named rules in the target stage.

The generated corpus becomes the input to a repeatable regression test system executed:

- locally against Minikube;
- in ordinary Go/integration tests where Kubernetes is not required;
- in GitHub Actions on pull requests and pushes to `main`;
- optionally in scheduled/nightly performance and stress runs.

The regression system must produce a durable, machine-readable result set for each build so behavior can be compared across commits and standards/tool upgrades.

---

## 2. Why this must come first

The next architectural changes are large:

```text
current monolithic validation worker
              |
              v
asynchronous XSD / EN16931 / Peppol workers
```

and separately:

```text
UBL + EN16931
      |
      v
add Peppol BIS Billing
```

Before either change, we need a frozen body of test evidence proving what the current validator accepts and rejects.

Without that corpus, a distributed refactor can remain internally consistent while silently changing standards behavior.

The generator therefore becomes a **test-data compiler** for the validator.

The implementation order is now:

```text
TODO-REGRESSION-CORPUS
        |
        +--> freeze deterministic UBL corpus
        +--> freeze EN16931 corpus
        +--> define Peppol corpus contract
        +--> regression result format
        +--> local + GitHub regression runner
        |
        v
TODO-VALIDATION-SPLIT
        |
        v
TODO-PEPPOL
```

Some Peppol corpus cases can only become executable once the Peppol validator exists, but their manifest/model/generation contract should be designed before the validator is implemented.

---

## 3. Current generator state

The repository already contains two useful generation approaches:

### Structural UBL generator

`createsamples/` is schema-driven and can generate UBL 2.1 documents from the authoritative XSD graph.

Its natural responsibility remains:

- cover all 65 UBL 2.1 main document types;
- produce deterministic XSD-valid samples;
- produce targeted XSD-invalid variants;
- exercise schema structure, namespaces, cardinalities and datatypes.

### EN 16931 semantic generator

`createsamples/en16931/` already implements a business-term-aware Invoice/CreditNote generator with:

- valid VAT-family variants;
- consistent monetary arithmetic;
- code-list-aware values;
- positive EN 16931 fixtures;
- targeted EN 16931 negative fixtures.

This separation is correct and should be retained.

Peppol should become a third semantic/profile layer rather than forcing Peppol rules into the generic XSD generator.

---

## 4. Target corpus taxonomy

The generator/regression package must produce six primary fixture classes.

### 4.1 `ubl21-positive`

Goal:

> The document must pass its authoritative UBL 2.1 XSD.

Coverage:

- all 65 UBL 2.1 main document types;
- deterministic minimal sample per type;
- representative richer samples for important types;
- optional large/stress variants.

Example manifest expectation:

```json
{
  "id": "ubl21.invoice.minimal",
  "documentType": "Invoice",
  "profile": "ubl21",
  "expected": {
    "ubl21-xsd": "passed"
  }
}
```

### 4.2 `ubl21-negative`

Goal:

> Deliberately fail XSD validation for one documented structural reason.

Examples:

- missing required element;
- illegal datatype;
- invalid namespace;
- wrong element order;
- illegal enumeration;
- malformed XML.

Expectation:

```json
{
  "id": "ubl21.invoice.missing-id",
  "profile": "ubl21",
  "expected": {
    "ubl21-xsd": "standards-invalid"
  }
}
```

No later semantic stage must execute.

### 4.3 `en16931-positive`

Goal:

> Pass both UBL XSD and EN 16931.

At minimum include:

- Invoice;
- CreditNote;
- VAT family variants S, Z, E, AE, G, K and O;
- relevant official EN 16931 reference fixtures;
- representative multi-line cases;
- large/structurally complex cases used by capacity tests.

Expected chain:

```text
ubl21-xsd  = passed
en16931     = passed
```

### 4.4 `en16931-negative`

Goal:

> Remain XSD-valid but fail a named EN 16931 rule or rule family.

Every fixture must declare the expected rule IDs.

Example:

```json
{
  "id": "en16931.invoice.br-co-15",
  "profile": "ubl21-en16931",
  "expected": {
    "ubl21-xsd": "passed",
    "en16931": "standards-invalid"
  },
  "expectedRuleIds": ["BR-CO-15"]
}
```

A mutation should alter the smallest possible business condition so failures remain attributable.

### 4.5 `peppol-positive`

Goal:

> Pass UBL 2.1, EN 16931 and the pinned Peppol BIS Billing profile.

At minimum include:

- Invoice Profile 01;
- CreditNote Profile 01;
- Invoice Profile 02 when supported;
- CreditNote Profile 02 when supported;
- multiple legal identifier schemes;
- legal Peppol code-list values;
- relevant official OpenPeppol examples;
- country-specific rule cases contained in the selected Peppol artefact.

Expected chain:

```text
ubl21-xsd          = passed
en16931             = passed
peppol-bis-billing = passed
```

### 4.6 `peppol-negative`

Goal:

> Pass UBL 2.1 and EN 16931, then fail one explicitly named Peppol rule.

Examples should eventually cover:

- wrong/missing `CustomizationID`;
- wrong/missing `ProfileID`;
- participant identifier scheme violations;
- Peppol-specific code-list violations;
- `PEPPOL-EN16931-*` rule families;
- `PEPPOL-COMMON-*` rules;
- country-specific rules included by the selected release.

Example:

```json
{
  "id": "peppol.invoice.invalid-endpoint-scheme",
  "profile": "peppol-bis-billing",
  "expected": {
    "ubl21-xsd": "passed",
    "en16931": "passed",
    "peppol-bis-billing": "standards-invalid"
  },
  "expectedRuleIds": ["PEPPOL-COMMON-R040"]
}
```

The exact rule ID above is illustrative until the pinned Peppol corpus is implemented; manifests must use only IDs verified against the selected official release.

---

## 5. Fixture manifest as source of truth

Every generated or imported fixture must have a machine-readable manifest entry.

The manifest is more important than the filename because it defines the expected behavior independently of the current validator implementation.

Suggested schema:

```json
{
  "schemaVersion": 1,
  "id": "en16931.invoice.br-co-15",
  "source": "generated",
  "generatorVersion": "...",
  "documentType": "Invoice",
  "profile": "ubl21-en16931",
  "file": "en16931/negative/br-co-15.xml",
  "sha256": "...",
  "standards": {
    "ubl": "2.1",
    "en16931": "1.3.16",
    "peppol": null
  },
  "expectedStages": [
    {
      "stage": "ubl21-xsd",
      "outcome": "passed"
    },
    {
      "stage": "en16931",
      "outcome": "standards-invalid",
      "ruleIds": ["BR-CO-15"]
    }
  ],
  "tags": ["invoice", "negative", "monetary-total"]
}
```

Requirements:

- stable fixture ID;
- deterministic file content;
- SHA-256 of generated XML;
- standards/ruleset versions;
- expected ordered stage outcomes;
- expected rule IDs for targeted negatives;
- tags for selecting subsets;
- provenance for imported official fixtures;
- no timestamps or nondeterministic data in the manifest unless intentionally normalized.

The generator must fail if two fixtures share an ID.

---

## 6. Proposed generator CLI

Evolve `createsamples` into one coherent regression-corpus tool.

Suggested commands:

```text
createsamples inventory
createsamples generate
createsamples validate
createsamples regression-manifest
```

Suggested profile selector:

```bash
go run ./createsamples generate --profile ubl21

go run ./createsamples generate --profile en16931

go run ./createsamples generate --profile peppol-bis-billing

go run ./createsamples generate --profile all
```

Suggested corpus selector:

```bash
--set positive
--set negative
--set representative
--set stress
--set all
```

Examples:

```bash
go run ./createsamples generate \
  --profile all \
  --set all \
  --output regression/corpus
```

and:

```bash
go run ./createsamples regression-manifest \
  --corpus regression/corpus \
  --output regression/manifest.json
```

Generation must be deterministic. Running the same generator version against the same pinned standards inputs must produce byte-identical XML and identical manifest hashes.

---

## 7. Directory layout

Recommended layout:

```text
regression/
├── corpus/
│   ├── ubl21/
│   │   ├── positive/
│   │   └── negative/
│   ├── en16931/
│   │   ├── positive/
│   │   └── negative/
│   └── peppol/
│       ├── positive/
│       └── negative/
│
├── manifest.json
├── schemas/
│   └── regression-result.schema.json
└── README.md
```

Official fixtures may either be checked in where licensing permits or downloaded at build/test preparation from immutable, checksum-pinned sources.

The manifest must record provenance either way.

---

## 8. Regression result set

Raw application logs alone are **not** the regression oracle because they contain timing, IDs, scheduling order and other volatile information.

Each regression run must create a canonical result set.

Recommended output:

```text
regression-results/<run-id>/
├── summary.json
├── results.jsonl
├── junit.xml
├── environment.json
├── manifest-used.json
├── logs/
│   ├── api.log
│   ├── orchestrator.log
│   ├── worker-xsd.log
│   ├── worker-en16931.log
│   └── worker-peppol.log
└── metrics/
    └── snapshot.txt
```

### `results.jsonl`

One normalized line per fixture:

```json
{
  "fixtureId": "en16931.invoice.br-co-15",
  "sha256": "...",
  "profile": "ubl21-en16931",
  "expectedFinal": "invalid",
  "actualFinal": "invalid",
  "expectedStages": [
    ["ubl21-xsd", "passed"],
    ["en16931", "standards-invalid"]
  ],
  "actualStages": [
    ["ubl21-xsd", "passed"],
    ["en16931", "standards-invalid"]
  ],
  "expectedRuleIds": ["BR-CO-15"],
  "actualRuleIds": ["BR-CO-15"],
  "result": "pass"
}
```

Normalize or exclude:

- upload IDs;
- validation IDs;
- exact timestamps;
- durations;
- pod names;
- JetStream sequences;
- generated trace IDs.

Those belong in diagnostics/telemetry, not the correctness diff.

### `summary.json`

Example:

```json
{
  "schemaVersion": 1,
  "commit": "...",
  "total": 124,
  "passed": 124,
  "failed": 0,
  "byProfile": {
    "ubl21": {"passed": 70, "failed": 0},
    "ubl21-en16931": {"passed": 54, "failed": 0},
    "peppol-bis-billing": {"passed": 0, "failed": 0}
  }
}
```

Counts above are illustrative; acceptance thresholds must use the actual committed manifest.

### `junit.xml`

Emit JUnit XML in addition to JSON so GitHub Actions and ordinary CI viewers can display fixture failures naturally.

### Raw logs

Retain logs as an artifact for investigation, but do not use byte-for-byte log comparison as the regression verdict.

---

## 9. Baseline comparison policy

The regression suite checks two things:

### A. Manifest expectation

Every fixture must produce the outcome encoded in the committed/pinned manifest.

This is the primary correctness gate.

### B. Previous-build comparison

Optionally compare the normalized result set against an earlier successful build.

Differences must be classified as:

- expected standards/ruleset change;
- intentional implementation change;
- regression.

Never automatically bless a changed result set just because the new implementation produced it.

When an official EN/Peppol ruleset is upgraded, the upgrade change must include an explicit reviewed manifest/result change where behavior intentionally differs.

---

## 10. Local Minikube regression environment

Minikube is the required local Kubernetes integration environment.

The full distributed regression suite should eventually deploy:

```text
NATS JetStream
API
validation orchestrator
worker-xsd
worker-en16931
worker-peppol          # once implemented
regression runner
```

The same built image should be used for all e-ubl21 process roles.

Suggested developer commands:

```bash
make minikube-build
make minikube-deploy
make regression-minikube
make regression-collect
make minikube-clean
```

or one convenience target:

```bash
make regression-minikube
```

Expected behavior:

1. verify/start the selected Minikube profile;
2. build the e-ubl21 image;
3. load the image into Minikube;
4. deploy NATS and e-ubl21 roles;
5. wait for readiness;
6. submit every selected regression fixture through the public API;
7. wait for durable final results;
8. compare actual stage results with the fixture manifest;
9. produce the normalized result set;
10. collect logs and selected metrics;
11. exit non-zero on any mismatch.

The runner must use the public API and durable result path for end-to-end tests, not call validators directly.

### Transitional behavior

Before `TODO-VALIDATION-SPLIT` is complete, the same regression corpus should run against the current monolithic worker.

After the split, the expected result set must remain unchanged even though the internal topology has changed.

That before/after equality is one of the principal acceptance gates for the refactor.

---

## 11. GitHub Actions regression execution

Yes, the regression tests can and should run in GitHub Actions.

Use two tiers.

### Tier 1 — mandatory correctness regression

Run on:

```text
pull_request
push to main
```

Includes:

- generator determinism check;
- manifest verification;
- ordinary Go tests;
- UBL XSD corpus;
- EN 16931 corpus;
- Peppol corpus once implemented;
- public API compliance regression;
- image/offline runtime verification.

Missing required Java, Saxon, xmllint, validation assets, or corpus files is a **failure**, never a skip.

### Tier 2 — Kubernetes/Minikube regression

Run on at least:

```text
pull_request
push to main
```

when the validation split becomes the production architecture.

A GitHub-hosted Linux runner can create a disposable Minikube cluster using its container/Docker runtime. The job should deploy the same manifests used locally and run `make regression-minikube` or an equivalent script.

If runtime becomes excessive, correctness coverage must not be weakened. Instead split jobs by corpus/profile and run them in parallel.

### Performance/stress suite

Do **not** make unstable wall-clock performance thresholds part of every correctness build.

Run larger capacity/stress suites:

- nightly;
- on demand;
- before releases;
- when validator/runtime versions change.

Store performance evidence separately from deterministic correctness results.

---

## 12. GitHub regression artifacts

Every CI regression run should upload artifacts even when tests fail.

Artifact contents:

```text
summary.json
results.jsonl
junit.xml
environment.json
manifest-used.json
logs/
metrics/
```

Recommended artifact name:

```text
regression-<commit-sha>-<run-attempt>
```

Recommended retention should be long enough to compare releases and standards upgrades. Release/tag builds should retain or publish regression evidence for substantially longer than ordinary branch builds.

The GitHub job summary should display at minimum:

```text
Corpus                     Passed   Failed
UBL 2.1                      ...       ...
EN 16931                     ...       ...
Peppol BIS Billing           ...       ...
--------------------------------------------
TOTAL                        ...       ...
```

and list every failed fixture ID with expected/actual stage outcome.

---

## 13. Result reproducibility metadata

`environment.json` must identify the execution environment without making it part of the semantic pass/fail comparison.

Record at least:

- Git commit SHA;
- dirty/clean state for local runs;
- generator version/commit;
- UBL schema version;
- EN 16931 artefact version and digest;
- Peppol release/version and digest when applicable;
- Go version;
- Java version;
- Saxon version;
- XML Resolver version;
- xmllint/libxml2 version;
- Docker image digest;
- Kubernetes version;
- Minikube version for local/CI Kubernetes runs;
- NATS server version;
- test start/end timestamps.

This answers the future question: "What exactly produced this result set?"

---

## 14. Regression invariants for the validation split

The generated corpus must explicitly protect the refactor defined in `TODO-VALIDATION-SPLIT.md`.

For every fixture, the final semantic result before and after the split must be identical.

Additional asynchronous invariants:

- XSD-invalid input never reaches EN/Peppol;
- EN-invalid input never reaches Peppol;
- successful stage results survive downstream worker downtime;
- scaling EN workers to zero accumulates EN requests without losing XSD results;
- restoring EN workers drains the queue and produces the expected final results;
- duplicate StageRequests do not create duplicate permanent stage/final results;
- duplicate StageResults do not advance the plan twice;
- orchestrator restart at transition boundaries does not change the final regression result;
- worker restart does not change expected standards diagnostics;
- stage order in final evidence matches the frozen ValidationPlan.

These scenarios should use generated fixtures rather than ad hoc XML embedded inside tests wherever possible.

---

## 15. Regression invariants for Peppol

The Peppol roadmap must consume the same manifest/corpus model.

Required invariants include:

- Peppol-positive samples pass UBL, EN and Peppol;
- every Peppol-negative sample passes UBL and EN before failing Peppol;
- missing/wrong Peppol identifiers cannot silently downgrade to EN-only when Peppol is explicitly requested;
- expected official Peppol rule IDs are preserved in structured results;
- a Peppol operational failure never becomes `invalid`;
- Peppol runtime/version upgrade cannot change expected fixture behavior without an explicit manifest/evidence review.

---

# TODO-REGRESSION-001 — Freeze regression corpus and result contracts

## Goal

Define stable fixture and result schemas before extending generation.

## Work

- Define fixture manifest JSON schema.
- Define normalized regression result JSON schema.
- Define stable fixture IDs.
- Define profile names and stage names.
- Define expected-rule-ID representation.
- Define how official imported fixtures record provenance.
- Define which volatile fields are excluded from semantic comparisons.
- Add schema validation tests for manifests/results.

## Acceptance criteria

A regression fixture and its expected result can be reviewed without reading Go test code.

---

# TODO-REGRESSION-002 — Make UBL generation deterministic and manifest-driven

## Goal

Produce the complete UBL 2.1 positive corpus reproducibly.

## Work

- Regenerate/discover all 65 main document types.
- Require stable deterministic values, order and namespaces.
- Produce SHA-256 per sample.
- Emit manifest entries automatically.
- Run independent xmllint validation.
- Fail if generated bytes differ between two consecutive clean generations.

## Acceptance criteria

A clean checkout produces the same UBL positive corpus and hashes twice in succession, with every sample XSD-valid.

---

# TODO-REGRESSION-003 — Formalize UBL targeted-negative generation

## Goal

Turn existing/ad hoc XSD-negative cases into declared regression fixtures.

## Work

- Implement mutation catalogue for common schema failure classes.
- Keep mutations deterministic.
- Ensure each sample records why it should fail.
- Verify later stages are not executed for these samples.

## Acceptance criteria

Every UBL-negative fixture fails XSD predictably and is represented in the manifest/result set.

---

# TODO-REGRESSION-004 — Integrate EN 16931 generator into the unified corpus CLI

## Goal

Expose existing semantic generation through the common regression interface.

## Work

- Add `--profile en16931` to the common CLI.
- Generate positive Invoice/CreditNote variants.
- Import/pin official EN fixtures where applicable.
- Emit common manifest entries instead of test-specific knowledge only.
- Preserve ruleset version/digest metadata.

## Acceptance criteria

EN fixtures can be generated and enumerated without importing `server` test packages.

---

# TODO-REGRESSION-005 — Formalize EN 16931 targeted-negative corpus

## Goal

Make expected EN rule behavior explicit regression data.

## Work

- Convert the existing rule-targeted negative corpus into manifest-defined fixtures.
- Preserve expected rule IDs.
- Verify every fixture remains XSD-valid.
- Add mutation provenance/description.
- Prevent a mutation from unexpectedly introducing an earlier XSD failure.

## Acceptance criteria

Every EN negative fixture passes UBL and fails EN with at least its expected official rule ID.

---

# TODO-REGRESSION-006 — Add Peppol semantic sample model and positive generator

## Goal

Extend semantic generation from EN 16931 to Peppol BIS Billing.

## Work

- Build Peppol variants on top of the EN Invoice/CreditNote business model.
- Add Peppol `CustomizationID`/`ProfileID` generation.
- Add required endpoint/party identifier scheme values.
- Add Peppol-specific code-list values.
- Pin values/provenance to the selected Peppol release.
- Generate Profile 01 and Profile 02 fixtures where supported.
- Import official Peppol examples with immutable provenance when permitted.

## Acceptance criteria

The generator can produce deterministic candidate Peppol Invoice/CreditNote fixtures before or alongside the validator implementation.

---

# TODO-REGRESSION-007 — Add Peppol targeted-negative mutation catalogue

## Goal

Generate Peppol-specific failures that still pass earlier stages.

## Work

- Define one-mutation-per-rule/family where practical.
- Require XSD pass.
- Require EN 16931 pass.
- Record expected Peppol rule IDs from the pinned release.
- Cover identifier/profile/code-list/country/common rule families.

## Acceptance criteria

Once the Peppol validator exists, every Peppol-negative fixture reaches the Peppol stage and fails with the expected rule evidence.

---

# TODO-REGRESSION-008 — Implement the normalized regression runner

## Goal

Run corpus fixtures through the public server and produce stable evidence.

## Work

- Read manifest.
- Submit each sample through authenticated public API.
- Select requested validation profile.
- Wait for durable completion.
- Retrieve structured final/stage result.
- Normalize volatile fields away.
- Compare expected vs actual.
- Emit `results.jsonl`, `summary.json` and `junit.xml`.
- Exit non-zero on any difference.

## Acceptance criteria

One command produces a deterministic semantic verdict for the entire corpus.

---

# TODO-REGRESSION-009 — Add regression evidence/log collector

## Goal

Keep enough diagnostics to explain every failed CI run.

## Work

- Capture API/orchestrator/worker logs.
- Capture Kubernetes events for Minikube runs.
- Capture selected NATS consumer/stream state.
- Capture bounded metrics snapshot.
- Write environment metadata.
- Never include raw invoice bodies in service logs.
- Package diagnostics separately from normalized correctness results.

## Acceptance criteria

A failed fixture can be investigated from the uploaded regression artifact without reproducing the exact CI worker immediately.

---

# TODO-REGRESSION-010 — Create local Minikube regression deployment

## Goal

Make the workstation Minikube cluster the canonical local distributed-system test environment.

## Work

- Add Kubernetes manifests/Helm/Kustomize layout for test deployment.
- Deploy namespace-isolated NATS.
- Deploy API and current worker initially.
- Migrate to API/orchestrator/XSD/EN/Peppol deployments as roadmaps land.
- Add image-build/load automation for Minikube.
- Add readiness checks.
- Add one-command regression execution and cleanup.
- Collect results/logs to a local ignored directory.

## Acceptance criteria

`make regression-minikube` from a clean working tree runs the selected corpus through the real Kubernetes topology and returns non-zero on regression.

---

# TODO-REGRESSION-011 — Add GitHub Actions correctness regression gate

## Goal

Run deterministic regression automatically for every proposed/merged change.

## Work

- Add workflow for pull requests and pushes to `main`.
- Generate/verify corpus.
- Verify deterministic hashes.
- Run required validator runtimes with skipping disabled.
- Execute regression runner.
- Upload result artifacts on success and failure.
- Publish JUnit/test summary.
- Block merge/build on any fixture mismatch.

## Acceptance criteria

A commit that changes a standards outcome unexpectedly receives a failing GitHub check with the affected fixture IDs visible.

---

# TODO-REGRESSION-012 — Add GitHub Minikube/Kubernetes regression gate

## Goal

Test the deployed asynchronous topology, not merely in-process Go code.

## Work

- Start disposable Minikube cluster on GitHub Linux runner.
- Build/load the candidate image.
- Deploy NATS + e-ubl21 components.
- Execute the same regression runner used locally.
- Upload all evidence.
- Tear cluster down after artifact collection.
- Parallelize corpora if required to keep runtime acceptable.

## Acceptance criteria

The same Minikube-oriented command succeeds locally and in GitHub Actions without maintaining separate semantic test logic.

---

# TODO-REGRESSION-013 — Establish pre/post validation-split equivalence gate

## Goal

Prove the distributed refactor does not change validation semantics.

## Work

- Capture a successful normalized baseline on the monolithic worker.
- Run the same manifest against split-stage architecture.
- Require identical normalized fixture outcomes/rule IDs.
- Separately test asynchronous queue/restart/scale-zero scenarios.

## Acceptance criteria

`TODO-VALIDATION-SPLIT` cannot be completed while any corpus fixture changes semantic result unexpectedly.

---

# TODO-REGRESSION-014 — Establish Peppol release baseline

## Goal

Freeze the expected behavior of the first supported Peppol release.

## Work

- Complete positive/negative Peppol corpus.
- Cross-check with official/reference validation artefacts.
- Record release/digests in manifest.
- Produce first accepted normalized Peppol result set.

## Acceptance criteria

`TODO-PEPPOL` cannot be completed until the full Peppol corpus passes its committed expectations.

---

# TODO-REGRESSION-015 — Add release/nightly stress and performance evidence

## Goal

Keep performance evolution visible without making deterministic correctness tests flaky.

## Work

- Run large/line-heavy/near-limit fixtures.
- Record queue wait, stage execution and end-to-end latency.
- Record resource usage by stage.
- Run nightly/on-demand/release rather than using tight performance assertions on every commit.
- Compare against prior capacity baselines and flag material unexplained regressions.

## Acceptance criteria

Correctness and performance evidence are both retained, but only deterministic semantic differences automatically fail the correctness gate unless an explicit resource ceiling is breached.

---

# TODO-REGRESSION-016 — Document standards upgrade workflow around corpus changes

## Goal

Prevent standards upgrades from silently rewriting expected behavior.

## Work

For every UBL/EN/Peppol upgrade:

1. pin new assets;
2. regenerate candidate corpus where needed;
3. run old and new rule sets where possible;
4. produce a normalized result diff;
5. classify every changed fixture;
6. review expected rule-ID/severity changes;
7. deliberately update manifest expectations;
8. retain before/after evidence with release notes.

## Acceptance criteria

No standards-version checksum can be changed merely to make tests pass; behavioral differences require explicit reviewed regression evidence.

---

## 17. Definition of completion

This roadmap is complete when:

1. one generator CLI produces deterministic UBL, EN 16931 and Peppol corpus families;
2. every fixture has a versioned manifest entry and hash;
3. positive and targeted-negative expectations are explicit data;
4. the regression runner exercises the public API and durable result path;
5. every run emits normalized JSON/JSONL plus JUnit and diagnostic logs;
6. local Minikube can execute the distributed regression suite;
7. GitHub Actions runs the mandatory correctness gate on PRs and `main`;
8. GitHub can run the Minikube topology gate;
9. the validation split is proven semantically equivalent to the monolithic baseline;
10. Peppol implementation is protected by its own generated/imported positive and negative corpus;
11. standards upgrades require reviewed result-set differences rather than automatic baseline replacement.

The regression corpus is therefore a first-class project artefact, not merely a collection of test XML files.