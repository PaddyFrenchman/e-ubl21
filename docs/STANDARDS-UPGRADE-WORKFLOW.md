# Standards upgrade workflow

This is the canonical change-control procedure for upgrading the OASIS UBL
schemas, EN 16931 validation artefacts, or Peppol BIS Billing rules used by
e-ubl21. Component-specific instructions remain authoritative for acquiring,
building, and packaging their assets:

- [EN 16931 runtime upgrades](EN16931-ASSET-UPGRADE.md)
- [Peppol runtime upgrades](PEPPOL-ASSET-UPGRADE.md)
- [Standards provenance](../standard/README.md)

The purpose of this workflow is to distinguish an intended upstream standards
change from a corpus mistake or an application regression. A green test run is
not, by itself, permission to change expected results.

## Non-negotiable invariants

1. Identify both an **accepted commit** containing the currently deployed
   standard and a separate **candidate commit** containing the proposed
   upgrade. Both commits must be immutable and the candidate working tree must
   be clean when final evidence is collected.
2. Pin every new source by release tag or full commit and SHA-256 before using
   it. Record the authoritative URL, release/publication dates, licence, file
   inventory, and acquisition method. Never select `latest`, `main`, `master`,
   `upcoming`, redirects, or another moving reference.
3. Add versioned assets alongside the accepted assets. Do not overwrite or
   remove the accepted version until comparison, rollout, and rollback review
   are complete.
4. Keep acquisition, rule/runtime changes, generator changes, application
   changes, and expectation changes attributable. If compatibility requires
   multiple kinds of change, use separate commits and record why they cannot
   be reviewed independently.
5. Generate and execute accepted and candidate evidence independently. Do not
   update canonical manifests or baselines before the candidate difference has
   been produced and classified.
6. Account for every added, removed, or changed fixture. A checksum change is
   identity evidence only; it never proves that new behavior is correct.
7. Promote only reviewed expectations. Never copy a candidate result set over
   a baseline merely because the candidate is internally consistent or the
   upgraded validator produced it.
8. Preserve the accepted runtime and its evidence as the rollback target until
   the new release is deployed and its migration obligations are complete.

## Required evidence layout

Keep all three execution cells separate. This working directory may be ignored
locally while evidence is being produced, but it is not the durable review
record:

```text
regression-results/upgrades/<standard>/<accepted>-to-<candidate>/
├── accepted-runtime/
│   └── accepted-corpus/
│       ├── local-gate/
│       │   ├── correctness/{ubl21,en16931}/
│       │   └── minikube/{ubl21,en16931}/
│       └── peppol/
├── candidate-runtime/
│   ├── accepted-corpus/
│   │   ├── ubl21/
│   │   ├── en16931/
│   │   └── peppol/
│   └── candidate-corpus/
│       ├── ubl21/
│       ├── en16931/
│       └── peppol/
├── staged-corpus/
│   ├── ubl21/
│   ├── en16931/
│   └── peppol/
├── comparisons/
│   ├── runtime-on-accepted-corpus/<profile>/{equivalence.json,results.diff}
│   ├── corpus-under-candidate-runtime/<profile>/{equivalence.json,results.diff}
│   └── complete-release-change/<profile>/{equivalence.json,results.diff}
├── source-and-rule.diff
├── fixture-change-ledger.md
├── approval.md
└── evidence.sha256
```

The cells mean:

| Cell | Question answered |
| --- | --- |
| `accepted-runtime/accepted-corpus` | Does the accepted release still reproduce its accepted behavior? |
| `candidate-runtime/accepted-corpus` | What did the validator/rules change for unchanged inputs and expectations? |
| `candidate-runtime/candidate-corpus` | What is the complete proposed release behavior after corpus regeneration? |

Each cell must identify its runtime commit, corpus commit, dirty state,
standards versions/digests, generator commits, runtime/tool versions, and image
digest where applicable. Retain `results.jsonl`, `summary.json`,
`equivalence.json`, `manifest-used.json`, environment metadata, and the
component-specific reference/runtime evidence produced by the applicable
gates. Never reuse an output directory between cells. Do not include secrets
or invoice bodies in the retained review record.

Before approval, archive this complete tree in durable storage attached to the
pull request/release record or record an immutable external artifact URI. The
review record must contain the archive SHA-256 and `evidence.sha256`, which
lists the digests of the files used for the decision. Evidence that exists only
in an ignored directory or on one workstation does not satisfy retention.

