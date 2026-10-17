# Sage 50 Accounts — UK/Ireland: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
Sage's Development Basics describes Sage Data Objects (SDO), an ActiveX DLL SDK with methods and properties for accessing Sage 50 Accounts data, including invoices and credits. Developer tools are restricted to UK Sage Developer Programme members. The UKI downloads page supplies version-specific SDO installers and compiled SDK help files.

This is not the Sage Accounting cloud REST API, nor the US/Canadian Sage 50 product.

## Connection to e-UBL21 — proposed
Use a Windows-side integration agent compatible with the installed SDO version and bitness. Read the invoice through SDO, transform it to the shared model and submit over HTTPS. Keep SDO interoperation outside the core Go invoice generator.

## Specification still needed
The public overview establishes the integration mechanism, but not a full invoice schema. Obtain the correct SDK help file and enumerate header/line objects, field names, types, lengths, numeric precision, tax code semantics and credit-note behaviour. Do not fabricate object names from other country editions.

## Required qualification
| Concern | Evidence |
| --- | --- |
| Compatibility | Accounts release, SDO DLL/version and process bitness |
| Access | Programme/SDK entitlement and company permissions |
| Completeness | Invoice header, all item lines, customer and company details |
| Extraction | Posted/draft distinction, change detection and locking |
| Alternatives | Exact supported export layout if a file route is chosen |

Full field mapping and sample payloads remain blocked on the version-specific SDK documentation and a test company. No SDK was downloaded or executed.

## Official sources
- [Development Basics](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=200518071050312)
- [UKI developer downloads and SDK documentation](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=250717132422557)
- [SDK Essentials](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=200619125740118)

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
