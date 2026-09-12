# Peppol envelope rules 3.0.21

`rules/PeppolBIS-Envelope-Rules.sch` is an exact copy from
`OpenPEPPOL/peppol-bis-invoice-3` commit
`806866bd2bd91d7e9623b68f08164e8fbe9e67a0` at
`rules/sch/PeppolBIS-Envelope-Rules.sch` (SHA-256
`96648791fb824b6b58ddae9cc5575e8298e0061fc43dd0934a0b1459396d76f9`).

The independently versioned `3.0.21-envelope.1` runtime contract implements
the official BR-SBDH-1 and BR-SBDH-2 checks for the supported Peppol Billing
profiles and local fail-closed checks for SBDH document and process
identifiers. BR-SBDH-3 and BR-SBDH-4 apply to self-billing, which the preceding
payload-profile stage does not accept. Local checks use `EUBL21-*` rule
identities and never masquerade as official Peppol assertions.

This Schematron is a checksum-pinned provenance/reference artefact. The
runtime applies the documented bounded checks directly in Go; it does not
execute this file or claim SBDH XML Schema validation. See
[`docs/PEPPOL-ENVELOPE-VALIDATION.md`](../../../docs/PEPPOL-ENVELOPE-VALIDATION.md)
for the acceptance boundary.
