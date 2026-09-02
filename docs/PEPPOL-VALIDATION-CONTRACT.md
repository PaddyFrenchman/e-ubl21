# Peppol BIS Billing validation contract

**Status:** Normative product contract for `TODO-PEPPOL-001` / issue #66

**Contract date:** 2026-09-02

**First supported release:** Peppol BIS Billing 3.0 version 3.0.21

## Meaning of Peppol-valid

An e-ubl21 result is Peppol-valid only when one frozen asynchronous validation
plan has durable `passed` evidence, in order, for all three required stages:

1. UBL 2.1 XSD;
2. EN 16931;
3. Peppol BIS Billing.

The orchestrator alone advances and finalizes this plan. A worker cannot call
the next validator, skip a stage, or publish the final result. UBL-valid and
EN 16931-valid are necessary but are not Peppol-valid.

The Peppol claim is limited to conformity of the submitted UBL payload. It is
not a claim that the document was transported through the Peppol network, that
either party is registered in SMP/SML, that an Access Point or certificate is
valid, or that a response message was sent or received.

## Frozen standards scope

| Item | Contract |
| --- | --- |
| BIS | Peppol BIS Billing 3.0 |
| BIS release | 3.0.21, published 2026-05-20, mandatory from 2026-08-17 |
| Validation artefact release | UBL/CII 1.3.16, published 2026-04-10; only the UBL artefact is in scope |
| Code-list publication | 2026-03-06 |
| Syntax | OASIS UBL 2.1 |
| Documents | `Invoice` and `CreditNote` |
| Process profiles | Profile 01 Billing and Profile 02 Billing with Response |
| Stage identity | `peppol-bis-billing` |

The release identity is frozen here. Issue #67 must pin the corresponding
immutable official archive/source bytes, filenames, checksums, licence, and
provenance before a production Peppol worker exists. No `main`, `master`,
`upcoming`, or other moving location can become runtime truth.

