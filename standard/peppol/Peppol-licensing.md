# Peppol licensing status

## Purpose

This note records the current licensing and redistribution position for the Peppol material used by e-ubl21. It is a project provenance and risk-tracking document, not a legal opinion.

## Current project decision

Issue #68 packages a narrow, pinned set of Peppol validation source material under `validation-assets/peppol/` so that the project can verify exact bytes, versions, provenance and checksums offline.

The implementation is accepted as-is for the current development milestone. Uncertainty about the precise redistribution terms of selected OpenPeppol source files is **not considered a blocker for issue #68**. The licensing position will be clarified separately before any distribution model requires a definitive answer.

The full Peppol standards/reference documentation downloaded through `standard/peppol/download.sh` remains outside Git under `standard/peppol/downloaded/` and is intentionally excluded from version control.

## Build and distribution boundary

The downloaded standards/reference documentation under `standard/peppol/downloaded/` is documentation and audit material only. It must never be copied into application container images, release archives, installers or other product builds.

The controlled files under `validation-assets/peppol/` are a separate category: they are selected validation source inputs used by the implementation and verifier. Their inclusion, transformation or redistribution in future runtime/release artefacts must be reviewed explicitly as part of packaging and release work.

Do not treat the presence of a file in the public upstream OpenPeppol repository as, by itself, proof of a particular redistribution licence.

## What is currently known

- The Peppol BIS Billing source material is pinned to immutable OpenPeppol repository commit `806866bd2bd91d7e9623b68f08164e8fbe9e67a0`.
- The complete downloaded source/reference tree is checksum-verified but ignored by Git.
- Issue #68 commits only the narrow validation bundle required by the controlled asset lifecycle: the selected Peppol Schematron, relevant code-list sources, official examples, provenance and digests.
- The Peppol Schematron contains a CEN notice concerning reproduction of EN 16931 business terms, but that notice should not be interpreted as a complete licence analysis for every OpenPeppol file in the bundle.
- No project document should claim broader redistribution rights unless those rights have been verified from an authoritative licence or written permission.

## Follow-up licensing review

Before e-ubl21 reaches a release or distribution stage where these Peppol validation inputs or derivatives are shipped to third parties, clarify and record:

1. the applicable OpenPeppol licence or terms for the selected Schematron, code lists and examples;
2. whether redistribution of the unmodified selected files is permitted;
3. whether compiled or transformed validation artefacts may be redistributed and under what conditions;
4. any attribution, notice, membership, consent or other obligations;
5. whether `THIRD_PARTY_NOTICES.md` or release packaging needs additional Peppol-specific language.

If redistribution rights are narrower than the repository layout assumes, the project can retain the same pinned provenance and verification model while changing packaging to obtain the files from the immutable authoritative source during setup/build rather than redistributing the originals.

## Rule until clarified

Keep the licensing question visible, preserve exact provenance and checksums, do not broaden the set of committed OpenPeppol material without review, and do not infer redistribution permission from public accessibility alone.
