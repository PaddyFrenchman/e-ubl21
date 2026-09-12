# Peppol envelope rules 3.0.21

`rules/PeppolBIS-Envelope-Rules.sch` is an exact copy from
`OpenPEPPOL/peppol-bis-invoice-3` commit
`806866bd2bd91d7e9623b68f08164e8fbe9e67a0` at
`rules/sch/PeppolBIS-Envelope-Rules.sch` (SHA-256
`96648791fb824b6b58ddae9cc5575e8298e0061fc43dd0934a0b1459396d76f9`).

The independently versioned `3.0.21-envelope.1` runtime contract implements
the official BR-SBDH-1 through BR-SBDH-4 checks and local fail-closed checks
for SBDH document and process identifiers. Local checks use `EUBL21-*` rule
identities and never masquerade as official Peppol assertions.
