# Sage 200 — UK/Ireland: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
Sage publishes separate UKI API references for Standard and Professional. The Professional reference has Sales Invoices, Sales Posted Transactions, and Customer SOP Invoice Credit Line Views. Posted transactions represent ledger postings; they are not necessarily complete itemised invoice documents. The line-view resource joins customers, invoice/credit data, lines and products.

The documentation includes a downloadable full specification. Pin the customer edition and release rather than silently combining examples across versions. Sage 200 Spain is a different contract.

## Connection to e-UBL21 — proposed
An authorised API connector retrieves the document plus its complete lines, enriches seller/customer/tax details and submits to e-UBL21. Confirm API provisioning and company selection with the customer. No built-in direct e-UBL21 submission has been established.

## Mapping and outstanding questions
| Area | Required evidence |
| --- | --- |
| Document header | Invoice number/date/currency and posted state |
| SOP lines | Complete invoice/credit item data and stable join keys |
| Ledger-only invoices | Whether descriptions/quantities are available at all |
| Taxes | Breakdown, tax codes, discounts and rounding |
| Company/customer | Legal names, addresses, VAT and bank details |

Do not use an API that posts a sales ledger transaction as evidence that a complete sales invoice can be read. Obtain the edition-specific OpenAPI schema, authentication/header requirements, pagination, supported filters, change tracking and representative invoices/credits. No field-complete mapping or test connection was performed.

## Official sources
- [UKI overview](https://developer.sage.com/200-uk)
- [Professional 2025.02 full API specification entry](https://developer.sage.com/200-uk/apis/sage-200-professional/2025.02/combined)
- [Sales Invoices, 2024.02](https://developer.sage.com/200-uk/apis/sage-200-professional/2024.02/combined/groups/sales/tags/sales-invoices)
- [Customer SOP invoice/credit line views, 2024.02](https://developer.sage.com/200-uk/apis/sage-200-professional/2024.02/combined/groups/sales/tags/customer-sop-invoice-credit-line-views)

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.

## Registration and system access

See the [registration and access checklist](../../REGISTRATION-AND-ACCESS-CHECKLIST.md). Sage 200 third-party API development requires enrolment in its paid UKI API Developer Programme under the official September 2026 notice; public documentation access alone does not provide API credentials.
