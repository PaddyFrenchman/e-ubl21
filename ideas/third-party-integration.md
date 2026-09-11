# Third-party integration

## Purpose

This document explores how e-ubl21 can accept invoices produced by third-party accounting and ERP products in their native API/data models, transform them into a shared business-semantic model, generate standards-compliant UBL XML, and then run the existing validation pipeline.

The strategic objective is broader than adding one Sage converter. The objective is to make accounting products adapters around a shared canonical billing model:

```text
Sage ──────────────┐
Xero ──────────────┤
QuickBooks ────────┤
SAP Business One ──┤
SAP S/4HANA ───────┼──► Canonical billing model ─► UBL renderer ─► validation
NetSuite ──────────┤
Custom JSON/CSV ───┤
Customer ERP ──────┘
```

The existing validation pipeline remains authoritative after transformation:

```text
third-party invoice
        │
        ▼
source connector
        │
        ▼
source mapper
        │
        ▼
canonical invoice draft
        │
        ▼
enrichment + semantic resolution
        │
        ▼
resolved canonical invoice
        │
        ▼
UBL 2.1 renderer
        │
        ▼
XSD → EN 16931 → Peppol
```

The hard problem is not XML generation. It is preserving business meaning while translating vendor-specific tax codes, party references, units, discounts, charges, totals, and identifiers into explicit EN 16931 / Peppol semantics.

The public APIs discussed below were reviewed in September 2026. Vendor APIs evolve, so any production adapter must pin and test the source API/schema version it supports.

---

# Sage

## Product opportunity

Supporting Sage invoice input is very feasible and does not require a redesign of e-ubl21. It is a natural extension of the existing product.

Today the public flow is broadly:

```text
UBL XML
   ↓
e-ubl21
   ↓
validation pipeline
```

The Sage integration adds a transformation layer in front:

```text
Sage invoice
     │
     ▼
Sage connector / adapter
     │
     ▼
canonical billing model
     │
     ▼
UBL generator
     │
     ▼
existing e-ubl21 validation
```

The project already contains important foundations for this approach. The roadmap anticipates canonical representations, XML/JSON conversion, creation/import endpoints, and serialization into UBL. The current `createsamples/en16931` package also contains an embryonic business-level invoice model and UBL rendering logic that demonstrate the transformation concept.

## Current Sage API observations

Sage is a product family, so the implementation must name the exact source product/API rather than claiming to support generic "Sage".

A particularly attractive first target is **Sage Active Public API V2**:

- GraphQL-based API;
- OAuth 2.0 authentication;
- explicit `SalesInvoice` resource;
- invoice lines, customer references, discounts and tax information;
- unit-of-measure identifiers;
- VAT percentages;
- tax and tax-treatment identifiers;
- legislation-aware behavior for European markets;
- current documentation includes e-invoicing-related fields.

That makes Sage Active a good source for a European canonical-model proof of concept because tax treatment is richer than a simple percentage alone.

Official reference:

- https://developer.sage.com/sageactive/resources/salesinvoices/
- https://developer.sage.com/sageactive/resources/taxtreatments/

## Core difficulty: semantic mapping

Generating XML is not the difficult part. The difficult part is mapping Sage's business model into EN 16931 / Peppol semantics without inventing information or losing meaning.

A conceptual Sage source line might contain:

```json
{
  "description": "Consulting",
  "quantity": 10,
  "unitPrice": 100,
  "unitOfMeasurementId": "...",
  "vatPercentage": 23,
  "taxTreatmentId": "..."
}
```

The adapter needs to resolve the source information into business semantics such as:

```text
Invoice number       → BT-1
Issue date           → BT-2
Currency             → BT-5
Seller               → BG-4
Buyer                → BG-7
Line quantity        → BT-129
Unit code            → BT-130
Item name            → BT-153
Net price            → BT-146
VAT category         → BT-151
VAT rate             → BT-152
```

Source identifiers such as `unitOfMeasurementId` or `taxTreatmentId` must never leak into the canonical model as if they were standard identifiers. The Sage adapter must resolve them to normalized unit/tax semantics first.

## Semantic gaps must fail explicitly

A source product may not contain everything necessary to construct a compliant e-invoice.

