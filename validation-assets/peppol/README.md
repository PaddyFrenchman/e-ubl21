# Peppol validation assets

This directory contains the immutable source inputs selected for the future
Peppol BIS Billing validation stage. The current bundle is
[`3.0.21/`](3.0.21/) and contains:

- the official self-contained `PEPPOL-EN16931-UBL.sch` Schematron;
- all 17 code-list XML sources from the pinned publication;
- all 10 UBL documents in the official `rules/examples` source directory;
- machine-readable release provenance and a SHA-256 manifest;
- the deterministic executable XSLT and its compiler provenance under
  [`runtime/`](3.0.21/runtime/).

The bundle deliberately excludes the CEN Schematron already represented by
the separate EN 16931 stage, all CII artefacts, SBDH envelope rules, and unit
or national test corpora. The source selection and exclusions are recorded in
[`provenance.json`](3.0.21/provenance.json).

Run the offline verifier from the repository root:

```sh
go run ./tools/verify-validation-assets
```

The verifier pins the source manifest and compiled-XSLT digests in Go, checks
every listed asset byte, rejects missing and unmanifested files, enforces the
exact 1/17/10 source inventory, compares the provenance documents with compiled
release metadata, and verifies that rule IDs, severity/role, diagnostics, and
namespace bindings survive compilation.

Run the mandatory deterministic compilation gate with:

```sh
make test-peppol-schematron
```

The gate downloads checksum-pinned SchXslt 1.10.1 and Saxon-HE 12.7 build
dependencies into the ignored `devsetup/tools/` directory. It compiles the
official Schematron twice with compilation timestamps disabled, requires
byte-identical output, compares that output with the accepted
`PEPPOL-EN16931-UBL-validation.xslt`, and executes the official positive example
offline to produce SVRL. The production image repeats this compilation check
but contains only the accepted XSLT and runtime Saxon dependencies; SchXslt and
all download tools remain build-only.

The files are copied unchanged from OpenPeppol repository commit
`806866bd2bd91d7e9623b68f08164e8fbe9e67a0`. Their upstream terms and the CEN
notice in the Schematron header continue to apply. This narrow source bundle
does not grant rights to the complete OpenPeppol documentation downloaded
under `standard/peppol/downloaded/`.
