# Country resource index for Codex
Checked: 2026-10-02
Status: generation in progress; full specification extraction is not complete.

## Current files
- [Australia](AU.md)
- [New Zealand](NZ.md)
- [Japan](JP.md)
- [Malaysia](MY.md)
- [Singapore](SG.md)
- [Oman](OM.md)
- [United Arab Emirates](AE.md)

JP, MY, SG, AU and NZ now include verified rule/applicability anchors and direct rule-catalogue links. OM and AE currently contain profile overviews. EU/BIS national-rule country annexes and additional national profiles remain outstanding.

## Shared applicability requirement
The official BIS national-rule methodology prioritises the seller VAT country prefix, then the tax-representative VAT country prefix, then seller address country. Do not replace this with a supplier postal-country-only check. Specific pinned XPath expressions remain authoritative.
PINT jurisdiction selection also depends on specification identity and receiver capabilities. Do not apply BIS methodology universally to every PINT profile.

## Generation completion gates
Extract every applicable rule and dependency from pinned official assets; document expression/severity and version; map valid/invalid/other-country fixtures through Createsamples; verify licence and source hashes. Account for unresolved profiles explicitly.
This index and the country overviews are not proof of complete coverage.

[Official national-rule methodology](https://docs.peppol.eu/poacc/billing/3.0/national-rules/)
[Worldwide plan](../../country-support.md)
