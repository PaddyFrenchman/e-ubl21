# EN 16931 provenance

e-ubl21 implements the semantic data model and business rules of
**EN 16931-1:2017**, including amendment **A1:2019** and corrigendum
**AC:2020**, through the official UBL 2.1 syntax binding and the maintained
technical validation artefacts. Normative CEN documents are not committed:
their redistribution rights have not been established for this repository.

Authoritative access points:

- [CEN project 71870 — EN 16931-1, semantic data model](https://standards.cencenelec.eu/dyn/www/f?p=205:110:0::::FSP_PROJECT:71870&cs=1D0C36D84F6C30D746A7F9FB3E16CC355)
- [European Commission — compliance with the eInvoicing standard](https://ec.europa.eu/digital-building-blocks/sites/spaces/DIGITAL/pages/467108926/Compliance+with+eInvoicing+standard)
- [European Commission — required syntaxes and validation](https://ec.europa.eu/digital-building-blocks/sites/spaces/DIGITAL/pages/467108934/Required+syntaxes)
- [European Commission registry of EN 16931 supporting artefacts](https://ec.europa.eu/digital-building-blocks/sites/display/DIGITAL/Registry+of+supporting+artefacts+to+implement+EN16931)

Access to the normative standard may require purchase or access through a
national CEN member. Obtain it from CEN or an authorized national standards
body; do not copy it into this repository.

## Pinned technical implementation

| Field | Value |
| --- | --- |
| Maintainer | European Commission DIGITAL / CEN-TC434 project |
| Source repository | `ConnectingEurope/eInvoicing-EN16931` |
| Release tag | `validation-1.3.16` |
| Release page | <https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/tag/validation-1.3.16> |
| UBL release archive | `en16931-ubl-1.3.16.zip` |
| Archive SHA-256 | `bafada015efbc5248bf5e05ad2191e1d9833ef96e9dd5f4bce420a747342da85` |
| Upstream release date | 2026-04-13 (upstream README records the artefact version as 2026-04-10) |
| Licence | European Union Public Licence 1.2 (`EUPL-1.2`) |

The server packages only the precompiled UBL stylesheet it executes:

| Repository file | SHA-256 |
| --- | --- |
| [`validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt`](../../validation-assets/en16931/1.3.16/EN16931-UBL-validation.xslt) | `39f9d282867f1a49e7708d9e29a53da89643e1ee56f10cec1ebcf1277595fcbd` |

Its exact raw source URL, tag, and acquisition date are recorded in
[`source-url.txt`](../../validation-assets/en16931/1.3.16/source-url.txt), and
the adjacent `sha256.txt` is verified by the Go test suite. The broader UBL
release archive digest above is the digest published by the GitHub release
API; the archive is not duplicated under `standard/`.

The validation artefacts are technical expressions of EN 16931, not copies of
the normative standard. Their source headers and upstream repository license
them under EUPL 1.2. Runtime redistribution details are also recorded in
[`THIRD_PARTY_NOTICES.md`](../../THIRD_PARTY_NOTICES.md).
