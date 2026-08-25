# EN 16931 server validation contract

This document freezes the target server contract for the initial EN 16931
milestone. It defines policy and scope; the production worker pipeline now
executes these stages and commits their terminal outcome through the durable
worker state machine.

## Supported scope and mandatory order

| UBL root | Required validation stages, in order |
|---|---|
| `Invoice` | `ubl21-xsd`, then `en16931` |
| `CreditNote` | `ubl21-xsd`, then `en16931` |
| Every other recognized UBL 2.1 document | `ubl21-xsd` only |

The EN 16931 stage never runs after an XSD-invalid result. For `Invoice` and
`CreditNote`, absence or operational failure of the mandatory EN 16931 stage
must never be interpreted as success.

## Overall terminal status

- `valid`: every stage required for the document type completed and passed.
- `invalid`: a required standards-validation stage completed and found the
  document non-compliant. An XSD failure short-circuits the semantic stage.
- `processing_failed`: validation could not be completed because a required
  stage was missing or suffered an operational/infrastructure failure after
  retry exhaustion. Examples include unavailable validator executables,
  timeout, malformed validator output, unreadable assets, and storage failure.

Standards invalidity and operational failure are distinct. A business-rule
failure is not retried as infrastructure failure, while infrastructure failure
must never produce `valid` or a false standards-invalid result.

## Durable completion and rolling upgrades

The worker does not persist an XSD-only intermediate success. It commits a
terminal `valid`/`invalid` record, its complete ordered stage results, and the
pending-result marker in one KV compare-and-set operation. A crash while EN
16931 is running therefore leaves the upload `pending`; redelivery reruns the
pipeline. EN runtime and SVRL failures follow the existing operational retry
and dead-letter path. The completion reconciler publishes only terminal records
whose required stage evidence is complete.

During an upgrade, a pre-EN terminal Invoice or CreditNote with
`resultPending=true` is handled according to the permanent result subject:

- If no result was ever accepted, reconciliation queues the document for the
  complete current validation pipeline. It cannot publish the old partial
  record as a new result.
- If the permanent first-result slot is already occupied, that historical
  result is immutable. Reconciliation clears only the ambiguous pending marker
  and neither overwrites the event nor rewrites the historical terminal record.

Older terminal records with no pending publication are likewise retained as
historical results. This compatibility rule preserves the one-result-per-upload
contract without allowing current workers to create a new XSD-only success.

## Initial SVRL severity policy

- `fatal` and `error` failed assertions make the EN 16931 stage invalid.
- `warning`/`warn` issues are retained but do not make the stage invalid.
- Empty or unknown severity values are retained verbatim and conservatively
  make the stage invalid. This avoids reporting compliance when the official
  result cannot be classified under the known policy.

## Explicit exclusions

This milestone does not include Peppol BIS Billing, national CIUS profiles,
CII, or other semantic profiles. Those are future validation layers and must
not be presented as having passed merely because UBL XSD and core EN 16931
validation passed.
