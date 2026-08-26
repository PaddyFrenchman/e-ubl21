# EN 16931 final compliance gate

The release gate is:

```sh
make test-en16931-compliance
make test-en16931-image
```

The first target is also available as `make test-en16931`; both execute the
same mandatory suite with runtime skipping disabled. Java, Saxon 12.7, XML
Resolver 6.0.17, `xmllint`, the pinned EN 16931 1.3.16 XSLT, embedded UBL 2.1
schemas, embedded NATS, Object Store, KV, JetStream work/result/DLQ streams,
the worker, and the authenticated public upload/status API are in the tested
path. The image target separately proves the packaged runtime initializes and
performs a real transform without network access.

## Acceptance evidence

| Required scenario | Gate evidence |
|---|---|
| Valid UBL + EN Invoice | `TestEN16931PublicComplianceScenarios/valid_Invoice...` uploads through the public API, waits for durable publication, and requires ordered passing XSD and EN stages plus exactly one final result. |
| Invalid UBL Invoice | `.../XSD-invalid_Invoice...` requires final `invalid`, one failing XSD stage with diagnostics, and no EN stage. |
| UBL-valid, EN-invalid Invoice | `.../XSD-valid_semantic-invalid...` requires XSD pass, EN failure, and API-visible failed-assert rule ID, layer, severity, and message. |
| Valid CreditNote | `.../valid_CreditNote...` uses a generated compliant CreditNote and requires both real stages and final `valid`. |
| EN operational failure and replay | `TestEN16931OperationalFailureReplayEndToEnd` forces EN runtime failure, requires `processing_failed`, one attempt, DLQ entry, no standards stages/result, restores the real runtime, replays, and requires one passing permanent result and cleared DLQ. |
| Restart between stages | `TestEN16931CrashRestartEndToEnd` cancels a worker while EN is active after XSD, proves the record remains pending with no stages/result and no consumed attempt, then starts a real-runtime replacement and requires one complete result. |
| Concurrent duplicate replay | `TestWorkerClaimsOneReplayAfterMessageIDDeduplicationWindow` publishes duplicate replay descriptors beyond broker deduplication, uses concurrent workers, and requires one durable replay claim and exactly one permanent result. |
| Non-Invoice UBL | `.../non-Invoice_UBL...` uploads a real Order fixture and requires exactly its applicable passing UBL XSD stage; absence of an EN stage explicitly proves it was not run or presented as passed. |

The positive and targeted-negative corpora and the XSD short-circuit control
remain in the same command. Thus the release gate checks broad rule coverage as
well as these transactional public-behavior scenarios.

## Completion boundary

A terminal public `valid` or `invalid` is not sufficient for the test to stop.
The gate waits until `resultPending` is clear, verifies the upload's permanent
result subject exists, decodes that event, checks required stage evidence, and
requires the isolated result stream to contain exactly one message. Operational
failure instead requires that the permanent standards-result slot remains
empty.

Hosted status exists only after the GitHub Actions workflow has run for the
commit. Local success must not be described as a hosted CI result.
