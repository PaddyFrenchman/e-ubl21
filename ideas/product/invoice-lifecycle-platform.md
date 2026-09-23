# Invoice Lifecycle Platform: Conversion, Routing, Exchange and Payments

## Strategic product direction

The long-term commercial product should evolve beyond an e-invoice validator into a **bidirectional invoice conversion, routing and exchange platform**, and later into an **invoice-to-payment lifecycle platform**.

The product should eventually answer one operational question:

> **Give us the invoice in the format your system already produces. We convert it into the format the destination requires, validate it, route it, deliver it, track it, receive the response, and do the reverse for incoming invoices. Later, we can also initiate or request payment, track settlement, reconcile the payment and produce the appropriate receipt/remittance evidence.**

Validation remains essential, but it becomes the **trust core** inside a larger invoice-processing platform.

The product therefore evolves around six primary capabilities:

```text
CONVERT
VALIDATE
ROUTE
SEND / RECEIVE
RECONCILE
PAY / RECEIPT
```

The commercial destination is not merely an e-invoice API. It is an **invoice lifecycle infrastructure layer for ERP, accounting, billing, EDI and vertical-SaaS products**.

---

# Product evolution at a glance

```text
PHASE 0
Validation trust core
    |
    v
PHASE 1
Canonical invoice model + generation
    |
    v
PHASE 2
Multi-format conversion
    |
    v
PHASE 3
Routing + delivery + receiving
    |
    v
PHASE 4
Bidirectional ERP / EDI exchange
    |
    v
PHASE 5
Payment requests + settlement tracking
    |
    v
PHASE 6
Automatic reconciliation + receipts/remittance
    |
    v
PHASE 7
Possible deeper payment-service integration
```

The implementation should preserve the existing rule that standards validation remains deterministic, versioned and independently testable even as side-effecting routing and payment workflows are added around it.

---

# Marketing target

## Primary ideal customer profile

The initial commercial target should be:

> **European ERP, accounting, billing and vertical-SaaS vendors that need to add multi-country e-invoicing and invoice exchange to an existing product without building an internal standards and network-integration team.**

Typical characteristics:

| Attribute | Target |
| --- | --- |
| Company type | ERP, accounting, billing or vertical business software vendor |
| Size | Approximately 20–500 employees |
| Engineering | Own development/integration team |
| Geography | Serves or intends to serve two or more European countries |
| Customer base | Hundreds to tens of thousands of business customers |
| Current problem | Existing invoice model does not naturally produce every required EU syntax/profile/network |
| Buying trigger | Customer or regulatory demand for Peppol, EN 16931, national mandates or invoice automation |
| Buyer | CTO, VP Engineering, Head of Product, Integration Lead |
| Desired solution | API/OEM infrastructure rather than a consulting project |

This segment has a repeatable problem, technical buyers, recurring document volume and meaningful switching costs once the service is embedded.

## Secondary target: EDI and enterprise integration

A second, distinct target should be:

> **EDI integration companies and enterprises with established EDIFACT/X12 infrastructure that need a bridge into modern European e-invoicing.**

Marketing message:

> **Keep your EDI. Add European e-invoicing.**

The customer should not need to replace a mature EDI estate merely because customers or regulators now require EN 16931, Peppol or national formats.

The platform can translate between legacy B2B formats and modern e-invoice networks in both directions.

## Later target: finance operations

Once delivery, payment and reconciliation exist, the buyer set expands to:

- finance systems vendors;
- accounts-receivable platforms;
- accounts-payable platforms;
- treasury/cash-management software;
- payment-service and banking integration providers;
- larger enterprises seeking one invoice/payment orchestration layer.

At that point the value proposition shifts from pure compliance to **invoice-to-cash and procure-to-pay automation**.

---

# Product positioning

## Initial validation wedge

> **One API to tell you whether a European e-invoice will pass.**

This remains a useful developer-led entry product and lead generator.

