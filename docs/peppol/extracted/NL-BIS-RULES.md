# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 9. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## NL-R-001

Severity: fatal. Source pattern 11, rule 1.

Context:
```xpath
cbc:CreditNoteTypeCode[$supplierCountryIsNL]
```

Test:
```xpath
/*/cac:BillingReference/cac:InvoiceDocumentReference/cbc:ID
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-001-valid`, `NL-R-001-invalid`, `NL-R-001-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-002

Severity: fatal. Source pattern 11, rule 2.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party/cac:PostalAddress[$supplierCountryIsNL]
```

Test:
```xpath
cbc:StreetName and cbc:CityName and cbc:PostalZone
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-002-valid`, `NL-R-002-invalid`, `NL-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-003

Severity: fatal. Source pattern 11, rule 3.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID[$supplierCountryIsNL]
```

Test:
```xpath
(contains(concat(' ', string-join(@schemeID, ' '), ' '), ' 0106 ') or contains(concat(' ', string-join(@schemeID, ' '), ' '), ' 0190 ')) and (normalize-space(.) != '')
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-003-valid`, `NL-R-003-invalid`, `NL-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-004

Severity: fatal. Source pattern 11, rule 4.

Context:
```xpath
cac:AccountingCustomerParty/cac:Party/cac:PostalAddress[$supplierCountryIsNL and $customerCountryIsNL]
```

Test:
```xpath
cbc:StreetName and cbc:CityName and cbc:PostalZone
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-004-valid`, `NL-R-004-invalid`, `NL-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-005

Severity: fatal. Source pattern 11, rule 5.

Context:
```xpath
cac:AccountingCustomerParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID[$supplierCountryIsNL and $customerCountryIsNL]
```

Test:
```xpath
(contains(concat(' ', string-join(@schemeID, ' '), ' '), ' 0106 ') or contains(concat(' ', string-join(@schemeID, ' '), ' '), ' 0190 ')) and (normalize-space(.) != '')
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-005-valid`, `NL-R-005-invalid`, `NL-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-006

Severity: fatal. Source pattern 11, rule 6.

Context:
```xpath
cac:TaxRepresentativeParty/cac:PostalAddress[$supplierCountryIsNL and $taxRepresentativeCountryIsNL]
```

Test:
```xpath
cbc:StreetName and cbc:CityName and cbc:PostalZone
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-006-valid`, `NL-R-006-invalid`, `NL-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-007

Severity: fatal. Source pattern 11, rule 7.

Context:
```xpath
cac:LegalMonetaryTotal[$supplierCountryIsNL]
```

Test:
```xpath
(/ubl-invoice:Invoice and xs:decimal(cbc:PayableAmount) <= 0.0) or (/ubl-creditnote:CreditNote and xs:decimal(cbc:PayableAmount) >= 0.0) or (//cac:PaymentMeans)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-007-valid`, `NL-R-007-invalid`, `NL-R-007-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-008

Severity: fatal. Source pattern 11, rule 8.

Context:
```xpath
cac:PaymentMeans[$supplierCountryIsNL and $customerCountryIsNL]
```

Test:
```xpath
normalize-space(cbc:PaymentMeansCode) = '30' or         normalize-space(cbc:PaymentMeansCode) = '48' or         normalize-space(cbc:PaymentMeansCode) = '49' or         normalize-space(cbc:PaymentMeansCode) = '57' or         normalize-space(cbc:PaymentMeansCode) = '58' or         normalize-space(cbc:PaymentMeansCode) = '59'
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-008-valid`, `NL-R-008-invalid`, `NL-R-008-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## NL-R-009

Severity: fatal. Source pattern 11, rule 9.

Context:
```xpath
cac:OrderLineReference/cbc:LineID[$supplierCountryIsNL]
```

Test:
```xpath
exists(/*/cac:OrderReference/cbc:ID)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsNL" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="taxRepresentativeCountryIsNL" value="(upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'NL')" />
    
```

Required fixture IDs: `NL-R-009-valid`, `NL-R-009-invalid`, `NL-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

