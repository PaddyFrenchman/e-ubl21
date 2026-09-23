# Third-party integration

## Purpose

This document explores how e-ubl21 can accept invoices produced by third-party accounting and ERP products in their native API/data models, transform them into a shared business-semantic model, generate standards-compliant UBL XML, and then run the existing validation pipeline.

The strategic objective is broader than adding one Sage converter. The objective is to make accounting products adapters around a shared canonical billing model:

```text
Sage ──────────────┐
Xero ──────────────┤
QuickBooks ────────┤
SAP Business One ──┤
SAP S/4HANA ───────┼──► Canonical billing model ─► target profile/rendering ─► validation
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
target-profile mapper / renderer
        │
        ▼
UBL 2.1 / other target representation
        │
        ▼
standards validation
```

The hard problem is not XML generation. It is preserving business meaning while translating vendor-specific tax codes, party references, units, discounts, charges, totals, and identifiers into explicit portable semantics that can subsequently be mapped to EN 16931, Peppol, or another national/regional e-invoicing profile.

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

Generating XML is not the difficult part. The difficult part is mapping Sage's business model into portable invoice semantics without inventing information or losing meaning.

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

For an EN 16931 target, the adapter and target-profile mapper ultimately need to resolve the source information into business terms such as:

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

Source identifiers such as `unitOfMeasurementId` or `taxTreatmentId` must never leak into the resolved canonical model as if they were standard identifiers. The Sage adapter must resolve them to normalized unit/tax semantics first.

## Semantic gaps must fail explicitly

A source product may not contain everything necessary to construct a compliant e-invoice.

For example, `vatPercentage = 0` alone is insufficient to identify the legal tax treatment. Zero-rated, exempt, reverse charge, outside-scope and intra-community transactions are not interchangeable.

Likewise, the source invoice may not contain an electronic endpoint identifier required by a particular network or target profile.

Missing semantics may be resolved only from controlled sources:

- the submitted source document;
- tenant/company configuration;
- customer/supplier master data;
- an explicitly configured mapping rule;
- human resolution through a UI or API;
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

`TaxType` is a Xero accounting/tax configuration value, not automatically a portable legal tax classification. The adapter therefore needs a versioned mapping layer from Xero tax configuration to normalized legal e-invoice tax semantics.

The invoice's `ContactID` is also only a reference. A connector normally needs to snapshot the associated contact so the deterministic mapper has legal name, tax number and postal address available.

Electronic endpoint identifiers, legal registration scheme identifiers and target-profile-specific data may need tenant/master-data enrichment.

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

The source can contain subtotal/description/group/discount lines that should not necessarily become one-for-one invoice lines. The mapper must interpret their accounting meaning.

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

SAP Business One uses configurable business-partner, tax-code and unit-of-measure master data. `CardCode`, `TaxCode` and `UoMCode` therefore cannot be copied directly into portable canonical fields without resolution.

User-defined fields are widely used in SAP Business One installations. The adapter should support declarative field mappings for customer-specific UDFs so target-network endpoint IDs, legal identifiers or buyer references can be sourced without custom Go code for every installation.

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

NetSuite's custom fields should be handled through declarative mapping rules where possible. This is likely to be necessary for network participant identifiers, organization registration identifiers and customer-specific references.

The `taxdetails` sublist is useful evidence but the adapter must still resolve NetSuite tax codes/types to portable tax semantics and, when relevant, EN 16931 VAT categories and exemption semantics.

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
| Tax/legal IDs | source/master data | Contact/org | customer/company data | BP/company/UDF | partner/company | entity/subsidiary |
| Unit code | source ID | often implicit/source | item/source | UoMCode | item/UoM | units reference |
| Electronic network endpoint ID | usually enrichment | usually enrichment | usually enrichment | usually UDF/enrichment | master/custom field | custom/master data |
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

The canonical business model is intentionally **global and target-neutral**. EN 16931 and Peppol are target profiles, not the semantic center of the model. This is necessary because the commercial target includes organizations outside the EU and because other jurisdictions can impose different tax schemes, identifiers, network addresses, invoice constraints and reporting requirements.

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
     ├──► EN 16931 / Peppol mapper
     ├──► another national/regional target mapper
     └──► future non-UBL target if required
             │
             ▼
     target document representation
             │
             ▼
          validation
