# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 10. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## IS-R-001

Severity: warning. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
( ( not(contains(normalize-space(cbc:InvoiceTypeCode),' ')) and contains( ' 380 381 ',concat(' ',normalize-space(cbc:InvoiceTypeCode),' ') ) ) ) or ( ( not(contains(normalize-space(cbc:CreditNoteTypeCode),' ')) and contains( ' 380 381 ',concat(' ',normalize-space(cbc:CreditNoteTypeCode),' ') ) ) )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-001-valid`, `IS-R-001-invalid`, `IS-R-001-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-002

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
exists(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID) and cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID/@schemeID = '0196'
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-002-valid`, `IS-R-002-invalid`, `IS-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-003

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
exists(cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cbc:StreetName) and exists(cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cbc:PostalZone)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-003-valid`, `IS-R-003-invalid`, `IS-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-006

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
exists(cac:PaymentMeans[cbc:PaymentMeansCode = '9']/cac:PayeeFinancialAccount/cbc:ID)        and string-length(normalize-space(cac:PaymentMeans[cbc:PaymentMeansCode = '9']/cac:PayeeFinancialAccount/cbc:ID)) = 12        or not(exists(cac:PaymentMeans[cbc:PaymentMeansCode = '9']))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-006-valid`, `IS-R-006-invalid`, `IS-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-007

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
exists(cac:PaymentMeans[cbc:PaymentMeansCode = '42']/cac:PayeeFinancialAccount/cbc:ID)        and string-length(normalize-space(cac:PaymentMeans[cbc:PaymentMeansCode = '42']/cac:PayeeFinancialAccount/cbc:ID)) = 12        or not(exists(cac:PaymentMeans[cbc:PaymentMeansCode = '42']))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-007-valid`, `IS-R-007-invalid`, `IS-R-007-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-008

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']) and string-length(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']/cbc:ID) = 10 and (string(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']/cbc:ID) castable as xs:date)) or not(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-008-valid`, `IS-R-008-invalid`, `IS-R-008-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-009

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']) and exists(cbc:DueDate)) or not(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-009-valid`, `IS-R-009-invalid`, `IS-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-010

Severity: fatal. Source pattern 10, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS'] | ubl-invoice:Invoice[$SupplierCountry = 'IS']
```

Test:
```xpath
(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']) and (cbc:DueDate) <= (cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']/cbc:ID)) or not(exists(cac:AdditionalDocumentReference[cbc:DocumentDescription = 'EINDAGI']))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-010-valid`, `IS-R-010-invalid`, `IS-R-010-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-004

Severity: fatal. Source pattern 10, rule 2.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS' and $CustomerCountry = 'IS']/cac:AccountingCustomerParty | ubl-invoice:Invoice[$SupplierCountry = 'IS' and $CustomerCountry = 'IS']/cac:AccountingCustomerParty
```

Test:
```xpath
exists(cac:Party/cac:PartyLegalEntity/cbc:CompanyID) and cac:Party/cac:PartyLegalEntity/cbc:CompanyID/@schemeID = '0196'
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-004-valid`, `IS-R-004-invalid`, `IS-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IS-R-005

Severity: fatal. Source pattern 10, rule 2.

Context:
```xpath
ubl-creditnote:CreditNote[$SupplierCountry = 'IS' and $CustomerCountry = 'IS']/cac:AccountingCustomerParty | ubl-invoice:Invoice[$SupplierCountry = 'IS' and $CustomerCountry = 'IS']/cac:AccountingCustomerParty
```

Test:
```xpath
exists(cac:Party/cac:PostalAddress/cbc:StreetName) and exists(cac:Party/cac:PostalAddress/cbc:PostalZone)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="SupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="CustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Required fixture IDs: `IS-R-005-valid`, `IS-R-005-invalid`, `IS-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