## Conversion platform positioning

> **One conversion layer between your software and European e-invoicing.**

Subheading:

> Send the invoice data or business format your application already produces. Generate and validate the compliant e-invoice required by the destination.

## Exchange platform positioning

> **Send us the invoice once. We convert it, validate it and deliver it through the right network. Incoming invoices arrive back in the format your software already understands.**

## EDI positioning

> **Keep EDIFACT or X12 internally. Bridge it to and from Peppol and European e-invoicing formats.**

## Long-term invoice-to-payment positioning

> **From invoice creation to delivery, payment and reconciliation through one API.**

The platform should ultimately make regulatory and network complexity disappear behind a stable application contract.

---

# Core architectural principle: canonical invoice semantics

## Do not make UBL the universal internal representation

UBL remains a first-class supported syntax, but it should not become the universal internal business model.

Introduce a **canonical semantic invoice model** closely aligned with EN 16931 business terms and groups, with controlled extension points for national regimes, networks and trading-partner information.

Every external syntax becomes an adapter around that model.

```text
                           INPUT ADAPTERS
                                |
       +------------------------+-------------------------+
       |            |           |           |             |
      JSON         UBL         CII       EDIFACT        X12
       |            |           |         INVOIC         810
       |            |           |           |             |
       +------------------------+-------------------------+
                                |
                                v
                    +-------------------------+
                    | Canonical invoice model |
                    | EN 16931 aligned        |
                    +-------------------------+
                                |
                     normalize / enrich
                                |
               +----------------+----------------+
               |                                 |
               v                                 v
          VALIDATION                         GENERATION
               |                                 |
               +----------------+----------------+
                                |
                                v
                             ROUTING
                                |
                  +-------------+-------------+
                  |             |             |
               Peppol        national       API/SFTP/
                 AP           network        webhook
```

This avoids a combinatorial explosion of point-to-point transforms. Instead:

```text
source format -> canonical model -> target format
```

---

# Input directions

| Input | Feasibility | Commercial value | Priority |
| --- | --- | --- | --- |
| Canonical JSON API | Very high | Very high | 1 |
| UBL 2.1 | Existing core | Very high | 1 |
| CII | High | High | 2 |
| CSV / flat file | Very high | Medium-high | 2 |
| EDIFACT INVOIC | High | Very high for EDI segment | 2 |
| ANSI X12 810 | High | High for multinational/US-linked customers | 3 |
| SAP IDoc | Medium-high | Very high | 3 |
| Dynamics / NetSuite / ERP APIs | Medium | High | Later |
| PDF/OCR extraction | Medium-low | Large market but ambiguous semantics | Much later |

## Canonical JSON first

The first generation API should allow an ERP or SaaS vendor to submit normalized invoice data and request a destination/profile such as:

```text
target = peppol-bis-billing
```

The platform then generates and validates the required invoice syntax.

This forces the canonical model and adapter contract to be correct before broader EDI support is attempted.

---

# EDI conversion feasibility

## Example: EDIFACT INVOIC to Peppol

```text
EDIFACT INVOIC
      |
      v
parse
      |
      v
trading-partner mapping profile
      |
      v
canonical invoice
      |
      v
enrichment / master data
      |
      v
Peppol UBL generation
      |
      v
UBL XSD -> EN 16931 -> Peppol validation
      |
      v
recipient resolution
      |
      v
certified Peppol Access Point
      |
      v
recipient
```

Technical feasibility is **high**.

Typical EDIFACT INVOIC concepts map naturally to invoice semantics:

```text
BGM    invoice identity
DTM    dates
NAD    buyer/seller parties
CUX    currency
LIN    invoice lines
QTY    quantities
PRI    prices
TAX    taxes
MOA    monetary amounts
```

The main difficulty is not parsing EDIFACT. It is **semantic mapping and trading-partner variation**.

Therefore the product needs:

```text
EDI parser
   +
versioned mapping profile
   +
customer/trading-partner master data
   +
canonical invoice
```

