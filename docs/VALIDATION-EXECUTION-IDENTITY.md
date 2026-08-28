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
- upload and validation IDs plus the durable creation-command ID;
- accepted UBL document type;
- positive generation number;
- creation trigger and timestamp;
- requested public validation profile;
- plan ID and plan-definition version;
- ordered, one-based stage ordinals;
- required/optional status for every stage;
- exact ruleset version selected for every stage.

Plan IDs and stage IDs use the shared stage-contract token grammar and are
bounded to 256 ASCII bytes. This prevents a frozen execution from creating a
valid record whose derived `StageRequest` cannot fit its durable queue.

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

## Idempotent lineage creation

Generation 1 is created only from a durable `ValidationExecutionIntent`. Its
`creationIntentId`, timestamp, upload, document type, requested profile, frozen
plan, trigger, and optional predecessor are immutable command data. Every
orchestrator replica handling a redelivery must use that same data.

The 128-bit validation ID is derived with a domain-separated SHA-256 digest:

```text
initial
  hash(trigger, uploadId, "initial")

revalidation
  hash(trigger, uploadId, creationIntentId)
```

Consequently, every initial command for one upload targets the same physical
generation-1 key. Redelivery of the identical command returns the already
created record and revision as success. A conflicting second initial command
cannot create another lineage because its differing command data collides with
that key and is rejected.

A revalidation command has its own deterministic lineage ID. Redelivery of
that command converges on the same key, while another explicit revalidation
command can create another lineage. Hash-derived IDs are identities, not
authentication secrets.

## Replay versus revalidation

The identity policy is deliberately explicit:

```text
initial validation
  new validationId, generation 1, trigger initial

operator replay of the identical frozen plan
  same validationId, next contiguous generation, trigger replay

revalidation with a changed profile, plan, or ruleset
  new command-derived validationId, generation 1, trigger revalidation
```

A replay generation is accepted only when its predecessor exists and its
upload ID, validation ID, document type, requested profile, and entire frozen
plan are equal. Every replay command ID may occur only once in that lineage.
Competing writers of the same command and generation converge through
create-only KV semantics; gaps and command reuse are rejected.

Generation is the sole ordering authority. `createdAt` is required audit data,
but clock skew, NTP adjustment, or another replica's clock may make it earlier
than the preceding generation and cannot invalidate an otherwise valid replay.

A revalidation stores an explicit `(validationId, generation)` predecessor.
That predecessor must exist for the same upload and document type, and either
the requested profile or complete frozen plan must differ. An identical plan
is replay, not revalidation.

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

The execution bucket is provisioned only through the deployer-owned
`ProvisionValidationExecutionInfra` operation. Runtime access performs the
read-only `VerifyValidationExecutionInfra` compatibility check; the legacy
runtime `EnsureInfra` path deliberately does not create, update, or open the
new bucket.

The 256-byte plan and stage identifier bounds were added to schema V1 before
execution records or stage messages were activated in a deployed runtime.
`ValidationExecutionSchemaVersion` therefore remains `1`: there is no durable
V1 population or active producer/consumer contract to migrate. After runtime
activation, an incompatible semantic tightening requires an explicit schema
version and migration or compatibility policy.
