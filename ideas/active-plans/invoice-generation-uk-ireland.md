# e-UBL21: Invoice generation and accounting integrations

Date: 16 October 2026
Status: Proposed feature scope; no implementation issues created by this summary.

## Objective

Generate actual electronic invoices from business data, initially targeting Ireland and the United Kingdom. This is separate from generating synthetic invoices for regression testing. Production deployment is outside the current milestone.

## Architecture

1. Retrieve business invoice data through a product-specific connector or accept a documented JSON payload.
2. Map it into a common invoice model.
3. Check required information, arithmetic, tax mappings and profile constraints.
4. Generate UBL XML.
5. Run the generated document through the existing applicable validation pipeline.
6. Return the document and actionable diagnostics.

Keep connectors separate from serialization and validation. Do not silently invent missing business data or treat a vendor tax code as a universal VAT category.

## Interfaces versus invoice syntaxes

SAP, Sage and the other vendors are software platforms, not single invoice syntaxes. Each product and version can expose different APIs, exports and data models. A CSV file is also not a universal interface: its columns and meanings require a versioned mapping.

UBL is the initial output syntax. EN 16931 defines the semantic requirements; it is not itself an XML syntax. CII is a potential later output syntax. Peppol BIS Billing adds profile requirements; producing conformant XML does not by itself provide Peppol network delivery.

## Initial connector matrix

| Product | Interface to assess | Proposed phase | Qualification needed |
| --- | --- | --- | --- |
| Generic custom integration | Versioned JSON input | 1 | Publish schema, required fields and error contract |
| Sage Accounting | Accounting API, sales invoice resources | 1 | Country edition, authentication, invoice/credit-note data and tax mappings |
| Xero | Accounting API, Invoices | 1 | Organisation access, tax types, invoice states and credit-note mapping |
| QuickBooks Online | Accounting API, Invoice | 1 | UK/Ireland availability, company configuration, tax and credit-memo mapping |
| Sage 50 Accounts | Product SDK/developer tools; assess supported exports separately | 2 | UK developer programme access, supported versions and local connector requirements |
| Sage 200 | Product-specific API | 2 | Edition/version and distinction between sales documents and posted ledger transactions |
| Dynamics 365 Business Central | Standard API v2.0 salesInvoice resources | 2 | Posted/unposted lifecycle, line access and extension fields |
| SAP Business One | Service Layer, OData | 2 | Version, invoice entities, deployment access and custom fields |
| SAP S/4HANA | Billing-document APIs, including OData | 2 | Exact release/API, extraction scope, communication configuration and extensions |
| FreeAgent | Invoice API | Candidate follow-up | Data completeness, credit notes, authentication and demand |

Phases express engineering recommendations, not verified market-share rankings. SAP Business One and S/4HANA require separate adapters; Sage Accounting, Sage 50 and Sage 200 do too. Additional platforms should be assessed against actual customer demand before implementation.

## Common invoice model

The model should capture seller/buyer identity and addresses, tax identifiers, invoice number and dates, currency, payment details, references, invoice lines, quantities and units, prices, allowances/charges, tax breakdowns and monetary totals. Preserve source identifiers and provenance. Support credit notes explicitly rather than assuming a negative invoice is equivalent.

Record missing or ambiguous mappings as diagnostics. Define decimal precision and rounding rules explicitly. Keep EUR/GBP and Ireland/UK tax scenarios in the acceptance corpus; determine Northern Ireland and cross-border treatment from the actual transaction rather than country labels alone.

## Proposed milestones

1. Define the canonical model, JSON schema and mapping/error contracts.
2. Deliver JSON-to-UBL generation with validation and representative business examples.
3. Add Sage Accounting, Xero and QuickBooks Online independently.
4. Expand to Sage 50/200, Business Central and SAP according to access and demand.
5. Assess CII, further profiles and delivery integrations as separate features.

For each connector, confirm exact product/version, supported operations, authentication, access/licensing requirements, available fields, pagination/rate limits, sample payloads and mapping gaps before creating implementation issues. Start with reading existing invoice data; writing invoices back to accounting platforms requires a separate scope decision.

## Existing related work

Existing issues cover test-data generation: #35 (EN 16931), #37 (Peppol samples), and #160–#167 (structural corpus expansion and qualification). They are not specifications for business-data invoice generation. No dedicated issue for that new capability was found in the searches performed during this discussion. Existing generator code may be reusable, subject to inspection.

## Market and standards context

Revenue's Irish eInvoicing material specifies structured invoices conforming to EN 16931 for the relevant rollout. UK public-procurement guidance also addresses standards-compliant electronic invoices. These establish standards context, not a claim that every Irish or UK business currently has the same obligations. Verify current scope and dates when defining compliance features.

## Sources

Official documentation consulted during the discussion:

- [Sage Accounting sales transactions API](https://developer.sage.com/accounting/apis/sagebusinesscloudaccounting/3.0.1/accounting/groups/sales-transactions)
- [Sage 200 sales invoices API](https://developer.sage.com/200-uk/apis/sage-200-professional/2024.02/combined/groups/sales/tags/sales-invoices) — illustrates product-specific posting semantics; select the supported version during qualification.
- [Sage 50 developer access and basics](https://gb-kb.sage.com/portal/app/portlets/results/viewsolution.jsp?solutionid=200518071050312)
- [Xero Invoices API](https://developer.xero.com/documentation/api/accounting/invoices)
- [QuickBooks Online Invoice API](https://developer.intuit.com/app/developer/qbo/docs/api/accounting/all-entities/invoice)
- [Business Central salesInvoice resource, API v2.0](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/api-reference/v2.0/resources/dynamics_salesinvoice)
- [SAP Business One Service Layer reference](https://help.sap.com/doc/056f69366b5345a386bb8149f1700c19/10.0/en-US/Service%20Layer%20API%20Reference.html)
- [SAP S/4HANA billing-document API](https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/fe6f3eda2f914d81b512493fd79e9769.html)
- [FreeAgent invoices API](https://dev.freeagent.com/docs/invoices)
- [Irish Revenue: Phase One of VAT modernisation](https://www.revenue.ie/en/vat/vida-vat-modernisation/large-corporates-vat-modernisation.aspx)
- [UK guidance: electronic invoicing and payment](https://www.gov.uk/government/publications/procurement-act-2023-guidance-documents-manage-phase/guidance-electronic-invoicing-and-payment-html)

API availability is supported by vendor documentation. Priorities, architecture and milestones are proposals. Detailed field coverage, commercial access and end-to-end interoperability remain to be qualified.
