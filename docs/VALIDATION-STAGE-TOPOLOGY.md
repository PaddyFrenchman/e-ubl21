# Namespace-derived validation stage queue topology

**Status:** Normative infrastructure contract for `TODO-VALIDATION-SPLIT-004`

The asynchronous validation split uses physically independent JetStream
request and result queues for each validation stage. This topology implements
the ownership boundaries in
[`VALIDATION-SPLIT-OWNERSHIP.md`](VALIDATION-SPLIT-OWNERSHIP.md) and carries the
versioned payloads in
[`VALIDATION-STAGE-CONTRACTS.md`](VALIDATION-STAGE-CONTRACTS.md).

This issue provisions infrastructure only. It does not activate an
orchestrator, dispatch a stage, or replace the current monolithic worker.

## Physical resources

`Namespace.StageQueue(stage)` is the single derivation point for every stage
subject, stream, and durable consumer. For the default namespace:

| Stage | Request subject | Result subject |
| --- | --- | --- |
| UBL 2.1 XSD | `ubl.validation.request.xsd` | `ubl.validation.result.xsd` |
| EN 16931 | `ubl.validation.request.en16931` | `ubl.validation.result.en16931` |
| Reserved Peppol Billing | `ubl.validation.request.peppol` | `ubl.validation.result.peppol` |

For `EUBL21_QUEUE_NAMESPACE=eubl21.prod`, the EN subjects become:

```text
eubl21.prod.validation.request.en16931
eubl21.prod.validation.result.en16931
```

The matching default-namespace XSD resources illustrate the physical naming
scheme:

```text
request stream:   UBL_VALIDATION_REQUEST_XSD
request consumer: UBL_VALIDATION_XSD_WORKER

result stream:    UBL_VALIDATION_RESULT_XSD
result consumer:  UBL_VALIDATION_XSD_ORCHESTRATOR
```

An explicit namespace prepends its normalized resource prefix, for example
`EUBL21_PROD_VALIDATION_REQUEST_XSD`. Unknown stage IDs are rejected rather
than mapped into an accidental subject. Reserving the Peppol resources now
does not add Peppol to a validation plan or public API profile.

The value `ubl` is reserved for the default deployment's legacy physical
namespace and cannot be configured as an explicit namespace. Although
`Namespace` remains an exported string type, `ConnectWithOptions` revalidates
it before dialing NATS. Direct conversions therefore cannot bypass the
character-set or reserved-name checks and create a client whose resources
collide with the default deployment.

Each stream has exactly one non-wildcard subject. Consequently an XSD worker
cannot consume EN or Peppol traffic through a broad filter, and namespaces
cannot cross-consume. Replicas of one logical worker role share that stage's
derived durable request consumer. Orchestrator replicas similarly share the
derived result consumer for that stage.

## Retention and delivery

Every request and result stream uses:

- file storage;
- work-queue retention;
- explicit acknowledgement;
- deliver-all and instant replay policies;
- unlimited broker redelivery for the same logical attempt until the later
  stage retry/DLQ policy is implemented.

An ACK removes a message only after its owning consumer has durably completed
the corresponding transition. With no worker or orchestrator replica running,
the message remains in its file-backed stream. The default `MaxAge` is zero,
meaning no time-based expiry of unacknowledged correctness work.

`StageQueueConfig` controls these values independently for each stage and
direction:

```text
request MaxAge, AckWait, MaxAckPending
result  MaxAge, AckWait, MaxAckPending
```

The initial default is a 60-second `AckWait` and 64 outstanding messages for
each durable consumer. These are safe initial controls, not performance
claims; later per-stage benchmarks and deployment issues may deliberately set
different values. A positive `MaxAge` is an explicit operator retention
policy. Negative ages, non-positive acknowledgement windows, duplicate stages,
unknown stages, and non-positive pending limits are invalid.

Request streams cap a single message at 16 KiB. A V1 `StageRequest` is a compact
identity descriptor and never contains raw XML; shared 256-byte plan/stage ID
bounds make every valid request transportable below that ceiling.

Result streams allow 2 MiB. The bound is derived conservatively from the V1
limits rather than from ordinary unescaped examples: JSON may expand each
valid source byte to six bytes (`NUL` becomes `\u0000`). Six times the complete
256 KiB issue-text allowance, 8 KiB top-level diagnostic, and two 256-byte
version fields, plus a maximum 256-byte layer on every issue and 64 KiB of
structural/identity allowance, remains below 2 MiB. A compile-time assertion
keeps that calculation below the configured stream limit.

The NATS server/account `max_payload` must therefore be at least 2 MiB
(`2097152`). The embedded test broker, Docker Compose environment, and Minikube
manifest set that value explicitly. Deployments using an external NATS server
must enforce the same prerequisite; a stream limit cannot override a smaller
server/account payload ceiling.

## Provisioning and runtime ownership

`ProvisionStageQueueInfra` is the infrastructure-provisioner mutation. It
idempotently creates or updates the configured stage streams and durable
consumers. `VerifyStageQueueInfra` is read-only and rejects missing or
incompatible subjects, retention, storage, message limits, filters,
acknowledgement policies, acknowledgement windows, or pending limits.

Runtime services use `StageRequestConsumer` and `StageResultConsumer` only to
open an already-provisioned durable consumer. Those methods never create or
update streams or consumers. This preserves #48's rule that application
services verify infrastructure compatibility but do not reconfigure it.

Stage workers use `VerifyStageWorkerInfra` for role-scoped startup checks. It
verifies the fixed request stream/consumer contract and the result stream used
for publication, while accepting provisioner-selected retention and returning
the actual request consumer `AckWait` and `MaxAckPending`. It never opens or
verifies the orchestrator-owned result consumer, whose delivery tuning is
irrelevant to worker readiness.

## Migration safety

Stage resources have new names and are provisioned through a separate entry
point. The existing `EnsureInfra` path deliberately does not create them, and
stage provisioning never updates, replaces, purges, or deletes
`UBL_DOCUMENTS` (or its namespaced equivalent). Existing document descriptors
and the monolithic worker therefore remain intact while the split topology is
deployed ahead of its runtime migration barrier.

Provisioning is restartable: a partial run can be repeated idempotently. The
future migration issue remains responsible for stop-and-drain, generation
barriers, activation ordering, rollback, and removal of the monolithic worker.
