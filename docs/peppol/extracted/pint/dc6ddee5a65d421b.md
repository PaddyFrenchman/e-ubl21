# jp/pint-jp-ntr 1.1.3 — PINT-jurisdiction-aligned-rules.sch

Source package: [https://docs.peppol.eu/poac/jp/pint-jp-ntr/resources.zip](https://docs.peppol.eu/poac/jp/pint-jp-ntr/resources.zip). Member: `trn-invoice/schematron/PINT-jurisdiction-aligned-rules.sch`. SHA-256: `dc6ddee5a65d421b62a65462dbc2217c69e6c668caae42a523e80195a0c95a6e`.

All 41 assertions and 0 reports extracted. Technical expressions are preserved; narrative messages are omitted. Use the official validator, preserving rule ordering, context selection and namespace bindings. Fixture references below are planned, not generated.

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
/ubl:Invoice/cac:AllowanceCharge[cbc:ChargeIndicator = false()] | /cn:CreditNote/cac:AllowanceCharge[cbc:ChargeIndicator = false()]
```

#### aligned-ibrp-032-jp

Kind: assert. Severity: fatal.

```xpath
(exists(../cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID = 'O') and not(exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:Percent))) or (exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID) and exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:Percent))
```

Planned samples: `aligned-ibrp-032-jp-valid`, `aligned-ibrp-032-jp-invalid`, `aligned-ibrp-032-jp-boundary`, `aligned-ibrp-032-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 2

```xpath
/ubl:Invoice/cac:AllowanceCharge[cbc:ChargeIndicator = true()] | /cn:CreditNote/cac:AllowanceCharge[cbc:ChargeIndicator = true()]
```

#### aligned-ibrp-037-jp

Kind: assert. Severity: fatal.

```xpath
(exists(../cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID = 'O') and not(exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:Percent))) or (exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID) and exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:Percent))
```

Planned samples: `aligned-ibrp-037-jp-valid`, `aligned-ibrp-037-jp-invalid`, `aligned-ibrp-037-jp-boundary`, `aligned-ibrp-037-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 3

```xpath
/ubl:InvoiceLine | /cn:CreditNoteLine
```

#### aligned-ibrp-050-jp

Kind: assert. Severity: fatal.

```xpath
(cac:Item/cac:ClassifiedTaxCategory[cac:TaxScheme/(normalize-space(upper-case(cbc:ID))='VAT')]/cbc:ID) and (cac:Item/cac:ClassifiedTaxCategory[cac:TaxScheme/(normalize-space(upper-case(cbc:ID))='VAT')]/cbc:Percent)
```

Planned samples: `aligned-ibrp-050-jp-valid`, `aligned-ibrp-050-jp-invalid`, `aligned-ibrp-050-jp-boundary`, `aligned-ibrp-050-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 4

```xpath
/ubl:Invoice | /cn:CreditNote
```

#### aligned-ibrp-001-jp-ntr

Kind: assert. Severity: fatal.

```xpath
starts-with(normalize-space(cbc:CustomizationID/text()), 'urn:peppol:pint:nontaxinvoice-1@jp-1')
```

Planned samples: `aligned-ibrp-001-jp-ntr-valid`, `aligned-ibrp-001-jp-ntr-invalid`, `aligned-ibrp-001-jp-ntr-boundary`, `aligned-ibrp-001-jp-ntr-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-002-jp

Kind: assert. Severity: fatal.

```xpath
/*/cbc:ProfileID and matches(normalize-space(/*/cbc:ProfileID), 'urn:peppol:bis:billing')
```

Planned samples: `aligned-ibrp-002-jp-valid`, `aligned-ibrp-002-jp-invalid`, `aligned-ibrp-002-jp-boundary`, `aligned-ibrp-002-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-009

Kind: assert. Severity: fatal.

```xpath
(count(cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/upper-case(cbc:ID)='VAT']/cbc:CompanyID) <= 1)
```

