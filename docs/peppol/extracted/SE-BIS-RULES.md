# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 13. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## SE-R-001

Severity: fatal. Source pattern 7, rule 1.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2) = 'SE']
```

Test:
```xpath
string-length(normalize-space(cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/cbc:CompanyID)) = 14
```

Required fixture IDs: `SE-R-001-valid`, `SE-R-001-invalid`, `SE-R-001-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-002

Severity: fatal. Source pattern 7, rule 1.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2) = 'SE']
```

Test:
```xpath
string(number(substring(cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/cbc:CompanyID, 3, 12))) != 'NaN'
```

Required fixture IDs: `SE-R-002-valid`, `SE-R-002-invalid`, `SE-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-003

Severity: fatal. Source pattern 7, rule 2.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity[../cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cbc:CompanyID]
```

Test:
```xpath
string(number(cbc:CompanyID)) != 'NaN'
```

Required fixture IDs: `SE-R-003-valid`, `SE-R-003-invalid`, `SE-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-004

Severity: fatal. Source pattern 7, rule 2.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity[../cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cbc:CompanyID]
```

Test:
```xpath
string-length(normalize-space(cbc:CompanyID)) = 10
```

Required fixture IDs: `SE-R-004-valid`, `SE-R-004-invalid`, `SE-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-013

Severity: fatal. Source pattern 7, rule 2.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity[../cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cbc:CompanyID]
```

Test:
```xpath
u:checkSEOrgnr(normalize-space(cbc:CompanyID))
```

Required fixture IDs: `SE-R-013-valid`, `SE-R-013-invalid`, `SE-R-013-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-005

Severity: fatal. Source pattern 7, rule 3.

Context:
```xpath
//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and exists(cac:PartyLegalEntity/cbc:CompanyID)]/cac:PartyTaxScheme[normalize-space(upper-case(cac:TaxScheme/cbc:ID)) != 'VAT']/cbc:CompanyID
```

Test:
```xpath
normalize-space(upper-case(.)) = 'GODKÄND FÖR F-SKATT'
```

Required fixture IDs: `SE-R-005-valid`, `SE-R-005-invalid`, `SE-R-005-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-006

Severity: fatal. Source pattern 7, rule 4.

Context:
```xpath
//cac:TaxCategory[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2) = 'SE'] and cbc:ID = 'S'] | //cac:ClassifiedTaxCategory[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE' and cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2) = 'SE'] and cbc:ID = 'S']
```

Test:
```xpath
number(cbc:Percent) = 25 or number(cbc:Percent) = 12 or number(cbc:Percent) = 6
```

Required fixture IDs: `SE-R-006-valid`, `SE-R-006-invalid`, `SE-R-006-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-007

Severity: warning. Source pattern 7, rule 5.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and normalize-space(cbc:PaymentMeansCode) = '30' and normalize-space(cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID) = 'SE:PLUSGIRO']/cac:PayeeFinancialAccount/cbc:ID
```

Test:
```xpath
string(number(normalize-space(.))) != 'NaN'
```

Required fixture IDs: `SE-R-007-valid`, `SE-R-007-invalid`, `SE-R-007-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-010

Severity: warning. Source pattern 7, rule 5.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and normalize-space(cbc:PaymentMeansCode) = '30' and normalize-space(cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID) = 'SE:PLUSGIRO']/cac:PayeeFinancialAccount/cbc:ID
```

Test:
```xpath
string-length(normalize-space(.)) >= 2 and string-length(normalize-space(.)) <= 8
```

Required fixture IDs: `SE-R-010-valid`, `SE-R-010-invalid`, `SE-R-010-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-008

Severity: warning. Source pattern 7, rule 6.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and normalize-space(cbc:PaymentMeansCode) = '30' and normalize-space(cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID) = 'SE:BANKGIRO']/cac:PayeeFinancialAccount/cbc:ID
```

Test:
```xpath
string(number(normalize-space(.))) != 'NaN'
```

Required fixture IDs: `SE-R-008-valid`, `SE-R-008-invalid`, `SE-R-008-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-009

Severity: warning. Source pattern 7, rule 6.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and normalize-space(cbc:PaymentMeansCode) = '30' and normalize-space(cac:PayeeFinancialAccount/cac:FinancialInstitutionBranch/cbc:ID) = 'SE:BANKGIRO']/cac:PayeeFinancialAccount/cbc:ID
```

Test:
```xpath
string-length(normalize-space(.)) = 7 or string-length(normalize-space(.)) = 8
```

Required fixture IDs: `SE-R-009-valid`, `SE-R-009-invalid`, `SE-R-009-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-011

Severity: warning. Source pattern 7, rule 7.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and (cbc:PaymentMeansCode = normalize-space('50') or cbc:PaymentMeansCode = normalize-space('56'))]
```

Test:
```xpath
false()
```

Required fixture IDs: `SE-R-011-valid`, `SE-R-011-invalid`, `SE-R-011-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## SE-R-012

Severity: warning. Source pattern 7, rule 8.

Context:
```xpath
//cac:PaymentMeans[//cac:AccountingSupplierParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE']  and //cac:AccountingCustomerParty/cac:Party[cac:PostalAddress/cac:Country/cbc:IdentificationCode = 'SE'] and (cbc:PaymentMeansCode = normalize-space('31'))]
```

Test:
```xpath
false()
```

Required fixture IDs: `SE-R-012-valid`, `SE-R-012-invalid`, `SE-R-012-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