For example, `vatPercentage = 0` alone is insufficient to identify the legal VAT treatment. Zero-rated, exempt, reverse charge, outside-scope and intra-community transactions are not interchangeable.

Likewise, the source invoice may not contain a Peppol electronic endpoint identifier.

Missing semantics may be resolved only from controlled sources:

- the submitted source document;
- tenant/company configuration;
- customer/supplier master data;
- an explicitly configured mapping rule;
- or a structured conversion error.

The converter must not invent legal information.

## Sage adapter recommendation

The first implementation should target one named Sage API/version, preferably Sage Active V2, and support sales Invoice first. Credit notes and additional Sage product families can be added as separate adapters or adapter versions.

Relative implementation difficulty: **medium**.

---

# Xero

## Source model

Xero has a comparatively clean accounting API and is a strong candidate for an early adapter.

The Xero Accounting API exposes invoices and credit notes. Its Invoice model contains fields including:

- `Type`;
- `Contact`;
- `LineItems`;
- `Date`;
- `DueDate`;
- `LineAmountTypes`;
- `InvoiceNumber`;
- `Reference`;
- `CurrencyCode` and currency rate;
- subtotal, total tax and total;
- discounts and payment-related totals.

A Xero line item exposes fields including:

- description;
- quantity;
- unit amount;
- item code;
- account code;
- tax type;
- tax amount;
- line amount;
- discount rate / amount;
- tax breakdown information.

The associated Contact resource can supply company/person information, tax number and addresses.

Official references:

- https://developer.xero.com/documentation/api/accounting/overview
- https://developer.xero.com/documentation/api/accounting/invoices
- https://developer.xero.com/documentation/api/accounting/contacts
- https://github.com/XeroAPI/Xero-OpenAPI

## Representative mapping

A representative Xero sales invoice can be thought of as:

```json
{
  "Type": "ACCREC",
  "InvoiceNumber": "INV-1024",
  "Date": "2026-09-11",
  "DueDate": "2026-10-11",
  "CurrencyCode": "EUR",
  "Contact": { "ContactID": "..." },
  "LineAmountTypes": "Exclusive",
  "LineItems": [
    {
      "Description": "Consulting",
      "Quantity": 10,
      "UnitAmount": 100,
      "TaxType": "...",
      "TaxAmount": 230,
      "LineAmount": 1000
    }
  ]
}
```

The mapping is structurally straightforward:

```text
InvoiceNumber              → document ID
Date                       → issue date
DueDate                    → due date
CurrencyCode               → document currency
Contact                    → buyer party lookup
LineItems[].Description    → item description/name
LineItems[].Quantity       → invoiced quantity
LineItems[].UnitAmount     → net price
LineItems[].LineAmount     → line net amount
LineItems[].Discount*      → line allowance
LineItems[].TaxType        → source tax classification
LineItems[].TaxAmount      → source tax evidence
```

## Important semantic gaps

The main difficulty is tax and party enrichment rather than basic invoice structure.

`TaxType` is a Xero accounting/tax configuration value, not automatically an EN 16931 VAT category. The adapter therefore needs a versioned mapping layer from Xero tax configuration to legal e-invoice tax semantics.

The invoice's `ContactID` is also only a reference. A connector normally needs to snapshot the associated contact so the deterministic mapper has legal name, tax number and postal address available.

Electronic endpoint identifiers, legal registration scheme identifiers and some Peppol-specific data may need tenant/master-data enrichment.

`LineAmountTypes` must be honored because Xero can represent lines as tax-exclusive, tax-inclusive or no-tax. The canonical model should normalize all monetary semantics rather than merely copy displayed amounts.

## Xero adapter recommendation

Xero should be implemented as a connector that retrieves the invoice plus all referenced data required for conversion, followed by a pure mapper from that immutable snapshot to the canonical draft.

Relative implementation difficulty: **medium-low to medium**.

Xero is one of the best candidates after Sage for validating the canonical model because its invoice shape is simple but still exercises contacts, discounts, taxes and multi-currency.

---

# QuickBooks Online

## Source model

QuickBooks Online uses the Accounting API Invoice entity. An invoice requires a customer reference and line data. The model supports invoice dates/due dates, addresses, multiple line-detail types, sales tax information and totals.

