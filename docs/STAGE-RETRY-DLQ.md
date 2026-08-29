# Independent stage retry, dead-letter, and replay contract

Issue #57 gives every validation stage an independent operational-failure
budget and operator recovery path. Standards-invalid evidence remains terminal
standards evidence and never enters an operational dead-letter queue.

## Retry ownership

`FrozenValidationRetryPolicy` retains a common `maxAttempts` default and may
freeze independent XSD, EN 16931, and Peppol overrides. The orchestrator owns
logical attempt authorization. Attempts are contiguous within each stage and
an EN failure cannot increment or exhaust the XSD counter.

The matching stage worker owns broker delivery state. On an operational result
that exhausts that stage's frozen budget, the worker publishes a permanent
`StageDeadLetter` before publishing `StageResult` and before ACKing the request.
The DLQ entry contains exact execution/request/result identity, stage ordinal,
ruleset version, logical attempt, observed broker delivery count, typed reason,
bounded diagnostic, and failure time. One subject per deterministic DLQ ID and
`MaxMsgsPerSubject=1` make the first record permanent beyond JetStream's finite
message-ID duplicate window. Duplicate publication reads back and compares the
durable record; a changed delivery count is harmless, while changed failure
evidence is a conflict.

The ordering is deliberate:

1. publish permanent stage DLQ entry when exhausted;
2. publish immutable operational `StageResult`;
3. ACK the stage request.

A crash after step 1 leaves an inspectable DLQ entry and a redeliverable
request. A crash after step 2 redelivers and converges on both permanent
records. No publication failure can ACK away the request.

## Replay decision

The earlier recommendation to reopen the same generation is incompatible with
the #56 permanent execution result: an exhausted generation already owns an
immutable `processing_failed` result and cannot later own a different result.
Therefore operational stage replay appends a new generation in the same
validation lineage while resuming at the failed ordinal.

Passing prerequisites are not rerun. The replay generation carries exact
references to their immutable `StageResult` evidence. Execution creation,
worker authorization, and final-result construction all verify those
references against the frozen plan. A replay cannot invent a prerequisite,
change a ruleset, skip a non-passing stage, or directly publish stage work.
The orchestrator alone creates and dispatches the replay generation.

The operator command is:

```text
e-ubl21 dead-letter stage-retry STAGE DEAD_LETTER_ID
```

It verifies the permanent DLQ record and finalized failed generation, creates
the deterministic replay generation, durably publishes the failed-stage
request, and only then removes the consumed DLQ entry. Explicit standards,
profile, plan, or ruleset changes remain revalidation and create a separate
validation lineage rather than using operational replay.