```

The accounting adapters should not need to understand UBL namespace mechanics or a particular country's e-invoicing rules.

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

The **resolved document** must contain only explicit portable semantics and must satisfy the canonical model's invariants before target rendering starts.

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

For global use a party can have multiple legal/tax registrations and multiple electronic addresses. The model must not assume that every jurisdiction uses EU VAT identifiers or that every network uses a Peppol endpoint.

```go
type Party struct {
    LegalName   string
    TradingName string

    Identifiers          []Identifier
    TaxRegistrations     []TaxRegistration
    ElectronicAddresses  []ElectronicAddress

    Address Address
    Contact *Contact
}

type Identifier struct {
    Value    string
    SchemeID string
}

type TaxRegistration struct {
    Jurisdiction string
    TaxScheme    string
    Identifier   Identifier
}

type ElectronicAddress struct {
    Network  string
    Address  Identifier
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

Identifiers must always be scheme-aware where the target standard requires a scheme. Vendor record IDs belong in `SourceProvenance` or the master-data external-ID mapping, not in `Party.Identifiers` unless they genuinely are business/legal identifiers.

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

`UnitCode` must be a portable/target-resolvable unit code after resolution. Source values such as Sage unit UUIDs, SAP `UoMCode`, NetSuite units or ERP-specific labels are mapping inputs, not final canonical values.

### Allowances and charges

Both document-level and line-level discounts/surcharges must use the same semantic type:

```go
type AllowanceCharge struct {
    IsCharge bool

    Amount      Decimal
    BaseAmount  *Decimal
    Percentage  *Decimal

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

The canonical tax model must be jurisdiction-aware rather than hard-wired to EN 16931 VAT categories.

```go
type TaxTreatment struct {
    Jurisdiction string
    TaxScheme    string
    Rate         Decimal

    Classification Identifier

    ExemptionReasonCode string
    ExemptionReason     string
}

type TaxBreakdown struct {
    Treatment     TaxTreatment
    TaxableAmount Decimal
    TaxAmount     Decimal
}
```

For an EN 16931 target, the target-profile mapper resolves the canonical classification into the appropriate VAT category code and business terms. Other jurisdictions can map the same canonical tax treatment differently.

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
    LineExtensionAmount  Decimal
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

    PayeeAccountID   string
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
    CanonicalPath    string
    SourcePath       string
    RuleID           string
    ResolutionSource string
}
```

Examples:

```text
lines[0].tax.classification
  source: LineItems[0].TaxType
  rule: xero-tax-map-ie-v1
  resolved from: tenant tax mapping

buyer.electronicAddresses[0]
  source: none
  rule: target-network-buyer-endpoint
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
        ├── target profile requirements
        └── human/API resolution when necessary
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

## Resolution precedence

Resolution should follow an explicit and auditable order:

```text
1. Explicit value present in the source document
2. Explicit source-system mapping
3. Party/customer/supplier-specific mapping
4. Tenant master data
5. Tenant default, only for fields that are safe to default
6. Human resolution through UI/API
7. Fail conversion
```

Not every field is eligible for tenant defaults. Mechanical conventions such as default currency, source unit mappings, seller identifiers or a default payment means can often be configured safely. Legally ambiguous semantics such as the meaning of a zero tax rate, reverse-charge treatment or an exemption reason must not be guessed from a convenient default unless an administrator has explicitly configured a valid rule for the relevant context.

---

# Master data is a required subsystem

## Why it is necessary

The third-party invoice payload alone is often insufficient to produce a legally and semantically complete e-invoice.

Accounting packages commonly store only references on the invoice itself:

```text
customer ID
supplier/business-partner ID
item ID
tax-code ID
unit-of-measure ID
payment-terms ID
subsidiary/company ID
```

The corresponding legal name, address, tax registrations, electronic addresses, product metadata, tax semantics, unit mappings and other values live in customer, supplier, item, tax and configuration tables elsewhere in the source system.

This is not an unusual edge case. It is a recurring characteristic across Sage, Xero, QuickBooks, SAP and NetSuite. Therefore master-data access is not an optional optimization. It is part of the semantic conversion architecture.

Without master data, the resolver will repeatedly encounter blocking gremlins such as:

```text
UNRESOLVED_PARTY
UNMAPPED_TAX_CODE
UNMAPPED_UNIT
MISSING_LEGAL_IDENTIFIER
MISSING_ELECTRONIC_ADDRESS
```

The system must therefore either have access to the relevant third-party master data at conversion time or maintain its own canonical copy of the information.

## Decision: e-ubl21 maintains canonical master data

The preferred architecture is for e-ubl21 to maintain a canonical tenant-scoped master-data layer rather than depend on synchronous live access to every source ERP for every invoice.

Third-party ERP/accounting systems remain the authoritative accounting systems, but e-ubl21 keeps the data needed to resolve invoice semantics reliably and quickly.

This avoids making invoice conversion dependent on:

- vendor API availability;
- network latency;
- rate limits;
- temporary authentication failures;
- expensive multi-object source lookups;
- changes in external source data after an invoice was submitted.

It also makes conversion deterministic and auditable because the version of the master data used for a conversion can be recorded.

## Ways to populate master data

All of the following acquisition modes should be supported because customers have different security and operational constraints.

### 1. Live connector synchronization

An authorized connector can read customer, supplier, item, tax and configuration data through the vendor API and synchronize the canonical master-data store.

This is convenient for SaaS products such as Xero, QuickBooks or Sage and for enterprise systems where API access is permitted.

### 2. Periodic synchronization

The integration can poll or receive change events and periodically synchronize only changed records.

This avoids querying the source ERP during the invoice hot path while keeping the canonical copy current.

### 3. Client data export/import

Customers can export master data from their system and upload it to e-ubl21, for example:

```text
customers.csv
suppliers.csv
products.csv
tax-codes.csv
units.csv
payment-methods.csv
```

JSON, XML and spreadsheet imports can be added where useful.

This mode is important for customers who do not want to grant e-ubl21 broad API access to their ERP or who use systems with weak integration APIs.

### 4. Manual administration and enrichment

The administration UI/API must allow missing information to be added or corrected directly in e-ubl21, particularly information that does not exist in the accounting package at all, such as a network-specific electronic address.

## Canonical party identity

Third-party record IDs must never become the primary identity of a party inside e-ubl21.

One business can appear under several source-system identities:

```text
e-ubl21 party: party-9f312

Sage customer ID:       742
Xero contact ID:        0a7e...
SAP BusinessPartner:    C10037
NetSuite entity ID:     220
Tax registration:       IE1234567A
Electronic endpoint:    0088:1234567890123
```

The canonical record therefore needs stable internal identity plus external identifiers, conceptually:

```go
type MasterParty struct {
    ID           string
    TenantID     string
    ExternalIDs  []ExternalSystemID

    LegalName    string
    TradingName  string
    Addresses    []Address

    Identifiers          []Identifier
    TaxRegistrations     []TaxRegistration
    ElectronicAddresses  []ElectronicAddress

    DefaultCurrency     string
    DefaultPaymentMeans string

    Version     int64
    UpdatedAt   time.Time
}

type ExternalSystemID struct {
    System string
    Value  string
}
```

The same principle applies to products/items, tax codes, units, payment methods and other source references.

## Master data and mapping data are different

Master data describes entities and facts:

```text
party/customer/supplier
address
tax registration
product/item
payment account
currency
```

Mapping data describes how source-system concepts should be interpreted:

```text
Sage tax code T0       → canonical tax treatment
Xero TaxType X         → canonical tax treatment
SAP unit HOUR          → portable unit code HUR
source payment code 7  → canonical payment-means semantics
```

These should be stored as related but distinct concepts. Mapping rules require their own versions and audit history because changing a mapping can change the legal meaning of subsequently generated invoices.

## Import/onboarding analysis

Master-data import can double as an onboarding diagnostic.

After an initial import e-ubl21 should be able to report, for example:

```text
1,842 customer records imported
37 supplier records imported
12 tax codes detected
8 unit codes detected

Potential resolution problems:
3 unmapped tax codes
2 unmapped units
7 customers missing country
412 customers missing electronic endpoint identifiers for the selected target network
```

This allows semantic gaps to be resolved before the first production invoice enters the system.

## Provenance of master data

Every imported/enriched value should retain sufficient provenance to answer questions such as:

> Why did e-ubl21 put this tax registration or endpoint on this invoice?

A stored value should therefore record, where practical:

```text
source system
external source record ID
source/import version or batch
imported/updated timestamp
human or rule responsible for an override
```

This provenance should flow into the mapping trace of the generated invoice.

---

# Human resolution and configurable defaults

The remaining semantic gremlins should be first-class product objects rather than opaque conversion failures.

A conceptual issue model is:

```go
type ResolutionIssue struct {
    Code         string
    Severity     string
    SourcePath   string
    CanonicalPath string
    CurrentValue any
    Candidates   []CandidateResolution
    Message      string
}
```

For example:

```json
{
  "code": "AMBIGUOUS_TAX_TREATMENT",
  "severity": "blocking",
  "sourcePath": "lines[3].tax_rate",
  "canonicalPath": "lines[3].tax",
  "currentValue": 0,
  "candidates": [
    "zero-rated",
    "exempt",
    "reverse-charge",
    "outside-scope"
  ]
}
```

A web UI can present the ambiguity to an administrator and optionally offer:

```text
[Resolve this invoice only]
[Resolve and remember this mapping for future invoices]
```

The second option turns recurring manual decisions into explicit, tenant-owned configuration.

Large ERP customers must not be forced through a graphical UI. The same issue/resolution contract should be exposed through an API so automated customer workflows can inspect and resolve blocking semantics programmatically.

The resulting operating modes are therefore:

```text
well-configured tenant  → fully automatic conversion
SME/admin workflow      → web resolution UI
enterprise integration  → API-driven resolution
```

---

# Global-market architecture

The commercial target should not be limited to Ireland or the EU.

The canonical billing model and persistence design should therefore assume that:

- tenants can be located anywhere in the world;
- tax schemes vary by jurisdiction;
- businesses can hold multiple tax registrations;
- electronic invoicing networks and participant identifiers vary by country;
- target syntaxes/profiles can differ from EN 16931/Peppol;
- data residency and latency requirements can differ by tenant.

This is why the canonical model above uses jurisdiction-aware tax registrations and potentially multiple electronic addresses rather than a single `VATIdentifier` or `PeppolEndpointID` field.

Target-specific interpretation belongs after canonical resolution:

```text
Sage / Xero / SAP / ...
          │
          ▼
Global Canonical Billing Model
          │
      ┌───┼────────────────┐
      ▼   ▼                ▼
 EN 16931 / Peppol    future national    future other
                       profiles           targets
```

---

# Persistence and cache architecture

## Decision

For this subsystem, and as the intended strategic relational database for e-ubl21 generally, use:

```text
YugabyteDB  → authoritative distributed SQL data store
Valkey      → high-speed disposable cache
NATS        → orchestration and cache-invalidation events
```

Do not add etcd as a business/master-data database.

## Why YugabyteDB

The master-data and mapping workload is relational:

- tenants own parties, products and mappings;
- parties have multiple addresses/identifiers/tax registrations;
- vendor identities map onto canonical identities;
- mappings are versioned and audited;
- billing documents refer to parties, mappings and provenance;
- administrative queries and reporting require joins and indexes.

At the same time, the product is intended for a global market. A single traditional database in one region would introduce unnecessary latency and eventually create a geographic availability bottleneck.

YugabyteDB is selected because it combines a PostgreSQL-compatible YSQL interface with distributed storage, replication and multi-region/data-locality capabilities. The Go application can retain a familiar SQL/PostgreSQL programming model while the database supplies horizontal distribution underneath.

The architecture should not mean that every tenant's data is synchronously written across every continent. Prefer **tenant locality**:

```text
Tenant A → home region Europe
Tenant B → home region USA
Tenant C → home region Asia-Pacific
```

The logical database can remain globally distributed while a tenant's data is placed near the region where it is normally processed, subject to availability and residency requirements.

Typical durable data includes:

```text
tenants
integration connections/configuration
parties
party external IDs
party addresses
party tax registrations
party electronic addresses
products/items
product external IDs
tax mappings
unit mappings
payment mappings
semantic-resolution rules
mapping versions/audit history
source/import provenance
billing-document metadata/status
```

Raw invoice/XML payload storage can remain a separate object-storage concern as described elsewhere in the project architecture.

## Why not etcd

etcd is useful for small strongly consistent control-plane data, configuration, coordination, leases, watches and leader election. It is not the preferred primary business database for a growing relational master-data domain.

Using etcd here would make relational querying, secondary indexes, historical audit models, large tenant datasets and global sharding significantly more awkward. It would also mix control-plane responsibilities with business-data persistence.

If etcd is ever introduced, keep it in the control plane rather than making it authoritative for parties, products, mappings or billing metadata.

## Valkey cache

Valkey is the selected cache. It is open source and commercially usable under a permissive license.

The critical rule is:

> YugabyteDB is authoritative. Valkey is disposable acceleration.

Loss of the Valkey cache must reduce performance, not correctness or durability.

The resolver will perform many repetitive lookups such as:

```text
party by tenant + source system + external ID
party by tenant + tax registration
product by tenant + source system + external ID
tax mapping by tenant + source system + source tax code
unit mapping by tenant + source system + source unit code
```

These are ideal cache candidates because master data is read frequently and changes relatively rarely.

Representative cache keys could be:

```text
party:{tenant}:{source}:{external-id}
party-tax:{tenant}:{jurisdiction}:{tax-id}
product:{tenant}:{source}:{external-id}
taxmap:{tenant}:{source}:{tax-code}
unitmap:{tenant}:{source}:{unit-code}
```

Cached objects should carry a record/mapping version and a TTL. TTL is a safety net rather than the primary invalidation mechanism.

## L1 process cache

A small in-process Go L1 cache can sit in front of Valkey for the hottest mappings if measurement shows it is useful:

```text
resolver
   │
   ▼
L1 Go memory cache
   │ miss
   ▼
regional Valkey
   │ miss
   ▼
YugabyteDB
```

The L1 layer is optional and should be introduced only when profiling demonstrates value. The architecture must work correctly with only Valkey + YugabyteDB.

## NATS-driven invalidation

Because e-ubl21 already uses NATS/JetStream, use NATS to publish cache-invalidation events when canonical master data or mappings change.

Example:

```text
master-data update
       │
       ▼
YugabyteDB commit
       │
       ▼
NATS event
masterdata.<tenant>.party.changed
       │
       ├──► Europe resolver: evict
       ├──► US resolver: evict
       └──► APAC resolver: evict
```

The write to YugabyteDB occurs first. Cache invalidation follows the durable commit. Consumers evict stale entries; the next request repopulates them from the authoritative database.

Cache events must include sufficient identity/version information to avoid an old delayed invalidation accidentally overwriting or reintroducing stale state.

## Consistency principle

For legal/semantic mapping data, correctness wins over cache availability.

If a cache entry is absent or cannot be trusted, read YugabyteDB. If the database cannot provide the authoritative mapping, fail or defer resolution rather than guess from stale/local data.

---

# Migration from existing PostgreSQL support to YugabyteDB

## Historical migration baseline

This section records the pre-YugabyteDB migration baseline. The current on-prem bootstrap installs YugabyteDB/YSQL; PostgreSQL and CloudNativePG references below describe legacy installations and the guarded upgrade path only.

PostgreSQL also appears in:

- `deploy/on-prem/scripts/install-on-prem.sh`;
- `deploy/on-prem/README.md`;
- `deploy/on-prem/deployment-on-prem.md`;
- `deployment-target.md`;
- `.github/roadmap-0.md`;
- `README.md`;
- `docs/SERVER-FUNCTIONALITY-AND-INVOICE-PLATFORM-ARCHITECTURE.md`;
- `e-invoice/E-INVOICE.md`;
- AWS/deployment idea documents;
- the then-future issue #89, which has since been retargeted to YugabyteDB/YSQL metadata persistence.

A repository search at the time of this decision did **not** find application Go code importing `database/sql` or `pgx`, which indicates that PostgreSQL is currently much more deeply represented in deployment/documentation/future architecture than in implemented application persistence code.

That makes this the preferred time to change the database decision. The migration cost is substantially lower now than after a large persistence layer has been written against PostgreSQL-specific behavior.

## Migration goal

Replace PostgreSQL/CloudNativePG as the official relational database platform with YugabyteDB while retaining the PostgreSQL-compatible application programming model through YSQL.

Conceptually:

```text
Before
------
e-ubl21
   │
   └── PostgreSQL / CloudNativePG

After
-----
e-ubl21
   │
   └── YugabyteDB YSQL
```

This does **not** mean abandoning the PostgreSQL ecosystem. The application should continue to use ordinary SQL and PostgreSQL-compatible drivers/tooling where they work with YugabyteDB.

## Application data-access policy

When the persistence layer is implemented:

- use YugabyteDB YSQL as the test and production database;
- prefer `pgx` or a similarly mature PostgreSQL-compatible Go driver;
- use parameterized SQL and explicit transactions;
- keep the repository/data-access layer isolated from business logic;
- avoid unnecessary PostgreSQL-specific extensions/functions unless verified on YugabyteDB;
- do not develop against PostgreSQL locally and assume YugabyteDB compatibility later;
- run integration tests against the actual supported YugabyteDB version;
- pin and regression-test schema/migration behavior.

PostgreSQL compatibility is a major advantage of YugabyteDB, but compatibility must not be treated as identity. SQL syntax, locking behavior, extensions, administrative operations and performance characteristics can differ.

## Database configuration

Avoid perpetuating PostgreSQL-specific configuration names in new application code.

Prefer generic application settings such as:

```text
EUBL21_DB_HOST
EUBL21_DB_PORT
EUBL21_DB_NAME
EUBL21_DB_USER
EUBL21_DB_PASSWORD
EUBL21_DB_SSLMODE
```

or a generic connection URL:

```text
EUBL21_DATABASE_URL
```

Deployment-specific Yugabyte settings can use a separate `YB_` namespace where they control the database cluster rather than the application connection.

This keeps the Go application coupled to a SQL contract rather than a deployment product name.

## Schema migration strategy

Schema migrations should be treated as product artifacts and tested on YugabyteDB.

The migration framework must support:

- ordered forward migrations;
- reproducible fresh-database creation;
- safe rolling upgrades where required;
- tenant-aware indexes/constraints;
- large-table migration planning;
- rollback/repair procedures or explicit forward-fix policy;
- compatibility testing against the supported YugabyteDB release.

Any chosen migration tool must be validated against YSQL before adoption.

## On-premises deployment migration

The current on-prem installer installs/configures YugabyteDB. The following retired names remain only to explain legacy detection and upgrade safety.

The retired concepts included:

```text
CLOUDNATIVEPG_CHART_VERSION
POSTGRES_STORAGE_SIZE
install_cloudnativepg
install_postgresql_cluster
e-ubl21-postgres
```

were replaced with YugabyteDB equivalents.

The replacement must cover:

- Helm/operator or supported Kubernetes installation method;
- storage sizing;
- service discovery;
- credentials/secrets;
- TLS;
- readiness/health checks;
- backups/restores;
- upgrades;
- observability;
- persistence-volume requirements.

For the small single-server on-premises target, a single-node YugabyteDB configuration can preserve the simple appliance model. It does not provide the same fault tolerance as a multi-node deployment, but it keeps one database technology across development, small on-prem installations and global SaaS.

Recommended deployment tiers:

```text
Development / small on-prem
    YugabyteDB single-node

Production on-prem HA
    YugabyteDB 3+ nodes

Cloud / global SaaS
    YugabyteDB multi-node / multi-region
```

## Cloud deployment migration

Existing design references to:

```text
Aurora PostgreSQL / RDS
Azure Database for PostgreSQL
PostgreSQL / CloudNativePG
```

should be reviewed and replaced by the selected YugabyteDB deployment model so the same database semantics are exercised in all supported environments.

Avoid supporting a matrix in which local/on-prem uses PostgreSQL, AWS uses Aurora, Azure uses Azure PostgreSQL and global SaaS uses YugabyteDB. That multiplies persistence behavior, operational procedures and regression requirements for little product benefit.

The target principle is:

```text
one logical database technology
        │
        ├── development
        ├── on-premises
        ├── AWS
        ├── Azure
        └── global SaaS
              │
              ▼
         YugabyteDB
```

## Data migration when PostgreSQL data exists

At present the project appears to have deployment support/planning rather than a mature PostgreSQL-backed application dataset. Nevertheless, the migration design should cover installations that may already have PostgreSQL data by the time this change is implemented.

The safe migration pattern is:

```text
1. freeze or quiesce writes
2. take verified PostgreSQL backup
3. create/upgrade YugabyteDB schema
4. copy data using a tested migration path
5. verify row counts, constraints and critical checksums
6. run application-level reconciliation/integration tests
7. switch connection endpoint to YugabyteDB
8. retain rollback backup until acceptance window closes
```

If a zero/minimal-downtime migration later becomes necessary, design a separate replication/cutover plan rather than complicating the first migration prematurely.

## Tests required for migration acceptance

The PostgreSQL-to-YugabyteDB architecture change is complete only when:

- fresh installs use YugabyteDB instead of PostgreSQL/CloudNativePG;
- all persistence integration tests run against YugabyteDB;
- schema migrations work from an empty database;
- backup/restore procedures are demonstrated;
- tenant isolation tests pass;
- concurrency/idempotency tests pass;
- billing/master-data queries and indexes are exercised at representative volume;
- Valkey can be completely flushed without changing functional correctness;
- NATS invalidation repopulates caches correctly after master-data changes;
- documentation no longer presents PostgreSQL as the intended production database where YugabyteDB is now the decision.

## Follow-up repository work

This design document records the decision but does not itself change all deployment/runtime files.

The migration implementation updated the installer, manifests and active architecture. Issue #89 now targets YugabyteDB-based future metadata persistence.

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

This will catch source-export defects and mapping errors before a malformed target document reaches downstream validators.

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

The existing asynchronous execution model is a good fit: return an import/validation ID and expose conversion diagnostics and generated target document through the resulting resource.

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
target-profile rendering
      │
      ▼
generated e-invoice
      │
      └── target validation
```

For EN 16931 / Peppol fixtures the target validation remains:

```text
UBL XML
  ├── XSD
  ├── EN 16931
  └── Peppol
```

Tests should cover at least:

- standard tax;
- zero rate;
- exemptions;
- reverse charge where applicable;
- cross-border scenarios;
- discounts/allowances;
- charges/freight;
- multiple tax categories;
- currencies and rounding;
- missing master data;
- unknown tax codes;
- unknown units;
- invoice and credit note;
- source totals that disagree with recomputed totals;
- target-profile-specific missing fields;
- master-data import/update behavior;
- cache invalidation and stale-version handling;
- regional/tenant data-locality policy where deployed globally.

Mapping rules should be versioned. A change to a tax mapping or semantic resolver should be treated like a standards/corpus change and run through regression review.

---

# Suggested implementation order

The canonical model should be implemented **before** several independent vendor adapters are written.

A sensible progression is:

1. establish YugabyteDB as the intended relational persistence layer and remove PostgreSQL/CloudNativePG from new architecture work;
2. introduce Valkey and define NATS-based cache invalidation semantics;
3. define canonical master-data records and import/synchronization contracts;
4. define `CanonicalBillingDocumentDraft` and resolved `CanonicalBillingDocument` as target-neutral global business models;
5. extract/reuse the existing EN 16931 sample-generator rendering knowledge without making production code depend on a sample-only model;
6. build the semantic resolver and structured `ResolutionIssue` contract;
7. build one Sage Active adapter;
8. build Xero as the second adapter and adjust the canonical model only when a genuinely vendor-independent concept is missing;
9. add QuickBooks Online to exercise polymorphic lines and locale-dependent tax behavior;
10. add SAP Business One to exercise enterprise master-data and UDF mapping;
11. add NetSuite to exercise subsidiaries, SuiteTax and extensive custom fields;
12. add SAP S/4HANA last as the enterprise stress test for normalized multi-entity billing and pricing-condition semantics.

After the first three independent adapters, the canonical model should be stable enough that new integrations become mostly source mapping rather than core-model redesign.

---

# Commercial significance

This capability materially changes the product proposition.

Without transformation:

> Send e-ubl21 an electronic invoice and it will validate it.

With third-party integration:

> Send e-ubl21 the invoice produced by your accounting system and it will turn it into a compliant electronic invoice and validate it.

With connected adapters and semantic master data:

> Connect your accounting system to e-ubl21 and e-ubl21 becomes the standards translation layer between your existing accounting workflow and the e-invoicing requirements of the destination market/network.

That moves e-ubl21 from being primarily a validator toward becoming a global e-invoicing interoperability and transformation platform.

The research across Sage, Xero, QuickBooks, SAP and NetSuite strongly suggests that a single vendor-neutral canonical billing model is practical. The common business structure is stable; most of the real complexity belongs in source-specific reference expansion, master-data enrichment, tax/unit mapping, semantic resolution, reconciliation and target-profile mapping, exactly where adapters, resolvers and profile renderers can contain it.
