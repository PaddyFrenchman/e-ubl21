# Peppol BIS Billing 3.0 provenance

The first Peppol payload profile supported by e-ubl21 is frozen as follows:

| Field | Pinned value |
| --- | --- |
| BIS | Peppol BIS Billing 3.0 |
| BIS version | `3.0.21` |
| Publication date | 2026-05-20 |
| Mandatory from | 2026-08-17 |
| Code-list publication | 2026-03-06 |
| UBL/CII validation artefact version | `1.3.16` (UBL only is in product scope) |
| Validation artefact publication | 2026-04-10 |
| Official repository | `OpenPEPPOL/peppol-bis-invoice-3` |
| Immutable source commit | `806866bd2bd91d7e9623b68f08164e8fbe9e67a0` |
| Source tree | `a8acc94f1216279839547e1670585fac62dfc122` |

The dates and versions are stated by the official
[3.0.21 release notes](https://docs.peppol.eu/poacc/billing/3.0/release-notes/).
The supported identifiers and product semantics are frozen separately in the
[`Peppol validation contract`](../../docs/PEPPOL-VALIDATION-CONTRACT.md).

## Immutable release identity

OpenPeppol did not publish a GitHub tag or GitHub release object for 3.0.21.
The UBL files published on the official rendered 3.0.21 site are byte-identical
to the repository files at the `2026-Q2-QA2` commit recorded above. Production
provenance therefore uses that full 40-character commit, never the branch
name. The two published UBL Schematron files have these identities:

| Published filename | Source path at the pinned commit | SHA-256 |
| --- | --- | --- |
| `CEN-EN16931-UBL.sch` | `rules/sch/CEN-EN16931-UBL.sch` | `268d4f7a2688676695e6c69cba6fba69a6802604fee12cb544a6b30ff09555a3` |
| `PEPPOL-EN16931-UBL.sch` | `rules/sch/PEPPOL-EN16931-UBL.sch` | `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3` |

The CEN Schematron supplies the adopted EN 16931 layer and the Peppol
Schematron supplies Peppol and country-qualified rules. The optional SBDH
envelope rules are excluded from this product milestone and belong to issue
#82. Later asset-packaging work will place the exact runtime inputs under
`validation-assets/peppol/`; this standards directory must link to that
location rather than hold a second runtime copy.

### Code-list edition

OpenPeppol identifies the code-list set by its **2026-03-06 publication date**;
it does not assign one independent semantic version to the complete set. The
pinned edition consists of `eas`, `icd`, ISO 3166-1, ISO 4217, MIME, SEPA,
UNECE Recommendation 20, VATEX, and the applicable UNCL subsets (`1001`,
`1153`, `2005`, `4461`, `5189`, `5305`, `7143`, and `7161`) under
`structure/codelist/` at the immutable commit. The downloader retrieves every
one of those 17 source files, and [`sha256.txt`](sha256.txt) records each exact
digest. The Schematron digests remain the production behavior identity because
the applicable code values are compiled into those validation rules.

## Download and verification

Run:

```sh
./standard/peppol/download.sh
```

The script requires `curl` and `sha256sum`. It downloads official rendered
documentation plus the two UBL Schematron inputs into:

```text
standard/peppol/downloaded/3.0.21/
├── assets/
│   ├── CEN-EN16931-UBL.sch
│   ├── PEPPOL-EN16931-UBL.sch
│   └── codelists/
│       └── ... 17 pinned XML source files
└── docs/
    ├── bis-index.html
    ├── index.html
    └── release-notes-index.html
```

Every byte is checked against [`sha256.txt`](sha256.txt). Artefacts and code
lists are fetched from raw URLs containing the immutable commit. The rendered
documentation is served at OpenPeppol's official stable 3.0 location, so its
pinned checksums prevent a later site update from being mistaken for 3.0.21.
A repeat run verifies the existing directory and performs no download.

To start over after a reported mismatch, remove only
`standard/peppol/downloaded/3.0.21/` and rerun the script.

## Licensing and redistribution

The OpenPeppol repository and rendered site do not provide a repository-level
licence grant that permits this project to redistribute the complete rendered
documentation. Consequently none of the downloaded files are committed, and
the entire `downloaded/` tree is ignored by Git. Users must comply with the
terms attached to OpenPeppol material and obtain any required membership or
permission directly from OpenPeppol.

The official Schematron headers state that business terms derived from CEN
EN 16931 are reproduced with permission and that CEN provides no warranty.
Keeping locally downloaded material does not grant a right to republish either
OpenPeppol or CEN content.
