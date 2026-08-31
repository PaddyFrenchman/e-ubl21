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

This avoids a combinatorial explosion of point-to-point transforms such as:

```text
EDIFACT -> UBL
EDIFACT -> CII
X12 -> UBL
X12 -> CII
JSON -> UBL
JSON -> CII
...
```

Instead:

```text
source format -> canonical model -> target format
```

---

# Input directions

## Priority order

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

The first generation API should allow an ERP or SaaS vendor to submit normalized invoice data such as:

```json
{
  "invoiceNumber": "INV-10023",
  "issueDate": "2026-08-31",
  "seller": {},
  "buyer": {},
  "currency": "EUR",
  "lines": []
}
```

with a requested destination/profile such as:

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

A controlled trading-partner configuration can provide information such as:

```text
partner: ACME-DE
VAT ID: DE123456789
Peppol endpoint: 0204:...
endpoint scheme: 0204
preferred outbound profile: Peppol BIS Billing 3
```

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

## Routing decision inputs

The router may consider:

- seller identity;
- buyer identity;
- country;
- requested/required invoice profile;
- Peppol participant capability;
- customer routing policy;
- national platform requirements;
- direct trading-partner configuration;
- fallback channel;
- retry policy.

## Keep delivery state separate from validation state

A valid invoice can fail delivery.

Therefore do not flatten everything into one status.

Use separate state dimensions such as:

```text
validation_status
conversion_status
delivery_status
payment_status
reconciliation_status
```

For example:

```text
validation_status = valid
delivery_status   = failed
payment_status    = not_requested
```

This distinction becomes crucial once payments are introduced.

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

The first payment objective should not be to hold funds.

It should be to connect invoice state with payment state.

## Initial payment capabilities

Possible capabilities include:

- generate payment instructions from invoice data;
- expose payment links or bank-payment initiation options;
- integrate with PSPs and banks;
- SEPA credit transfer / instant-payment initiation through regulated providers;
- SEPA Request-to-Pay where appropriate;
- track payment initiation and settlement status;
- record payment references/remittance information;
- handle partial payments;
- handle overpayments;
- handle failed/rejected payments;
- associate payments with invoices automatically;
- expose payment-status webhooks.

A useful lifecycle becomes:

```text
invoice generated
      |
      v
invoice validated
      |
      v
invoice delivered
      |
      v
recipient accepts / receives
      |
      v
payment requested
      |
      v
payment initiated
      |
      v
settlement confirmed
      |
      v
invoice reconciled
      |
      v
receipt / remittance evidence produced
```

## SEPA Request-to-Pay

SEPA Request-to-Pay is especially interesting architecturally because it is a request/messaging layer around payment initiation rather than the payment instrument itself.

That aligns naturally with an invoice platform:

```text
invoice
   |
   v
payment request
   |
   v
payer approval
   |
   v
underlying SEPA payment
   |
   v
settlement event
   |
   v
reconciliation
```

## Instant payments

Instant euro transfers are increasingly useful for invoice settlement because settlement can occur within seconds and around the clock.

The platform can use regulated bank/PSP integrations while maintaining invoice-level orchestration and reconciliation itself.

---

# Later extension: receipts and remittance

The word **receipt** should be modeled explicitly rather than treated as a PDF afterthought.

There are several related business objects:

- payment confirmation;
- payment receipt issued to the payer;
- remittance advice;
- settlement evidence;
- credit note/refund information;
- invoice-payment allocation;
- accounting reconciliation record.

The platform should support machine-readable evidence first, with human-readable PDF receipts as a presentation layer.

## Remittance advice

UBL 2.1 already contains business documents such as `RemittanceAdvice`, which fits naturally with the broader platform direction.

A future flow may be:

```text
payment settles
     |
     v
payment event
     |
     v
match against invoice(s)
     |
     v
reconciliation
     |
     +--> canonical remittance record
     |
     +--> UBL RemittanceAdvice
     |
     +--> JSON event
     |
     +--> PDF receipt
     |
     +--> ERP posting/update
```

The important concept is that payment and receipt data become part of the same durable business workflow as the invoice, while remaining distinct objects with their own identities and audit histories.

---

# Automatic reconciliation

This may become one of the highest-value later features.

The platform already knows:

- invoice identifier;
- supplier/customer identities;
- amount due;
- currency;
- due date;
- payment reference;
- delivery destination.

With bank/PSP settlement data it can match:

```text
payment transaction
        |
        v
candidate invoice(s)
        |
        v
reference + amount + counterparty matching
        |
        +--> exact match
        |
        +--> partial payment
        |
        +--> overpayment
        |
        +--> ambiguous/unmatched payment
```

This expands the commercial value from compliance infrastructure into **accounts-receivable automation**.

Inbound supplier invoices plus payment execution can later extend the same architecture into **accounts-payable automation**.

---

# Payment architecture boundary

## Stage 1: orchestration, not custody

The recommended first payment architecture is:

```text
invoice platform
      |
      | payment request / initiation
      v
regulated bank / PSP
      |
      v
payment network
      |
      v
settlement
      |
      v
bank / PSP event
      |
      v
invoice platform reconciliation
```

The platform should initially avoid:

- holding customer funds;
- maintaining stored balances;
- directly acquiring cards;
- storing raw card details;
- acting as merchant-of-record by default;
- claiming settlement before authoritative confirmation.

This captures most of the customer value while keeping regulated payment execution with licensed providers.

## Stage 2: deeper regulated payment role only if justified

Much further down the road, evaluate whether becoming or partnering more deeply as a regulated payment provider is strategically worthwhile.

That decision should be driven by:

- transaction volume;
- payment revenue opportunity;
- customer demand;
- geography;
- regulatory/licensing cost;
- fraud and AML obligations;
- treasury/custody requirements;
- operational risk.

It should not be a prerequisite for invoice conversion, routing or reconciliation.

---

# End-state architecture

```text
                              CUSTOMER SYSTEMS
                                    |
              +---------------------+---------------------+
              |                     |                     |
             ERP                   EDI                  API
              |                     |                     |
              +---------------------+---------------------+
                                    |
                                    v
                          +-------------------+
                          | INGEST / ADAPTERS |
                          +-------------------+
                                    |
                                    v
                          +-------------------+
                          | CANONICAL INVOICE |
                          +-------------------+
                                    |
                  +-----------------+------------------+
                  |                 |                  |
                  v                 v                  v
             ENRICHMENT         VALIDATION         GENERATION
                  |                 |                  |
                  +-----------------+------------------+
                                    |
                                    v
                               +---------+
                               | ROUTER  |
                               +---------+
                                    |
              +---------------------+----------------------+
              |                     |                      |
              v                     v                      v
           Peppol               National               Direct/API
              |                  network                 channel
              +---------------------+----------------------+
                                    |
                             SEND / RECEIVE
                                    |
                                    v
                            DELIVERY EVENTS
                                    |
                                    v
                         +---------------------+
                         | PAYMENT ORCHESTRATOR|
                         +---------------------+
                                    |
                     +--------------+--------------+
                     |                             |
                     v                             v
                   BANK                           PSP
                     |                             |
                     +--------------+--------------+
                                    |
                                    v
                              SETTLEMENT
                                    |
                                    v
                           RECONCILIATION
                                    |
                     +--------------+--------------+
                     |                             |
                     v                             v
                REMITTANCE                      RECEIPT
                     |                             |
                     +--------------+--------------+
                                    |
                                    v
                              ERP / LEDGER
```

---

# Core product invariants

As the product expands, several rules should remain non-negotiable.

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

---

# Product roadmap

## Current foundation

- UBL 2.1 validation;
- EN 16931 validation;
- asynchronous durable processing;
- independent validation stages;
- Peppol validation roadmap.

## Phase A — Canonical invoice and generation API

Build:

- canonical semantic invoice model;
- canonical JSON contract;
- JSON/canonical -> UBL generation;
- target-profile selection;
- generated-document validation;
- deterministic transformation provenance.

## Phase B — Multi-format conversion

Add:

- CII;
- CSV/flat-file mapping;
- EDIFACT INVOIC;
- X12 810;
- selected ERP adapters such as SAP IDoc.

## Phase C — Routing and delivery

Add:

- recipient/profile resolution;
- Peppol participant lookup;
- Access Point connector abstraction;
- delivery state machine;
- retries/DLQ;
- delivery evidence;
- webhooks/callbacks.

## Phase D — Inbound exchange

Receive invoices and convert them into:

- canonical JSON;
- EDIFACT INVOIC;
- X12 810;
- ERP-specific formats.

## Phase E — National profile/network expansion

Add country capability packs according to commercial demand and regulatory timing.

Potential examples:

- France;
- Germany;
- Belgium;
- Italy;
- Poland;
- additional Peppol/national regimes.

## Phase F — Payment requests and payment status

Add:

- payment instruction extraction;
- payment request creation;
- PSP/bank connectors;
- SEPA payment initiation through regulated providers;
- SEPA Request-to-Pay integration where commercially useful;
- payment-status events;
- durable payment state.

## Phase G — Reconciliation and receipts

Add:

- settlement ingestion;
- invoice/payment matching;
- partial/multiple payment allocation;
- remittance advice;
- payment receipts;
- accounting/ERP update hooks;
- reconciliation APIs.

## Phase H — Advanced payment capabilities

Evaluate:

- deeper bank integrations;
- open-banking payment initiation;
- payment routing optimisation;
- refunds;
- payment collection options;
- potential regulated payment-provider role;
- treasury/cash-management capabilities.

Do this only after invoice exchange volume and customer demand justify the regulatory and operational cost.

---

# Commercial differentiation

A simple valid/invalid API is commodity territory.

The defensible product becomes the combination of:

1. deterministic standards validation;
2. canonical invoice semantics;
3. configurable and versioned conversion;
4. multi-format generation;
5. trading-partner enrichment;
6. recipient/profile resolution;
7. bidirectional routing;
8. network/provider abstraction;
9. permanent delivery evidence;
10. payment orchestration;
11. automatic reconciliation;
12. receipts/remittance;
13. regression testing across standards upgrades;
14. private deployment.

The deeper into the lifecycle the platform goes, the harder it becomes to replace because it owns not only compliance but also the customer's business-process integration.

---

# Developer experience as a moat

Validation diagnostics should remain excellent:

```json
{
  "valid": false,
  "profile": "PEPPOL-BIS-3",
  "errors": [
    {
      "rule": "BR-CO-10",
      "severity": "fatal",
      "path": "/Invoice/...",
      "field": "invoice_total",
      "explanation": "...",
      "suggested_fix": "..."
    }
  ]
}
```

Conversion failures should be equally explicit:

```json
{
  "converted": false,
  "source": "edifact-invoic-d96a",
  "target": "peppol-bis-billing-3",
  "errors": [
    {
      "field": "buyer.endpoint_id",
      "source": "NAD+BY",
      "explanation": "A buyer electronic endpoint identifier is required by the target profile and none is present in the message or approved partner configuration."
    }
  ]
}
```

Payment/reconciliation APIs should follow the same principle:

```json
{
  "invoiceId": "INV-10023",
  "amountDue": "1200.00",
  "currency": "EUR",
  "paymentStatus": "partially_paid",
  "settledAmount": "800.00",
  "remainingAmount": "400.00",
  "allocations": [
    {
      "paymentId": "pay_01",
      "amount": "800.00",
      "status": "settled"
    }
  ]
}
```

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

# Pricing direction

Validation-only pricing can remain simple at first.

The broader platform should eventually price around value and transaction volume.

Potential dimensions:

- invoices converted;
- invoices delivered/received;
- active trading partners;
- enabled country/profile packs;
- EDI/ERP connectors;
- delivery-network connectors;
- payment requests initiated;
- settlements reconciled;
- private deployment;
- SLA/support tier.

Customers should buy an invoice-lifecycle outcome rather than internal pipeline stages.

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

The next architectural design work should therefore still be the **canonical semantic invoice model and adapter contract**, followed by a **routing/transport abstraction**.

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
| Payment request / PSP-bank orchestration | High |
| SEPA Request-to-Pay integration | High |
| Settlement ingestion and reconciliation | High |
| Receipt/remittance generation | High |
| Own certified Peppol AP | Technically high, operationally harder |
| Holding/custody of customer funds | Technically possible, regulatory complexity high |
| PDF/OCR -> legally reliable canonical invoice | Medium and should be deferred |

The overall expansion is technically credible and fits the current asynchronous, durable e-ubl21 architecture.

The long-term product can be summarized as:

> **Convert any supported business invoice format into the format the destination requires, validate it, route it, send and receive it, then connect the resulting obligation to payment, reconciliation and receipt evidence.**

Validation creates trust. Conversion creates interoperability. Routing makes the system operational. Bidirectional exchange embeds it into ERP/EDI workflows. Payment and reconciliation eventually turn it into an invoice lifecycle platform.

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

European Commission instant payments:  
https://finance.ec.europa.eu/news/new-eu-rules-make-instant-euro-payments-faster-and-safer-2025-10-10_en

European Payments Council SEPA Request-to-Pay:  
https://www.europeanpaymentscouncil.eu/what-we-do/other-schemes/sepa-request-pay
