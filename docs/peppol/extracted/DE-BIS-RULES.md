# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 31. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## DE-R-001

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:PaymentMeans
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-001-valid`, `DE-R-001-invalid`, `DE-R-001-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-015

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:BuyerReference[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-015-valid`, `DE-R-015-invalid`, `DE-R-015-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-016

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
         (not(           ($BT-95-UBL-Inv = $supportedVATCodes or $BT-95-UBL-CN = $supportedVATCodes) or           ($BT-102 = $supportedVATCodes) or           ($BT-151 = $supportedVATCodes)         ) or         (cac:TaxRepresentativeParty, $BT-31orBT-32Path))         
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-016-valid`, `DE-R-016-invalid`, `DE-R-016-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-017

Severity: warning. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
normalize-space(cbc:InvoiceTypeCode) = $supportedInvAndCNTypeCodes         or normalize-space(cbc:CreditNoteTypeCode) = $supportedInvAndCNTypeCodes
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-017-valid`, `DE-R-017-invalid`, `DE-R-017-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-018

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
every $line in             cac:PaymentTerms/cbc:Note[1]/tokenize(. , '(\r?\n)')[starts-with( normalize-space(.) , '#')]              satisfies matches ( normalize-space ($line), $XR-SKONTO-REGEX)                                  and                                 matches( cac:PaymentTerms/cbc:Note[1]/tokenize(. ,  '#.+#')[last()], '^\s*\n' )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-018-valid`, `DE-R-018-invalid`, `DE-R-018-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-022

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
count(cac:AdditionalDocumentReference) =                      count(cac:AdditionalDocumentReference[not(./cac:Attachment/cbc:EmbeddedDocumentBinaryObject/@filename = preceding-sibling::cac:AdditionalDocumentReference/cac:Attachment/cbc:EmbeddedDocumentBinaryObject/@filename)])
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-022-valid`, `DE-R-022-invalid`, `DE-R-022-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-026

Severity: warning. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
((not(normalize-space(cbc:InvoiceTypeCode) = '384' or normalize-space(cbc:CreditNoteTypeCode) = '384') or                     (cac:BillingReference/cac:InvoiceDocumentReference)))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-026-valid`, `DE-R-026-invalid`, `DE-R-026-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-030

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(cac:PaymentMeans/cac:PaymentMandate)                        or (cac:AccountingSupplierParty/cac:Party/cac:PartyIdentification/cbc:ID[@schemeID='SEPA']                          | cac:PayeeParty/cac:PartyIdentification/cbc:ID[@schemeID='SEPA'])
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-030-valid`, `DE-R-030-invalid`, `DE-R-030-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-031

Severity: fatal. Source pattern 12, rule 1.

Context:
```xpath
(/ubl-invoice:Invoice | /ubl-creditnote:CreditNote)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(cac:PaymentMeans/cac:PaymentMandate) or (cac:PaymentMeans/cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedVATCodes" value="('S', 'Z', 'E', 'AE', 'K', 'G', 'L', 'M')" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-31orBT-32Path" value="cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID[boolean(normalize-space(.))]" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-Inv" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false' and         following-sibling::cac:TaxScheme/cbc:ID = 'VAT']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-95-UBL-CN" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'false']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-102" value="cac:AllowanceCharge/cac:TaxCategory/cbc:ID[ancestor::cac:AllowanceCharge/cbc:ChargeIndicator = 'true']" />
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="BT-151" value="(cac:InvoiceLine | cac:CreditNoteLine)/cac:Item/cac:ClassifiedTaxCategory/cbc:ID" />
      
      
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supportedInvAndCNTypeCodes" value="('326', '380', '384', '389', '381', '875', '876', '877')" />
      
```

Required fixture IDs: `DE-R-031-valid`, `DE-R-031-invalid`, `DE-R-031-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-T02

Severity: warning. Source pattern 12, rule 2.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AdditionalDocumentReference/cac:Attachment/cac:ExternalReference | /ubl-creditnote:CreditNote/cac:AdditionalDocumentReference/cac:Attachment/cac:ExternalReference)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
matches(cbc:URI, $XR-URL-REGEX)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-T02-valid`, `DE-R-T02-invalid`, `DE-R-T02-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-002

Severity: fatal. Source pattern 12, rule 3.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:Party/cac:Contact
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-002-valid`, `DE-R-002-invalid`, `DE-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-003

Severity: fatal. Source pattern 12, rule 4.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:CityName[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-003-valid`, `DE-R-003-invalid`, `DE-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-004

Severity: fatal. Source pattern 12, rule 4.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:PostalZone[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-004-valid`, `DE-R-004-invalid`, `DE-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-005

Severity: fatal. Source pattern 12, rule 5.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:Contact | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:Contact)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:Name[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-005-valid`, `DE-R-005-invalid`, `DE-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-006

Severity: fatal. Source pattern 12, rule 5.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:Contact | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:Contact)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:Telephone[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-006-valid`, `DE-R-006-invalid`, `DE-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-007

Severity: fatal. Source pattern 12, rule 5.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:Contact | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:Contact)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:ElectronicMail[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-007-valid`, `DE-R-007-invalid`, `DE-R-007-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-027

Severity: warning. Source pattern 12, rule 5.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:Contact | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:Contact)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
matches(normalize-space(cbc:Telephone), $XR-TELEPHONE-REGEX)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-027-valid`, `DE-R-027-invalid`, `DE-R-027-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-028

Severity: warning. Source pattern 12, rule 5.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:Contact | /ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:Contact)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
matches(normalize-space(cbc:ElectronicMail), $XR-EMAIL-REGEX)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-028-valid`, `DE-R-028-invalid`, `DE-R-028-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-008

Severity: fatal. Source pattern 12, rule 6.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress | /ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:CityName[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-008-valid`, `DE-R-008-invalid`, `DE-R-008-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-009

Severity: fatal. Source pattern 12, rule 6.

Context:
```xpath
(/ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress | /ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:PostalZone[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-009-valid`, `DE-R-009-invalid`, `DE-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-010

Severity: fatal. Source pattern 12, rule 7.

Context:
```xpath
(/ubl-invoice:Invoice/cac:Delivery/cac:DeliveryLocation/cac:Address | /ubl-creditnote:CreditNote/cac:Delivery/cac:DeliveryLocation/cac:Address)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:CityName[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-010-valid`, `DE-R-010-invalid`, `DE-R-010-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-011

Severity: fatal. Source pattern 12, rule 7.

Context:
```xpath
(/ubl-invoice:Invoice/cac:Delivery/cac:DeliveryLocation/cac:Address | /ubl-creditnote:CreditNote/cac:Delivery/cac:DeliveryLocation/cac:Address)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cbc:PostalZone[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-011-valid`, `DE-R-011-invalid`, `DE-R-011-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-019

Severity: warning. Source pattern 12, rule 8.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(normalize-space(cbc:PaymentMeansCode) = '58') or                     matches(normalize-space(replace(cac:PayeeFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')), '^[A-Z]{2}[0-9]{2}[a-zA-Z0-9]{0,30}$') and                     xs:integer(string-join(for $cp in string-to-codepoints(concat(substring(normalize-space(replace(cac:PayeeFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),5),upper-case(substring(normalize-space(replace(cac:PayeeFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),1,2)),substring(normalize-space(replace(cac:PayeeFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),3,2))) return  (if($cp > 64) then string($cp - 55) else  string($cp - 48)),'')) mod 97 = 1
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-019-valid`, `DE-R-019-invalid`, `DE-R-019-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-023-1

Severity: fatal. Source pattern 12, rule 8.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:PayeeFinancialAccount
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-023-1-valid`, `DE-R-023-1-invalid`, `DE-R-023-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-023-2

Severity: fatal. Source pattern 12, rule 8.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('30','58')])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(cac:CardAccount) and                     not(cac:PaymentMandate)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-023-2-valid`, `DE-R-023-2-invalid`, `DE-R-023-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-024-1

Severity: fatal. Source pattern 12, rule 9.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('48','54','55')] |/ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('48','54','55')])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:CardAccount
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-024-1-valid`, `DE-R-024-1-invalid`, `DE-R-024-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-024-2

Severity: fatal. Source pattern 12, rule 9.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('48','54','55')] |/ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = ('48','54','55')])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(cac:PayeeFinancialAccount) and                     not(cac:PaymentMandate)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-024-2-valid`, `DE-R-024-2-invalid`, `DE-R-024-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-020