Common invoice concepts include:

- `DocNumber`;
- `TxnDate`;
- `DueDate`;
- `CustomerRef`;
- billing/shipping addresses;
- `Line` entries;
- `SalesItemLineDetail`;
- item reference;
- quantity;
- unit price;
- line amount;
- tax-code reference;
- transaction tax detail;
- totals and balance.

QuickBooks also supports line types beyond ordinary product/service lines, including discount and subtotal lines. That is important because the adapter must translate presentation/accounting constructs into semantic invoice allowances/charges rather than preserve the source line layout blindly.

Official references:

- https://developer.intuit.com/app/developer/qbo/docs/api/accounting/all-entities/invoice
- https://github.com/intuit/QuickBooks-V3-PHP-SDK

## Representative mapping

Conceptually:

```json
{
  "DocNumber": "101",
  "TxnDate": "2026-09-11",
  "DueDate": "2026-10-11",
  "CustomerRef": { "value": "1" },
  "Line": [
    {
      "Description": "Consulting",
      "Amount": 1000,
      "DetailType": "SalesItemLineDetail",
      "SalesItemLineDetail": {
        "ItemRef": { "value": "7" },
        "UnitPrice": 100,
        "Qty": 10,
        "TaxCodeRef": { "value": "..." }
      }
    }
  ],
  "TxnTaxDetail": { "TotalTax": 230 }
}
```

Mapping examples:

```text
DocNumber                         → document ID
TxnDate                           → issue date
DueDate                           → due date
CustomerRef                       → buyer lookup
BillAddr                          → buyer postal address candidate
Line[].Description                → item description/name
SalesItemLineDetail.Qty           → quantity
SalesItemLineDetail.UnitPrice     → net/unit price candidate
Line[].Amount                     → line amount
SalesItemLineDetail.TaxCodeRef    → source tax mapping key
TxnTaxDetail                      → source tax evidence/breakdown
DiscountLineDetail                → allowance candidate
```

## Important semantic gaps

QuickBooks tax handling is strongly dependent on company configuration and localization. A `TaxCodeRef` or tax-rate reference is a QuickBooks identifier, not a portable legal classification.

The adapter therefore needs to snapshot referenced tax objects or resolve them through an integration-specific tax map before creating the resolved canonical model.

`CustomerRef` and `ItemRef` similarly require master-data lookup when legal names, addresses, tax registrations, product identifiers or units are needed.

The source can contain subtotal/description/group/discount lines that should not necessarily become one-for-one UBL invoice lines. The mapper must interpret their accounting meaning.

QuickBooks may calculate tax/totals itself. e-ubl21 should retain those source totals as reconciliation evidence but independently derive canonical totals and reject unexplained mismatches.

## QuickBooks adapter recommendation

Use two layers:

1. a QuickBooks connector that obtains the Invoice plus referenced Customer, Item, tax and currency/configuration data required for conversion;
2. a pure mapper that interprets QuickBooks line types and produces a canonical draft.

Relative implementation difficulty: **medium**.

The largest source of complexity is not line mapping but locale/configuration-dependent tax interpretation.

---

# SAP

"SAP" cannot be treated as one source format. SAP Business One and SAP S/4HANA expose materially different integration models and should be separate adapters.

## SAP Business One

### Source model

SAP Business One Service Layer exposes business objects over HTTP/OData. Current SAP guidance prefers OData v4 for Service Layer clients.

Marketing documents share a `Document` model with `DocumentLines`. Commonly available invoice-related fields include:

- document number / entry;
- posting/document/due dates;
- business partner code/name;
- document currency;
- tax date;
- document total;
- document lines;
- item code/description;
- quantity;
- unit price / price;
- tax code;
- line total;
- tax total;
- unit-of-measure code;
- discounts and other commercial information.

Official references:

- https://help.sap.com/docs/SAP_BUSINESS_ONE/f110a154dd0f4c20bf7f3ebca9eeb794/60c7a0b745bd486589f05a1da77041f3.html
- https://help.sap.com/docs/SAP_BUSINESS_ONE/f110a154dd0f4c20bf7f3ebca9eeb794/9060bf762eaa4a5eb7cb532a09ac167d.html

### Representative mapping

