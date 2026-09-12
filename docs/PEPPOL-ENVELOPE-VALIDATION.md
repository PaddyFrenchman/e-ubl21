# Peppol envelope validation

Complete Peppol Business Message Envelopes are opt-in. Submit an SBDH
`StandardBusinessDocument` with `validationProfile=peppol-envelope`. Bare UBL
documents cannot select this profile, and SBDH documents cannot silently enter
the payload-only profiles.

The frozen plan has four independently reported stages:

1. `ubl21-xsd` validates the byte-exact embedded Invoice or CreditNote.
2. `en16931` validates that embedded payload.
3. `peppol-bis-billing` validates that embedded payload.
4. `peppol-envelope` validates the original complete SBDH document against
   the bounded envelope contract described below.

The fourth stage implements official `BR-SBDH-1` and `BR-SBDH-2` sender and
receiver comparisons for the supported Billing profiles. It also checks that
DocumentIdentification Standard, TypeVersion and Type, the `DOCUMENTID`
scope, and the `PROCESSID` scope agree with the payload namespace, document
type, CustomizationID and ProfileID. Product checks have `EUBL21-*` rule IDs;
official Peppol rule IDs are not manufactured.

The accepted payload profiles are Peppol Billing profile 01 and Billing with
Response. Peppol self-billing is not currently an accepted payload profile, so
the upstream `BR-SBDH-3` and `BR-SBDH-4` self-billing assertions are outside
the executable contract. A self-billing payload is rejected by the preceding
Peppol payload-profile stage and cannot acquire envelope-conformity evidence.

"Envelope validation" here has a deliberately bounded meaning. The parser
requires well-formed XML with one SBDH `StandardBusinessDocument` root, one
header, and one immediate UBL Invoice or CreditNote payload, and rejects XML
directives and ambiguous payloads. The stage then applies the bindings listed
above in Go. It does not execute the pinned Schematron and does not claim full
validation against an SBDH XML Schema. The pinned upstream Schematron is the
immutable provenance/reference artefact for the supported official rules, not
a runtime stylesheet.

Any failure short-circuits later stages. Consequently an envelope defect is a
`peppol-envelope` standards-invalid result after three passing payload stages,
never an XSD, EN 16931, or Peppol Billing payload failure.

The reference envelope Schematron is pinned byte-for-byte from Peppol BIS
Billing 3.0.21 and the narrower executable contract is independently versioned
as `3.0.21-envelope.1`; see
[`validation-assets/peppol-envelope/`](../validation-assets/peppol-envelope/).
