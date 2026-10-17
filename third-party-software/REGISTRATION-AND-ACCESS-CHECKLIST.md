# Registration and access checklist

Checked: 17 October 2026. Scope: completing invoice-interface specifications for e-UBL21. This checklist does not register accounts or purchase subscriptions.

## What is needed per product

| Order | Product | Public work available now | Registration / entitlement | Customer-system evidence |
| --- | --- | --- | --- | --- |
| 1 | SAP S/4HANA | Help Portal billing API operations and Business Accelerator Hub catalogue | SAP account may be needed for particular downloads or sandbox features; requirements depend on chosen API. No blanket registration requirement established | Exact release/deployment, enabled service metadata, custom fields, authorised communication setup and invoices/credits |
| 1 | SAP ECC | INVOIC02 references and general integration documentation | SAP Support materials may require entitled access; a basic website account does not grant an ECC system | Export actual IDoc basic type/extensions and documentation through SAP administrator; message/partner configuration and representative outbound payloads |
| 1 | SAP Business One | Public Service Layer API reference | A generic SAP account does not provide a Business One installation; test access must be arranged separately | Version/feature pack, Service Layer metadata, permitted account, invoice/credit-note samples and custom fields |
| 2 | Sage Accounting | Public overview, OAuth guidance and invoice resource catalogue; continue extracting versioned schema | Developer application registration and business-user authorisation for API testing | Test business, country edition, tax setup, related company/contact data and samples |
| 3 | Xero | Public invoice and OAuth documentation | Developer app plus authorised organisation for API testing | Complete invoice lines, tax types, organisation/contact data and samples |
| 4 | QuickBooks Online | Public Invoice reference and authentication guidance | Intuit developer app and appropriate sandbox/company authorisation | Country-specific VAT configuration, company/customer records and invoice/credit examples |
| 5 | Sage 50 Accounts UKI | Public SDK overview and versioned download index | UKI Developer Programme entitlement for SDK tools/help; confirm applicable subscription directly with Sage | Accounts/SDO version and bitness, test company, full SDK invoice dictionary and samples |
| 5 | Sage 200 UKI | Public edition/version API references and specification download entry | Official September 2026 notice requires third-party developers to enrol in the paid Sage 200 UKI API Developer Programme and manage their own credentials | Enabled API/company access, exact edition/version, full document and line schema and samples |
| 6 | Dynamics 365 Business Central | Public invoice/header/line API v2.0 documentation | Authorised tenant/application setup for testing; documentation access does not provide a tenant | Company ID, release/extensions, API permissions, invoice and credit samples |
| 6 | Dynamics 365 Finance | Public OData/data-entity documentation | Authorised environment and application setup for testing | Actual public entity metadata, invoice header/line/tax entities and company context |
| Later | FreeAgent | Public invoices and OAuth documentation | Registered OAuth application; authorised account or separately configured sandbox | Company/contact data, nested items, tax and credit examples |

No account creation is needed merely to continue reading the public pages already identified. Failure to extract a JavaScript-rendered reference is not evidence that the document requires payment or login.

## SAP first: evidence package to request

- [ ] S/4HANA: deployment type and exact release; chosen billing API/version; exported service metadata and custom-field definitions.
- [ ] ECC: exact release; invoice message/basic type and extension; complete segment hierarchy, field dictionary and qualifier meanings. Ask the administrator for a documentation/schema export from the installed IDoc tools; do not assume every installation exports the same file format.
- [ ] Business One: feature pack, platform and Service Layer version; service metadata and user-defined fields.
- [ ] For each: de-identified issued invoice, credit note and cancellation examples, including full lines, VAT, discounts and payment references.
- [ ] Confirm outbound trigger and transport separately from data schema. No website registration supplies customer-specific configuration automatically.

## Public research still outstanding

- [ ] Retrieve full Sage Accounting invoice and credit-note schemas from its current official portal.
- [ ] Retrieve the Sage 200 full specification for the selected edition and release.
- [ ] Complete SAP billing-property and Business One Document mappings from release-matched definitions.
- [ ] Complete tax, credit-note, pagination, throttling and lifecycle details for each cloud API.
- [ ] Preserve source/version/retrieval date for permitted schema downloads.
- [ ] Verify all mappings using real test payloads before declaring a connector ready.

These remain open tasks. The ten existing research notes are initial summaries, not exhaustive vendor specifications or tested adapters.

## Sage 200 correction to earlier guidance

The official notice dated 1 September 2026 introduces paid API subscription tiers, effective 1 September for new applicants and 1 October for existing API users. It states that third-party developers must enrol and manage their own credentials; customers should not obtain credentials on their behalf. It distinguishes customer-only internal development and potentially separate fintech onboarding. Do not assume that either exception applies to e-UBL21 without checking the actual integration scope with Sage. Pricing has not been established in this checklist.

## Official entry points

- [SAP Business Accelerator Hub](https://api.sap.com/)
- [SAP S/4HANA billing API](https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/fe6f3eda2f914d81b512493fd79e9769.html)
- [SAP ERP INVOIC02 reference](https://help.sap.com/docs/SAP_ERP_SPV/349992fb60854a62a264d716ad5c8f54/e9a062543da42357e10000000a44176d.html)
- [Business One Service Layer](https://help.sap.com/doc/056f69366b5345a386bb8149f1700c19/10.0/en-US/Service%20Layer%20API%20Reference.html)
- [Sage Accounting overview](https://developer.sage.com/accounting/docs/v1.0.0/guides/learning/key-concepts/overview)
- [Sage 50 development basics](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=200518071050312)
- [Sage UKI SDK downloads/help index](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=250717132422557)
- [Sage 200 UKI programme notice, registration and pricing links](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=260901141130493&view=print)
- [Sage 200 Professional specification entry](https://developer.sage.com/200-uk/apis/sage-200-professional/2025.02/combined)
- [Xero OAuth](https://developer.xero.com/documentation/guides/oauth2/overview/)
- [Intuit authentication](https://developer.intuit.com/app/developer/qbo/docs/develop/authentication-and-authorization)
- [Business Central invoice API](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/api-reference/v2.0/resources/dynamics_salesinvoice)
- [Finance OData](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/data-entities/odata)
- [FreeAgent OAuth and sandbox guidance](https://dev.freeagent.com/docs/oauth)

See [research register](INTEGRATION-RESEARCH.md) for all product documents.