not merely:

```text
EDI parser -> UBL
```

## Enrichment is a product feature

An inbound invoice may lack information needed by the destination profile or network.

A controlled trading-partner configuration can provide information such as VAT identifiers, Peppol endpoint identifiers, endpoint schemes, target profiles and approved mapping defaults.

The platform must never invent legally or semantically required data. Missing required information should either be supplied from approved master data or fail conversion/validation with a precise diagnostic.

---

# Output directions

| Output | Feasibility | Commercial value / comment |
| --- | --- | --- |
| EN 16931 UBL | Very high | Natural first generator |
| Peppol BIS Billing UBL | Very high | Natural extension of current roadmap |
| EN 16931 CII | High | Practical once canonical model exists |
| XRechnung | High | Profile-specific generation/validation |
| Factur-X / ZUGFeRD | Medium-high | Requires CII plus PDF/A packaging |
| Human-readable PDF | High | Presentation copy, not canonical compliance representation |
| EDIFACT INVOIC | High | Valuable reverse bridge |
| ANSI X12 810 | High | Valuable reverse bridge |
| Canonical JSON | Very high | Receiving/integration API |
| Webhook / REST callback | Very high | Delivery and status integration |

---

# Bidirectional exchange

The system should treat outbound and inbound flows as equal product capabilities.

## Outbound

```text
ERP / EDIFACT / JSON
        |
        v
 invoice conversion platform
        |
        v
 target compliant invoice
        |
        v
 Peppol / national network / direct channel
        |
        v
 recipient
```

## Inbound

```text
supplier
   |
   v
Peppol / national network / direct channel
   |
   v
invoice conversion platform
   |
   v
canonical invoice
   |
   +--> EDIFACT
   +--> X12
   +--> JSON
   +--> ERP-specific format
   |
   v
customer ERP / accounts payable
```

This lets customers adopt modern European e-invoicing without replacing their internal ERP/EDI model.

---

# Routing and delivery

Routing should be a first-class subsystem rather than hard-coded transport logic.

```text
INGEST
  |
PARSE
  |
NORMALIZE
  |
ENRICH
  |
GENERATE
  |
VALIDATE
  |
RESOLVE DESTINATION
  |
ROUTE
  |
DELIVER
  |
ACK / STATUS
```

The router may consider seller identity, buyer identity, country, requested profile, Peppol participant capability, customer routing policy, national platform requirements, direct partner configuration, fallback channel and retry policy.

## Keep delivery state separate from validation state

A valid invoice can fail delivery. Therefore do not flatten everything into one status.

Use separate state dimensions such as:

```text
validation_status
conversion_status
delivery_status
payment_status
reconciliation_status
```

---

# Do not initially become a Peppol Access Point

The first routing implementation should integrate with one or more existing certified Peppol Access Points through stable connector interfaces.

```text
platform
   |
   | AP connector API
   v
certified Peppol Access Point
   |
   v
Peppol network
```

Operating an Access Point introduces certification, membership, PKI certificates, AS4 transport, operational guarantees and network-level security obligations.

It may become strategically attractive later, but it should not block the conversion and routing product.

The routing layer must therefore abstract transport providers from day one.

---

# Later extension: payments

Payments should be considered a **later invoice-lifecycle capability**, after conversion, delivery and receiving are stable.

The first payment objective should not be to hold funds. It should be to connect invoice state with payment state.

Possible capabilities include:

- generate payment instructions from invoice data;
- expose payment links or bank-payment initiation options;
- integrate with PSPs and banks;
- SEPA credit transfer / instant-payment initiation through regulated providers;
- SEPA Request-to-Pay where appropriate;
- track payment initiation and settlement status;
- record payment references/remittance information;
- handle partial payments and overpayments;
- handle failed/rejected payments;
- associate payments with invoices automatically;
- expose payment-status webhooks.

A useful lifecycle becomes:

