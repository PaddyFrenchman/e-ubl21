# FreeAgent: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
The API exposes `GET https://api.freeagent.com/v2/invoices` and individual invoice retrieval. Use `nested_invoice_items=true` to include items in collection responses. The documented minimum access level is Estimates and Invoices. OAuth 2.0 uses user authorisation with access/refresh tokens; a sandbox is documented separately.

## Initial field mapping
| Source | Proposed shared-model target |
| --- | --- |
| url | Stable source invoice identity |
| reference | Invoice number/reference |
| dated_on / due_on | Issue/due date |
| contact | Buyer resource link |
| currency / exchange_rate | Document currency and exchange data |
| invoice_items | Lines |
| net_value / sales_tax_value / total_value | Totals |
| due_value / paid_value | Outstanding/paid values |
| po_reference | Purchase-order reference when present |

## Connection to e-UBL21 — proposed
An authorised connector polls with `updated_since`, obtains complete invoice items and related company/contact data, then submits to e-UBL21. Define eligible statuses explicitly; draft extraction must not imply issuance.

## Remaining evidence
Qualify item descriptions, quantities, prices, unit codes, VAT exemptions, discounts, credit-note treatment, seller identifiers and payment details. Confirm pagination, rate limits and current token behaviour before implementation. Obtain a representative GBP case and establish EUR/Irish suitability from actual customer needs. No live API or sandbox was accessed.

## Official sources
- [Invoices](https://dev.freeagent.com/docs/invoices)
- [OAuth and sandbox](https://dev.freeagent.com/docs/oauth)

See the [shared research register](../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
