# Extracted BIS Billing validation rules

Release: 3.0.21. Source: [https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch](https://github.com/OpenPEPPOL/peppol-bis-invoice-3/blob/806866bd2bd91d7e9623b68f08164e8fbe9e67a0/rules/sch/PEPPOL-EN16931-UBL.sch).

Source SHA-256: `62e5b67892f12755352d78b06f63229a02cc2eccc748677c56efbc8dbcb336e3`. Extraction checked 2026-10-02. Expressions are technical extracts; official narrative specifications remain authoritative. No generated fixtures or regression results are claimed.

## Namespace bindings and dependency definitions

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2" prefix="cbc" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2" prefix="cac" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="urn:oasis:names:specification:ubl:schema:xsd:CreditNote-2" prefix="ubl-creditnote" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2" prefix="ubl-invoice" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="http://www.w3.org/2001/XMLSchema" prefix="xs" />
  
```

```xml
<ns0:ns xmlns:ns0="http://purl.oclc.org/dsdl/schematron" uri="utils" prefix="u" />
  
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="profile" value="        if (normalize-space(/*/cbc:ProfileID) = (         'urn:fdc:peppol.eu:2017:poacc:billing:01:1.0',         'urn:peppol:france:billing:regulated',         'urn:peppol:france:billing:non-regulated'        ))        then '01'        else if (normalize-space(/*/cbc:ProfileID) = 'urn:peppol:bis:billing_with_response')        then '02'        else 'Unknown'      " />
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountry" value="       if (/*/cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)) then         upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)))       else         if (/*/cac:TaxRepresentativeParty/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)) then           upper-case(normalize-space(/*/cac:TaxRepresentativeParty/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)))         else           if (/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode) then             upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode))           else             'XX'" />
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountry" value="   if (/*/cac:AccountingCustomerParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)) then   upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)))   else   if (/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode) then   upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode))   else   'XX'" />
  
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="supplierCountryIsDE" value="(upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'DE')" />
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="customerCountryIsDE" value="(upper-case(normalize-space(/*/cac:AccountingCustomerParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode)) = 'DE')" />

  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="documentCurrencyCode" value="/*/cbc:DocumentCurrencyCode" />
  
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:gln" as="xs:boolean">
    <ns0:param name="val" />
    <ns0:variable name="length" select="string-length($val) - 1" />
    <ns0:variable name="digits" select="reverse(for $i in string-to-codepoints(substring($val, 0, $length + 1)) return $i - 48)" />
    <ns0:variable name="weightedSum" select="sum(for $i in (0 to $length - 1) return $digits[$i + 1] * (1 + ((($i + 1) mod 2) * 2)))" />
    <ns0:sequence select="(10 - ($weightedSum mod 10)) mod 10 = number(substring($val, $length + 1, 1))" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:slack" as="xs:boolean">
    <ns0:param name="exp" as="xs:decimal" />
    <ns0:param name="val" as="xs:decimal" />
    <ns0:param name="slack" as="xs:decimal" />
    <ns0:sequence select="xs:decimal($exp + $slack) &gt;= $val and xs:decimal($exp - $slack) &lt;= $val" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:mod11" as="xs:boolean">
    <ns0:param name="val" />
    <ns0:variable name="length" select="string-length($val) - 1" />
    <ns0:variable name="digits" select="reverse(for $i in string-to-codepoints(substring($val, 0, $length + 1)) return $i - 48)" />
    <ns0:variable name="weightedSum" select="sum(for $i in (0 to $length - 1) return $digits[$i + 1] * (($i mod 6) + 2))" />
    <ns0:sequence select="number($val) &gt; 0 and (11 - ($weightedSum mod 11)) mod 11 = number(substring($val, $length + 1, 1))" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:mod97-0208" as="xs:boolean">
    <ns0:param name="val" />
    <ns0:variable name="checkdigits" select="substring($val,9,2)" />
    <ns0:variable name="calculated_digits" select="xs:string(97 - (xs:integer(substring($val,1,8)) mod 97))" />
    <ns0:sequence select="number($checkdigits) = number($calculated_digits)" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkCodiceIPA" as="xs:boolean">
    <ns0:param name="arg" as="xs:string?" />
    <ns0:variable name="allowed-characters">ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789</ns0:variable>
    <ns0:sequence select="if ( (string-length(translate($arg, $allowed-characters, '')) = 0) and (string-length($arg) = 6) ) then true() else false()" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkCF" as="xs:boolean">
    <ns0:param name="arg" as="xs:string?" />
    <ns0:sequence select="   if ( (string-length($arg) = 16) or (string-length($arg) = 11) )   then   (    if ((string-length($arg) = 16))    then    (     if (u:checkCF16($arg))     then     (      true()     )     else     (      false()     )    )    else    (     if(($arg castable as xs:integer)) then true() else false()     )   )   else   (    false()   )   " />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkCF16" as="xs:boolean">
    <ns0:param name="arg" as="xs:string?" />
    <ns0:variable name="allowed-characters">ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz</ns0:variable>
    <ns0:sequence select="     if (  (string-length(translate(substring($arg,1,6), $allowed-characters, '')) = 0) and       (substring($arg,7,2) castable as xs:integer) and       (string-length(translate(substring($arg,9,1), $allowed-characters, '')) = 0) and       (substring($arg,10,2) castable as xs:integer) and       (substring($arg,12,3) castable as xs:string) and       (substring($arg,15,1) castable as xs:integer) and       (string-length(translate(substring($arg,16,1), $allowed-characters, '')) = 0)      )     then true()     else false()     " />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkPIVAseIT" as="xs:boolean">
    <ns0:param name="arg" as="xs:string" />
    <ns0:variable name="paese" select="substring($arg,1,2)" />
    <ns0:variable name="codice" select="substring($arg,3)" />
    <ns0:sequence select="     if ( $paese = 'IT' or $paese = 'it' )    then    (     if ( ( string-length($codice) = 11 ) and ( if (u:checkPIVA($codice)!=0) then false() else true() ))     then     (      true()     )     else     (      false()     )    )    else    (     true()    )    " />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkPIVA" as="xs:integer">
    <ns0:param name="arg" as="xs:string?" />
    <ns0:sequence select="     if (not($arg castable as xs:integer))      then 1      else ( u:addPIVA($arg,xs:integer(0)) mod 10 )" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:addPIVA" as="xs:integer">
    <ns0:param name="arg" as="xs:string" />
    <ns0:param name="pari" as="xs:integer" />
    <ns0:variable name="tappo" select="if (not($arg castable as xs:integer)) then 0 else 1" />
    <ns0:variable name="mapper" select="if ($tappo = 0) then 0 else                   ( if ($pari = 1)                    then ( xs:integer(substring('0246813579', ( xs:integer(substring($arg,1,1)) +1 ) ,1)) )                    else ( xs:integer(substring($arg,1,1) ) )                   )" />
    <ns0:sequence select="if ($tappo = 0) then $mapper else ( xs:integer($mapper) + u:addPIVA(substring(xs:string($arg),2), (if($pari=0) then 1 else 0) ) )" />
  </ns0:function>
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:abn" as="xs:boolean">
    <ns0:param name="val" />
    <ns0:sequence select="( ((string-to-codepoints(substring($val,1,1)) - 49) * 10) + ((string-to-codepoints(substring($val,2,1)) - 48) * 1) + ((string-to-codepoints(substring($val,3,1)) - 48) * 3) + ((string-to-codepoints(substring($val,4,1)) - 48) * 5) + ((string-to-codepoints(substring($val,5,1)) - 48) * 7) + ((string-to-codepoints(substring($val,6,1)) - 48) * 9) + ((string-to-codepoints(substring($val,7,1)) - 48) * 11) + ((string-to-codepoints(substring($val,8,1)) - 48) * 13) + ((string-to-codepoints(substring($val,9,1)) - 48) * 15) + ((string-to-codepoints(substring($val,10,1)) - 48) * 17) + ((string-to-codepoints(substring($val,11,1)) - 48) * 19)) mod 89 = 0 " />
  </ns0:function>

  
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:TinVerification" as="xs:boolean">
    <ns0:param name="val" as="xs:string" />
    <ns0:variable name="digits" select="    for $ch in string-to-codepoints($val)    return codepoints-to-string($ch)" />
    <ns0:variable name="checksum" select="    (number($digits[8])*2) +    (number($digits[7])*4) +    (number($digits[6])*8) +    (number($digits[5])*16) +    (number($digits[4])*32) +    (number($digits[3])*64) +    (number($digits[2])*128) +    (number($digits[1])*256) " />
    <ns0:sequence select="($checksum  mod 11) mod 10 = number($digits[9])" />
  </ns0:function>

  
  
