# Microsoft Dynamics 365: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Business Central — verified interface
API v2.0 exposes `companies({id})/salesInvoices` and `salesInvoiceLines`, using JSON and bearer authentication. The invoice resource includes separate navigation for lines, customer, currency, payment terms and attachments.

| Header field | Proposed shared-model target |
| --- | --- |
| id | Source GUID |
| number | Invoice number |
| invoiceDate / dueDate | Issue/due date |
| customerPurchaseOrderReference | Purchase-order reference |
| customerId / customerName | Buyer lookup/name |
| billToAddressLine1 and related fields | Buyer billing address |
| salesInvoiceLines navigation | Complete item lines |

The resource has posting and sending actions. Extraction must not invoke these actions. Qualify posted/unposted identity and lifecycle before selecting exportable documents.

## Finance — separate adapter
Finance and Operations exposes public data entities through OData V4 at `{organisation}/data`; the service's `$metadata` defines exposed entity contracts and keys. The protocol documentation alone does not establish a complete posted-customer-invoice entity set. Discover actual header, line and tax entities in the target environment; never reuse Business Central routes.

## Connection to e-UBL21 — proposed
Use a product-specific connector or configured extension to submit the mapped common model. For each product, qualify Entra authorisation, company boundaries, API permissions, pagination, change detection, tax/discount semantics, legal seller and credit-note mapping.

## Remaining evidence
Obtain Business Central header/line examples and Finance environment metadata. Record exact product/release, extensions and country localisation. No tenant was accessed and neither adapter is implemented.

## Official sources
- [Business Central invoice](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/api-reference/v2.0/resources/dynamics_salesinvoice)
- [Business Central invoice lines](https://learn.microsoft.com/en-us/dynamics365/business-central/dev-itpro/api-reference/v2.0/resources/dynamics_salesinvoiceline)
- [Finance OData](https://learn.microsoft.com/en-us/dynamics365/fin-ops-core/dev-itpro/data-entities/odata)

See the [shared research register](../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