## Procedure

### 1. Open and scope the upgrade

- Use one issue and issue branch for one standards release transition.
- Name the accepted and candidate standards versions and the immutable
  upstream sources in the issue.
- Record whether the change affects syntax, business rules, severities, code
  lists, examples, compiler/runtime compatibility, API metadata, frozen
  validation plans, or deployment migration.
- Avoid unrelated application changes. Tool-only upgrades must keep standards
  inputs and expected semantics fixed; rules-only upgrades must keep the
  compiler/runtime fixed unless an incompatibility is documented.

### 2. Freeze the accepted evidence

Create a clean checkout or worktree at the accepted commit. Run the applicable
existing gates before changing assets, corpora, manifests, or baselines. At
minimum, run the authoritative local regression gate:

```sh
REGRESSION_RUN_DIR="$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate" \
  ./run-regression-tests.sh
```

The local runner has no top-level `results.jsonl`. Its normalized outputs are:

```text
$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate/correctness/ubl21/results.jsonl
$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate/correctness/en16931/results.jsonl
$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate/minikube/ubl21/results.jsonl
$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate/minikube/en16931/results.jsonl
```

Use the `correctness/<corpus>/results.jsonl` records as the accepted UBL/EN
comparison inputs. The corresponding Minikube records are independent deployed
topology confirmation and must remain retained with the cell.

The authoritative local gate does not execute Peppol. For a Peppol-affecting
upgrade, run this additional accepted-runtime command from the accepted
checkout against the accepted deployment:

```sh
go run . regression \
  --base-url "$ACCEPTED_URL" \
  --token "$ACCEPTED_TOKEN" \
  --manifest createsamples/samples-peppol/manifest.json \
  --profile all \
  --output "$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/peppol"
```

Also run the profile-specific gates from the matrix below. If the accepted
release can no longer be executed, record the exact reason and use its already
reviewed baseline plus retained release evidence; absence of old-runtime
execution is a review risk, not permission to omit comparison.

### 3. Pin and inspect candidate assets

- Acquire from the authoritative immutable source and independently calculate
  all digests.
- Add a new versioned asset location and update provenance, checksum manifests,
  runtime selectors, packaged paths, notices, and public version metadata
  together.
- For UBL, inventory every main and common XSD and review schema additions,
  removals, namespace changes, imports, datatypes, cardinalities, and code
  lists. A future UBL version must not silently replace the unversioned `XSD/`
  tree without recording a complete immutable distribution identity.
- For EN 16931, follow `EN16931-ASSET-UPGRADE.md` and review official rule IDs,
  severities/flags, diagnostics, and SVRL compatibility.
- For Peppol, follow `PEPPOL-ASSET-UPGRADE.md`; review the complete source
  inventory and code lists, reproduce the executable XSLT twice, and retain
  independent official/reference comparison evidence.

The candidate must fail closed if any recorded digest, inventory, compiler
identity, or runtime identity does not match.

### 4. Regenerate a candidate corpus without blessing it

Commit the asset/generator inputs first so generated provenance can name an
immutable candidate commit. Generate into a staging location, not over the
tracked corpus:

```sh
go run ./createsamples generate --profile ubl21 --set all \
  --xsd XSD --output "$EVIDENCE_ROOT/staged-corpus/ubl21"
go run ./createsamples generate --profile en16931 --set all \
  --xsd XSD --output "$EVIDENCE_ROOT/staged-corpus/en16931"
go run ./createsamples generate --profile peppol-bis-billing --set all \
  --xsd XSD --output "$EVIDENCE_ROOT/staged-corpus/peppol"
```

Run only the profiles affected by the upgrade, plus every downstream profile
that consumes the changed standard. For example, a UBL change requires UBL,
EN, and Peppol review; an EN change requires EN and Peppol review. Verify
deterministic regeneration and independently validate the candidate fixtures.

At this point the tracked manifest and accepted baseline remain unchanged. A
candidate that differs is expected to fail the existing equivalence gate; that
failure is the input to review, not a reason to weaken the gate.

### 5. Execute all three cells and produce separate diffs

Run the accepted runtime against the accepted corpus, the candidate runtime
against the unchanged accepted corpus, and the candidate runtime against the
candidate corpus where regeneration is required. Running the candidate against
the old corpus separates validator behavior changes from fixture-generation
changes.

