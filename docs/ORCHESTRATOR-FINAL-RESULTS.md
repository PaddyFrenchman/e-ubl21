# Orchestrator-owned final validation results

**Status:** Implemented persistence contract for `TODO-VALIDATION-SPLIT-009`

The validation orchestrator is the sole publisher of execution-scoped
canonical `ValidationResult` records in the split architecture. Stage workers
publish only `StageResult` evidence and cannot finalize document validity.

## Durable finalization order

For every terminal execution, the orchestrator performs these durable steps:

1. create or confirm the complete immutable `StageResult` for the accepted
   attempt;
2. CAS the execution to `valid`, `invalid`, or `processing-failed` with a
   deterministic `finalResultId` and `finalResultStatus=pending`;
3. reload every terminal stage's permanent evidence and verify it against the
   frozen plan and accepted attempt projection;
4. build and validate the ordered canonical `ValidationResult`;
5. publish it to its execution-scoped permanent result subject;
6. read the permanent slot back and require semantic equality;
7. CAS the outbox to `finalResultStatus=published` with a publication time.

The result identity is deterministic from `validationId` plus `generation`.
JetStream retains at most the first record on that result subject, so replay
generations for one upload receive independent permanent slots while a delayed
duplicate publication cannot replace an earlier result.

## Outcome safety

A `valid` result contains every stage in the frozen plan, in order, and every
stage has persisted `passed` evidence. A standards failure contains the passed
prefix plus the terminal `standards-invalid` stage. Exhausted operational work
is published as `processing_failed` with an explicit
`operational-failure` outcome and typed reason; it is never represented as a
standards-invalid result.

Intermediate evidence cannot be published because non-terminal execution
states have no final-result outbox and `EnsureFinalResult` rejects them. The
canonical payload carries the frozen plan entries, allowing its own validation
to reject a partial `valid` result even outside the orchestrator.

## Peppol layer identity

The canonical layered result model reserves `peppol-bis-billing` independently
from UBL XSD and EN 16931. Its `version` identifies the executable machine
validation artefact, while `bisVersion`, `bisReleaseDate`, and
`bisMandatoryFrom` identify the Peppol publication. `customizationId` and
`profileId` retain the normalized document identifiers actually inspected, and
`processProfile` records stable process code `01` or `02` when resolution
succeeds.

A passing Peppol stage requires all resolved identifiers. A standards-invalid
stage may omit identifiers that were missing from the document but still
retains its BIS identity and structured Peppol issues. Peppol identity fields
are rejected on unrelated layers. Fatal/error assertions remain standards
failures; warning/warn assertions and successful reports remain visible
advisory evidence without changing a passing verdict.

The public upload API accepts `validationProfile=auto`, `ubl21`,
`ubl21-en16931`, `peppol-bis-billing`, or explicit `peppol-envelope`. Status projections expose both the
requested value and the concrete resolved profile; the legacy `profile` field
remains a compatibility alias for the resolved profile.

Durable executions and canonical results bind each profile to its stage
sequence: `ubl21` requires XSD, `ubl21-en16931` requires XSD then EN 16931, and
`peppol-bis-billing` requires XSD then EN 16931 then Peppol, while
`peppol-envelope` adds the independent envelope stage fourth. Profile and plan
cannot be validated independently, so a record cannot claim Peppol conformity
while omitting the Peppol stage.

## Crash recovery and concurrency

`ReconcilePendingFinalResults` scans terminal executions whose outbox is absent
or pending. An absent marker is supported for terminal records written before
this contract: reconciliation first claims the deterministic pending intent
under CAS. A publication failure leaves the intent pending. If publication
succeeded but its acknowledgement or final CAS was lost, a retry reads and
compares the permanent first-result slot before marking the outbox published.

Concurrent replicas may publish or reconcile the same execution. Permanent
per-subject insertion, deterministic identity, read-back comparison, and CAS
make identical work converge and conflicting content fail closed.

Finalization never reads, rewrites, or deletes the Object Store XML. Existing
raw-content retention and tombstone policy therefore remains unchanged.

## Migration boundary

The existing combined worker continues its legacy upload-scoped result path
until the stop-and-drain migration in `TODO-VALIDATION-SPLIT-014` (#61). That
legacy compatibility path is not an execution-scoped result and must not run
against the same active validation generation as the split roles. Issue #61
owns the deployment barrier and API projection cutover; this issue supplies the
complete orchestrator-owned persistence contract needed by that migration.