```json
{
  "DocNum": 10042,
  "CardCode": "C001",
  "CardName": "Example Customer",
  "DocDate": "2026-09-11",
  "DocDueDate": "2026-10-11",
  "TaxDate": "2026-09-11",
  "DocCurrency": "EUR",
  "DocumentLines": [
    {
      "ItemCode": "CONSULT",
      "ItemDescription": "Consulting",
      "Quantity": 10,
      "UnitPrice": 100,
      "TaxCode": "VAT23",
      "UoMCode": "HOUR",
      "LineTotal": 1000,
      "TaxTotal": 230
    }
  ]
}
```

Conceptual mapping:

```text
DocNum                 → document ID candidate
DocDate                → issue/posting date candidate
DocDueDate             → due date
TaxDate                → tax point/date candidate
DocCurrency            → document currency
CardCode                → buyer master-data lookup
CardName                → buyer display/legal-name candidate
DocumentLines[].Item*   → item identifiers/name
Quantity                → quantity
UnitPrice               → unit price
UoMCode                 → source unit mapping key
TaxCode                 → source tax mapping key
LineTotal               → source line-net evidence
TaxTotal                → source tax evidence
```

### Important semantic gaps

SAP Business One uses configurable business-partner, tax-code and unit-of-measure master data. `CardCode`, `TaxCode` and `UoMCode` therefore cannot be copied directly into EN 16931 fields without resolution.

User-defined fields are widely used in SAP Business One installations. The adapter should support declarative field mappings for customer-specific UDFs so Peppol endpoint IDs, legal identifiers or buyer references can be sourced without custom Go code for every installation.

The same `Document` structure can represent different commercial document types. The connector must preserve the exact SAP object/entity type so the mapper can distinguish invoice, credit memo and other records.

### Recommendation

Relative implementation difficulty: **medium-high**.

The core shape maps well, but enterprise/customer configuration and user-defined fields create more variability than Sage, Xero or QuickBooks.

## SAP S/4HANA

### Source model

S/4HANA's billing API is more normalized. The Billing Document APIs expose separate entities for:

- billing-document header;
- items;
- business partners;
- texts;
- pricing elements.

SAP also distinguishes read/process APIs from APIs used to create billing documents from external billing data.

Official references:

- https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/fe6f3eda2f914d81b512493fd79e9769.html
- https://help.sap.com/docs/SAP_S4HANA_CLOUD/03c04db2a7434731b7fe21dca77440da/9bc63bb02f844b66b906cd8e00e3251a.html

### Integration implication

An S/4HANA connector should not expect one self-contained invoice JSON blob. It should construct an immutable **source snapshot** by joining the relevant header, item, partner, pricing and text entities for one billing document.

That snapshot is then mapped to the same canonical billing model as the other products.

Pricing conditions are particularly important. Discounts, surcharges, freight and taxes may be represented as pricing elements rather than simple invoice fields. The adapter must interpret the relevant condition semantics and normalize them into line/document allowances, charges and tax breakdowns.

### Recommendation

Relative implementation difficulty: **high**.

S/4HANA should be implemented after the canonical model has already been proven with simpler accounting sources. It will be an excellent stress test of the architecture because it forces proper handling of normalized source data, partner roles, pricing conditions and customer-specific extensions.

---

# NetSuite

## Source model

NetSuite exposes the Invoice record through REST Web Services. The record includes numerous sublists and references, including:

- customer/entity reference;
- transaction/invoice number;
- transaction date;
- due date;
- terms;
- currency and exchange rate;
- subsidiary;
- item lines;
- discounts;
- tax registration information;
- tax details;
- tax point date;
- totals.

The item sublist exposes values such as quantity, rate, amount, tax amount, tax rate, units and tax-detail reference. The `taxdetails` sublist exposes tax basis, net amount, gross amount, tax amount, tax rate, tax code and tax type.

REST taxation requires SuiteTax, which is important for integration assumptions.

Official references:

- https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/section_161488248489.html
- https://docs.oracle.com/en/cloud/saas/netsuite/ns-online-help/SBADVTemplates_85934086.html

## Representative mapping

