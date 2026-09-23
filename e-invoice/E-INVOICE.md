E-INVOICE
============================

An **e-invoice validator** answers a deceptively simple question:

> “If I send this electronic invoice, will the recipient/accounting platform/tax system accept it?”

The important distinction is that an e-invoice is not merely a PDF sent by email. In the EU context it is a **structured electronic document** conforming to a defined semantic model such as **EN 16931**, commonly represented using formats such as UBL or UN/CEFACT CII. ([European Commission][1])

For you, the interesting product would be an **API-first validation service**.

## What actually gets validated

Imagine somebody sends you this:

```http
POST /api/v1/validate
Content-Type: application/xml
```

with a UBL invoice.

Your service runs it through several layers:

```text
                invoice.xml
                     │
                     ▼
             XML well-formed?
                     │
                     ▼
               XSD validation
                     │
                     ▼
            EN 16931 validation
                     │
                     ▼
             Peppol BIS rules
                     │
                     ▼
            Country-specific rules
                     │
                     ▼
             Arithmetic/VAT checks
                     │
                     ▼
               Validation report
```

### Layer 1: XML

You detect basic nonsense:

```xml
<Invoice>
    ...
</InvoiceBROKEN>
```

That's trivial.

### Layer 2: Schema

You verify that the document obeys the UBL/CII structure.

For example:

```text
Invoice
 ├── Supplier
 ├── Customer
 ├── InvoiceNumber
 ├── IssueDate
 ├── Currency
 ├── TaxTotal
 ├── InvoiceLines
 └── PayableAmount
```

### Layer 3: EN 16931

This is where it gets interesting.

The European standard defines the semantic requirements for an electronic invoice. The official CEN validation artefacts are implemented using **Schematron**, with current artefacts supporting both UBL 2.1 and Cross Industry Invoice. ([GitHub][2])

You start checking things such as:

```text
✓ supplier name exists
✓ invoice number exists
✓ invoice date exists
✓ currency valid
✓ VAT categories valid
✓ totals mathematically consistent
✓ payable amount valid
```

This is much more than an XML schema.

XML can be structurally valid while the **invoice itself is logically invalid**.

For example:

```text
Subtotal                 €100
VAT 20%                   €20
Total                    €137
```

Perfectly valid XML.

Nonsensical invoice.

Your validator catches it.

## Then comes Peppol

Peppol BIS Billing 3 adds another set of business rules on top of the EN 16931 model. OpenPeppol publishes both the specifications and downloadable Schematron validation artefacts. ([Peppol Docs][3])

You might therefore return:

```json
{
  "valid": false,
  "syntax": "UBL-2.1",
  "profile": "PEPPOL-BIS-BILLING-3",
  "errors": [
    {
      "code": "BR-CO-10",
      "severity": "fatal",
      "message": "Sum of invoice line net amounts is incorrect"
    },
    {
      "code": "PEPPOL-EN16931-R003",
      "severity": "warning",
      "message": "Buyer reference or purchase order reference expected"
    }
  ]
}
```

That API is the product.

And there is a very real maintenance requirement here: Peppol validation rules and code lists are versioned. The May 2026 Peppol Billing release became mandatory on **17 August 2026**, literally yesterday relative to today, 18 August 2026. ([Peppol Docs][4])

That's actually **good news commercially**.

Software vendors don't want to track all those changes themselves.

They pay you.

## Country-specific rules are where money appears

Suppose your customer asks:

> “Is this invoice valid in Germany?”

versus:

> “Can I send this invoice to an Italian customer?”

versus:

> “Is this acceptable through Peppol?”

Those are different questions.

Peppol explicitly supports country-specific validation rules based on the supplier's country, layered on top of the common Billing specification. ([Peppol Docs][5])

So your API could eventually expose:

```http
POST /validate?profile=peppol
POST /validate?country=DE
POST /validate?country=FR
POST /validate?country=NL
POST /validate?country=BE
```

This becomes much harder for an accounting-software developer to implement internally.

That complexity is your moat.

## Why this market is becoming interesting

