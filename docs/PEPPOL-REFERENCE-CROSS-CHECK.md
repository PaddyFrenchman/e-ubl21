# Peppol reference cross-check

`make test-peppol-reference` is the mandatory reference-behaviour gate for
Peppol BIS Billing 3.0.21 and validation artefacts 1.3.16. It binds the server's
pinned runtime to the exact OpenPeppol source commit recorded in
`validation-assets/peppol/3.0.21/provenance.json`.

The reviewed contract is
`validation-assets/peppol-reference/3.0.21.json`. It is separate from the
generated corpus manifest, checksum protected, and freezes:

- the release, source commit, source Schematron digest, and runtime digest;
- all ten byte-for-byte official OpenPeppol examples and their passing outcome;
- six representative invalid inputs, including exact ordered rule IDs and
  severities across core, code-list, common, arithmetic, profile, and national
  rules;
- 13 active patterns, 99 rules, 165 assertions, zero reports, and all six
  Schematron namespace bindings.

The gate first verifies the source and accepted assets. It then independently
compiles the official Schematron with the pinned SchXslt 1.10.1 and Saxon-HE
12.7 toolchain. Every case is executed with both that fresh compilation and the
accepted server stylesheet. Both normalized semantic results must exactly
match the reviewed contract. The production SVRL parser performs the
normalization, so the comparison also covers what the server records.

Only presentation differences are acceptable: XML indentation, namespace
declaration order, generated source-document URIs, whitespace within messages,
and diagnostic prose formatting. Those fields are intentionally outside this
oracle. Outcome, issue kind, issue order, rule ID, severity, active construct
counts, namespace bindings, fixture bytes, source bytes, or release identity
must not differ. Any such difference fails the gate and requires an explicit
reviewed manifest update as part of a pinned Peppol release upgrade.

This is an offline reproducibility cross-check, not a claim that a hosted
third-party validation service was queried. The independent side is a fresh
compilation of the unmodified official source at the exact pinned commit; the
accepted side is the stylesheet used by the server.