For UBL and EN, the accepted cell is populated by the local gate above. Populate
the other two cells with the regression command directly so each command writes
one `results.jsonl` at a known profile-specific location. This EN example is
copy-pasteable after the named URL, token, and worktree variables are set:

```sh
CANDIDATE_RUNTIME_ACCEPTED="$EVIDENCE_ROOT/candidate-runtime/accepted-corpus/en16931"
CANDIDATE_RUNTIME_CANDIDATE="$EVIDENCE_ROOT/candidate-runtime/candidate-corpus/en16931"

go run . regression \
  --base-url "$CANDIDATE_URL" \
  --token "$CANDIDATE_TOKEN" \
  --manifest "$ACCEPTED_WORKTREE/createsamples/samples-en16931/manifest.json" \
  --profile all \
  --output "$CANDIDATE_RUNTIME_ACCEPTED"

go run . regression \
  --base-url "$CANDIDATE_URL" \
  --token "$CANDIDATE_TOKEN" \
  --manifest "$EVIDENCE_ROOT/staged-corpus/en16931/manifest.json" \
  --profile all \
  --output "$CANDIDATE_RUNTIME_CANDIDATE"
```

For UBL, substitute `samples/manifest.json` and the `ubl21` output directory.
For Peppol, substitute `samples-peppol/manifest.json`, the staged Peppol
manifest, and `peppol`; the candidate URL must expose the complete split-stage
Peppol topology. Run `make test-peppol-compliance` and the reference gates in
addition to, not instead of, these retained normalized runs. Tokens remain
secret inputs and must not be copied into evidence.

For each affected corpus, produce three comparisons. These concrete EN paths
match the local runner and direct commands above:

```sh
ACCEPTED_RESULTS="$EVIDENCE_ROOT/accepted-runtime/accepted-corpus/local-gate/correctness/en16931/results.jsonl"
CANDIDATE_OLD_RESULTS="$EVIDENCE_ROOT/candidate-runtime/accepted-corpus/en16931/results.jsonl"
CANDIDATE_NEW_RESULTS="$EVIDENCE_ROOT/candidate-runtime/candidate-corpus/en16931/results.jsonl"

make regression-equivalence \
  BASELINE="$ACCEPTED_RESULTS" \
  CANDIDATE="$CANDIDATE_OLD_RESULTS" \
  OUTPUT="$EVIDENCE_ROOT/comparisons/runtime-on-accepted-corpus/en16931/equivalence.json"

make regression-equivalence \
  BASELINE="$CANDIDATE_OLD_RESULTS" \
  CANDIDATE="$CANDIDATE_NEW_RESULTS" \
  OUTPUT="$EVIDENCE_ROOT/comparisons/corpus-under-candidate-runtime/en16931/equivalence.json"

make regression-equivalence \
  BASELINE="$ACCEPTED_RESULTS" \
  CANDIDATE="$CANDIDATE_NEW_RESULTS" \
  OUTPUT="$EVIDENCE_ROOT/comparisons/complete-release-change/en16931/equivalence.json"
```

The first comparison isolates runtime/rules behavior on unchanged inputs. The
second isolates corpus regeneration under one runtime. The third records the
complete release delta. Repeat with the exact `ubl21` or `peppol` locations as
applicable. A non-equivalent result is expected during review, but a command or
evidence-write failure is not; retain each `equivalence.json` even when the
command exits non-zero.

Also retain a `diff -u` of each results pair under the same comparison
directory, the upstream source/rule diff, and, when generated bytes changed,
the manifest and XML diff. An exit status of `1` from `diff` means differences
were found and must be classified; status greater than `1` is an
evidence-generation failure. Raw line order must not substitute for the
normalized semantic comparison.

### 6. Classify every changed fixture

`fixture-change-ledger.md` must contain one row for every fixture named as
changed, missing, or unexpected by the equivalence result:

| Field | Required content |
| --- | --- |
| Fixture | Stable fixture ID and positive/targeted-negative/control class |
| Byte identity | Old/new fixture SHA-256 and whether generation changed |
| Old behavior | Final outcome, ordered stages, exact rule IDs, and relevant severity |
| New behavior | Final outcome, ordered stages, exact rule IDs, and relevant severity |
| Classification | One category from the list below |
| Authority | Upstream rule/schema/code-list reference or defect evidence |
| Disposition | Accept expectation change, fix corpus, fix implementation, or defer/block |
| Files | Exact manifest, fixture, baseline, reference, contract, and release-note changes |
| Reviewer | Human reviewer and approval reference |