Planned samples: `aligned-ibrp-009-valid`, `aligned-ibrp-009-invalid`, `aligned-ibrp-009-boundary`, `aligned-ibrp-009-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-014

Kind: assert. Severity: fatal.

```xpath
count(//cbc:PaymentID[not(preceding::cbc:PaymentID/. = .)]) <= 1
```

Planned samples: `aligned-ibrp-014-valid`, `aligned-ibrp-014-invalid`, `aligned-ibrp-014-boundary`, `aligned-ibrp-014-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-052

Kind: assert. Severity: fatal.

```xpath
exists(//cac:InvoicePeriod)
```

Planned samples: `aligned-ibrp-052-valid`, `aligned-ibrp-052-invalid`, `aligned-ibrp-052-boundary`, `aligned-ibrp-052-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-e-01

Kind: assert. Severity: fatal.

```xpath
((exists(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'E']) or exists(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'E'])) and (count(cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'E']) = 1)) or (not(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'E']) and not(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'E']))
```

Planned samples: `aligned-ibrp-e-01-valid`, `aligned-ibrp-e-01-invalid`, `aligned-ibrp-e-01-boundary`, `aligned-ibrp-e-01-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-g-01

Kind: assert. Severity: fatal.

```xpath
((exists(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'G']) or exists(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'G'])) and (count(cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'G']) = 1)) or (not(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'G']) and not(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'G']))
```

Planned samples: `aligned-ibrp-g-01-valid`, `aligned-ibrp-g-01-invalid`, `aligned-ibrp-g-01-boundary`, `aligned-ibrp-g-01-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-o-01

Kind: assert. Severity: fatal.

```xpath
((exists(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'O']) or exists(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'O'])) and (count(cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'O']) = 1)) or (not(//cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'O']) and not(//cac:ClassifiedTaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID[normalize-space(.) = 'O']))
```

Planned samples: `aligned-ibrp-o-01-valid`, `aligned-ibrp-o-01-invalid`, `aligned-ibrp-o-01-boundary`, `aligned-ibrp-o-01-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-sr-12

Kind: assert. Severity: fatal.

```xpath
(count(cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/upper-case(cbc:ID)='VAT']/cbc:CompanyID) <= 1)
```

Planned samples: `aligned-ibrp-sr-12-valid`, `aligned-ibrp-sr-12-invalid`, `aligned-ibrp-sr-12-boundary`, `aligned-ibrp-sr-12-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-sr-13

Kind: assert. Severity: fatal.

```xpath
(count(cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/upper-case(cbc:ID)!='VAT']/cbc:CompanyID) <= 1)
```

Planned samples: `aligned-ibrp-sr-13-valid`, `aligned-ibrp-sr-13-invalid`, `aligned-ibrp-sr-13-boundary`, `aligned-ibrp-sr-13-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibr-jp-04-ntr

Kind: assert. Severity: fatal.

```xpath
not(cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme/cbc:CompanyID)
```

Planned samples: `aligned-ibr-jp-04-ntr-valid`, `aligned-ibr-jp-04-ntr-invalid`, `aligned-ibr-jp-04-ntr-boundary`, `aligned-ibr-jp-04-ntr-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibr-jp-05

Kind: assert. Severity: fatal.

```xpath
not(exists(cbc:TaxCurrencyCode)) or cbc:TaxCurrencyCode = 'JPY'
```

Planned samples: `aligned-ibr-jp-05-valid`, `aligned-ibr-jp-05-invalid`, `aligned-ibr-jp-05-boundary`, `aligned-ibr-jp-05-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-053-jp

Kind: assert. Severity: fatal.

```xpath
(count(cac:TaxTotal/cbc:TaxAmount[@currencyID=/*/cbc:DocumentCurrencyCode])) <= 1
```

Planned samples: `aligned-ibrp-053-jp-valid`, `aligned-ibrp-053-jp-invalid`, `aligned-ibrp-053-jp-boundary`, `aligned-ibrp-053-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 5

```xpath
cac:TaxSubtotal[cbc:TaxAmount/@currencyID=/ubl:Invoice/cbc:DocumentCurrencyCode/text()]
```

#### aligned-ibrp-045

Kind: assert. Severity: fatal.

```xpath
exists(cbc:TaxableAmount)
```

Planned samples: `aligned-ibrp-045-valid`, `aligned-ibrp-045-invalid`, `aligned-ibrp-045-boundary`, `aligned-ibrp-045-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 6

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
exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:ID)
```

Planned samples: `aligned-ibrp-047-valid`, `aligned-ibrp-047-invalid`, `aligned-ibrp-047-boundary`, `aligned-ibrp-047-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibrp-048

Kind: assert. Severity: fatal.

```xpath
exists(cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/cbc:Percent) or (cac:TaxCategory[cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']/normalize-space(cbc:ID)='O')
```

