# Peppol envelope validation

Complete Peppol Business Message Envelopes are opt-in. Submit an SBDH
`StandardBusinessDocument` with `validationProfile=peppol-envelope`. Bare UBL
documents cannot select this profile, and SBDH documents cannot silently enter
the payload-only profiles.

The frozen plan has four independently reported stages:

1. `ubl21-xsd` validates the byte-exact embedded Invoice or CreditNote.
2. `en16931` validates that embedded payload.
3. `peppol-bis-billing` validates that embedded payload.
4. `peppol-envelope` validates the original complete SBDH document.

The fourth stage implements official `BR-SBDH-1` and `BR-SBDH-2` sender and
receiver comparisons for the supported Billing profiles. It also checks that
DocumentIdentification Standard, TypeVersion and Type, the `DOCUMENTID`
scope, and the `PROCESSID` scope agree with the payload namespace, document
type, CustomizationID and ProfileID. Product checks have `EUBL21-*` rule IDs;
official Peppol rule IDs are not manufactured.

Any failure short-circuits later stages. Consequently an envelope defect is a
`peppol-envelope` standards-invalid result after three passing payload stages,
never an XSD, EN 16931, or Peppol Billing payload failure.

The official envelope Schematron is pinned byte-for-byte from Peppol BIS
Billing 3.0.21 and the executable contract is independently versioned as
`3.0.21-envelope.1`; see
[`validation-assets/peppol-envelope/`](../validation-assets/peppol-envelope/).
