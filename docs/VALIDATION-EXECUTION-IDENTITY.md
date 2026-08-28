# Durable validation execution identity

**Status:** Normative contract for `TODO-VALIDATION-SPLIT-002`

An upload and a validation execution are different identities:

| Identity | Meaning | Lifetime |
| --- | --- | --- |
| `uploadId` | One immutable raw XML object and its acceptance metadata. | Stable until the content-retention policy removes the body; metadata remains independently addressable. |
| `validationId` | One logical validation lineage for that upload and frozen plan. | Stable across explicit replay generations of that same plan. |
| `generation` | One durable run of a validation lineage. | Starts at 1 and increases only through explicit replay. |
| stage attempt | One try of one stage inside a generation. | Retries do not change the validation ID or generation. |

The durable address is the complete tuple:

```text
(uploadId, validationId, generation)
```

No API, worker, or operator may identify an execution as merely the "latest"
record for an upload. A polling response or link must retain the explicit
`validationId` and `generation`, so repeated polling addresses the same record
even if another validation is later created for the upload.

## Frozen record

Every generation stores:

- durable schema version;
- upload and validation IDs;
- accepted UBL document type;
- positive generation number;
- creation trigger and timestamp;
- requested public validation profile;
- plan ID and plan-definition version;
- ordered, one-based stage ordinals;
- required/optional status for every stage;
- exact ruleset version selected for every stage.

The record contains no invoice bytes and no mutable upload-acceptance fields.
It is stored in the namespace-derived `*_VALIDATION_EXECUTIONS` JetStream KV
bucket under:

```text
<uploadId>.<validationId>.<generation>
```

The upload prefix permits bounded, namespace-local lookup of every execution
associated with an upload. Creation uses the KV create operation, never put or
update, so an existing generation cannot be replaced. Retrieval validates both
the record and its agreement with the physical key.

## Replay versus revalidation

The identity policy is deliberately explicit:

```text
initial validation
  new validationId, generation 1, trigger initial

operator replay of the identical frozen plan
  same validationId, next contiguous generation, trigger replay

revalidation with a changed profile, plan, or ruleset
  new validationId, generation 1, trigger revalidation
```

A replay generation is accepted only when its predecessor exists and its
upload ID, validation ID, document type, requested profile, and entire frozen
plan are equal.
Its creation timestamp must advance. Competing writers for the same next
generation converge through create-only KV semantics; gaps are rejected.

Changing a ruleset version under the same validation ID is forbidden. A new
validation ID is what preserves the old plan and evidence while allowing the
same immutable upload to be evaluated against a newer standards release.

## Ownership and rollout

The orchestrator is the logical creator of these records under the ownership
contract in [`VALIDATION-SPLIT-OWNERSHIP.md`](VALIDATION-SPLIT-OWNERSHIP.md).
This issue adds the durable model, validation, namespace isolation, and
create/read/list primitives. The later orchestrator issue activates record
creation in the runtime; the StageRequest/StageResult issue carries the same
identity tuple over the wire. Until that cutover, the existing monolithic
upload/worker path and its public response remain unchanged.
