# EN 16931 validation runtime upgrade procedure

The runtime uses the official UBL stylesheet pinned in
`validation-assets/en16931/<version>/`. Runtime startup and document
validation must never download rules from GitHub, Maven, or a standards site.

Every upgrade is an explicit, reviewed release change. Never combine an EN
ruleset bump with a Saxon, Java, or libxml2 bump unless compatibility requires
it; independent changes make regressions attributable.

This component procedure supplements the canonical
[`standards upgrade workflow`](STANDARDS-UPGRADE-WORKFLOW.md). That workflow's
accepted/candidate evidence separation, fixture change ledger, baseline
promotion boundary, downstream Peppol review, and approval record are
mandatory for every EN ruleset change.

## EN 16931 artefact

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
6. Run `make test-en16931`. This mandatory gate includes the positive Invoice
   and CreditNote corpus, all targeted semantic-negative cases, and the
   XSD-invalid short-circuit control.
7. Review changes to rule identifiers, severities, SVRL structure, code lists,
   Saxon compatibility, result serialization, and API compatibility.
8. Record the compatibility findings and make an explicit profile version bump
   visible in release notes and operational metadata before deployment.

The checksum must never be changed merely to make verification pass. A changed
digest requires a provenance and content review, even if the release tag has
the same name.

## Saxon-HE or XML Resolver

1. Select published upstream releases and review their release notes, supported
   Java versions, license, and any security advisories.
2. Update both versions, Maven URLs, filenames, and independently calculated
   SHA-256 values in `devsetup/download_saxon_he.sh` as applicable.
3. Update the matching version and checksum source of truth in
   `validation-assets/saxon.go`. Update explicit Compose paths if a filename
   changes.
4. Delete the local cached JAR being changed and rerun the download script;
   never validate only a previously cached file.
5. Run `go run ./tools/verify-validation-assets -saxon-dir
   devsetup/tools/saxon`, then `make test-en16931`.
6. Review the startup smoke transform, SVRL parse, timeout/cancellation tests,
   no-network resolution tests, and maximum-SVRL boundary. Confirm the license
   and source link in `THIRD_PARTY_NOTICES.md` still apply.

Do not accept a transitive runtime dependency without pinning, packaging,
checksum verification, license review, and an offline startup test for it.

## Java, xmllint, or container base

1. Change the exact Alpine package version in `Dockerfile`; an unavailable pin
   must fail the build rather than silently select another release.
2. Review upstream compatibility and security notes. Keep Java on a version
   supported by the selected Saxon release.
3. Run `go test ./... -count=1`, `make test-en16931`, and
   `make test-en16931-image`. The last target proves that the freshly packaged
   runtime initializes and transforms with container networking disabled.
4. Update the version table in `EN16931-DEPLOYMENT.md` and the third-party
   inventory when applicable.

## Release regression evidence

Before deployment, retain the following with the release or change request:

- the changed component versions, upstream release/tag URLs, and SHA-256
  values;
- a reviewed rule/rule-severity diff for an EN artefact change;
- successful `go test ./... -count=1`, `make test-en16931`, and
  `make test-en16931-image` output;
- `BENCHTIME=4x make benchmark-en16931` results compared with the prior
  baseline when rules, Saxon, Java, or libxml2 changed;
- the `validation runtime verified` event from the candidate image;
- confirmation that `THIRD_PARTY_NOTICES.md` remains accurate.

A change is blocked by a formerly valid positive fixture becoming invalid, a
targeted negative no longer firing its intended rule, malformed/different SVRL
semantics, loss of cancellation/resource bounds, or unexplained material
capacity regression. Intentional standards changes require an explicit
compatibility note and fixture/contract update, not a weakened assertion.
