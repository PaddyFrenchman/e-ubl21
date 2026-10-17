# SAP S/4HANA: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
SAP documents the OData V2 service `API_BILLING_DOCUMENT_SRV` for reading billing documents, cancelling them and retrieving PDFs. The header entity is `A_BillingDocument`; item and item-text entities are separate. SAP also documents an OData V4 Billing Document API. These are different contracts: pin one API and release before implementation.

Example documented read:
```http
GET {host}/sap/opu/odata/sap/API_BILLING_DOCUMENT_SRV/A_BillingDocument(BillingDocument='90002930')
```

## Connection to e-UBL21 — proposed
A customer-side SAP extension or middleware reads the issued billing document, enriches it, then submits the common e-UBL21 contract. API availability does not establish that SAP automatically calls e-UBL21. Authentication, communication arrangements and outbound trigger depend on Public Cloud, Private Cloud or on-premise deployment.

## Mapping work
| Source area | Target information | Qualification |
| --- | --- | --- |
| Billing header | Invoice identifier, date, currency and totals | Pin exact properties from service metadata |
| Items | Description, quantity, unit, price | Include item texts where needed |
| Partners | Seller, buyer and addresses | Resolve legal seller and VAT identifiers |
| Pricing/tax | Allowances, charges and VAT breakdown | Explicit condition/tax-code mapping |
| References/payment | Purchase order and payment details | Enrich if absent |

## Remaining evidence
Obtain customer `$metadata`, release, enabled entities, sample invoice and credit note, custom fields and communication configuration. Prove pagination and change detection; do not assume all financial invoices are SD billing documents. No customer system or sandbox was tested.

## Official sources
- [V2 service](https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/fe6f3eda2f914d81b512493fd79e9769.html)
- [V2 operations](https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/a9120b013c9a44efaf417e1a66704901.html)
- [V4 Billing Document](https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/65680dabb62546029175655482f85edc.html)

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
