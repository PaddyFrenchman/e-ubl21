# Third-party integration

This document captures ideas for allowing external accounting, ERP, and invoicing products to submit documents in their own native formats and have e-ubl21 transform them into standards-compliant electronic invoices before running the existing validation pipeline.

The long-term objective is to make third-party products adapters around a shared canonical invoice model rather than embedding vendor-specific semantics into the core validation service.

## Sage

### Product opportunity

Supporting Sage invoice input is very feasible and would not require a redesign of e-ubl21. Architecturally, it is a natural extension of the existing product.

Today the public flow is broadly:

```text
UBL XML
   ↓
e-ubl21
   ↓
validation pipeline
```

The proposed Sage integration adds a transformation layer in front:

```text
Sage JSON/XML
     │
     ▼
┌──────────────────────┐
│ Sage adapter         │
│ Sage → canonical     │
└──────────┬───────────┘
           ▼
┌──────────────────────┐
│ Canonical Invoice    │
│ business model       │
└──────────┬───────────┘
           ▼
┌──────────────────────┐
│ UBL generator        │
└──────────┬───────────┘
           ▼
       UBL XML
           │
           ▼
    Existing e-ubl21
 validation pipeline
           │
           ▼
 XSD → EN16931 → Peppol
```

The project already contains important foundations for this approach. The roadmap anticipates a canonical UBL document model, XML/JSON conversion, creation/import endpoints, and serialization into UBL representations. The existing `createsamples/en16931` package also contains an embryonic business-level invoice model and UBL rendering logic that demonstrate the core transformation concept.

### Core difficulty: semantic mapping

Generating XML is not the difficult part. The hard part is mapping a third-party product's business model into EN 16931 / Peppol semantics without inventing information or losing meaning.

For example, Sage might provide:

```json
{
  "invoice_number": "INV-1024",
  "customer": {},
  "lines": [
    {
      "description": "Consulting",
      "quantity": 10,
      "unit_price": 100,
      "tax_rate": 23
    }
  ]
}
```

The adapter would need to map those fields to EN 16931 business terms, for example:

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

Once a valid canonical invoice has been produced, the existing UBL generation and validation capabilities can take over.

### Semantic gaps must fail explicitly

A source product may not carry enough information to construct a compliant e-invoice automatically.

For example, `tax_rate = 0` does not identify the legal VAT treatment. EN 16931 may need to distinguish between zero-rated, exempt, reverse charge, outside scope, intra-community supply, and other cases. These map to different VAT category codes and may require exemption reasons or additional fields.

Likewise, Sage data may not contain an electronic endpoint identifier required by a specific Peppol profile.

The converter must never invent missing legal or business information. Missing required semantics should instead be resolved from one of the following sources:

- the submitted Sage document;
- tenant/company configuration;
- customer or supplier master data;
- explicit mapping rules configured for that integration;
- or a structured conversion error returned to the caller.

### Adapter architecture

Sage should be implemented as one adapter, not as special behavior inside the core invoice service.

A possible package layout is:

```text
/internal/import
    canonical.go

/internal/import/adapters
    sage/
    xero/
    quickbooks/
    sap/
    netsuite/
```

The conceptual adapter interface could be as small as:

```go
type InvoiceAdapter interface {
    Parse([]byte) (*CanonicalInvoice, error)
}
```

The architecture then becomes:

```text
Sage ─────────┐
Xero ─────────┤
QuickBooks ───┤
SAP ──────────┼──► CanonicalInvoice ─► UBL ─► validation
Custom JSON ──┤
CSV ──────────┘
```

This keeps vendor-specific field names, API versions, quirks, and defaults outside the standards engine.

### API options

A Sage-specific endpoint would be possible:

```http
POST /api/v1/convert/sage
Content-Type: application/json
```

However, a generic import API is preferable:

```http
POST /api/v1/invoices/import
Content-Type: application/vnd.sage.invoice+json

X-Source-System: sage
X-Source-Version: ...
X-Target-Profile: peppol-bis-billing
```

A synchronous response could resemble:

```json
{
  "status": "valid",
  "sourceFormat": "sage",
  "targetFormat": "ubl-2.1",
  "profile": "peppol-bis-billing",
  "documentId": "...",
  "xml": "..."
}
```

Alternatively, the existing asynchronous architecture can be reused so the import endpoint returns a document/job identifier and the client later retrieves the conversion and validation result.

### Canonical model recommendation

The current `createsamples/en16931.InvoiceDocument` should not become the permanent production canonical invoice model unchanged.

It is a good starting point, but it was designed for deterministic EN 16931 sample generation and deliberately represents a constrained subset. For example, the current sample model simplifies areas such as document-level allowances/charges and prepaid/rounding amounts.

A production model should be extracted into a neutral package, for example:

```text
invoice/
    model.go
    calculate.go
    validate.go

ubl/
    invoice_renderer.go

adapters/
    sage/
```

The sample generator could then consume the same production invoice model rather than the production service depending on a test/sample model.

The desired dependency direction is therefore:

```text
Sage adapter ─┐
Xero adapter ─┤
API JSON ─────┼──► canonical invoice model
Samples ──────┘             │
                            ▼
                       UBL renderer
                            │
                            ▼
                       validation
```

### Suggested implementation phases

#### Phase 1 — Sage proof of concept

Support one well-defined Sage invoice payload and Invoice only.

Deliverables:

- define the canonical invoice model;
- implement Sage → canonical mapping;
- reuse/extract the existing UBL invoice rendering logic;
- generate UBL 2.1 Invoice XML;
- pass generated XML through XSD and EN 16931 validation;
- return structured mapping errors for missing mandatory semantics;
- add deterministic fixtures and regression tests.

Estimated effort: roughly **2–3 weeks** for a solid first implementation when the Sage source data contains the required EN 16931 information.

#### Phase 2 — Production Sage connector

Expand to:

- Invoice and CreditNote;
- discounts and allowances;
- document-level charges;
- multiple VAT categories and legal VAT treatments;
- tenant defaults and master-data enrichment;
- Peppol-specific identifiers and output;
- source schema/API version handling;
- robust structured conversion diagnostics;
- comprehensive regression corpus.

Estimated effort: roughly **4–8 weeks**, depending on Sage variants and API/source-format coverage.

#### Phase 3 — Generic third-party adapter framework

Generalize the infrastructure so Sage is simply the first connector.

Potential follow-on adapters include:

- Xero;
- QuickBooks;
- SAP;
- NetSuite;
- CSV/custom JSON mappings;
- customer-specific ERP exports.

Estimated effort for a reusable adapter framework: roughly **6–12 weeks** in total, with later connectors potentially requiring only several days to a couple of weeks depending on source complexity.

### Commercial significance

This capability changes the product proposition materially.

Without transformation:

> Send e-ubl21 an electronic invoice and it will validate it.

With third-party integration:

> Send e-ubl21 the invoice produced by your accounting system and it will turn it into a compliant electronic invoice and validate it.

That moves e-ubl21 from being primarily a validator toward becoming an e-invoicing interoperability and transformation platform.

This capability should therefore be considered a strategically important product extension once the core standards/Peppol roadmap is sufficiently stable.