```json
{
  "tranId": "INV-1024",
  "tranDate": "2026-09-11",
  "dueDate": "2026-10-11",
  "entity": { "id": "220" },
  "currency": { "id": "1" },
  "terms": { "id": "1" },
  "subsidiary": { "id": "1" },
  "item": {
    "items": [
      {
        "line": 1,
        "item": { "id": "144" },
        "quantity": 10,
        "rate": 100,
        "amount": 1000,
        "taxrate": 23,
        "taxamount": 230
      }
    ]
  }
}
```

Conceptual mapping:

```text
tranId                 → document ID
tranDate               → issue date
dueDate                → due date
entity                 → buyer lookup
currency               → currency lookup
terms                  → payment-terms lookup
subsidiary             → seller/legal-entity lookup
item[].quantity        → quantity
item[].rate            → price
item[].amount          → line amount
taxdetails             → source tax breakdown/evidence
entitytaxregnum        → buyer tax-registration candidate
subsidiarytaxregnum    → seller tax-registration candidate
taxpointdate           → tax-point date candidate
```

## Important semantic gaps

NetSuite is extremely configurable. Record references, custom fields, subsidiaries, tax configuration and SuiteTax determine how much semantic information is present directly on a record.

The connector should therefore fetch/snapshot referenced customer, subsidiary, item, currency and tax information required by the mapping policy rather than allowing the mapper to make live network calls.

NetSuite's custom fields should be handled through declarative mapping rules where possible. This is likely to be necessary for Peppol participant identifiers, organization registration identifiers and customer-specific references.

The `taxdetails` sublist is useful evidence but the adapter must still resolve NetSuite tax codes/types to EN 16931 VAT categories and exemption semantics.

## NetSuite adapter recommendation

Relative implementation difficulty: **medium-high**.

The base invoice structure is rich, but customization and reference expansion make the connector more involved than Xero/QuickBooks.

---

# Cross-product comparison

The comparison strongly supports defining a canonical model before implementing multiple adapters.

| Semantic concept | Sage Active | Xero | QuickBooks Online | SAP Business One | SAP S/4HANA | NetSuite |
| --- | --- | --- | --- | --- | --- | --- |
| Invoice identifier | direct | direct | direct | direct | direct | direct |
| Issue/document date | direct | direct | direct | direct | direct | direct |
| Due date / terms | available | available | available | available | available | available/reference |
| Currency | available | direct | available/configuration | direct | direct | reference |
| Buyer | reference/object | Contact | CustomerRef | CardCode | partner entity | entity reference |
| Seller | organization context | organisation context | company context | company DB/context | company/partner context | subsidiary |
| Lines | direct | direct | polymorphic lines | DocumentLines | item entities | item sublist |
| Quantity / price | direct | direct | direct for sales lines | direct | item/pricing data | direct |
| Discounts | direct | direct | special/detail lines | fields/pricing | pricing conditions | fields/items |
| Tax rate | direct | available | tax detail/configuration | tax code/detail | pricing/tax data | taxdetails |
| Legal tax category | requires mapping | requires mapping | requires mapping | requires mapping | requires mapping | requires mapping |
| VAT/legal IDs | source/master data | Contact/org | customer/company data | BP/company/UDF | partner/company | entity/subsidiary |
| Unit code | source ID | often implicit/source | item/source | UoMCode | item/UoM | units reference |
| Peppol endpoint ID | usually enrichment | usually enrichment | usually enrichment | usually UDF/enrichment | master/custom field | custom/master data |
| Custom extensions | API/product fields | limited/vendor model | custom fields | UDFs | extensibility | extensive custom fields |

The striking commonality is that **every system has the same business skeleton**:

```text
identity
+ dates
+ seller
+ buyer
+ currency
+ lines
+ quantity/price
+ discounts/charges
+ tax
+ totals
+ payment/delivery/references
```

The differences are mostly:

- how those concepts are named;
- whether they are embedded or referenced;
- how tax semantics are encoded;
- how discounts/charges are represented;
- how much required e-invoice data lives in master data rather than on the invoice;
- how customer-specific extensions are represented.

That is exactly the situation in which a canonical model pays off.

---

# Proposed canonical billing model

## Important distinction: two canonical layers

The project should avoid conflating two different concepts.

### 1. Canonical billing business model

