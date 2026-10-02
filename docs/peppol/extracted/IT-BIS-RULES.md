# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

Extracted assertion count: 4. [Global dependencies](BIS-DEPENDENCIES.md). All assertions in this source group are included; common rules must also run. Preserve official rule ordering and XPath 2.0 evaluation.

## IT-R-001

Severity: fatal. Source pattern 6, rule 1.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party[$supplierCountry = 'IT']/cac:PartyTaxScheme[normalize-space(cac:TaxScheme/cbc:ID) != 'VAT']
```

Test:
```xpath
matches(normalize-space(cbc:CompanyID),'^[A-Z0-9]{11,16}$')
```

Required fixture IDs: `IT-R-001-valid`, `IT-R-001-invalid`, `IT-R-001-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IT-R-002

Severity: fatal. Source pattern 6, rule 2.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party[$supplierCountry = 'IT']
```

Test:
```xpath
cac:PostalAddress/cbc:StreetName
```

Required fixture IDs: `IT-R-002-valid`, `IT-R-002-invalid`, `IT-R-002-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IT-R-003

Severity: fatal. Source pattern 6, rule 2.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party[$supplierCountry = 'IT']
```

Test:
```xpath
cac:PostalAddress/cbc:CityName
```

Required fixture IDs: `IT-R-003-valid`, `IT-R-003-invalid`, `IT-R-003-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

## IT-R-004

Severity: fatal. Source pattern 6, rule 2.

Context:
```xpath
cac:AccountingSupplierParty/cac:Party[$supplierCountry = 'IT']
```

Test:
```xpath
cac:PostalAddress/cbc:PostalZone
```

Required fixture IDs: `IT-R-004-valid`, `IT-R-004-invalid`, `IT-R-004-not-applicable`, plus boundary cases. These are planned IDs, not generated samples. Fatal failures must invalidate; warnings must remain separately observable. For non-applicable cases assert absence of this ID, not overall document validity.

