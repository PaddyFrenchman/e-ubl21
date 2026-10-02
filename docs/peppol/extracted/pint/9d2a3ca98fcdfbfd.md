# my/pint-my 1.3.1 — PINT-jurisdiction-aligned-rules.sch

Source package: [https://docs.peppol.eu/poac/my/pint-my/resources.zip](https://docs.peppol.eu/poac/my/pint-my/resources.zip). Member: `trn-creditnote/schematron/PINT-jurisdiction-aligned-rules.sch`. SHA-256: `9d2a3ca98fcdfbfdebfc1b545518a542770c3717d4cb67937081c0f68e0271a3`.

All 28 assertions and 0 reports extracted. Technical expressions are preserved; narrative messages are omitted. Use the official validator, preserving rule ordering, context selection and namespace bindings. Fixture references below are planned, not generated.

## Global definitions

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:slack" as="xs:boolean">
    <ns0:param name="exp" as="xs:decimal" />
    <ns0:param name="val" as="xs:decimal" />
    <ns0:param name="slack" as="xs:decimal" />
    <ns0:value-of select="xs:decimal($exp + $slack) &gt;= $val and xs:decimal($exp - $slack) &lt;= $val" />
  </ns0:function>
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="ext" uri="urn:oasis:names:specification:ubl:schema:xsd:CommonExtensionComponents-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="cbc" uri="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="cac" uri="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="qdt" uri="urn:oasis:names:specification:ubl:schema:xsd:QualifiedDataTypes-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="udt" uri="urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="cn" uri="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="ubl" uri="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="u" uri="utils" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" prefix="xs" uri="http://www.w3.org/2001/XMLSchema" />
  
```

```xml
<ns0:phase xmlns:ns0="http://purl.oclc.org/dsdl/schematron" id="PINTmodelaligned_phase">
    <ns0:active pattern="UBL-modelaligned" />
  </ns0:phase>
  
```

```xml
<ns0:phase xmlns:ns0="http://purl.oclc.org/dsdl/schematron" id="codelistaligned_phase">
    <ns0:active pattern="Codesmodelaligned" />
  </ns0:phase>
  
```

## Pattern 1 UBL-modelaligned

### Context 1

```xpath
/ubl:Invoice | /cn:CreditNote
```

#### aligned-ibrp-001-my

Kind: assert. Severity: fatal.

```xpath
starts-with(normalize-space(cbc:CustomizationID/text()), 'urn:peppol:pint:billing-1@my-1')
```

Planned samples: `aligned-ibrp-001-my-valid`, `aligned-ibrp-001-my-invalid`, `aligned-ibrp-001-my-boundary`, `aligned-ibrp-001-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-002

Kind: assert. Severity: fatal.

```xpath
/*/cbc:ProfileID and matches(normalize-space(/*/cbc:ProfileID), 'urn:peppol:bis:billing')
```

Planned samples: `aligned-ibrp-002-valid`, `aligned-ibrp-002-invalid`, `aligned-ibrp-002-boundary`, `aligned-ibrp-002-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### ibr-02-my

Kind: assert. Severity: fatal.

```xpath
exists(cac:AccountingSupplierParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID)
```

Planned samples: `ibr-02-my-valid`, `ibr-02-my-invalid`, `ibr-02-my-boundary`, `ibr-02-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### ibr-03-my

Kind: assert. Severity: fatal.

```xpath
exists(cac:AccountingCustomerParty/cac:Party/cac:PartyLegalEntity/cbc:CompanyID)
```

Planned samples: `ibr-03-my-valid`, `ibr-03-my-invalid`, `ibr-03-my-boundary`, `ibr-03-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### ibr-04-my

Kind: assert. Severity: fatal.

```xpath
exists(cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID != 'VAT']/cbc:CompanyID) or exists(cac:AccountingSupplierParty/cac:Party/cac:PartyIdentification/cbc:ID[not(@schemeID)])
```

Planned samples: `ibr-04-my-valid`, `ibr-04-my-invalid`, `ibr-04-my-boundary`, `ibr-04-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 2

```xpath
cac:TaxSubtotal
```

#### aligned-ibrp-046

