# UBL 2.1 and Peppol

## Overview

Peppol and **UBL 2.1** are closely related, but they are not the same thing.

The simplest model is:

```text
UBL 2.1
   ↓
defines the XML document structures

EN 16931
   ↓
defines the semantic rules for European electronic invoices

Peppol BIS Billing 3.0
   ↓
selects and constrains UBL 2.1
adds Peppol-specific business rules
adds identifiers, code lists and transport expectations
```

A **Peppol invoice is usually a UBL 2.1 invoice with a much stricter rulebook applied to it**.

---

## 1. UBL 2.1 is the document vocabulary

OASIS UBL 2.1 defines XML schemas for many business documents, including:

```text
Invoice
CreditNote
Order
OrderResponse
DespatchAdvice
ReceiptAdvice
Catalogue
Quotation
RemittanceAdvice
FreightInvoice
...
```

UBL provides the XML structure and reusable elements such as:

```xml
<cac:AccountingSupplierParty>
<cac:AccountingCustomerParty>
<cac:InvoiceLine>
<cbc:DocumentCurrencyCode>
<cac:TaxTotal>
<cac:LegalMonetaryTotal>
```

UBL is deliberately broad. It is intended to support many industries and business processes.

Because of this, other standards can take UBL and define a more restrictive profile for a specific purpose.

---

## 2. Peppol uses a constrained subset of UBL

Peppol BIS Billing 3.0 uses:

- **UBL 2.1 Invoice**
- **UBL 2.1 CreditNote**

However, being valid against the UBL XSD does **not** mean that an invoice is Peppol-compliant.

For example:

```text
invoice.xml
   │
   ├── XML well-formed             ✓
   │
   ├── UBL 2.1 XSD valid           ✓
   │
   ├── EN 16931 valid              ✗
   │
   └── Peppol BIS valid            ✗
```

This distinction is fundamental.

UBL permits considerably more than Peppol allows.

Peppol effectively says:

> For this specific European invoicing process, use these UBL elements, with these cardinalities, codes, identifiers and additional business rules.

---

## 3. Peppol identifiers

Peppol invoices normally contain identifiers such as:

```xml
<cbc:CustomizationID>
urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0
</cbc:CustomizationID>

<cbc:ProfileID>
urn:fdc:peppol.eu:2017:poacc:billing:01:1.0
</cbc:ProfileID>
```

These identify:

- the customization/profile being used;
- the Peppol business process.

These values are not simply generic UBL requirements.

---

## 4. Validation must therefore be layered

For the UBL 2.1 validator project, validation should be implemented as a pipeline:

```text
                   XML document
                        │
                        ▼
              ┌──────────────────┐
              │ XML well-formed? │
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ UBL 2.1 XSD      │
              │ validation       │
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ EN 16931 rules   │
              │ Schematron       │
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ Peppol BIS rules │
              │ Schematron       │
              └────────┬─────────┘
                       │
                       ▼
              ┌──────────────────┐
              │ Code lists       │
              │ identifiers      │
              │ country rules    │
              └────────┬─────────┘
                       │
                       ▼
                    VALID
```

The important architectural conclusion is:

```text
UBL XSD validation
        ↓
is necessary
        ↓
but is not sufficient
        ↓
for Peppol validation
```

---

## 5. Concrete example

Consider a valid UBL 2.1 invoice:

```xml
<Invoice
 xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2"
 xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
 xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">

    <cbc:ID>INV-001</cbc:ID>
    ...
</Invoice>
```

This XML document could satisfy the UBL 2.1 Invoice XSD.

Peppol could still reject it because it lacks required information such as:

```text
CustomizationID
ProfileID
seller electronic address
buyer electronic address
payment information
VAT information
```

or because a Peppol-specific business rule is violated.

Therefore:

```text
UBL-valid ≠ Peppol-valid
```

while:

```text
Peppol BIS Billing UBL invoice
        ⇒
UBL 2.1-based
```

---

## 6. Peppol is more than an invoice format

Peppol is not merely an XML invoice specification.

It is a broader interoperability framework and network.

Conceptually:

```text
Peppol network
│
├── transport infrastructure
│   └── Access Points
│
├── participant discovery
│
├── identifiers
│
├── security and certificates
│
└── Business Interoperability Specifications
    │
    ├── Billing
    ├── Ordering
    ├── Despatch
    ├── Catalogue
    ├── Self-Billing
    └── other procurement processes
```

UBL supplies many of the underlying business document structures used by those processes.

---

## 7. Relationship summary

The relationship can be summarized as:

```text
                 PEPPOL
        interoperability ecosystem
                   │
                   │ uses
                   ▼
          Peppol BIS specifications
                   │
                   │ constrain/map
                   ▼
               UBL 2.1
                   │
                   │ encoded as
                   ▼
                  XML
```

UBL provides the syntax and document model.

EN 16931 defines the European semantic model for electronic invoicing.

Peppol BIS defines how that semantic model is implemented within the Peppol environment using UBL 2.1, together with additional business rules, identifiers, code lists and process requirements.

---

## 8. Implications for the UBL 2.1 validator project

The UBL 2.1 validator should treat Peppol as an additional validation layer rather than as part of the base UBL XSD validation.

Recommended architecture:

```text
Validator
│
├── XML parser
│
├── UBL 2.1 XSD validator
│
├── EN 16931 validator
│
├── Peppol BIS validator
│
├── code-list validator
│
└── national profile validators
```

The base validator can remain generic and validate all UBL 2.1 document types.

Peppol support can then be implemented as an optional profile layer.

For example:

```text
POST /validate
```

could detect:

```text
document type
UBL version
CustomizationID
ProfileID
```

and select the appropriate validation pipeline automatically.

Possible validation modes:

```text
ubl21
en16931
peppol-bis3
peppol-country-profile
```

This separation keeps the architecture clean and extensible.

---

## 9. Strategic significance

UBL 2.1 XSD validation provides a useful technical foundation, but its commercial value is limited if validation stops at schema correctness.

Adding:

```text
EN 16931
+
Peppol BIS
+
national CIUS/profile rules
```

turns the validator into something much more useful for:

- ERP vendors;
- accounting software providers;
- Peppol access-point providers;
- EDI companies;
- invoice-processing platforms;
- integration consultancies;
- enterprise finance systems.

The commercial path therefore looks like:

```text
UBL 2.1 validation
        ↓
EN 16931 validation
        ↓
Peppol validation
        ↓
national e-invoicing profiles
        ↓
developer API / CI validation
```

This is the bridge from a generic XML validator to a European e-invoicing validation platform.

---

## References

- OASIS UBL 2.1  
  https://docs.oasis-open.org/ubl/prd3-UBL-2.1/UBL-2.1.html

- Peppol BIS Billing 3.0  
  https://docs.peppol.eu/poacc/billing/3.0/

- Peppol Billing specification  
  https://docs.peppol.eu/poacc/billing/3.0/2022-Q4/bis/

- OpenPeppol  
  https://peppol.org/