```text
invoice generated
      |
invoice validated
      |
invoice delivered
      |
recipient accepts / receives
      |
payment requested
      |
payment initiated
      |
settlement confirmed
      |
invoice reconciled
      |
receipt / remittance evidence produced
```

## Payment architecture boundary

The recommended first payment architecture is orchestration, not custody:

```text
invoice platform
      |
payment request / initiation
      |
regulated bank / PSP
      |
payment network
      |
settlement
      |
bank / PSP event
      |
invoice platform reconciliation
```

The platform should initially avoid holding customer funds, maintaining stored balances, directly acquiring cards, storing raw card details, acting as merchant-of-record by default, or claiming settlement before authoritative confirmation.

A deeper regulated payment role should only be evaluated later if transaction volume and economics justify the regulatory, AML, fraud, treasury and operational burden.

---

# Later extension: receipts and remittance

Receipts should be modeled explicitly rather than treated as a PDF afterthought.

Related business objects include:

- payment confirmation;
- payment receipt;
- remittance advice;
- settlement evidence;
- credit note/refund information;
- invoice-payment allocation;
- accounting reconciliation record.

UBL 2.1 already includes `RemittanceAdvice`, which fits naturally with the broader platform direction.

A future flow may be:

```text
payment settles
     |
payment event
     |
match against invoice(s)
     |
reconciliation
     |
     +--> canonical remittance record
     +--> UBL RemittanceAdvice
     +--> JSON event
     +--> PDF receipt
     +--> ERP posting/update
```

Payment and receipt data become part of the same durable business workflow as the invoice while remaining distinct objects with their own identities and audit histories.

---

# Automatic reconciliation

This may become one of the highest-value later features.

The platform already knows invoice identity, supplier/customer identities, amount due, currency, due date, payment reference and delivery destination.

With bank/PSP settlement data it can match:

```text
payment transaction
        |
candidate invoice(s)
        |
reference + amount + counterparty matching
        |
        +--> exact match
        +--> partial payment
        +--> overpayment
        +--> ambiguous/unmatched payment
```

This expands the commercial value from compliance infrastructure into **accounts-receivable automation**. Inbound supplier invoices plus payment execution can later extend the same architecture into **accounts-payable automation**.

---

# Core product invariants

1. **Conversion is reproducible.** Every output identifies the input, mapping version, target profile and transformation version.
2. **Validation is evidence-based.** A document cannot be reported profile-valid without complete required validation evidence.
3. **Delivery is separate from validity.** A standards-valid document may still fail transport.
4. **Payment initiation is separate from settlement.** Initiating a payment never marks an invoice paid.
5. **Settlement requires authoritative evidence.** Only bank/PSP/network confirmation advances payment state to settled.
6. **Reconciliation is explicit.** Partial, multiple, overpaid and ambiguous settlements are represented rather than hidden.
7. **Receipts/remittance are durable business artifacts.** They are not reconstructed casually from current mutable state.
8. **No invented compliance data.** Missing required identifiers or tax/payment information must fail or require controlled enrichment.
9. **Adapters and mappings are versioned.** Trading-partner behavior must be reproducible.
10. **Transport providers are replaceable.** Peppol APs, national networks, banks and PSPs sit behind connector interfaces.
11. **Usage billing is idempotent.** Infrastructure retries and duplicate queue deliveries must never create duplicate customer charges.
12. **SaaS billing credits are separate from invoice-payment money.** The commercial credit ledger must never become a customer-funds ledger.

---

# Product roadmap

## Current foundation

- UBL 2.1 validation;
- EN 16931 validation;
- asynchronous durable processing;
- independent validation stages;
- Peppol validation roadmap.

## Phase A — Canonical invoice and generation API

Build the canonical semantic invoice model, canonical JSON contract, UBL generation, target-profile selection, generated-document validation and deterministic transformation provenance.

## Phase B — Multi-format conversion

