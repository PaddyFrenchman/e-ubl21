# SAP Business One: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface
Service Layer exposes OData entities and JSON operations. SAP's current 10.0 reference targets OData V4, with `/b1s/v2/` routes; OData V3 is deprecated from FP 2405. The `Invoices` entity represents the invoice interface, with `DocumentLines` carrying lines. DI API is a separate SDK route; SAP says it shares most metadata with Service Layer.

## Connection to e-UBL21 — proposed
A local connector or Business One integration extension reads an invoice and submits mapped data to e-UBL21. Enable and authenticate against the installed Service Layer; retrieve its metadata. Do not assume its availability for every older version or deployment.

## Initial mapping candidates
| Business One field | Shared-model purpose |
| --- | --- |
| DocEntry | Source record identity |
| DocNum | Business invoice number; qualify numbering series |
| CardCode | Customer lookup |
| DocumentLines | Invoice line collection |

Resolve remaining document dates, currency, quantities, prices, VAT, legal identities, addresses, payment and reference fields using the installed `Document` metadata. These are mapping tasks, not a completed schema.

## Remaining evidence
Pin feature pack, database platform, session/login procedure and permitted integration account. Retrieve full invoice and credit-note examples, tax/rounding settings, service versus item invoices and user-defined fields. Establish incremental extraction, pagination, source revision tracking and cancellation handling. No live API request has been tested.

## Official sources
- [Service Layer API reference](https://help.sap.com/doc/056f69366b5345a386bb8149f1700c19/10.0/en-US/Service%20Layer%20API%20Reference.html)
- [About Service Layer](https://help.sap.com/docs/SAP_BUSINESS_ONE/f110a154dd0f4c20bf7f3ebca9eeb794/60c7a0b745bd486589f05a1da77041f3.html)

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
