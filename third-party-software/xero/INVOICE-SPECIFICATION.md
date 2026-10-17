# Xero: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
The Accounting API provides `GET https://api.xero.com/api.xro/2.0/Invoices`. Request JSON explicitly. Sales invoices use `Type=ACCREC`; bills use `ACCPAY`. Unpaged collection responses may omit lines: use paged results or retrieve an individual invoice. Avoid `summaryOnly=true` for generation.

Authorisation uses OAuth 2.0. Requests carry a bearer token and `xero-tenant-id`, obtained from the authorised connections.

## Initial field mapping
| Source | Proposed shared-model target |
| --- | --- |
| InvoiceID | Source identity |
| InvoiceNumber | Invoice number |
| Date / DueDate | Issue/due dates |
| Contact | Buyer; retrieve full contact details |
| CurrencyCode | Document currency |
| LineItems | Lines; preserve quantity, price and tax semantics |
| LineAmountTypes | Tax-inclusive/exclusive interpretation |
| SubTotal / TotalTax / Total | Monetary reconciliation |

## Connection to e-UBL21 — proposed
Run an authorised connector that reads invoice detail then calls e-UBL21. Use a modified-since polling cursor with overlap and deduplication; webhook support can be assessed separately. Source invoice status must meet an agreed issue policy.

## Remaining evidence
Qualify tax-type mapping, units, seller organisation identity, VAT identifiers, bank details, buyer reference and electronic endpoint identifiers. Specify credit notes independently. Obtain authorisation scopes, current rate limits and EUR/GBP samples from a test organisation. No end-to-end connector test was performed.

## Official sources
- [Invoices and fields](https://developer.xero.com/documentation/api/accounting/invoices)
- [OAuth token types and tenant header](https://developer.xero.com/documentation/guides/oauth2/token-types/)

See the [shared research register](../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