Kind: assert. Severity: fatal.

```xpath
exists(cbc:TaxAmount)
```

Planned samples: `aligned-ibrp-046-valid`, `aligned-ibrp-046-invalid`, `aligned-ibrp-046-boundary`, `aligned-ibrp-046-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-047

Kind: assert. Severity: fatal.

```xpath
exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID) or exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='AAL'][normalize-space(cbc:ID)='TTX'])
```

Planned samples: `aligned-ibrp-047-valid`, `aligned-ibrp-047-invalid`, `aligned-ibrp-047-boundary`, `aligned-ibrp-047-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-048

Kind: assert. Severity: fatal.

```xpath
(exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT' and (cbc:Percent or normalize-space(cbc:ID)='O')])) or (exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='AAL' and normalize-space(cbc:ID)='TTX' and not(cbc:Percent)]))
```

Planned samples: `aligned-ibrp-048-valid`, `aligned-ibrp-048-invalid`, `aligned-ibrp-048-boundary`, `aligned-ibrp-048-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 3

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'SA'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-sa-08

Kind: assert. Severity: fatal.

```xpath
every $rate in xs:decimal((cbc:Percent, 0)[1]) satisfies (((exists(//cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'SA'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='SA'][cac:Item/ cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))) or ((exists(//cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'SA'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='SA'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='SA'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))))
```

Planned samples: `aligned-ibrp-sa-08-valid`, `aligned-ibrp-sa-08-invalid`, `aligned-ibrp-sa-08-boundary`, `aligned-ibrp-sa-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-sa-09

Kind: assert. Severity: fatal.

```xpath
u:slack(abs(xs:decimal(../cbc:TaxAmount)) , round((abs(xs:decimal(../cbc:TaxableAmount)) * (xs:decimal((cbc:Percent, 0)[1]) div 100)) * 10 * 10) div 100 ,0.02 )
```

Planned samples: `aligned-ibrp-sa-09-valid`, `aligned-ibrp-sa-09-invalid`, `aligned-ibrp-sa-09-boundary`, `aligned-ibrp-sa-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-sa-10

Kind: assert. Severity: fatal.

```xpath
not(cbc:TaxExemptionReason) and not(cbc:TaxExemptionReasonCode)
```

Planned samples: `aligned-ibrp-sa-10-valid`, `aligned-ibrp-sa-10-invalid`, `aligned-ibrp-sa-10-boundary`, `aligned-ibrp-sa-10-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 4

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'SE'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-se-08

Kind: assert. Severity: fatal.

```xpath
every $rate in xs:decimal((cbc:Percent, 0)[1]) satisfies (((exists(//cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'SE'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='SE'][cac:Item/ cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))) or ((exists(//cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'SE'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='SE'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='SE'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))))
```

Planned samples: `aligned-ibrp-se-08-valid`, `aligned-ibrp-se-08-invalid`, `aligned-ibrp-se-08-boundary`, `aligned-ibrp-se-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-se-09

Kind: assert. Severity: fatal.

```xpath
u:slack(abs(xs:decimal(../cbc:TaxAmount)) , round((abs(xs:decimal(../cbc:TaxableAmount)) * (xs:decimal((cbc:Percent, 0)[1]) div 100)) * 10 * 10) div 100 ,0.02 )
```

Planned samples: `aligned-ibrp-se-09-valid`, `aligned-ibrp-se-09-invalid`, `aligned-ibrp-se-09-boundary`, `aligned-ibrp-se-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-se-10

Kind: assert. Severity: fatal.

```xpath
not(cbc:TaxExemptionReason) and not(cbc:TaxExemptionReasonCode)
```

Planned samples: `aligned-ibrp-se-10-valid`, `aligned-ibrp-se-10-invalid`, `aligned-ibrp-se-10-boundary`, `aligned-ibrp-se-10-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 5

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'HVG'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-hvg-08

Kind: assert. Severity: fatal.

```xpath
every $rate in xs:decimal((cbc:Percent, 0)[1]) satisfies (((exists(//cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'HVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='HVG'][cac:Item/ cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))) or ((exists(//cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'HVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='HVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='HVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))))
```

Planned samples: `aligned-ibrp-hvg-08-valid`, `aligned-ibrp-hvg-08-invalid`, `aligned-ibrp-hvg-08-boundary`, `aligned-ibrp-hvg-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-hvg-09

Kind: assert. Severity: fatal.

```xpath
u:slack(abs(xs:decimal(../cbc:TaxAmount)) , round((abs(xs:decimal(../cbc:TaxableAmount)) * (xs:decimal((cbc:Percent, 0)[1]) div 100)) * 10 * 10) div 100 ,0.02 )
```

Planned samples: `aligned-ibrp-hvg-09-valid`, `aligned-ibrp-hvg-09-invalid`, `aligned-ibrp-hvg-09-boundary`, `aligned-ibrp-hvg-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-hvg-10

Kind: assert. Severity: fatal.

```xpath
not(cbc:TaxExemptionReason) and not(cbc:TaxExemptionReasonCode)
```

Planned samples: `aligned-ibrp-hvg-10-valid`, `aligned-ibrp-hvg-10-invalid`, `aligned-ibrp-hvg-10-boundary`, `aligned-ibrp-hvg-10-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 6

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'LVG'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-lvg-08

Kind: assert. Severity: fatal.

```xpath
every $rate in xs:decimal((cbc:Percent, 0)[1]) satisfies (((exists(//cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'LVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='LVG'][cac:Item/ cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))) or ((exists(//cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID) = 'LVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]) or exists(//cac:AllowanceCharge[cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate])) and (u:slack(../xs:decimal(cbc:TaxableAmount), sum(../../../cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='LVG'][cac:Item/cac:ClassifiedTaxCategory/xs:decimal((cbc:Percent, 0)[1]) =$rate]/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='LVG'][cac:TaxCategory/xs:decimal((cbc:Percent, 0)[1]) = $rate]/xs:decimal(cbc:Amount)),0.02))))
```

Planned samples: `aligned-ibrp-lvg-08-valid`, `aligned-ibrp-lvg-08-invalid`, `aligned-ibrp-lvg-08-boundary`, `aligned-ibrp-lvg-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-lvg-09

Kind: assert. Severity: fatal.

```xpath
u:slack(abs(xs:decimal(../cbc:TaxAmount)) , round((abs(xs:decimal(../cbc:TaxableAmount)) * (xs:decimal((cbc:Percent, 0)[1]) div 100)) * 10 * 10) div 100 ,0.02 )
```

Planned samples: `aligned-ibrp-lvg-09-valid`, `aligned-ibrp-lvg-09-invalid`, `aligned-ibrp-lvg-09-boundary`, `aligned-ibrp-lvg-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-lvg-10

Kind: assert. Severity: fatal.

```xpath
not(cbc:TaxExemptionReason) and not(cbc:TaxExemptionReasonCode)
```

Planned samples: `aligned-ibrp-lvg-10-valid`, `aligned-ibrp-lvg-10-invalid`, `aligned-ibrp-lvg-10-boundary`, `aligned-ibrp-lvg-10-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 7

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID)='TTX'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='AAL']
```

#### aligned-ibrp-ttx-08

Kind: assert. Severity: fatal.

```xpath
not(cbc:Percent)
```

Planned samples: `aligned-ibrp-ttx-08-valid`, `aligned-ibrp-ttx-08-invalid`, `aligned-ibrp-ttx-08-boundary`, `aligned-ibrp-ttx-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-ttx-09

Kind: assert. Severity: fatal.

```xpath
(exists(//cac:InvoiceLine)               and u:slack(                     abs(xs:decimal(../cbc:TaxAmount)),                     sum(../../../cac:InvoiceLine                           [cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='TTX']                           /cac:TaxTotal/xs:decimal(cbc:TaxAmount))                     + sum(../../../cac:AllowanceCharge                           [cbc:ChargeIndicator=true()]                           [cac:TaxCategory/normalize-space(cbc:ID)='TTX']                           /xs:decimal(cbc:Amount))                     - sum(../../../cac:AllowanceCharge                           [cbc:ChargeIndicator=false()]                           [cac:TaxCategory/normalize-space(cbc:ID)='TTX']                           /xs:decimal(cbc:Amount)),                     0.02                  )           )           or           (exists(//cac:CreditNoteLine)               and u:slack(                     abs(xs:decimal(../cbc:TaxAmount)),                     sum(../../../cac:CreditNoteLine                           [cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='TTX']                           /cac:TaxTotal/xs:decimal(cbc:TaxAmount))                     + sum(../../../cac:AllowanceCharge                           [cbc:ChargeIndicator=true()]                           [cac:TaxCategory/normalize-space(cbc:ID)='TTX']                           /xs:decimal(cbc:Amount))                     - sum(../../../cac:AllowanceCharge                           [cbc:ChargeIndicator=false()]                           [cac:TaxCategory/normalize-space(cbc:ID)='TTX']                           /xs:decimal(cbc:Amount)),                     0.02                  )           )
```

Planned samples: `aligned-ibrp-ttx-09-valid`, `aligned-ibrp-ttx-09-invalid`, `aligned-ibrp-ttx-09-boundary`, `aligned-ibrp-ttx-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 8

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-08

Kind: assert. Severity: fatal.

```xpath
(exists(//cac:InvoiceLine) and (xs:decimal(../cbc:TaxableAmount) = (sum(../../../cac:InvoiceLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:Amount))))) or (exists(//cac:CreditNoteLine) and (xs:decimal(../cbc:TaxableAmount) = (sum(../../../cac:CreditNoteLine[cac:Item/cac:ClassifiedTaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:LineExtensionAmount)) + sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=true()][cac:TaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:Amount)) - sum(../../../cac:AllowanceCharge[cbc:ChargeIndicator=false()][cac:TaxCategory/normalize-space(cbc:ID)='E']/xs:decimal(cbc:Amount)))))
```

Planned samples: `aligned-ibrp-e-08-valid`, `aligned-ibrp-e-08-invalid`, `aligned-ibrp-e-08-boundary`, `aligned-ibrp-e-08-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-e-09

Kind: assert. Severity: fatal.

```xpath
xs:decimal(../cbc:TaxAmount) = 0
```

Planned samples: `aligned-ibrp-e-09-valid`, `aligned-ibrp-e-09-invalid`, `aligned-ibrp-e-09-boundary`, `aligned-ibrp-e-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 9

```xpath
cac:InvoiceLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT'] | cac:CreditNoteLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-05

Kind: assert. Severity: fatal.

```xpath
(xs:decimal((cbc:Percent, 0)[1]) = 0)
```

Planned samples: `aligned-ibrp-e-05-valid`, `aligned-ibrp-e-05-invalid`, `aligned-ibrp-e-05-boundary`, `aligned-ibrp-e-05-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 10

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-o-09

Kind: assert. Severity: fatal.

```xpath
xs:decimal(../cbc:TaxAmount) = 0
```

Planned samples: `aligned-ibrp-o-09-valid`, `aligned-ibrp-o-09-invalid`, `aligned-ibrp-o-09-boundary`, `aligned-ibrp-o-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

## Pattern 2 Codesmodelaligned

### Context 1

```xpath
cac:TaxCategory/cbc:ID | cac:ClassifiedTaxCategory/cbc:ID
```

#### aligned-ibrp-cl-01-my

Kind: assert. Severity: fatal.

```xpath
( ( not(contains(normalize-space(.),' ')) and contains( ' SA SE HVG LVG E O TTX ',concat(' ',normalize-space(.),' ') ) ) )
```

Planned samples: `aligned-ibrp-cl-01-my-valid`, `aligned-ibrp-cl-01-my-invalid`, `aligned-ibrp-cl-01-my-boundary`, `aligned-ibrp-cl-01-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 2

```xpath
cbc:TaxCurrencyCode
```

#### ibr-cl-05-my

Kind: assert. Severity: fatal.

```xpath
( ( not(contains(normalize-space(.),' ')) and contains( ' MYR ',concat(' ',normalize-space(.),' ') ) ) )
```

Planned samples: `ibr-cl-05-my-valid`, `ibr-cl-05-my-invalid`, `ibr-cl-05-my-boundary`, `ibr-cl-05-my-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