Add CII, CSV/flat-file mapping, EDIFACT INVOIC, X12 810 and selected ERP adapters such as SAP IDoc.

## Phase C — Routing and delivery

Add recipient/profile resolution, Peppol participant lookup, Access Point connector abstraction, delivery state machine, retries/DLQ, delivery evidence and callbacks.

## Phase D — Inbound exchange

Receive invoices and convert them into canonical JSON, EDIFACT INVOIC, X12 810 and ERP-specific formats.

## Phase E — National profile/network expansion

Add country capability packs according to commercial demand and regulatory timing.

## Phase F — Payment requests and payment status

Add payment instruction extraction, payment request creation, PSP/bank connectors, SEPA initiation through regulated providers, SEPA Request-to-Pay where commercially useful, payment-status events and durable payment state.

## Phase G — Reconciliation and receipts

Add settlement ingestion, invoice/payment matching, partial/multiple payment allocation, remittance advice, payment receipts, ERP update hooks and reconciliation APIs.

## Phase H — Advanced payment capabilities

Evaluate deeper bank integrations, open-banking payment initiation, payment routing optimisation, refunds, payment collection options, a possible regulated payment-provider role and treasury/cash-management capabilities.

---

# Commercial differentiation

A simple valid/invalid API is commodity territory.

The defensible product becomes the combination of deterministic standards validation, canonical invoice semantics, configurable/versioned conversion, multi-format generation, trading-partner enrichment, recipient/profile resolution, bidirectional routing, network/provider abstraction, permanent delivery evidence, payment orchestration, automatic reconciliation, receipts/remittance, regression testing across standards upgrades and private deployment.

The deeper into the lifecycle the platform goes, the harder it becomes to replace because it owns not only compliance but also the customer's business-process integration.

---

# Developer experience as a moat

Validation diagnostics, conversion diagnostics, delivery failures and payment/reconciliation state should all be machine-readable, stable and developer-friendly.

The public API should expose business outcomes and durable operation identifiers rather than leaking internal worker topology.

---

# Sales and marketing

The first motion should remain developer-led growth.

Build:

- free validator;
- excellent API documentation;
- CLI;
- GitHub examples;
- Docker demo image;
- exact rule-ID SEO pages;
- `JSON -> Peppol` examples;
- `EDIFACT INVOIC -> Peppol` examples;
- inbound Peppol -> JSON/EDI examples;
- migration guides for ERP and EDI vendors.

Later high-intent topics include:

```text
EDIFACT to Peppol
X12 810 to Peppol
JSON to Peppol invoice
ERP Peppol API
Peppol receive API
invoice conversion API
invoice routing API
SEPA invoice payment API
invoice reconciliation API
```

---

# Usage-credit commercial model

## Commercial principle

The platform should support a **prepaid usage-credit model** as a primary self-service purchasing mechanism.

The customer buys a bundle of platform credits and spends them on invoice-processing outcomes. This is a natural fit for API customers because cost scales with document volume rather than seats or user accounts.

A simple commercial message is:

> **No subscription required. Buy invoice-processing credits and use them for validation, conversion, delivery and receiving.**

Enterprise customers can later use monthly committed volume, contract pricing or postpaid invoicing while using the same underlying metering model.

## Make the credit value transparent

Use a stable, understandable unit:

```text
1 credit = €0.01
```

This makes invoices, dashboards and rate cards easy to understand. Avoid opaque credit exchange rates that force customers to calculate the true euro price of every operation.

## Do not expose every internal stage as a billable item

The implementation may internally execute many steps:

```text
parse
normalize
enrich
generate
XSD validate
EN 16931 validate
Peppol validate
recipient lookup
route
send
acknowledgement tracking
```

Customers should not receive eleven microcharges for one invoice journey.

Internal stages should be metered for observability and cost accounting, but the public rate card should expose a small number of **business-level operations**.

Suggested initial billable operations:

