# Sage Accounting: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface and evidence limits
The official Accounting API overview confirms OAuth 2.0 with business-user authorisation, and API V3/V3.1. The official Sales Transactions reference lists Sales Invoices and Sales Credit Notes. Its current portal rendered only a title during direct extraction; search indexing exposed the resource catalogue. A complete invoice property schema was not extracted.

Sage Active and Embedded Accounting have different APIs. Their invoice schemas must not be substituted for Sage Accounting UK/Ireland.

## Connection to e-UBL21 — proposed
An authorised connector reads existing sales invoices, retrieves related contact/business/tax data, and submits the shared invoice model. Use polling initially unless a supported notification mechanism is separately verified. No native direct submission to e-UBL21 has been demonstrated.

## Mapping requirements
| Data | Qualification needed |
| --- | --- |
| Invoice identity, dates, currency | Exact response properties and read routes |
| Customer and seller | Contact plus business details |
| Lines | Description, quantity, unit price, discounts |
| Tax | Rate IDs to explicit VAT category/rate/exemption |
| Totals and payment | Net, VAT, gross, amount due, bank details |
| Credits | Separate credit-note resource and invoice reference |

## Remaining evidence
Download the version-specific official API specification from the portal; pin the API base/version and country edition. Confirm business-selection headers, token refresh, pagination, modified-date filters, invoice lifecycle and rate limits from that specification before coding. Obtain a test business, authorised app and representative EUR/GBP invoices. No live access or field-level conformance testing was performed.

## Official sources
- [API overview and authentication](https://developer.sage.com/accounting/docs/v1.0.0/guides/learning/key-concepts/overview)
- [Sales Transactions reference](https://developer.sage.com/accounting/apis/sagebusinesscloudaccounting/3.0.1/accounting/groups/sales-transactions)

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
