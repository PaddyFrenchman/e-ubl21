# EN 16931 validation asset upgrade procedure

The runtime uses the official UBL stylesheet pinned in
`validation-assets/en16931/<version>/`. Runtime startup and document
validation must never download rules from GitHub, Maven, or a standards site.

An upgrade is an explicit release change:

1. Select and review a new official release from
   `ConnectingEurope/eInvoicing-EN16931`; never select a branch or an unpinned
   URL.
2. Download the release's `ubl/xslt/EN16931-UBL-validation.xslt` during
   development and independently review its SHA-256 digest and provenance.
3. Add a new versioned asset directory. Do not overwrite the old version in
   place.
4. Update the version and checksum passed to the `en16931` profile constructor
   in `validation-assets/assets.go`, plus the checksum manifest, provenance
   file, and download script in the same change. Asset and release paths are
   derived from that selected version; the Dockerfile has no version to bump.
5. Run `go run ./tools/verify-validation-assets`. A digest mismatch blocks the
   build before the stylesheet is packaged.
6. Run the full official and project fixture suite, including known-valid,
   known-invalid, warning-only, Invoice, and CreditNote cases.
7. Review changes to rule identifiers, severities, SVRL structure, code lists,
   Saxon compatibility, result serialization, and API compatibility.
8. Record the compatibility findings and make an explicit profile version bump
   visible in release notes and operational metadata before deployment.

The checksum must never be changed merely to make verification pass. A changed
digest requires a provenance and content review, even if the release tag has
the same name.
