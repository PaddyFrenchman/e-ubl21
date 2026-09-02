# Standards provenance

This directory is the canonical entry point for the standards implemented by
e-ubl21. It records the exact implementation artefacts used by the server and
how those artefacts relate to normative publications. It is not a substitute
for the standards themselves.

| Standard/profile | Implemented scope | Pinned implementation input |
| --- | --- | --- |
| OASIS UBL 2.1 | XML syntax for all 65 UBL 2.1 main document types | Schemas committed under [`XSD/`](../XSD/) |
| EN 16931 | Core invoice semantic validation for UBL 2.1 Invoice and CreditNote | ConnectingEurope validation artefacts `1.3.16`; see [`en16931/`](en16931/) |
| Peppol BIS Billing 3.0 | Planned payload-conformity stage for UBL Invoice and CreditNote | BIS `3.0.21`, UBL validation artefacts `1.3.16`; see [`peppol/`](peppol/) |

## What is stored here

`standard/` contains project-authored provenance, licensing notes, acquisition
instructions, and checksum manifests. Runtime files stay in their one
authoritative repository location:

- UBL schemas: [`XSD/`](../XSD/);
- EN 16931 runtime stylesheet: [`validation-assets/en16931/`](../validation-assets/en16931/);
- Peppol validation source bundle:
  [`validation-assets/peppol/`](../validation-assets/peppol/), verified here by
  reference rather than duplicated under `standard/`.

OpenPeppol documentation and other material whose redistribution permission is
not established are downloaded only into the ignored
`standard/peppol/downloaded/` tree. They must not be committed.

## Versioning and provenance rules

1. Production truth must resolve to an immutable release tag or full commit
   identifier. Moving branches and mutable `latest`, `main`, `master`, or
   `upcoming` URLs are never version selectors.
2. Every acquired byte used for validation or audit must have a recorded
   SHA-256 digest and an authoritative source URL.
3. Changing a version, source, filename, or digest is a reviewed standards
   upgrade. It must follow the relevant upgrade contract and regression gates.
4. Normative CEN publications, rendered OpenPeppol documentation, and other
   restricted material are not copied wholesale into Git. Only the narrowly
   selected validation inputs tracked by the controlled asset lifecycle are
   vendored unchanged; broader sources use a reproducible acquisition
   procedure instead.
5. The component's own licence and terms continue to apply. See
   [`THIRD_PARTY_NOTICES.md`](../THIRD_PARTY_NOTICES.md) for material packaged
   in the runtime image.