The EU's **VAT in the Digital Age**, ViDA, package was formally adopted in March 2025. From **1 July 2030**, digital reporting requirements for intra-EU cross-border B2B transactions will be based on mandatory electronic invoicing. ([Taxation and Customs Union][6])

Member states can also impose domestic electronic-invoicing requirements, so Europe is moving toward a landscape where accounting systems increasingly need to generate and validate structured invoices rather than PDFs. The Commission now publishes country factsheets specifically covering B2B e-invoicing developments across the EU and EEA. ([European Commission][7])

That's the business opportunity.

You're selling the plumbing needed by:

**ERP developers, accounting SaaS companies, invoicing products, payroll/accounting firms, e-commerce platforms, procurement systems and custom enterprise software.**

Not individual consumers.

## I would make the product ridiculously simple

Something like:

### **InvoiceCheck API**

```text
POST /v1/validate
```

Input:

```xml
<Invoice ...>
   ...
</Invoice>
```

Output:

```json
{
    "valid": true,
    "standard": "EN16931",
    "syntax": "UBL",
    "profile": "PEPPOL-BIS-3",
    "country": "IE",
    "validation_version": "2026-05",
    "errors": [],
    "warnings": []
}
```

And:

```text
POST /v1/validate/file
GET  /v1/standards
GET  /v1/profiles
GET  /v1/countries
```

Eventually:

```text
POST /v1/convert
```

could convert:

```text
JSON
  ↓
EN16931
  ↓
UBL
```

That may actually become more valuable than validation.

A developer sends:

```json
{
  "invoiceNumber": "INV-1234",
  "date": "2026-08-18",
  "supplier": {...},
  "customer": {...},
  "lines": [...]
}
```

You return compliant UBL.

Now you've removed XML completely from the customer's application.

Very attractive.

## There are really three products hiding here

You could sell:

```text
1. VALIDATE
   Is this invoice compliant?

2. GENERATE
   JSON → compliant e-invoice

3. CONVERT
   UBL ↔ CII ↔ national profiles
```

Then eventually:

```text
4. DELIVER
   Send through Peppol
```

Number 4 is significantly more operationally involved.

I would **not start there**.

Stay away from being the transmission network initially.

Be the validator.

Much less monitoring.

## Your Go architecture

This product suits Go extremely well.

```text
                  Internet

                     │
               Cloudflare/nginx
                     │
                     ▼
              ┌────────────┐
              │   Go API   │
              └──────┬─────┘
                     │
             ┌───────▼────────┐
             │ Validation Core│
             └───────┬────────┘
                     │
        ┌────────────┼────────────┐
        ▼            ▼            ▼
       XSD       EN16931       Peppol
     validator   Schematron    Schematron
        │            │            │
        └────────────┼────────────┘
                     ▼
               JSON result

                    │
             ┌──────▼──────┐
             │ YugabyteDB  │
             └─────────────┘
```

The official EN 16931 project already publishes precompiled XSLT alongside the Schematron sources, which gives you a practical way to consume authoritative validation rules rather than reimplementing hundreds of them manually. ([GitHub][8])

Go could orchestrate:

```text
parse
↓
detect profile
↓
select validation ruleset
↓
run XSD
↓
run Schematron/XSLT
↓
normalize errors
↓
return JSON
```

You'd probably embed the rule sets directly into the Go binary with:

```go
//go:embed rules/*
var rulesFS embed.FS
```

Version every ruleset:

```text
EN16931/1.3.16
Peppol/2026-05
DE/2026-01
FR/2026-09
```

That is important because rules change over time. The EN 16931 official validation artefacts themselves are currently at **v1.3.16**, dated 10 April 2026. ([GitHub][2])

## Pricing could be almost completely automatic

I'd do API credits:

| Plan       | Validations | Price |
| ---------- | ----------: | ----: |
| Developer  |   500/month |  Free |
| Starter    |      10,000 |   €19 |
| Business   |     100,000 |   €79 |
| Pro        |   1,000,000 |  €299 |
| Enterprise |      Custom | €500+ |

Your marginal cost per validation should be tiny.

You aren't storing giant videos.

No GPUs.

No AI bill.

No human moderation.

