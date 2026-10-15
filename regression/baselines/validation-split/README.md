# Validation-split semantic baseline

This directory freezes the normalized standards behavior of the last
monolithic worker architecture at commit
`579f5f6a96bd3ec63574cad8d88b9bf1924ca936`. Both result files were successful
and byte-identical between the authenticated correctness topology and the
deployed Minikube topology before capture.

The equivalence gate compares complete normalized fixture records. A candidate
must contain exactly the same fixture IDs, fixture hashes, profiles, expected
contract, final outcomes, ordered stage outcomes, and exact actual rule-ID
sets. JSONL line order is not significant. Missing, unexpected, or changed
fixtures fail the gate and are named in `equivalence.json`.

Run a comparison with:

```bash
go run . regression equivalence \
  --baseline regression/baselines/validation-split/ubl21.results.jsonl \
  --candidate path/to/candidate/results.jsonl \
  --output path/to/candidate/equivalence.json
```

The standard correctness and Minikube lifecycles perform this comparison
automatically for the two canonical manifests. A noncanonical manifest is not
silently compared with an unrelated baseline; callers may supply
`EQUIVALENCE_BASELINE` explicitly to the Minikube lifecycle.

Baseline updates are review events, not generated approvals. A standards or
ruleset upgrade may intentionally change results only when the manifest,
baseline records, hashes in `baseline.json`, and rationale are reviewed in the
same change. A new implementation result must never automatically replace this
baseline.

The UBL fixture hashes were rebound in `02c177bcc6cf28dcaae89b408cf62e5a47b789be`
after the reviewed corpus regeneration. The authenticated runtime evidence
recorded all 71 fixtures as passing with unchanged final and stage outcomes;
only the generated fixture bytes and their manifest hashes changed.

## Separate asynchronous behavior gate

Semantic equality cannot prove durable orchestration behavior. Completion of
the validation split must additionally exercise these scenarios against the
generated fixtures and frozen validation plans:

| Scenario | Required invariant |
| --- | --- |
| XSD short circuit | XSD-invalid input creates no EN or Peppol request. |
| Downstream outage | Successful XSD evidence survives while EN is unavailable. |
| EN scale to zero | EN requests accumulate durably and drain after restoration. |
| Duplicate requests/results | A stage executes/finalizes at most once logically. |
| Orchestrator restart | Restart at a transition boundary neither loses nor advances work twice. |
| Worker restart | Redelivery preserves the final outcome and standards diagnostics. |
| Frozen stage order | Permanent evidence matches the ordered validation plan. |

Those queue/restart/scale scenarios remain behavioral integration tests; their
transport timing and delivery counts are deliberately excluded from the
normalized semantic baseline. They are mandatory in the split-stage chaos and
final end-to-end gates, while this directory remains the semantic oracle.
