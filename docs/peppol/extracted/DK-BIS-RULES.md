# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 14. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## DK-R-002

Severity: fatal. Source pattern 5, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK'] | ubl-invoice:Invoice[$DKSupplierCountry = 'DK']
```

Test:
```xpath
(normalize-space(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID/text()) != '')
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-002-valid`, `DK-R-002-invalid`, `DK-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-014

Severity: fatal. Source pattern 5, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK'] | ubl-invoice:Invoice[$DKSupplierCountry = 'DK']
```

Test:
```xpath
not(((boolean(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID))           and (normalize-space(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID/@schemeID) != '0184'))       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-014-valid`, `DK-R-014-invalid`, `DK-R-014-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-016

Severity: fatal. Source pattern 5, rule 1.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK'] | ubl-invoice:Invoice[$DKSupplierCountry = 'DK']
```

Test:
```xpath
not((boolean(/ubl-creditnote:CreditNote) and ($DKCustomerCountry = 'DK'))       and (number(cac:LegalMonetaryTotal/cbc:PayableAmount/text()) < 0)       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-016-valid`, `DK-R-016-invalid`, `DK-R-016-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-013

Severity: fatal. Source pattern 5, rule 2.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingSupplierParty/cac:Party/cac:PartyIdentification | ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingCustomerParty/cac:Party/cac:PartyIdentification | ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingSupplierParty/cac:Party/cac:PartyIdentification | ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingCustomerParty/cac:Party/cac:PartyIdentification
```

Test:
```xpath
not((boolean(cbc:ID))        and (normalize-space(cbc:ID/@schemeID) = '')       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-013-valid`, `DK-R-013-invalid`, `DK-R-013-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-005

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
contains(' 1 10 31 42 48 49 50 58 59 93 97 ', concat(' ', cbc:PaymentMeansCode, ' '))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-005-valid`, `DK-R-005-invalid`, `DK-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-006

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not(((cbc:PaymentMeansCode = '31') or (cbc:PaymentMeansCode = '42'))       and not((normalize-space(cac:PayeeFinancialAccount/cbc:ID/text()) != '') and (normalize-space(cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID/text()) != ''))       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-006-valid`, `DK-R-006-invalid`, `DK-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-007

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not((cbc:PaymentMeansCode = '49')       and not((normalize-space(cac:PaymentMandate/cbc:ID/text()) != '')           and (normalize-space(cac:PaymentMandate/cac:PayerFinancialAccount/cbc:ID/text()) != ''))       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-007-valid`, `DK-R-007-invalid`, `DK-R-007-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-008

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not((cbc:PaymentMeansCode = '50')       and not(((substring(cbc:PaymentID, 1, 3) = '01#')           or (substring(cbc:PaymentID, 1, 3) = '04#')           or (substring(cbc:PaymentID, 1, 3) = '15#'))         and matches(cac:PayeeFinancialAccount/cbc:ID, '^[0-9]{7,8}$')         )       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-008-valid`, `DK-R-008-invalid`, `DK-R-008-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-009

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not((cbc:PaymentMeansCode = '50')       and ((substring(cbc:PaymentID, 1, 3) = '04#')          or (substring(cbc:PaymentID, 1, 3)  = '15#'))       and not(string-length(cbc:PaymentID) = 19)       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-009-valid`, `DK-R-009-invalid`, `DK-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-010

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not((cbc:PaymentMeansCode = '93')       and not(((substring(cbc:PaymentID, 1, 3) = '71#')           or (substring(cbc:PaymentID, 1, 3) = '73#')           or (substring(cbc:PaymentID, 1, 3) = '75#'))         and (string-length(cac:PayeeFinancialAccount/cbc:ID/text()) = 8)         )       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-010-valid`, `DK-R-010-invalid`, `DK-R-010-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-011

Severity: fatal. Source pattern 5, rule 3.

Context:
```xpath
ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:PaymentMeans
```

Test:
```xpath
not((cbc:PaymentMeansCode = '93')       and ((substring(cbc:PaymentID, 1, 3) = '71#')          or (substring(cbc:PaymentID, 1, 3)  = '75#'))       and not((string-length(cbc:PaymentID) = 18)          or (string-length(cbc:PaymentID) = 19))       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-011-valid`, `DK-R-011-invalid`, `DK-R-011-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-017

Severity: fatal. Source pattern 5, rule 4.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingCustomerParty/cac:Party | ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:AccountingCustomerParty/cac:Party
```

Test:
```xpath
not(((boolean(cac:PartyLegalEntity/cbc:CompanyID)) and (normalize-space(cac:PartyLegalEntity/cbc:CompanyID/@schemeID) != '0184')))
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-017-valid`, `DK-R-017-invalid`, `DK-R-017-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-003

Severity: fatal. Source pattern 5, rule 5.

Context:
```xpath
ubl-creditnote:CreditNote[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:CreditNoteLine | ubl-invoice:Invoice[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']/cac:InvoiceLine
```

Test:
```xpath
not((cac:Item/cac:CommodityClassification/cbc:ItemClassificationCode/@listID = 'TST')       and not((cac:Item/cac:CommodityClassification/cbc:ItemClassificationCode/@listVersionID = '19.05.01')           or (cac:Item/cac:CommodityClassification/cbc:ItemClassificationCode/@listVersionID = '19.0501')                                or (cac:Item/cac:CommodityClassification/cbc:ItemClassificationCode/@listVersionID = '26.08.01')                                or (cac:Item/cac:CommodityClassification/cbc:ItemClassificationCode/@listVersionID = '26.0801')           )       )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-003-valid`, `DK-R-003-invalid`, `DK-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## DK-R-004

Severity: fatal. Source pattern 5, rule 6.

Context:
```xpath
cac:AllowanceCharge[$DKSupplierCountry = 'DK' and $DKCustomerCountry = 'DK']
```

Test:
```xpath
not((cbc:AllowanceChargeReasonCode = 'ZZZ')       and not(((string-length(normalize-space(cbc:AllowanceChargeReason/text())) = 4)         and (number(cbc:AllowanceChargeReason) >= 0)         and (number(cbc:AllowanceChargeReason) <= 9999)) or         (((cbc:AllowanceChargeReason and contains(cbc:AllowanceChargeReason, '#')                                   and not(starts-with(cbc:AllowanceChargeReason, '#'))                                   and not(ends-with(cbc:AllowanceChargeReason, '#')))) )                                )      )
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKSupplierCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
```

Scoped dependency:
```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="DKCustomerCountry" value="concat(ubl-creditnote:CreditNote/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode, ubl-invoice:Invoice/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)" />
    
    
```

Required fixture IDs: `DK-R-004-valid`, `DK-R-004-invalid`, `DK-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