Use exactly one primary classification:

- **upstream-required semantic change** — the pinned authoritative release
  intentionally changes the result, rule ID, or severity;
- **corpus correction** — the former fixture or expectation did not express
  its claimed business condition;
- **implementation regression** — e-ubl21 differs from the authoritative
  release or reference behavior and must be fixed before promotion;
- **coverage addition/removal** — a fixture was deliberately introduced or
  retired, with provenance and coverage impact reviewed;
- **non-semantic representation change** — normalized behavior is identical;
  only bytes or diagnostics changed, with the reason recorded;
- **unexplained** — blocks the upgrade.

Review rule severity separately when it is not represented in normalized
results. Use the authoritative rule diff, SVRL/reference output, and product
contract to prove whether a severity change alters final validity.

### 7. Deliberately update tracked expectations

Only after the ledger is complete and reviewed may the issue branch update
tracked corpora, manifests, reference evidence, or baselines.

- Change manifest expectations only for ledger entries whose disposition
  explicitly accepts the new behavior.
- Preserve stable fixture IDs when the business case is unchanged. New IDs are
  required when the semantic purpose changes.
- Update fixture hashes, ruleset versions/digests, provenance, expected stage
  outcomes, and expected rule IDs together.
- Promote only the reviewed normalized records. Update baseline metadata,
  manifest/result/reference digests, fixture counts, release identity, and the
  baseline README in the same commit.
- Update API contracts, operational metadata, migration policy, notices, and
  release notes wherever the exposed standards identity or behavior changes.

Re-run the equivalence gate against the deliberately updated baseline. It must
now pass for the reviewed candidate, while tests must still reject a digest or
result change that is not reflected consistently across the asset, manifest,
reference, and baseline metadata.

### 8. Run final verification and retain approval

The candidate commit must pass:

```sh
go test ./... -count=1
./run-regression-tests.sh
```

Then run every applicable profile gate:

| Changed input | Additional mandatory verification |
| --- | --- |
| UBL schemas | deterministic UBL/EN/Peppol regeneration; all downstream profile tests and image gates |
| EN rules | `make test-en16931-compliance`, `make test-en16931-image`, and all Peppol correctness/reference/image gates |
| Peppol rules/code lists | `make test-peppol-schematron`, `make test-peppol-positive`, `make test-peppol-negative`, `make test-peppol-reference`, `make test-peppol-compliance`, and `make test-peppol-image` |
| Saxon/Java/compiler/container | affected old/new ruleset matrix, offline image gates, cancellation/resource-bound tests, and applicable capacity comparison |

The pull request or change record must link the immutable upstream release,
accepted and candidate commits, evidence root, source/rule diff, completed
fixture ledger, compatibility/migration notes, and explicit reviewer approval.
`approval.md` records who reviewed the behavior changes and which ledger
entries were accepted. An empty ledger is valid only when the equivalence
result proves there were no semantic fixture changes. Approval is blocked until
the complete evidence archive has a durable URI and verified archive/file
digests in the review record.

## Blocking conditions

Block promotion when any of the following is true:

- provenance, licence, inventory, or checksum evidence is missing;
- accepted and candidate evidence is mixed or either commit/dirty state is
  ambiguous;
- a candidate baseline was generated from the implementation and accepted
  without an independent authority or reviewed rationale;
- any changed, missing, or unexpected fixture lacks a ledger entry;
- a positive becomes invalid, a targeted negative loses its intended rule, or
  a severity changes without an approved upstream explanation;
- candidate behavior diverges from applicable official/reference evidence;
- a downstream profile affected by UBL or EN was not exercised;
- deterministic generation, offline operation, timeout/cancellation, durable
  migration, or rollback guarantees regress;
- the final local regression gate or an applicable component gate fails.

## Rollout and rollback

Standards identity is part of each frozen validation plan. Drain old plans with
a compatible worker or migrate them using the documented generation barriers;
never reinterpret old evidence under a new ruleset. Deploy by immutable image
digest to a non-production namespace first, retain its runtime-verification
event, and preserve the accepted image/assets until rollback is no longer
required. Peppol-specific transition rules are in
`PEPPOL-DURABILITY-MIGRATION.md`.