Planned samples: `aligned-ibrp-048-valid`, `aligned-ibrp-048-invalid`, `aligned-ibrp-048-boundary`, `aligned-ibrp-048-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

#### aligned-ibr-jp-06

Kind: assert. Severity: fatal.

```xpath
not(contains(cbc:TaxAmount[@currencyID='JPY'], '.'))
```

Planned samples: `aligned-ibr-jp-06-valid`, `aligned-ibr-jp-06-invalid`, `aligned-ibr-jp-06-boundary`, `aligned-ibr-jp-06-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 7

```xpath
cac:TaxRepresentativeParty
```

#### aligned-ibrp-010

Kind: assert. Severity: fatal.

```xpath
(count(cac:PartyTaxScheme/cbc:CompanyID) <= 1)
```

Planned samples: `aligned-ibrp-010-valid`, `aligned-ibrp-010-invalid`, `aligned-ibrp-010-boundary`, `aligned-ibrp-010-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 8

```xpath
cac:PaymentMeans[cbc:PaymentMeansCode='30' or cbc:PaymentMeansCode='58']/cac:PayeeFinancialAccount
```

#### aligned-ibrp-016

Kind: assert. Severity: fatal.

```xpath
(cbc:ID) != ''
```

Planned samples: `aligned-ibrp-016-valid`, `aligned-ibrp-016-invalid`, `aligned-ibrp-016-boundary`, `aligned-ibrp-016-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 9

```xpath
cac:PaymentMeans/cac:CardAccount
```

#### aligned-ibrp-017

Kind: assert. Severity: fatal.

```xpath
string-length(cbc:PrimaryAccountNumberID)>=4 and string-length(cbc:PrimaryAccountNumberID)<=6
```

Planned samples: `aligned-ibrp-017-valid`, `aligned-ibrp-017-invalid`, `aligned-ibrp-017-boundary`, `aligned-ibrp-017-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 10

```xpath
cac:PaymentMeans
```

#### aligned-ibrp-049

Kind: assert. Severity: fatal.

```xpath
exists(cbc:PaymentMeansCode)
```

Planned samples: `aligned-ibrp-049-valid`, `aligned-ibrp-049-invalid`, `aligned-ibrp-049-boundary`, `aligned-ibrp-049-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 11

```xpath
cac:PaymentMeans[(cbc:PaymentMeansCode='49' or cbc:PaymentMeansCode='59')]
```

#### aligned-ibrp-019

Kind: assert. Severity: fatal.

```xpath
cac:PaymentMandate/cbc:ID
```

