# Third-party accounting software

Planned integrations through which accounting packages submit invoice data to e-UBL21 for UBL generation and validation. These folders reserve space for interface specifications, mappings, examples and adapters; integrations are not implemented yet.

## Priority

1. SAP: S/4HANA, ECC and Business One (all three in initial SAP scope).
2. Sage Accounting.
3. Xero.
4. QuickBooks.
5. Sage 50 and Sage 200.
6. Microsoft Dynamics.

FreeAgent is also retained for later assessment for the UK and Ireland market.

Each product adapter will target a shared e-UBL21 invoice submission contract. Product versions, deployment variants, authentication, submission triggers, field mappings, retries, idempotency and result handling must be specified before implementation.

## Folders

- [SAP](sap/README.md)
- [Sage](sage/README.md)
- [Xero](xero/README.md)
- [QuickBooks](quickbooks/README.md)
- [Microsoft Dynamics](microsoft-dynamics/README.md)
- [FreeAgent](freeagent/README.md)

Background: [UK and Ireland invoice generation plan](../ideas/active-plans/invoice-generation-uk-ireland.md).

## Official invoice interface research

See the [research register and product specifications](INTEGRATION-RESEARCH.md) for official documentation links, verified interface details, initial mappings, access restrictions and remaining qualification work. Research reviewed on 17 October 2026; adapters are not implemented.
