# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 19. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## GR-R-001-1

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
count($IdSegments) = 6
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-1-valid`, `GR-R-001-1-invalid`, `GR-R-001-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-2

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length(normalize-space($IdSegments[1])) = 9                                  and u:TinVerification($IdSegments[1])                                  and ($IdSegments[1] = /*/cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 3, 9)                                  or $IdSegments[1] = /*/cac:TaxRepresentativeParty/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 3, 9) )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-2-valid`, `GR-R-001-2-invalid`, `GR-R-001-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-3

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length(normalize-space($IdSegments[2]))>0                                  and matches($IdSegments[2],$dateRegExp)                                  and ($tokenizedIdDate[1] = $tokenizedUblIssueDate[3]                                    and $tokenizedIdDate[2] = $tokenizedUblIssueDate[2]                                    and $tokenizedIdDate[3] = $tokenizedUblIssueDate[1])
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-3-valid`, `GR-R-001-3-invalid`, `GR-R-001-3-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-4

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length(normalize-space($IdSegments[3]))>0 and string(number($IdSegments[3])) != 'NaN' and xs:integer($IdSegments[3]) >= 0
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-4-valid`, `GR-R-001-4-invalid`, `GR-R-001-4-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-5

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length(normalize-space($IdSegments[4]))>0 and (some $c in $greekDocumentType satisfies $IdSegments[4] = $c)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-5-valid`, `GR-R-001-5-invalid`, `GR-R-001-5-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-6

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length($IdSegments[5]) > 0 
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-6-valid`, `GR-R-001-6-invalid`, `GR-R-001-6-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-001-7

Severity: fatal. Source pattern 8, rule 1.

Context:
```xpath
/ubl-invoice:Invoice/cbc:ID[$isGreekSender] | /ubl-creditnote:CreditNote/cbc:ID[$isGreekSender]
```

Test:
```xpath
string-length($IdSegments[6]) > 0 
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="IdSegments" value="tokenize(.,'\|')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedIdDate" value="tokenize($IdSegments[2],'/')" />
      
```

Required fixture IDs: `GR-R-001-7-valid`, `GR-R-001-7-invalid`, `GR-R-001-7-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-002

Severity: fatal. Source pattern 8, rule 2.

Context:
```xpath
cac:AccountingSupplierParty[$isGreekSender]/cac:Party
```

Test:
```xpath
string-length(./cac:PartyName/cbc:Name)>0
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-002-valid`, `GR-R-002-invalid`, `GR-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-S-011

Severity: warning. Source pattern 8, rule 2.

Context:
```xpath
cac:AccountingSupplierParty[$isGreekSender]/cac:Party
```

Test:
```xpath
count(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID)=1 and                             substring(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID,1,2) = 'EL' and                             u:TinVerification(substring(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID,3))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-S-011-valid`, `GR-S-011-invalid`, `GR-S-011-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-003

Severity: fatal. Source pattern 8, rule 3.

Context:
```xpath
cac:AccountingSupplierParty[$isGreekSender]/cac:Party/cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID
```

Test:
```xpath
substring(.,1,2) = 'EL' and u:TinVerification(substring(.,3))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-003-valid`, `GR-R-003-invalid`, `GR-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-004-1

Severity: fatal. Source pattern 8, rule 4.

Context:
```xpath
/ubl-invoice:Invoice[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')] | /ubl-creditnote:CreditNote[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')]
```

Test:
```xpath
count(cac:AdditionalDocumentReference[cbc:DocumentDescription = '##M.AR.K##'])=1
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-004-1-valid`, `GR-R-004-1-invalid`, `GR-R-004-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-S-008-1

Severity: warning. Source pattern 8, rule 4.

Context:
```xpath
/ubl-invoice:Invoice[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')] | /ubl-creditnote:CreditNote[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')]
```

Test:
```xpath
count(cac:AdditionalDocumentReference[cbc:DocumentDescription = '##INVOICE|URL##'])=1
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-S-008-1-valid`, `GR-S-008-1-invalid`, `GR-S-008-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-008-2

Severity: fatal. Source pattern 8, rule 4.

Context:
```xpath
/ubl-invoice:Invoice[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')] | /ubl-creditnote:CreditNote[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR')]
```

Test:
```xpath
(count(cac:AdditionalDocumentReference[cbc:DocumentDescription = '##INVOICE|URL##']) = 0 ) or (count(cac:AdditionalDocumentReference[cbc:DocumentDescription = '##INVOICE|URL##']) = 1 )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-008-2-valid`, `GR-R-008-2-invalid`, `GR-R-008-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-004-2

Severity: fatal. Source pattern 8, rule 5.

Context:
```xpath
cac:AdditionalDocumentReference[$isGreekSender and ( /*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'GR') and cbc:DocumentDescription = '##M.AR.K##']/cbc:ID
```

Test:
```xpath
matches(.,'^[1-9]([0-9]*)')
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-004-2-valid`, `GR-R-004-2-invalid`, `GR-R-004-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-008-3

Severity: fatal. Source pattern 8, rule 6.

Context:
```xpath
cac:AdditionalDocumentReference[$isGreekSender and cbc:DocumentDescription = '##INVOICE|URL##']
```

Test:
```xpath
string-length(normalize-space(cac:Attachment/cac:ExternalReference/cbc:URI))>0
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-008-3-valid`, `GR-R-008-3-invalid`, `GR-R-008-3-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-005

Severity: fatal. Source pattern 8, rule 7.

Context:
```xpath
cac:AccountingCustomerParty[$isGreekSender]/cac:Party
```

Test:
```xpath
string-length(./cac:PartyName/cbc:Name)>0
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-005-valid`, `GR-R-005-invalid`, `GR-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-009

Severity: fatal. Source pattern 8, rule 8.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party[$accountingSupplierCountry='GR' or $accountingSupplierCountry='EL']/cbc:EndpointID
```

Test:
```xpath
./@schemeID='9933' and u:TinVerification(.)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="dateRegExp" value="'^(0?[1-9]|[12][0-9]|3[01])[-\\/ ]?(0?[1-9]|1[0-2])[-\\/ ]?(19|20)[0-9]{2}'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="greekDocumentType" value="tokenize('1.1 1.6 2.1 2.4 5.1 5.2 ','\s')" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="tokenizedUblIssueDate" value="tokenize(/*/cbc:IssueDate,'-')" />
    
    
```

Required fixture IDs: `GR-R-009-valid`, `GR-R-009-invalid`, `GR-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-006

Severity: fatal. Source pattern 9, rule 1.

Context:
```xpath
cac:AccountingCustomerParty[$isGreekSenderandReceiver]/cac:Party
```

Test:
```xpath
count(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID)=1 and                             substring(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID,1,2) = 'EL' and                             u:TinVerification(substring(cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) = 'VAT']/cbc:CompanyID,3))
```

Required fixture IDs: `GR-R-006-valid`, `GR-R-006-invalid`, `GR-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## GR-R-010

Severity: fatal. Source pattern 9, rule 2.

Context:
```xpath
cac:AccountingCustomerParty[$isGreekSenderandReceiver]/cac:Party/cbc:EndpointID
```

Test:
```xpath
./@schemeID='9933' and u:TinVerification(.)
```

Required fixture IDs: `GR-R-010-valid`, `GR-R-010-invalid`, `GR-R-010-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

