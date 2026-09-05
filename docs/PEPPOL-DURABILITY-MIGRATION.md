# Peppol durability and migration contract

Peppol validation is the third stage of a frozen validation plan:

```text
UBL 2.1 XSD -> EN 16931 -> Peppol BIS Billing
```

The frozen plan and requested profile are authoritative for the full lineage.
A `valid` Peppol result requires permanent passing evidence for all three
ordered stages. An older EN-only terminal result remains an EN-only result; it
must never be relabelled or interpreted as Peppol-valid because Peppol support
was deployed later.

## Upgrade barrier

Moving an existing document from EN-only validation to Peppol requires an
explicit revalidation intent. That intent creates a new validation lineage at
generation 1 with the Peppol profile, three-stage plan, and current pinned
rulesets. Retry and failed-stage replay do not change profile, plan, or ruleset
versions and therefore cannot serve as an upgrade mechanism.

Before split activation, provision and verify the execution infrastructure and
the XSD, EN 16931, and Peppol request/result/DLQ resources. Activation refuses
to proceed if any of the three worker stages is absent. During a rolling
deployment, keep runtimes for every pinned ruleset referenced by outstanding
executions. A worker that cannot run the frozen ruleset reports an operational
failure; it does not substitute a newer ruleset or produce standards evidence.

## Failure and replay

Peppol has its own frozen retry budget. Standards-invalid evidence terminates
the plan as `invalid`; runtime, timeout, asset, and transport failures consume
the Peppol operational retry budget. Exhaustion writes the permanent stage DLQ
record and a `processing_failed` final result for that generation.

A stage replay command creates the next generation at Peppol, carries only the
verified passing XSD and EN evidence, and leaves the exhausted generation and
its final result immutable. Recovery therefore does not rerun prerequisites.
The replayed Peppol stage writes a separate permanent result for its generation.
Deterministic identities, compare-and-swap execution state, and one permanent
result slot per lineage generation make duplicate commands, requests, results,
and ambiguous broker acknowledgements converge instead of advancing twice.

Requests and results for an older generation can only reconcile that older
generation. They cannot mutate or finalize the current replay generation.

## Shutdown and replacement

If shutdown cancels an active Peppol validation, the worker publishes no
cancelled standards result and negatively acknowledges the request. A
replacement worker receives the same authorized request and resumes from the
durable execution state. Before invoking the validator, every worker verifies
the exact lineage, generation, ordinal, attempt, frozen ruleset, and permanent
passing prerequisite evidence. Stale or superseded deliveries are terminated
without running validation.