| Operation | Illustrative credits | Illustrative price |
| --- | ---: | ---: |
| Basic UBL/XSD validation | 10 | €0.10 |
| EN 16931 validation | 20 | €0.20 |
| Full Peppol validation | 25 | €0.25 |
| Canonical JSON -> compliant invoice conversion | 25 | €0.25 |
| CII/UBL conversion | 25 | €0.25 |
| EDIFACT/X12 -> canonical conversion surcharge | 20–35 | €0.20–€0.35 |
| Route/send through supported e-invoice network | 40+ | €0.40+ |
| Receive invoice through supported network | 25+ | €0.25+ |
| Payment request | later | TBD |
| Automatic reconciliation | later | TBD |

These figures are **provisional commercial hypotheses**, not frozen prices. In particular, routing must not be priced permanently until wholesale Access Point/network/provider costs and support costs are known.

## Sell invoice journeys as well as atomic operations

The API may expose atomic operations, but the commercial offer should increasingly sell complete outcomes.

Illustrative journey pricing:

| Invoice journey | Illustrative price |
| --- | ---: |
| Validate only | €0.25 |
| Convert + validate | €0.40 |
| Convert + validate + route/send | €0.65 |
| Receive + validate + convert | €0.55 |
| EDIFACT -> Peppol complete outbound | €0.80 |
| Peppol -> EDIFACT complete inbound | €0.70 |

This keeps customer pricing simple while allowing the platform to maintain detailed internal cost accounting.

## Credit bundles

An initial bundle structure could be:

| Pack | Customer pays | Credits issued | Effective bonus |
| --- | ---: | ---: | ---: |
| Starter | €50 | 5,000 | none |
| Developer | €200 | 22,000 | 10% |
| Business | €1,000 | 120,000 | 20% |
| Scale | €5,000 | 650,000 | 30% |
| Enterprise | negotiated | negotiated | contract |

Bundle sizes and discounts should be tested against actual acquisition cost, support burden, external network cost and observed volume distribution.

## Auto-recharge

Self-service accounts should optionally support an automatic recharge rule such as:

```text
if credit_balance < 2,000:
    purchase 20,000 credits
```

The customer must control the threshold, recharge amount and spending limits.

## Volume economics

A flat €0.25 validation price is reasonable as a low-volume self-service hypothesis, but probably too expensive for very large customers.

The effective unit price should decline with committed or prepaid volume.

Illustrative validation economics:

```text
1–1,000 invoices          €0.25 each
1,001–10,000              €0.15 each
10,001–100,000            €0.08 each
100,000+                  negotiated
```

Credit-pack bonuses can implement much of this decline without creating a complicated per-request tariff table.

## Routing price must include external provider economics

Routing is not equivalent to validation because it may incur an external Peppol Access Point, national platform or delivery-provider charge.

The internal economic model should therefore be:

```text
customer routing price
    =
platform routing cost
    + external network/provider cost
    + support/risk allowance
    + margin
```

The public API can still expose one simple price when provider costs are predictable.

A provisional €0.40 routing price is plausible only if the underlying wholesale/provider economics leave adequate margin.

## Billing rules for success, failure and retries

Billing semantics must be frozen as carefully as validation semantics.

Recommended principles:

```text
VALIDATION
- charge once when the requested validation actually executes
- charge whether the business result is valid or invalid
- do not charge again for infrastructure retries

CONVERSION
- charge once when a requested conversion reaches its defined billable outcome
- malformed source rejection before meaningful processing may be free
- platform failure must not create a charge

ROUTING / DELIVERY
- define the exact billable boundary explicitly
- normally charge once when the platform successfully hands the document to the configured external delivery connector or reaches another contractually defined submission point
- platform retry/redelivery must never create another charge

RECEIVING
- charge once per accepted inbound business document, not once per internal delivery attempt

IDEMPOTENT CUSTOMER RETRY
- same idempotency key and same operation must never create a duplicate charge

PLATFORM FAILURE
- never leave a customer permanently charged for an operation that failed solely because of platform infrastructure
- use a reversal/credit transaction rather than deleting billing history
```

