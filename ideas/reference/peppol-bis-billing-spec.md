# Peppol BIS Billing 3.0 Specification Reference

**Project:** UBL-21  
**Filename:** `peppol-spec.md`  
**Checked:** 21 August 2026  
**Scope:** Peppol BIS Billing 3.0 validation on top of UBL 2.1

---

## 1. Current baseline

For the UBL-21 validator, implement:

- **Specification:** Peppol BIS Billing 3.0
- **Release:** May 2026
- **Version:** 3.0.21
- **Published:** 20 May 2026
- **Mandatory from:** 17 August 2026
- **Documents:** UBL Invoice and UBL Credit Note
- **Semantic base:** EN 16931
- **XML syntax:** OASIS UBL 2.1

OpenPeppol currently exposes this release under the `upcoming` documentation path.

Official release register:

- https://peppol.org/documentation/technical-documentation/post-award-documentation/

Current May 2026 release root:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/

Full BIS:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/bis/

---

## 2. Validation model

Peppol BIS Billing 3.0 is a **Core Invoice Usage Specification (CIUS)** of EN 16931.

It is not a replacement XSD.

A valid Peppol billing document passes several independent layers:

```text
XML well-formedness
        |
        v
UBL 2.1 XSD
        |
        v
EN 16931 rules
        |
        v
Peppol BIS rules
        |
        v
Peppol code lists / identifier schemes
        |
        v
Applicable national rules
        |
        v
PEPPOL COMPLIANT
```

Therefore:

```text
UBL 2.1 XSD valid != Peppol compliant
```

---

## 3. Main Peppol documentation

### Main documentation

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/

### Full Peppol BIS Billing 3.0 specification

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/bis/

### Release notes

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/release-notes/

### BIS compliance

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/compliance/

### OpenPeppol post-award specifications index

- https://peppol.org/documentation/technical-documentation/post-award-documentation/

---

## 4. UBL syntax

Peppol Billing 3.0 supports UBL `Invoice` and `CreditNote`.

### UBL Invoice

Syntax documentation:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-invoice/

Complete element tree:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-invoice/tree/

### UBL Credit Note

Syntax documentation:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-creditnote/

Complete element tree:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-creditnote/tree/

The syntax trees provide:

- element paths;
- cardinalities;
- corresponding EN 16931 business terms;
- descriptions;
- code-list requirements;
- identifier-scheme requirements;
- applicable validation rules.

---

## 5. Peppol document identifiers

Typical Peppol Billing documents contain:

### CustomizationID

```xml
<cbc:CustomizationID>
urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0
</cbc:CustomizationID>
```

### ProfileID

For the normal Billing profile:

```xml
<cbc:ProfileID>
urn:fdc:peppol.eu:2017:poacc:billing:01:1.0
</cbc:ProfileID>
```

Identifier documentation:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/bis/#_peppol_identifiers

---

## 6. Validation rules

Rules index:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/

### EN 16931 rules mapped to UBL

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/ubl-tc434/

Typical identifiers:

```text
BR-01
BR-02
BR-CO-...
BR-S-...
BR-Z-...
BR-E-...
```

### Peppol BIS Billing rules

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/ubl-peppol/

Typical identifiers include:

```text
PEPPOL-EN16931-R...
PEPPOL-EN16931-P...
```

The same rule set also includes country-specific Peppol rules where applicable.

### Enveloping compliance

Machine-readable rules:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/PeppolBIS-Envelope-Rules.sch

---

## 7. Machine-readable Schematron artefacts

These are the primary files the validator should execute.

### EN 16931 UBL Schematron

Direct download:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/CEN-EN16931-UBL.sch

Suggested local name:

```text
CEN-EN16931-UBL.sch
```

Purpose:

```text
EN 16931 semantic/business validation
mapped onto UBL
```

### Peppol Billing UBL Schematron

Direct download:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/PEPPOL-EN16931-UBL.sch

Suggested local name:

```text
PEPPOL-EN16931-UBL.sch
```

Purpose:

```text
Peppol BIS Billing validation
+
applicable Peppol national rules
```

### Enveloping Schematron

Direct download:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/PeppolBIS-Envelope-Rules.sch

---

## 8. Official example documents

Official Peppol Billing examples ZIP:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/BIS-Billing3-Examples.zip

Recommended use:

```text
official examples
      |
      +-- positive regression tests
      |
      +-- mutation tests
            |
            +-- remove mandatory fields
            +-- alter code-list values
            +-- corrupt totals
            +-- corrupt identifier schemes
            +-- trigger Peppol rule failures
```

---

## 9. Code lists

Code-list root:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/

### ISO 6523 ICD

Organisation / party identifier schemes:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/ICD/

### Electronic Address Scheme, EAS

Used by `cbc:EndpointID/@schemeID`:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/eas/

### ISO 3166-1 Alpha-2 country codes

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/ISO3166/

### ISO 4217 currency codes

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/ISO4217/

### UN/ECE Recommendation 20 / 21 unit codes

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNECERec20/

### VAT / tax category codes, UNCL5305 subset

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL5305/

### Item type identification codes, UNCL7143

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL7143/

### Invoice type codes, UNCL1001 subset

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL1001-inv/

Typical example:

```text
380 = Commercial invoice
```

### Credit Note type codes, UNCL1001 subset

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL1001-cn/

Typical example:

```text
381 = Credit note
```

### Invoiced object identifier schemes, UNCL1153

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL1153/

### VAT date codes, UNCL2005

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL2005/

### Payment means codes, UNCL4461

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL4461/

### Allowance reason codes, UNCL5189

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL5189/

### Charge reason codes, UNCL7161

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/UNCL7161/

