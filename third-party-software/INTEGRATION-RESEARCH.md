# Accounting integration research register

Reviewed: 17 October 2026. Scope: invoice data flowing from accounting packages into e-UBL21 for generation and validation. Production deployment is excluded.

## Product specifications

| Priority | Product | Research document | Evidence status |
| --- | --- | --- | --- |
| 1 | SAP S/4HANA | [Specification](sap/s4hana/INVOICE-SPECIFICATION.md) | Billing API operations located; deployment metadata needed |
| 1 | SAP ECC | [Specification](sap/ecc/INVOICE-SPECIFICATION.md) | INVOIC02 reference located; full customer IDoc dictionary outstanding |
| 1 | SAP Business One | [Specification](sap/business-one/INVOICE-SPECIFICATION.md) | Service Layer reference reviewed; installed metadata needed |
| 2 | Sage Accounting | [Specification](sage/accounting/INVOICE-SPECIFICATION.md) | API/auth/catalogue verified; full invoice schema not extracted |
| 3 | Xero | [Specification](xero/INVOICE-SPECIFICATION.md) | Invoice and token/header documentation reviewed |
| 4 | QuickBooks Online | [Specification](quickbooks/INVOICE-SPECIFICATION.md) | Invoice operations/fields reviewed; local VAT qualification needed |
| 5 | Sage 50 Accounts UKI | [Specification](sage/sage-50/INVOICE-SPECIFICATION.md) | SDK mechanism verified; versioned field help requires developer access |
| 5 | Sage 200 UKI | [Specification](sage/sage-200/INVOICE-SPECIFICATION.md) | API catalogue/line views located; exact edition/schema needed |
| 6 | Dynamics 365 | [Specification](microsoft-dynamics/INVOICE-SPECIFICATION.md) | Business Central invoice API reviewed; Finance metadata outstanding |
| Later | FreeAgent | [Specification](freeagent/INVOICE-SPECIFICATION.md) | Invoice and OAuth documentation reviewed |

These are original summaries with official source links, not copies of vendor manuals. Some vendor sites render only JavaScript shells through public extraction; discovery of a page is distinguished from inspection of its field schema. No customer credentials, paid SDK access, live account or sandbox was used. A missing public field schema does not establish that the vendor has no specification.

## Shared architecture — proposed, not implemented

Accounting product extension, middleware or connector -> common invoice submission model -> UBL generation -> existing applicable validation -> result returned to caller.

Vendor APIs generally expose their own data; this does not establish that the accounting product natively pushes to e-UBL21. Each adapter needs an explicit trigger (extension, supported event or polling). Do not prescribe an e-UBL21 URL as existing until the actual OpenAPI contract is implemented.

## Mapping requirements

| Information | Normalisation requirement |
| --- | --- |
| Source product, company, record ID and revision | Preserve provenance; distinguish business number from internal key |
| Invoice/credit-note type and references | Explicit document type; preserve preceding invoice link |
| Issue, tax-point and due dates | Separate dates; preserve date/time semantics |
| Seller/buyer legal names, addresses and IDs | Enrich from company/contact resources; retain identifier schemes |
| Currency and amounts | Exact decimal arithmetic, currency codes and documented rounding |
| Lines, quantities, units and prices | Map unit codes and price bases; preserve line identity |
| Allowances and charges | Distinguish line/document scope and tax treatment |
| VAT rate, category and exemption | Versioned product/country tax map; never infer category from rate alone |
| Net, tax, gross, prepaid and payable totals | Reconcile source totals before serialisation |
| Payment means, account and terms | Obtain actual business values; do not fabricate |
| Buyer/order/contract references | Preserve meaning rather than copying arbitrary notes |
| Electronic addresses and profile identifiers | Require explicit values and schemes when target profile requires them |

Missing mandatory information must produce actionable diagnostics. Vendor invoice validity does not establish EN 16931 or Peppol conformity. A PDF is useful as a comparison artefact, not a substitute for structured source fields.

## Delivery and reliability requirements — proposed

- Bind each connection to the correct customer/company; protect credentials and refresh tokens.
- Read complete headers and all lines; follow vendor pagination.
- Use source revision plus product/company/record identity for deduplication; retain a content hash and mapping version.
- Persist extraction checkpoints only after durable handling. Use overlap windows and reconciliation for polling; a webhook is a change signal until completeness is established.
- Respect vendor throttling and Retry-After when present. Retry transient failures; report permanent mapping/authentication errors separately.
- Keep source state separate from e-UBL21 generation, validation and any future network-delivery state.
- Do not create, post, email, cancel or modify source invoices as a side effect of extraction.
- Return source-linked diagnostics and generated-document identity to the submitting integration.

## Before implementing each adapter

Obtain the exact edition/release and machine-readable schema or SDK dictionary, field types/lengths/cardinalities, authentication/permissions, rate limits, paging, triggers and lifecycle documentation. Save permitted schema exports with retrieval date and version; obtain de-identified customer samples. These artefacts have not yet been collected in this pass.

Qualify standard VAT, zero-rated and exempt cases, mixed rates, discounts, charges, rounding, credit notes, prepaid balances, foreign currency, missing identifiers, amended/cancelled documents, repeated delivery and throttling/retry behaviour. Include EUR and GBP cases where supported. Confirm actual country and transaction tax treatment, including Northern Ireland scenarios.

Acceptance requires mapping evidence, generated XML passing the applicable validators and reconciliation against source totals. Documentation research alone is not connector acceptance.