Planned samples: `aligned-ibrp-019-valid`, `aligned-ibrp-019-invalid`, `aligned-ibrp-019-boundary`, `aligned-ibrp-019-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 12

```xpath
cac:InvoiceLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT'] | cac:CreditNoteLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-05

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-e-05-valid`, `aligned-ibrp-e-05-invalid`, `aligned-ibrp-e-05-boundary`, `aligned-ibrp-e-05-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 13

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=false()]/cac:TaxCategory[normalize-space(cbc:ID)='E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-06

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-e-06-valid`, `aligned-ibrp-e-06-invalid`, `aligned-ibrp-e-06-boundary`, `aligned-ibrp-e-06-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 14

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=true()]/cac:TaxCategory[normalize-space(cbc:ID)='E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-07

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-e-07-valid`, `aligned-ibrp-e-07-invalid`, `aligned-ibrp-e-07-boundary`, `aligned-ibrp-e-07-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 15

```xpath
cbc:TaxAmount
```

#### aligned-ibr-jp-07-ntr

Kind: assert. Severity: fatal.

```xpath
. = 0
```

Planned samples: `aligned-ibr-jp-07-ntr-valid`, `aligned-ibr-jp-07-ntr-invalid`, `aligned-ibr-jp-07-ntr-boundary`, `aligned-ibr-jp-07-ntr-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 16

```xpath
cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'E'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-e-09

Kind: assert. Severity: fatal.

```xpath
xs:decimal(../cbc:TaxAmount) = 0
```

Planned samples: `aligned-ibrp-e-09-valid`, `aligned-ibrp-e-09-invalid`, `aligned-ibrp-e-09-boundary`, `aligned-ibrp-e-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 17

```xpath
cac:InvoiceLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'G'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT'] | cac:CreditNoteLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'G'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-g-05

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-g-05-valid`, `aligned-ibrp-g-05-invalid`, `aligned-ibrp-g-05-boundary`, `aligned-ibrp-g-05-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 18

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=false()]/cac:TaxCategory[normalize-space(cbc:ID)='G'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-g-06

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-g-06-valid`, `aligned-ibrp-g-06-invalid`, `aligned-ibrp-g-06-boundary`, `aligned-ibrp-g-06-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 19

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=true()]/cac:TaxCategory[normalize-space(cbc:ID)='G'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-g-07

Kind: assert. Severity: fatal.

```xpath
(xs:decimal(cbc:Percent) = 0)
```

Planned samples: `aligned-ibrp-g-07-valid`, `aligned-ibrp-g-07-invalid`, `aligned-ibrp-g-07-boundary`, `aligned-ibrp-g-07-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 20

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'G'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-g-09

Kind: assert. Severity: fatal.

```xpath
xs:decimal(../cbc:TaxAmount) = 0
```

Planned samples: `aligned-ibrp-g-09-valid`, `aligned-ibrp-g-09-invalid`, `aligned-ibrp-g-09-boundary`, `aligned-ibrp-g-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 21

```xpath
cac:InvoiceLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT'] | cac:CreditNoteLine/cac:Item/cac:ClassifiedTaxCategory[normalize-space(cbc:ID) = 'O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-o-05

Kind: assert. Severity: fatal.

```xpath
not(cbc:Percent)
```

Planned samples: `aligned-ibrp-o-05-valid`, `aligned-ibrp-o-05-invalid`, `aligned-ibrp-o-05-boundary`, `aligned-ibrp-o-05-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 22

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=false()]/cac:TaxCategory[normalize-space(cbc:ID)='O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-o-06

Kind: assert. Severity: fatal.

```xpath
not(cbc:Percent)
```

Planned samples: `aligned-ibrp-o-06-valid`, `aligned-ibrp-o-06-invalid`, `aligned-ibrp-o-06-boundary`, `aligned-ibrp-o-06-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 23

```xpath
cac:AllowanceCharge[cbc:ChargeIndicator=true()]/cac:TaxCategory[normalize-space(cbc:ID)='O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-o-07

Kind: assert. Severity: fatal.

```xpath
not(cbc:Percent)
```

Planned samples: `aligned-ibrp-o-07-valid`, `aligned-ibrp-o-07-invalid`, `aligned-ibrp-o-07-boundary`, `aligned-ibrp-o-07-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 24

```xpath
/*/cac:TaxTotal/cac:TaxSubtotal/cac:TaxCategory[normalize-space(cbc:ID) = 'O'][cac:TaxScheme/normalize-space(upper-case(cbc:ID))='VAT']
```

#### aligned-ibrp-o-09

Kind: assert. Severity: fatal.

```xpath
xs:decimal(../cbc:TaxAmount) = 0
```

Planned samples: `aligned-ibrp-o-09-valid`, `aligned-ibrp-o-09-invalid`, `aligned-ibrp-o-09-boundary`, `aligned-ibrp-o-09-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

### Context 25

```xpath
cac:TaxCategory/cac:TaxScheme/cbc:ID | cac:ClassifiedTaxCategory/cac:TaxScheme/cbc:ID
```

#### aligned-ibr-jp-03

Kind: assert. Severity: fatal.

```xpath
matches(normalize-space(.),'VAT')
```

Planned samples: `aligned-ibr-jp-03-valid`, `aligned-ibr-jp-03-invalid`, `aligned-ibr-jp-03-boundary`, `aligned-ibr-jp-03-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

## Pattern 2 Codesmodelaligned

### Context 1

```xpath
cac:TaxCategory/cbc:ID | cac:ClassifiedTaxCategory/cbc:ID
```

#### aligned-ibrp-cl-01-jp

Kind: assert. Severity: fatal.

```xpath
( ( not(contains(normalize-space(.),' ')) and contains( ' AA E S G O ',concat(' ',normalize-space(.),' ') ) ) )
```

Planned samples: `aligned-ibrp-cl-01-jp-valid`, `aligned-ibrp-cl-01-jp-invalid`, `aligned-ibrp-cl-01-jp-boundary`, `aligned-ibrp-cl-01-jp-not-applicable`. Qualify IDs by profile, release, member and document type. Assert the exact ID/severity when applicable and its absence when not applicable; overall validity may be affected by other rules.

