# Extraction verification

Verified 2026-10-02 by parsing official Schematron XML and counting every assert/report.

- BIS Billing 3.0.21: 165 assertions, split into 102 country-prefixed and 63 common assertions. Every assertion has a context, exact test and severity; global functions/namespaces and pattern/rule lets are preserved.
- 16 downloaded jurisdiction/shared packages: 9107 assertion/report occurrences across package members (not unique business rules). Identical rule assets are deduplicated by full SHA-256 and referenced from every member.
- 26 distinct jurisdiction/shared Schematron byte sequences are represented. Invoice and CreditNote packages sometimes contain identical validators; those identities remain explicit.
- No include or extends elements were found in these acquired jurisdiction/shared Schematron files. All direct global and scoped declarations are retained.
- ZIP and member SHA-256 values are recorded. Every packaged schema, code list, example and PDF is inventoried for exact acquisition, not copied as narrative text.
- Source assertion/report counts were compared with extracted rule-section counts; every source check has a section.

This completes executable-rule extraction for the listed packages, not implementation or legal certification. Planned test IDs have not been generated or regression-qualified. Wider national reporting systems and unpublished rules are outside this inventory.

[Country index](../countries/README.md) · [Package manifest](PACKAGE-MANIFEST.md)