## Immutable credit ledger

Do not implement billing as a mutable `balance = balance - 25` counter with no durable explanation.

Use an immutable ledger.

Conceptually:

```text
credit_account
    tenant_id
    currency
    current_balance_projection

credit_transaction
    id
    tenant_id
    timestamp
    invoice_id / document_id
    operation_id
    operation_type
    rate_card_version
    credits
    direction
    idempotency_key
    external_cost_reference
    metadata
```

Example ledger:

```text
+10000  CREDIT_PURCHASE
   -25  PEPPOL_VALIDATE
   -35  EDIFACT_CONVERT
   -40  PEPPOL_SEND
   +40  REVERSAL_PLATFORM_FAILURE
```

The displayed balance should be a projection/reconciliation of immutable ledger entries, not the only accounting truth.

## Bill each business operation exactly once

The existing durable/idempotent architecture is a strong foundation for usage billing.

Every billable operation should have a deterministic billing idempotency key derived from business identity, operation type and billable generation/attempt semantics.

For example:

```text
billing_key = tenant + invoice_generation + operation_type
```

The exact contract needs refinement for revalidation, replay, replacement conversions and customer-requested resend, but queue redelivery and infrastructure retry must never alter the billable identity.

## Version the rate card

Historical charges must remain reproducible after prices change.

Example:

```text
rate_card = 2026-01
operation = peppol-send
credits = 40
```

Later:

```text
rate_card = 2027-01
operation = peppol-send
credits = 35
```

Every ledger debit must record the rate-card version that produced it.

Rate-card changes should never retroactively rewrite previous ledger entries.

## Separate commercial credits from future payment funds

When payment orchestration arrives, there will be two completely different concepts of money:

```text
SERVICE CREDITS
customer -> platform
used to buy SaaS/API operations
```

and:

```text
INVOICE FUNDS
payer -> supplier
tracked/orchestrated as part of invoice settlement
```

These must be different domains, ledgers, identifiers and security boundaries.

```text
SaaS billing ledger != invoice payment ledger
```

A prepaid API-credit balance must never evolve accidentally into a stored-value or customer-funds wallet.

## Customer billing surfaces

The commercial API/dashboard should expose at least:

- current credit balance;
- purchased bundles;
- credit expiry policy, ideally no arbitrary short expiry;
- rate-card version;
- per-operation usage;
- per-invoice usage breakdown;
- monthly usage export;
- invoice/receipt for credit purchases;
- debit and reversal history;
- auto-recharge settings;
- spending alerts;
- optional tenant/sub-account budgets.

For ERP vendors serving many downstream businesses, sub-account or customer attribution will be particularly valuable even when the ERP vendor owns the master credit balance.

## Market precedent

Credit/wallet usage billing already exists in adjacent e-invoicing APIs.

Examples observed in 2026 include:

- Scell.io: prepaid credits, no subscription, with a published per-invoice issuance price;
- OpenAPI: wallet/pay-as-you-go and subscription pricing for electronic invoicing, with lower effective unit prices at higher committed volumes.

These examples support the commercial feasibility of prepaid invoice-processing credits, while also showing that large-volume transaction pricing can fall into the low-cent range. Therefore premium prices such as €0.25 validation should be positioned as low-volume/self-service prices rather than assumed enterprise-volume prices.

---

# Pricing strategy by customer segment

## Self-service / developer

Primary model:

```text
prepaid credits
no mandatory subscription
public rate card
auto-recharge optional
```

Goal: minimal purchasing friction.

## Growing SaaS / ERP vendor

Primary model:

```text
larger prepaid bundle
volume bonus
monthly minimum optional
shared tenant/sub-account reporting
```

Goal: predictable marginal invoice cost.

## Enterprise / integrator

Primary model:

