# Peppol country resources for Codex

Extraction snapshot: 2026-10-02. Country-specific inventory: 16 countries, plus the shared EU profile. This is not a geographic restriction on standard BIS Billing use.

All assertions/reports in the acquired Schematron packages are extracted, with contexts, tests, scoped/global definitions, severities and source checksums. Full narrative standards are not reproduced. Implementation, generated fixtures and regression qualification remain separate work.

| Country | Resource |
| --- | --- |
| Australia | [AU](AU.md) |
| New Zealand | [NZ](NZ.md) |
| Japan | [JP](JP.md) |
| Malaysia | [MY](MY.md) |
| Singapore | [SG](SG.md) |
| Oman | [OM](OM.md) |
| United Arab Emirates | [AE](AE.md) |
| Germany | [DE](DE.md) |
| Denmark | [DK](DK.md) |
| Greece | [GR](GR.md) |
| Iceland | [IS](IS.md) |
| Italy | [IT](IT.md) |
| Netherlands | [NL](NL.md) |
| Norway | [NO](NO.md) |
| Sweden | [SE](SE.md) |
| Belgium | [BE](BE.md) |

[Shared EU resources](EU.md) · [Extraction verification](../extracted/VERIFICATION.md) · [Package manifest](../extracted/PACKAGE-MANIFEST.md)

Country determination is rule-specific. For this BIS release, generic supplierCountry uses VAT prefix, tax-representative VAT prefix, then postal country; Germany has separate postal-country booleans. Never substitute a single country heuristic for all rules.