```

```xml
<ns0:function xmlns:ns0="http://www.w3.org/1999/XSL/Transform" name="u:checkSEOrgnr" as="xs:boolean">
    <ns0:param name="number" as="xs:string" />
    <ns0:choose>
      
      <ns0:when test="not(matches($number, '^\d+$'))">
        <ns0:sequence select="false()" />
      </ns0:when>
      <ns0:otherwise>
        
        <ns0:variable name="mainPart" select="substring($number, 1, 9)" />
        <ns0:variable name="checkDigit" select="substring($number, 10, 1)" />
        <ns0:variable name="sum" as="xs:integer">
          <ns0:sequence select="xs:integer(sum(       for $pos in 1 to string-length($mainPart) return        if ($pos mod 2 = 1)        then (number(substring($mainPart, string-length($mainPart) - $pos + 1, 1)) * 2) mod 10 +          (number(substring($mainPart, string-length($mainPart) - $pos + 1, 1)) * 2) idiv 10        else number(substring($mainPart, string-length($mainPart) - $pos + 1, 1))      ))" />
        </ns0:variable>
        <ns0:variable name="calculatedCheckDigit" select="(10 - $sum mod 10) mod 10" />
        <ns0:sequence select="$calculatedCheckDigit = number($checkDigit)" />
      </ns0:otherwise>
    </ns0:choose>
  </ns0:function>
  
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="isGreekSender" value="($supplierCountry ='GR') or ($supplierCountry ='EL')" />
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="isGreekReceiver" value="($customerCountry ='GR') or ($customerCountry ='EL')" />
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="isGreekSenderandReceiver" value="$isGreekSender and $isGreekReceiver" />
  
  
```

```xml
<ns0:let xmlns:ns0="http://purl.oclc.org/dsdl/schematron" name="accountingSupplierCountry" value="     if (/*/cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)) then     upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PartyTaxScheme[cac:TaxScheme/cbc:ID = 'VAT']/substring(cbc:CompanyID, 1, 2)))     else     if (/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode) then     upper-case(normalize-space(/*/cac:AccountingSupplierParty/cac:Party/cac:PostalAddress/cac:Country/cbc:IdentificationCode))     else     'XX'" />
  
  
```