No customer data entry.

Just:

```text
XML → CPU → JSON
```

Very clean economics.

## There's another clever revenue stream

Offer a web interface:

**Drop your invoice here.**

```text
invoice.xml
     ↓
████████████████
     ↓
❌ INVALID

3 Errors
2 Warnings
```

Then explain:

```text
BR-CO-15
Invoice total including VAT is incorrect.

Expected: €1,242.63
Actual:   €1,243.63

Difference: €1.00
```

Free for occasional use.

Underneath:

> **Need automated validation? Get an API key.**

That's your developer acquisition funnel.

Google can index pages explaining every validation error:

```text
/peppol/errors/BR-CO-15
/peppol/errors/BR-CO-10
/peppol/errors/BR-CL-03
...
```

Each becomes a little SEO fishing net.

People Google an obscure Peppol error at 16:53 on Friday afternoon, land on your site, validate their invoice, discover your API and perhaps subscribe.

That's considerably better acquisition economics than buying ads.

## The main risk

This is **not zero maintenance**.

Your workload isn't running servers.

It is keeping:

```text
EN16931
Peppol
code lists
country rules
```

up to date.

There are even active issues in the official EN 16931 validation artefact repository involving discrepancies and missing checks, illustrating why serious implementers need version-aware validation rather than assuming there is one eternal ruleset. ([GitHub][9])

But notice what that means commercially:

**The annoying part is precisely what customers are paying you to absorb.**

That's a healthier SaaS proposition than selling something they could reproduce with a weekend script.

### What I would build first

I would narrow the MVP to:

**UBL 2.1 + EN 16931 + Peppol BIS Billing 3.0 validation API.**

Nothing national initially.

No invoice delivery.

No accounting.

No Peppol Access Point.

No AI.

Then:

```text
Phase 1
UBL + EN16931 + Peppol
        ↓
Phase 2
Web validator + API
        ↓
Phase 3
JSON → UBL generation
        ↓
Phase 4
Germany / France / Belgium etc.
        ↓
Phase 5
conversion between formats
```

Between this and **SiteSentinel**, I think the e-invoice product has a **higher technical moat and potentially much larger B2B revenue per customer**, while SiteSentinel would probably be easier to sell initially.

E-invoice validation is more of a **developer infrastructure business**. That fits your background particularly well.

[1]: https://ec.europa.eu/digital-building-blocks/sites/spaces/DIGITAL/pages/467108950/EN%2B16931%2Bcompliance?utm_source=chatgpt.com "EN 16931 compliance - European Commission"
[2]: https://github.com/ConnectingEurope/eInvoicing-EN16931?utm_source=chatgpt.com "ConnectingEurope/eInvoicing-EN16931 - Validation artefacts"
[3]: https://docs.peppol.eu/poacc/billing/3.0/?utm_source=chatgpt.com "Peppol BIS Billing 3.0 - May 2026 Release"
[4]: https://docs.peppol.eu/poacc/billing/3.0/upcoming/release-notes/?utm_source=chatgpt.com "Release notes for Peppol BIS Billing v3"
[5]: https://docs.peppol.eu/poacc/billing/3.0/national-rules/?utm_source=chatgpt.com "Peppol methodolgy for national rules based on Sellers country"
[6]: https://taxation-customs.ec.europa.eu/news/adoption-vat-digital-age-package-2025-03-11_en?utm_source=chatgpt.com "Adoption of the VAT in the Digital Age package"
[7]: https://ec.europa.eu/digital-building-blocks/sites/spaces/DIGITAL/pages/467108874/eInvoicing%2BCountry%2BFactsheets%2Bfor%2Beach%2BMember%2BState%2Band%2Bother%2Bcountries?utm_source=chatgpt.com "eInvoicing Country Factsheets for each Member State and ..."
[8]: https://github.com/CenPC434/validation/releases?utm_source=chatgpt.com "Releases · ConnectingEurope/eInvoicing-EN16931"
[9]: https://github.com/ConnectingEurope/eInvoicing-EN16931/issues?utm_source=chatgpt.com "Issues · ConnectingEurope/eInvoicing-EN16931"