### MIME codes

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/MimeCode/

### SEPA indicator

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/SEPA/

### VAT exemption reason codes, VATEX

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/vatex/

---

## 10. OASIS UBL 2.1 base specification

Peppol Billing 3.0 uses **OASIS UBL 2.1**.

Official UBL 2.1 specification:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/UBL-2.1.html

Distribution root:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/

XSD root:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/

Invoice XSD:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/UBL-Invoice-2.1.xsd

Credit Note XSD:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/UBL-CreditNote-2.1.xsd

Common XSDs:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/common/

---

## 11. EN 16931

Peppol BIS Billing 3.0 is a CIUS of:

```text
EN 16931-1:2017
Electronic invoicing
Semantic data model of the core elements of an electronic invoice
```

European Commission eInvoicing information:

- https://ec.europa.eu/digital-building-blocks/sites/display/DIGITAL/eInvoicing

For executable validation, Peppol publishes the UBL-bound EN 16931 rules here:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/CEN-EN16931-UBL.sch

---

## 12. Recommended UBL-21 implementation pipeline

```text
XML input
   |
   v
XML parser
   |
   +-- malformed -> XML error
   |
   v
UBL 2.1 XSD
   |
   +-- XSD errors[]
   |
   v
CEN-EN16931-UBL.sch
   |
   +-- EN16931 errors[]
   |
   v
PEPPOL-EN16931-UBL.sch
   |
   +-- Peppol errors[]
   |
   v
Code-list / identifier validation
   |
   +-- code-list errors[]
   |
   v
National rules
   |
   +-- national errors[]
   |
   v
VALID
```

Suggested response model:

```text
ValidationResult
|
+-- XML
|   +-- errors[]
|
+-- XSD
|   +-- errors[]
|
+-- EN16931
|   +-- errors[]
|   +-- warnings[]
|
+-- Peppol
|   +-- errors[]
|   +-- warnings[]
|
+-- CodeLists
|   +-- errors[]
|
+-- NationalRules
    +-- errors[]
    +-- warnings[]
```

Each reported issue should expose, when available:

```text
validation layer
rule ID
severity
message
XML location
source specification
```

---

## 13. Recommended artefact layout

Do not rewrite Peppol's Schematron manually. Download and pin official artefacts.

Suggested layout:

```text
spec/
└── peppol/
    └── billing-3.0/
        └── 3.0.21/
            ├── schematron/
            │   ├── CEN-EN16931-UBL.sch
            │   ├── PEPPOL-EN16931-UBL.sch
            │   └── PeppolBIS-Envelope-Rules.sch
            │
            ├── examples/
            │   └── BIS-Billing3-Examples.zip
            │
            ├── codelists/
            │   └── ...
            │
            └── metadata/
                ├── release.txt
                ├── source-urls.txt
                └── checksums.txt
```

For each downloaded artefact record:

```text
source URL
Peppol release
download date
SHA-256 checksum
```

---

## 14. Version pinning

The validator should **not** fetch the moving `upcoming` URL during normal runtime.

Instead:

1. identify the release;
2. download the official artefacts;
3. calculate hashes;
4. record provenance;
5. pin the validator to the release;
6. upgrade deliberately after regression testing.

Logical release identifier:

```text
peppol-billing-3.0.21
```

---

## 15. Related Peppol specifications

Official post-award index:

- https://peppol.org/documentation/technical-documentation/post-award-documentation/

### Peppol Self-Billing 3.0

May 2026 release:

- https://docs.peppol.eu/poacc/self-billing/3.0/upcoming/

### Other Peppol BIS 3.0 transactions

May 2026 release:

- https://docs.peppol.eu/poacc/upgrade-3/upcoming/

This family includes specifications such as:

```text
Order
Ordering
Order Agreement
Catalogue
Despatch Advice
Punch Out
Message Level Response
Invoice Response
```

These should be implemented as separate validation profiles rather than folded into the Billing profile.

---

## 16. Copyright note

The Peppol BIS documentation states that OpenPeppol holds copyright in the BIS and that the BIS document may not be modified, redistributed, sold or repackaged without prior consent.

Therefore this project should:

```text
link to the official specification
+
download official executable validation artefacts as needed
+
record provenance and checksums
```

rather than reproduce the complete Peppol BIS prose.

Official BIS:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/bis/

---

## 17. Core links at a glance

### Release authority

- https://peppol.org/documentation/technical-documentation/post-award-documentation/

### BIS

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/bis/

### Syntax

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-invoice/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-invoice/tree/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-creditnote/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/syntax/ubl-creditnote/tree/

### Rules

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/ubl-tc434/
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/ubl-peppol/

### Schematron

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/CEN-EN16931-UBL.sch
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/PEPPOL-EN16931-UBL.sch
- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/PeppolBIS-Envelope-Rules.sch

### Examples

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/files/BIS-Billing3-Examples.zip

### Code lists

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/codelist/

### UBL 2.1

- https://docs.oasis-open.org/ubl/os-UBL-2.1/UBL-2.1.html
- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/

---

## 18. Architectural conclusion

Peppol support should be a profile layered on the generic UBL validator:

```text
Generic UBL 2.1 validator
        |
        +-- Invoice XSD
        +-- CreditNote XSD
        |
        v
EN 16931 profile
        |
        v
Peppol Billing 3.0.21 profile
        |
        +-- Peppol Schematron
        +-- code lists
        +-- identifier schemes
        +-- national rules
```

This keeps the system extensible for:

```text
Peppol Self-Billing
Peppol Order
Peppol Despatch Advice
national EN 16931 CIUS profiles
future Peppol releases
```

The core invariant remains:

```text
XSD valid != Peppol compliant
```