This is the model proposed in this document. It represents invoice/credit-note **business semantics** independently of Sage, Xero, QuickBooks, SAP, NetSuite and independently of UBL XML syntax.

Vendor adapters target this model.

### 2. Canonical UBL document model

The project roadmap also anticipates a lossless canonical representation capable of full UBL XML ↔ JSON round trips, including namespaces, attributes, extensions and element ordering.

That is a different layer.

The dependency should be:

```text
vendor invoice
     │
     ▼
CanonicalBillingDocument
     │
     ▼
UBL semantic renderer
     │
     ▼
UBL XML / full canonical UBL representation
     │
     ▼
validation
```

The accounting adapters should not need to understand UBL namespace mechanics.

## Draft versus resolved model

A single structure is not quite enough because third-party input frequently contains unresolved references or ambiguous legal semantics.

Use two states:

```text
CanonicalBillingDocumentDraft
        │
        │ enrichment + mapping + semantic resolution
        ▼
CanonicalBillingDocument
```

The **draft** may retain unresolved source tax/unit/reference keys.

The **resolved document** must contain only explicit portable semantics and must satisfy the canonical model's invariants before UBL rendering starts.

This prevents source-specific values such as `TaxCodeRef=7`, `TaxType=OUTPUT2`, `TaxCode=VAT23` or a Sage tax UUID from masquerading as legal standard values.

## Proposed conceptual Go model

The exact package/types should be designed separately, but the model should look broadly like this:

```go
type BillingDocumentKind string

const (
    BillingInvoice    BillingDocumentKind = "invoice"
    BillingCreditNote BillingDocumentKind = "credit-note"
)

type CanonicalBillingDocument struct {
    Kind BillingDocumentKind

    ID           string
    IssueDate    Date
    DueDate      *Date
    TaxPointDate *Date

    CurrencyCode    string
    TaxCurrencyCode string

    BuyerReference         string
    PurchaseOrderReference string
    ContractReference      string

    Seller Party
    Buyer  Party

    Delivery *Delivery
    Payment  []PaymentMeans

    Lines            []BillingLine
    AllowanceCharges []AllowanceCharge
    TaxBreakdown     []TaxBreakdown
    Totals           MonetaryTotals

    Notes []string

    Source SourceProvenance
    Trace  []MappingDecision
}
```

### Party

```go
type Party struct {
    LegalName   string
    TradingName string

    Identifiers       []Identifier
    VATIdentifier     *Identifier
    ElectronicAddress *Identifier

    Address Address
    Contact *Contact
}

type Identifier struct {
    Value    string
    SchemeID string
}

type Address struct {
    Street1     string
    Street2     string
    City        string
    Region      string
    PostalCode  string
    CountryCode string
}
```

Identifiers must always be scheme-aware where the target standard requires a scheme. Vendor record IDs belong in `SourceProvenance`, not in `Party.Identifiers` unless they genuinely are business/legal identifiers.

### Lines

```go
type BillingLine struct {
    ID          string
    Name        string
    Description string

    SellerItemID   string
    BuyerItemID    string
    StandardItemID *Identifier

    Quantity Decimal
    UnitCode string

    Price Price
    NetAmount Decimal

    AllowanceCharges []AllowanceCharge
    Tax              TaxTreatment
}

type Price struct {
    Amount       Decimal
    BaseQuantity Decimal
    BaseUnitCode string
}
```

`UnitCode` must be a target-standard unit code after resolution. Source values such as Sage unit UUIDs, SAP `UoMCode`, NetSuite units or ERP-specific labels are mapping inputs, not final canonical values.

### Allowances and charges

Both document-level and line-level discounts/surcharges must use the same semantic type:

```go
type AllowanceCharge struct {
    IsCharge bool

    Amount     Decimal
    BaseAmount *Decimal
    Percentage *Decimal

    ReasonCode string
    Reason     string

    Tax *TaxTreatment
}
```

This is necessary because products represent discounts differently:

- Xero has line discount fields;
- QuickBooks can use discount lines;
- SAP may use pricing conditions or discount percentages;
- NetSuite has transaction/item discounts.

The source presentation is discarded after its economic meaning is captured.

### Tax treatment