Severity: warning. Source pattern 12, rule 10.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(normalize-space(cbc:PaymentMeansCode) = '59') or                     matches(normalize-space(replace(cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')), '^[A-Z]{2}[0-9]{2}[a-zA-Z0-9]{0,30}$') and                     xs:decimal(string-join(for $cp in string-to-codepoints(concat(substring(normalize-space(replace(cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),5),upper-case(substring(normalize-space(replace(cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),1,2)),substring(normalize-space(replace(cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID, '([ \n\r\t\s])', '')),3,2))) return  (if($cp > 64) then string($cp - 55) else  string($cp - 48)),'')) mod 97 = 1
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-020-valid`, `DE-R-020-invalid`, `DE-R-020-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-025-1

Severity: fatal. Source pattern 12, rule 10.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:PaymentMandate
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-025-1-valid`, `DE-R-025-1-invalid`, `DE-R-025-1-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-025-2

Severity: fatal. Source pattern 12, rule 10.

Context:
```xpath
(/ubl-invoice:Invoice/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'] | /ubl-creditnote:CreditNote/cac:PaymentMeans[normalize-space(cbc:PaymentMeansCode) = '59'])[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
not(cac:PayeeFinancialAccount) and                     not(cac:CardAccount)
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-025-2-valid`, `DE-R-025-2-invalid`, `DE-R-025-2-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DE-R-014

Severity: fatal. Source pattern 12, rule 11.

Context:
```xpath
(/ubl-invoice:Invoice/cac:TaxTotal/cac:TaxSubtotal | /ubl-creditnote:CreditNote/cac:TaxTotal/cac:TaxSubtotal)[$supplierCountryIsDE and $customerCountryIsDE]
```

Test:
```xpath
cac:TaxCategory/cbc:Percent[boolean(normalize-space(.))]
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-SKONTO-REGEX" value="'(^|\r?\n)#(SKONTO)#TAGE=([0-9]+#PROZENT=[0-9]+\.[0-9]{2})(#BASISBETRAG=-?[0-9]+\.[0-9]{2})?#$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-EMAIL-REGEX" value="'^[^@\s]+@([^@.\s]+\.)+[^@.\s]+$'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-TELEPHONE-REGEX" value="'.*([0-9].*){3,}.*'" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="XR-URL-REGEX" value="'^([a-zA-Z])([a-zA-Z0-9+.-])+:.*'" />
    
```

Required fixture IDs: `DE-R-014-valid`, `DE-R-014-invalid`, `DE-R-014-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