The release dates and Profile 02 introduction are recorded in the official
[3.0.21 release notes](https://docs.peppol.eu/poacc/billing/3.0/release-notes/).
The stable BIS identifies UBL 2.1 Invoice and CreditNote and defines both
[Billing profiles](https://docs.peppol.eu/poacc/billing/3.0/bis/#_peppol_identifiers).

## Supported process identifiers

Both profiles use this `cbc:CustomizationID` base value:

```text
urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0
```

Profile 01 Billing uses:

```text
urn:fdc:peppol.eu:2017:poacc:billing:01:1.0
```

Profile 02 Billing with Response uses:

```text
urn:peppol:bis:billing_with_response
```

Profile 02 changes the collaboration identifier and means that an Invoice
Response is part of the business process. For this product milestone, its
Invoice/CreditNote payload passes the same ordered three-stage conformity
contract. e-ubl21 does not validate or transport the separate response message
and does not assert the parties' required SMP registrations.

## Requested and resolved profiles

The public requested-profile vocabulary is:

| Requested value | Meaning |
| --- | --- |
| `auto` | Resolve from document type and positively recognized identifiers. This is the behavior of an omitted request option. |
| `ubl21` | Force UBL 2.1 XSD only. |
| `ubl21-en16931` | Force UBL 2.1 XSD followed by EN 16931. |
| `peppol-bis-billing` | Force all three stages and require supported Peppol identifiers. |

The resolved profile stored with a frozen execution is never `auto`; it is one
of the other three values. Existing `ubl21` and `ubl21-en16931` explicit
requests never acquire a Peppol stage merely because identifiers are present.

Automatic resolution is deterministic:

1. A document other than `Invoice` or `CreditNote` resolves to `ubl21`.
2. An `Invoice` or `CreditNote` resolves to `peppol-bis-billing` only when its
   whitespace-normalized `CustomizationID` starts with the supported base and
   its `ProfileID` exactly equals the Profile 01 or Profile 02 value.
3. Every other `Invoice` or `CreditNote` resolves to `ubl21-en16931`.

Identifier comparison is case-sensitive and performs no URI decoding or
Unicode substitution. Positive automatic resolution is a convenience, not a
substitute for an explicit Peppol request. An automatic EN-only result must
expose `resolvedProfile: ubl21-en16931` and must never be described as
Peppol-valid.

The base-prefix rule follows the official
[PEPPOL-EN16931-R004](https://docs.peppol.eu/poacc/billing/3.0/rules/ubl-peppol/PEPPOL-EN16931-R004/)
model and permits an official extension suffix to be validated rather than
discarded during profile selection.

An explicit `peppol-bis-billing` request always freezes the three-stage plan.
Missing, malformed, mismatched, or unsupported Peppol identifiers then produce
Peppol `standards-invalid` evidence after XSD and EN pass; they can never cause
fallback to EN-only validity. Applying `ubl21-en16931` or
`peppol-bis-billing` explicitly to any other UBL document type is rejected at
request acceptance because no applicable plan exists.

The implementation keeps resolution independent from Schematron execution.
Its bounded XML scanner reads only unambiguous top-level UBL
`cbc:CustomizationID` and `cbc:ProfileID` values, applies XPath-compatible XML
whitespace normalization, and performs case-sensitive comparisons without URI
decoding or Unicode substitution. It returns a concrete frozen-plan profile
plus normalized identifiers and, for explicit invalid Peppol requests,
structured R001/R004/R007 evidence. This resolver and plan builder do not
enable the public HTTP selector; that API wiring remains owned by issue #75.

## Final status semantics

The final status belongs to the resolved frozen plan:

- `valid`: every required stage has durable `passed` evidence;
- `invalid`: the first reached standards stage has durable
  `standards-invalid` evidence; later stages are not requested;
- `processing_failed`: a stage cannot produce a standards decision after its
  frozen operational retry policy is exhausted or forbids retry.

An operational failure is never standards-invalid. Before retry exhaustion,
the execution remains non-terminal (`pending`) and may advance only after a
later authorized attempt passes or finds a standards failure.

## Normative truth table

`PASS`, `INVALID`, and `OP-FAIL` below are durable terminal stage outcomes.
`-` means the stage is not in the plan or must not be requested.

| Document | Requested profile | Identifier resolution | XSD | EN 16931 | Peppol | Final result and claim |
| --- | --- | --- | --- | --- | --- | --- |
| Other recognized UBL | `auto` or `ubl21` | n/a | PASS | - | - | `valid`; UBL 2.1-valid only |
| Other recognized UBL | `auto` or `ubl21` | n/a | INVALID | - | - | `invalid` |
| Other recognized UBL | `auto` or `ubl21` | n/a | OP-FAIL | - | - | `processing_failed` |
| Other recognized UBL | `ubl21-en16931` or `peppol-bis-billing` | n/a | - | - | - | Request rejected: profile is not applicable; no validation execution |
| Any recognized UBL | unknown requested value | n/a | - | - | - | Request rejected: unsupported profile; no validation execution |
| Invoice/CreditNote | `ubl21` | ignored | PASS | - | - | `valid`; UBL 2.1-valid only |
| Invoice/CreditNote | `auto` | supported CustomizationID and Profile 01/02 ID | PASS | PASS | PASS | `valid`; Peppol BIS Billing-valid for resolved Profile 01/02 |
| Invoice/CreditNote | `auto` | identifiers do not positively resolve | PASS | PASS | - | `valid`; EN 16931-valid only, never Peppol-valid |
| Invoice/CreditNote | `ubl21-en16931` | ignored | PASS | PASS | - | `valid`; EN 16931-valid only |
| Invoice/CreditNote | `peppol-bis-billing` | supported Profile 01/02 identifiers | PASS | PASS | PASS | `valid`; Peppol BIS Billing-valid for resolved Profile 01/02 |
| Invoice/CreditNote | `peppol-bis-billing` | missing, wrong, or unsupported identifiers | PASS | PASS | INVALID | `invalid`; no downgrade to EN-only validity |
| Invoice/CreditNote | any applicable profile | any | INVALID | - | - | `invalid`; EN and Peppol are not requested |
| Invoice/CreditNote | `auto`, `ubl21-en16931`, or `peppol-bis-billing` | any | PASS | INVALID | - | `invalid`; Peppol is not requested |
| Invoice/CreditNote | resolved Peppol plan | supported | PASS | PASS | INVALID | `invalid`; UBL and EN pass evidence remains visible |
| Invoice/CreditNote | any applicable profile | any | OP-FAIL | - | - | `processing_failed` after XSD retry exhaustion |
| Invoice/CreditNote | resolved EN or Peppol plan | any | PASS | OP-FAIL | - | `processing_failed` after EN retry exhaustion |
| Invoice/CreditNote | resolved Peppol plan | supported | PASS | PASS | OP-FAIL | `processing_failed` after Peppol retry exhaustion; never `invalid` |

For the rows with `auto`, the resolved profile is frozen before dispatch. A
later replay cannot reinterpret identifiers or adopt a newer release. A ruleset
upgrade requires an explicit revalidation lineage with a newly frozen plan.

## Peppol issue severity

The Peppol worker preserves bounded official rule IDs and evidence:

| SVRL evidence | Stage effect |
| --- | --- |
| `failed-assert` with `fatal` or `error` | Peppol `standards-invalid` |
| `failed-assert` with `warning` or `warn` | Retained advisory evidence; does not invalidate |
| `successful-report` | Retained informational evidence; does not invalidate |
| `failed-assert` with empty or unknown severity | Fail closed as Peppol `standards-invalid` |
| Malformed/missing SVRL, timeout, crash, missing assets, or resource bound | `operational-failure`, never a standards verdict |

Warnings remain visible to clients even when the final result is `valid`.
Implementations must not relabel or discard official fatal/error evidence.

## Client-visible Peppol identity

When the Peppol stage is in the frozen plan, status and permanent results must
expose enough identity to make the claim unambiguous:

| Field | Required value |
| --- | --- |
| `requestedProfile` | `auto` or the caller's explicit value |
| `resolvedProfile` | `peppol-bis-billing` |
| Peppol stage `layer` | `peppol-bis-billing` |
| Peppol stage `version` | machine validation artefact version, initially `1.3.16` |
| Peppol stage `bisVersion` | `3.0.21` |
| Peppol stage `bisReleaseDate` | `2026-05-20` |
| Peppol stage `bisMandatoryFrom` | `2026-08-17` |
| Peppol stage `customizationId` | normalized identifier actually resolved/validated |
| Peppol stage `profileId` | normalized Profile 01 or Profile 02 identifier |
| Peppol stage `processProfile` | stable code `01` or `02` |

These fields describe standards and process identity; they do not contain raw
SVRL. Issue #70 adds them to the layered result model and issue #75 exposes the
requested/resolved profile through the HTTP/OpenAPI contract.

## Explicit exclusions

The first Peppol payload milestone excludes CII, Self-Billing, Ordering,
Despatch, national regimes outside the pinned artefact, SBDH/envelope
conformity, Invoice Response payload validation, Peppol transport, participant
discovery, Access Point operation, certificates, accreditation, and proof of
delivery. Optional envelope validation is separately tracked by issue #82.

No implementation may broaden the phrase "Peppol-valid" beyond this contract
without a new versioned contract and regression evidence.