```go
type TaxTreatment struct {
    CategoryCode string
    Rate         Decimal

    ExemptionReasonCode string
    ExemptionReason     string
}

type TaxBreakdown struct {
    Treatment     TaxTreatment
    TaxableAmount Decimal
    TaxAmount     Decimal
}
```

`CategoryCode` represents resolved target semantics. It must never contain an arbitrary source tax-code ID.

The draft model may contain a source-tax structure such as:

```go
type SourceTaxEvidence struct {
    SystemCode string
    Rate       *Decimal
    TaxAmount  *Decimal
    Metadata   map[string]string
}
```

A resolver converts that evidence into a valid `TaxTreatment`, or returns an ambiguity/error.

### Monetary totals

```go
type MonetaryTotals struct {
    LineExtensionAmount Decimal
    AllowanceTotalAmount Decimal
    ChargeTotalAmount    Decimal
    TaxExclusiveAmount   Decimal
    TaxAmount            Decimal
    TaxInclusiveAmount   Decimal
    PrepaidAmount        Decimal
    RoundingAmount       Decimal
    PayableAmount        Decimal
}
```

Do **not** use `float64` for canonical monetary arithmetic.

The current sample-generator `Money` type is intentionally simple and fixed to two decimal places. A production canonical model should use exact decimal arithmetic (or integer + explicit scale) because source prices, exchange rates and some currencies require different precision rules.

### Payment and delivery

```go
type PaymentMeans struct {
    MeansCode string

    PayeeAccountID string
    PayeeAccountName string
    PaymentReference string

    DueDate *Date
    Terms   string
}

type Delivery struct {
    Date    *Date
    Address *Address
}
```

Payment terms and actual payment means should remain distinct because accounting products frequently expose one but not the other.

## Provenance and mapping trace

Conversion should be auditable.

Every imported document should retain source provenance:

```go
type SourceProvenance struct {
    System        string // sage-active, xero, quickbooks-online, sap-b1, sap-s4, netsuite
    SchemaVersion string
    TenantID      string
    SourceID      string
    SourceHash    string
}
```

Important enrichment/mapping decisions should be traceable:

```go
type MappingDecision struct {
    CanonicalPath string
    SourcePath    string
    RuleID        string
    ResolutionSource string
}
```

Examples:

```text
lines[0].tax.categoryCode
  source: LineItems[0].TaxType
  rule: xero-tax-map-ie-v1
  resolved from: tenant tax mapping

buyer.electronicAddress
  source: none
  rule: peppol-buyer-endpoint
  resolved from: customer master data
```

This makes support, regression analysis and compliance review dramatically easier.

---

# Connector versus mapper

Network/API behavior should be kept separate from semantic conversion.

```text
Vendor API
   │
   ▼
Connector
   │ fetch invoice + expand required references
   ▼
immutable SourceSnapshot
   │
   ▼
Mapper (pure/deterministic)
   │
   ▼
CanonicalBillingDocumentDraft
```

Conceptual interfaces:

```go
type SourceConnector interface {
    FetchBillingDocument(ctx context.Context, id string) (SourceSnapshot, error)
}

type BillingDocumentMapper interface {
    Map(snapshot SourceSnapshot) (CanonicalBillingDocumentDraft, []MappingIssue)
}
```

Benefits:

- deterministic unit/regression testing without vendor network calls;
- exact source payload can be retained/hash-pinned for debugging;
- connector authentication/pagination/rate limits do not contaminate business mapping;
- source API changes can be isolated to adapter versions;
- multiple acquisition methods can reuse the same mapper, e.g. webhook, API pull, uploaded JSON export.

---

# Enrichment and semantic resolution

A separate resolver layer should handle facts that are not self-contained in the source invoice:

```text
CanonicalInvoiceDraft
        │
        ├── tenant/company configuration
        ├── customer/supplier master data
        ├── tax-code mappings
        ├── unit mappings
        ├── identifier scheme mappings
        └── target profile requirements
        │
        ▼
CanonicalInvoice
```

This layer should never silently guess.

Useful structured failure categories include:

```text
MISSING_SEMANTIC
AMBIGUOUS_TAX_TREATMENT
UNMAPPED_TAX_CODE
UNMAPPED_UNIT
UNRESOLVED_PARTY
MISSING_LEGAL_IDENTIFIER
MISSING_ELECTRONIC_ADDRESS
UNSUPPORTED_SOURCE_LINE_TYPE
TOTAL_MISMATCH
CURRENCY_MISMATCH
UNSUPPORTED_DOCUMENT_TYPE
```

