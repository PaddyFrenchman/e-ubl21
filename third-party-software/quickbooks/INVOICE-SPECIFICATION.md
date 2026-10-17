# QuickBooks Online: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
Intuit documents `GET /v3/company/{realmID}/invoice/{invoiceId}` on `https://quickbooks.api.intuit.com`. The Invoice entity supports read/query operations and JSON. OAuth 2.0 authorises access to the chosen company. QuickBooks Desktop is outside this specification.

## Initial field mapping
| Source | Proposed shared-model target |
| --- | --- |
| Id | Source identity, scoped by realmID |
| DocNumber | Invoice business number |
| TxnDate | Issue date |
| CustomerRef | Buyer lookup |
| BillAddr / ShipAddr | Billing/delivery address |
| CurrencyRef | Document currency when applicable |
| Line | Lines; interpret DetailType before mapping |
| TotalAmt | Gross total for reconciliation |
| SyncToken | Source concurrency version; not invoice number |

The vendor requires CustomerRef and at least one suitable line when creating invoices. These creation rules do not establish that every stored invoice contains all EN 16931 or Peppol data.

## Connection to e-UBL21 — proposed
An authorised connector reads invoice and dependent customer/company/tax resources, then submits the common model. Do not call Intuit's create/send operations as part of extraction.

## Remaining evidence
Pin UK/Ireland company availability and tax configuration. Qualify non-US VAT fields, inclusive/exclusive amounts, discounts, credit memos, voiding, units, seller VAT and payment details. Verify query pagination, change notification/reconciliation, scopes and rate limits. Test using a country-appropriate sandbox; none was accessed in this review.

## Official sources
- [Invoice reference](https://developer.intuit.com/app/developer/qbo/docs/api/accounting/all-entities/invoice)
- [Authentication](https://developer.intuit.com/app/developer/qbo/docs/develop/authentication-and-authorization)
- [Official OAuth playground guidance](https://help.developer.intuit.com/s/article/Configuring-the-Oauth-2-0-Playground-for-QuickBooks-Online)

See the [shared research register](../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
