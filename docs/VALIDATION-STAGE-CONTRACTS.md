# Versioned validation stage contracts

**Status:** Normative contract for `TODO-VALIDATION-SPLIT-003`
**Wire version:** 1

`queue.StageRequest` and `queue.StageResult` are the only shared queue payloads
between the validation orchestrator and stage workers. They live in the
`queue` package, not in API, orchestrator, XSD, EN 16931, or future Peppol
worker implementation packages. A stage process therefore needs no knowledge
of another process's internal types.

This contract implements the ownership rules in
[`VALIDATION-SPLIT-OWNERSHIP.md`](VALIDATION-SPLIT-OWNERSHIP.md) and carries the
complete execution identity frozen by
[`VALIDATION-EXECUTION-IDENTITY.md`](VALIDATION-EXECUTION-IDENTITY.md).

## Version and decoding policy

Every message has `contractVersion: 1`. Both request and result implement
validating JSON marshal/unmarshal methods:

- missing, zero, and unknown versions are rejected with
  `ErrUnsupportedStageContractVersion`;
- unknown JSON fields and trailing JSON values are rejected;
- invalid in-memory values cannot be marshalled;
- no consumer guesses a schema or silently upgrades a durable message.

Changing a field's meaning, removing a field, or adding a field requires a new
contract version and an explicit compatibility rollout. Version 1 messages
remain stable fixtures for as long as version 1 can exist in a retained stream.

## StageRequest version 1

`StageRequest` contains references and frozen orchestration identity only. It
never contains raw XML or worker configuration.

```json
{
  "contractVersion": 1,
  "requestId": "<64 lowercase hex characters>",
  "validationId": "<32 lowercase hex characters>",
  "uploadId": "<32 lowercase hex characters>",
  "documentType": "Invoice",
  "plan": "ubl21-en16931",
  "planVersion": "1",
  "generation": 1,
  "stage": "en16931",
  "stageOrdinal": 2,
  "rulesetVersion": "1.3.16",
  "attempt": 1,
  "createdAt": "2026-08-28T18:00:00Z"
}
```

`NewStageRequest` selects the named one-based ordinal from an already valid
`ValidationExecution`. Consequently the stage ID and ruleset version are
copied from the immutable plan rather than being chosen independently by a
caller. `attempt` identifies one orchestrator-authorized logical attempt.
Broker redelivery does not increment it.

## StageResult version 1

`StageResult` repeats the complete request identity so the orchestrator can
validate it against durable execution state without trusting message headers.
It also correlates the evidence to the deterministic `requestId`.

```json
{
  "contractVersion": 1,
  "resultId": "<64 lowercase hex characters>",
  "requestId": "<64 lowercase hex characters>",
  "validationId": "<32 lowercase hex characters>",
  "uploadId": "<32 lowercase hex characters>",
  "documentType": "Invoice",
  "plan": "ubl21-en16931",
  "planVersion": "1",
  "generation": 1,
  "stage": "en16931",
  "stageOrdinal": 2,
  "rulesetVersion": "1.3.16",
  "attempt": 1,
  "outcome": "standards-invalid",
  "issues": [],
  "startedAt": "2026-08-28T18:00:01Z",
  "completedAt": "2026-08-28T18:00:02Z",
  "durationMs": 1000
}
```

The outcome vocabulary is closed:

| Outcome | Meaning | Reason field |
| --- | --- | --- |
| `passed` | The requested standards stage completed successfully. | Forbidden. |
| `standards-invalid` | The validator completed and found non-conformity. | Forbidden; bounded standards evidence is required. |
| `operational-failure` | This attempt could not produce a standards decision. | Required and restricted to the enum below. |

An operational failure is immutable attempt evidence, not a terminal stage or
document decision. Only the orchestrator applies retry policy and decides
whether to authorize another attempt or record an exhausted terminal outcome.

The version 1 operational reasons are:

- `timeout`;
- `cancelled`;
- `content-unavailable`;
- `runtime-unavailable`;
- `runtime-failure`;
- `malformed-output`;
- `resource-limit`;
- `internal-error`.

Diagnostic detail is kept out of this low-cardinality enum. Operational
results cannot contain standards issues. Passed and standards-invalid results
cannot carry an operational reason.

## Deterministic message identity

The request identity covers:

```text
contract version
validation ID + upload ID + document type
plan ID + plan version + generation
stage ID + ordinal + ruleset version + attempt
```

Each UTF-8 component is encoded as an unsigned 64-bit big-endian byte length
followed by its bytes. SHA-256 is applied with the domain
`e-ubl21/stage-request`; the complete lowercase hexadecimal digest is
`requestId`. This length-prefixing makes the construction unambiguous without
depending on delimiter restrictions.

`resultId` is SHA-256 over the same encoding with domain
`e-ubl21/stage-result` and `requestId` as its component. Outcome, diagnostics,
and timestamps deliberately do not change either ID: recomputing the same
logical attempt after a crash must contend for the same immutable evidence
slot rather than create another attempt.

Publishers must use `requestId` or `resultId`, respectively, as the
`Nats-Msg-Id` header. The payload validator recomputes both IDs and rejects any
tampering or identity drift. Stream deduplication is an optimization; the
orchestrator's durable compare-and-swap acceptance remains authoritative after
the broker duplicate window expires.

## Time and diagnostic bounds

All timestamps are non-zero canonical UTC RFC 3339 values. `durationMs` is a
non-negative measured duration and does not derive ordering authority from
wall-clock time.

Version 1 applies these byte/count bounds before publication or acceptance:

| Evidence | Maximum |
| --- | ---: |
| Issues per result | 256 |
| Top-level diagnostic | 8 KiB |
| Total issue text | 256 KiB |
| Rule | 256 bytes |
| Severity | 64 bytes |
| Location | 2 KiB |
| Test expression | 16 KiB |
| Message | 8 KiB |
| Per-issue diagnostic | 4 KiB |

Every text field must be valid UTF-8. Each issue must use a recognized issue
kind and its `layer` must equal the result's stage. The bounds preserve the
existing XSD diagnostic and EN 16931 SVRL limits while preventing a durable
queue message from becoming an unbounded diagnostic container.

## Rollout boundary

This issue defines and tests the shared payloads only. It does not provision
subjects, streams, or consumers and does not activate the split runtime. Queue
topology belongs to `TODO-VALIDATION-SPLIT-004`; orchestrator publication and
acceptance belong to the following orchestrator issues. The current monolithic
worker and public HTTP result remain unchanged until that migration.