A failure should identify both the canonical field that could not be produced and the relevant source field/value.

---

# Reconciliation policy

Third-party accounting systems often calculate their own totals and tax. e-ubl21 should not blindly trust them, but it should not discard them either.

The adapter should retain source amounts as evidence while independently computing the resolved canonical totals using exact arithmetic.

For example:

```text
source line total   = 1000.00
canonical computed  = 1000.00
                 → OK

source tax total    = 229.99
canonical computed  = 230.00
                 → conversion/reconciliation error or explicit rounding rule required
```

This will catch source-export defects and mapping errors before a malformed UBL document reaches downstream validators.

---

# API shape

Two integration modes are useful.

## Push mode

The accounting product/integration submits a native document snapshot:

```http
POST /api/v1/invoices/import
Content-Type: application/json
X-Source-System: xero
X-Source-Version: ...
X-Target-Profile: peppol-bis-billing
```

## Pull/connected mode

The client asks e-ubl21 to retrieve a specific document through a configured integration:

```http
POST /api/v1/integrations/xero/invoices/{external-id}/import
```

The connector uses stored integration credentials, creates the immutable source snapshot, and sends it through the same mapper/resolver/renderer pipeline.

The existing asynchronous execution model is a good fit: return an import/validation ID and expose conversion diagnostics and generated UBL through the resulting resource.

---

# Testing strategy

Each adapter should have a deterministic regression corpus containing vendor-native source snapshots and expected canonical output.

For each fixture:

```text
vendor snapshot
      │
      ▼
expected canonical JSON
      │
      ▼
generated UBL XML
      │
      ├── XSD
      ├── EN 16931
      └── Peppol
```

Tests should cover at least:

- standard VAT;
- zero rate;
- exemptions;
- reverse charge;
- intra-community scenarios;
- discounts/allowances;
- charges/freight;
- multiple tax categories;
- currencies and rounding;
- missing master data;
- unknown tax codes;
- unknown units;
- invoice and credit note;
- source totals that disagree with recomputed totals;
- target-profile-specific missing fields.

Mapping rules should be versioned. A change to a tax mapping or semantic resolver should be treated like a standards/corpus change and run through regression review.

---

# Suggested implementation order

The canonical model should be implemented **before** several independent vendor adapters are written.

A sensible progression is:

1. define `CanonicalBillingDocumentDraft` and resolved `CanonicalBillingDocument`;
2. extract/reuse the existing EN 16931 sample-generator rendering knowledge without making production code depend on a sample-only model;
3. build one Sage Active adapter;
4. build Xero as the second adapter and adjust the canonical model only when a genuinely vendor-independent concept is missing;
5. add QuickBooks Online to exercise polymorphic lines and locale-dependent tax behavior;
6. add SAP Business One to exercise enterprise master-data and UDF mapping;
7. add NetSuite to exercise subsidiaries, SuiteTax and extensive custom fields;
8. add SAP S/4HANA last as the enterprise stress test for normalized multi-entity billing and pricing-condition semantics.

After the first three independent adapters, the canonical model should be stable enough that new integrations become mostly source mapping rather than core-model redesign.

---

# Commercial significance

This capability materially changes the product proposition.

Without transformation:

> Send e-ubl21 an electronic invoice and it will validate it.

With third-party integration:

> Send e-ubl21 the invoice produced by your accounting system and it will turn it into a compliant electronic invoice and validate it.

With connected adapters:

> Connect your accounting system to e-ubl21 and e-ubl21 becomes the standards translation layer between your existing accounting workflow and EN 16931 / Peppol e-invoicing.

That moves e-ubl21 from being primarily a validator toward becoming an e-invoicing interoperability and transformation platform.

The research across Sage, Xero, QuickBooks, SAP and NetSuite strongly suggests that a single vendor-neutral canonical billing model is practical. The common business structure is stable; most of the real complexity belongs in source-specific reference expansion, tax/unit mapping, enrichment and reconciliation, exactly where adapters and resolvers can contain it.