```text
annual committed volume
negotiated credit rate
postpaid or prepaid contract
SLA/support
private networking/deployment options
country/profile packs
```

The underlying meter remains operation/credit based even when commercial invoicing becomes monthly or annual.

---

# Scope discipline

Do not attempt all of the following simultaneously:

```text
validator
+ generic mapper
+ every EDI dialect
+ every national regime
+ certified Peppol AP
+ banking platform
+ payment institution
+ OCR product
```

Recommended expansion order:

```text
validation trust core
        |
canonical semantic model
        |
generation
        |
conversion adapters
        |
routing + delivery
        |
inbound exchange
        |
national expansion
        |
payment orchestration
        |
reconciliation + receipts
        |
possible deeper payment role
```

The next architectural design work should still be the **canonical semantic invoice model and adapter contract**, followed by a **routing/transport abstraction**.

The billing architecture can be designed in parallel because deterministic operation identity, idempotency and immutable usage evidence should be built into later conversion/routing workflows rather than bolted on after launch.

Payments should be designed into the long-term data model now, but implemented only after the invoice exchange platform is stable.

---

# Feasibility assessment

| Capability | Feasibility |
| --- | --- |
| Canonical JSON -> UBL generation | Very high |
| UBL/CII normalization | High |
| EDIFACT INVOIC -> canonical -> Peppol | High |
| X12 810 -> canonical -> Peppol | High |
| Canonical -> EDIFACT/X12 | High |
| Partner-AP Peppol routing | High |
| Inbound Peppol -> EDI/ERP | High |
| National profile packs | High but ongoing standards work |
| Prepaid usage-credit billing | Very high |
| Immutable metering/credit ledger | Very high |
| Payment request / PSP-bank orchestration | High |
| SEPA Request-to-Pay integration | High |
| Settlement ingestion and reconciliation | High |
| Receipt/remittance generation | High |
| Own certified Peppol AP | Technically high, operationally harder |
| Holding/custody of customer funds | Technically possible, regulatory complexity high |
| PDF/OCR -> legally reliable canonical invoice | Medium and should be deferred |

The overall expansion is technically credible and fits the current asynchronous, durable e-ubl21 architecture.

The long-term product can be summarized as:

> **Convert any supported business invoice format into the format the destination requires, validate it, route it, send and receive it, then connect the resulting obligation to payment, reconciliation and receipt evidence. Bill the service through transparent usage credits tied to durable business operations rather than seats or opaque subscriptions.**

Validation creates trust. Conversion creates interoperability. Routing makes the system operational. Bidirectional exchange embeds it into ERP/EDI workflows. Payment and reconciliation eventually turn it into an invoice lifecycle platform. Usage credits make the commercial model scale naturally with transaction volume.

---

# References

ViDA:  
https://taxation-customs.ec.europa.eu/taxation/vat/vat-digital-age-vida_en

France e-invoicing rollout:  
https://www.impots.gouv.fr/professionnel/questions/partir-de-quand-suis-je-concerne-par-la-reforme-de-la-facturation

Germany mandatory B2B e-invoice FAQ:  
https://www.bundesfinanzministerium.de/Content/DE/FAQ/e-rechnung.html

Peppol certified providers:  
https://peppol.org/members/peppol-certified-service-providers/

Peppol BIS Billing documentation:  
https://docs.peppol.eu/poacc/billing/3.0/

European Commission payment services:  
https://finance.ec.europa.eu/consumer-finance-and-payments/payment-services/payment-services_en

European Payments Council SEPA Request-to-Pay:  
https://www.europeanpaymentscouncil.eu/what-we-do/other-schemes/sepa-request-pay

Scell.io prepaid-credit model:  
https://scell.io/en/faq

OpenAPI electronic-invoicing wallet pricing:  
https://console.openapi.com/apis/sdi/pricing

OpenAPI electronic-invoicing product pricing:  
https://openapi.com/products/italian-electronic-invoicing
