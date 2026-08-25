# EN 16931 server validation contract

This document freezes the target server contract for the initial EN 16931
milestone. It defines policy and scope; the current runtime remains UBL 2.1
XSD-only until the later implementation steps in `TODO-EN-16931.md` are
completed and deployed.

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
