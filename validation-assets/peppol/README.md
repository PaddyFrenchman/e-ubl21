# Peppol validation source assets

This directory contains the immutable source inputs selected for the future
Peppol BIS Billing validation stage. The current bundle is
[`3.0.21/`](3.0.21/) and contains:

- the official self-contained `PEPPOL-EN16931-UBL.sch` Schematron;
- all 17 code-list XML sources from the pinned publication;
- all 10 UBL documents in the official `rules/examples` source directory;
- machine-readable release provenance and a SHA-256 manifest.

The bundle deliberately excludes the CEN Schematron already represented by
the separate EN 16931 stage, all CII artefacts, SBDH envelope rules, and unit
or national test corpora. The source selection and exclusions are recorded in
[`provenance.json`](3.0.21/provenance.json).

Run the offline verifier from the repository root:

```sh
go run ./tools/verify-validation-assets
```

The verifier pins the manifest digest in Go, checks every listed asset byte,
rejects missing and unmanifested files, enforces the exact 1/17/10 inventory,
and compares the provenance document with the compiled release metadata. The
Schematron remains source input; deterministic compilation into executable
XSLT belongs to issue #69.

The files are copied unchanged from OpenPeppol repository commit
`806866bd2bd91d7e9623b68f08164e8fbe9e67a0`. Their upstream terms and the CEN
notice in the Schematron header continue to apply. This narrow source bundle
does not grant rights to the complete OpenPeppol documentation downloaded
under `standard/peppol/downloaded/`.
