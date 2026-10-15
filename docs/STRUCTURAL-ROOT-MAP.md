# Uncovered targets grouped by reachable document roots

Source: `createsamples/samples-structural/coverage.json`; 50 distinct targets from 24,692 fixtures; 65 main-document roots.

Reachability follows XSD element references, declared types, inheritance, nested particles, and inline complex types. A target is assigned to every root that can reach its owning element or type. This is structural schema reachability; a valid value may still require generator support for choices, wildcards, recursive edges, or signature vocabularies.

Targets reachable from at least one document root: **0**. Targets without a mapped document root: **50**.

## Root groups

| Document root | Reachable uncovered targets |
|---|---:|

## Shared target groups

Targets reachable from many roots are good candidates for a small number of richer shared fixtures. Counts below are numbers of roots that can reach the target.

| Reachable roots | Targets |
|---:|---:|
| 0 | 50 |

## Greedy root portfolio for reachable targets

This is a planning heuristic: repeatedly select the root that reaches the most currently uncovered targets. It identifies useful root families, but it does not prove that one generated fixture per selected root will cover those targets or be XSD-valid.

| Selected root | Newly reachable targets |
|---|---:|

## Target clusters with identical root reachability

Targets in each cluster share the same set of reachable roots, so they can be considered together when selecting a minimal fixture portfolio.

| Reachable root set | Targets |
|---|---:|
| no main-document root | 50 |

## Unreachable targets

These targets are not reached from any main document root by explicit element/type references. Signature vocabulary is expected to need a signature fixture strategy because UBL admits it through extension/wildcard boundaries. Other declarations need review as standalone component roots or unused declarations before deciding they require a dedicated fixture.

- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|OtherTimeStampType/@Id`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|XAdESTimeStampType/@Id`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|complexType|QualifierType`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|OtherTimeStampType/choice[0]/choice[1]/XMLTimeStamp`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|ResponderIDType/choice[1]/ByKey`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SPURI/SPURI`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SigningTime/SigningTime`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@Id`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@UR`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|complexType|ValidationDataType`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[0]/CertificateValues`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[1]/RevocationValues`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|global/TimeStampValidationData/TimeStampValidationData`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|CanonicalizationMethodType/sequence`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|PGPDataType/sequence[1]/sequence`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|SignatureMethodType/sequence`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[0]`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[1]`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|PGPDataType`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|SignaturePropertyType`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|element|PGPDataType/sequence[1]/sequence[0]/PGPKeyPacket`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Branch/Branch`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Communication/Communication`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Duty/Duty`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/MonetaryTotal/MonetaryTotal`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Payment/Payment`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Regulation/Regulation`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/ServiceFrequency/ServiceFrequency`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|GraphicType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|PictureType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|SoundType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|VideoType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|DateTimeType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|GraphicType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|PictureType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|SoundType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|ValueType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|VideoType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|DateTimeType->#dateTime`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|GraphicType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|PictureType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|SoundType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|ValueType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#NumericType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|VideoType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|DateTimeType/@format`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|IndicatorType/@format`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|DateTimeType`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|IndicatorType`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|DateTimeType->#string`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|IndicatorType->#string`

## Full target-to-root map

Each entry lists every reachable main-document root. The machine-readable version is available in the adjacent JSON artifact.

- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|OtherTimeStampType/@Id` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|XAdESTimeStampType/@Id` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|complexType|QualifierType` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|OtherTimeStampType/choice[0]/choice[1]/XMLTimeStamp` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|ResponderIDType/choice[1]/ByKey` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SPURI/SPURI` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SigningTime/SigningTime` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@Id` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@UR` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|complexType|ValidationDataType` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[0]/CertificateValues` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[1]/RevocationValues` — **no root** (signature-vocabulary)
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|global/TimeStampValidationData/TimeStampValidationData` — **no root** (signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|CanonicalizationMethodType/sequence` — **no root** (wildcard-content, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|PGPDataType/sequence[1]/sequence` — **no root** (wildcard-content, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|SignatureMethodType/sequence` — **no root** (wildcard-content, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[0]` — **no root** (choice-branch, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[1]` — **no root** (choice-branch, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|PGPDataType` — **no root** (choice-branch, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|SignaturePropertyType` — **no root** (choice-branch, signature-vocabulary)
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|element|PGPDataType/sequence[1]/sequence[0]/PGPKeyPacket` — **no root** (signature-vocabulary)
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Branch/Branch` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Communication/Communication` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Duty/Duty` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/MonetaryTotal/MonetaryTotal` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Payment/Payment` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Regulation/Regulation` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/ServiceFrequency/ServiceFrequency` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|GraphicType/@mimeCode` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|PictureType/@mimeCode` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|SoundType/@mimeCode` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|VideoType/@mimeCode` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|DateTimeType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|GraphicType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|PictureType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|SoundType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|ValueType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|VideoType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|DateTimeType->#dateTime` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|GraphicType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|PictureType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|SoundType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|ValueType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#NumericType` — **no root**
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|VideoType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|DateTimeType/@format` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|IndicatorType/@format` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|DateTimeType` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|IndicatorType` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|DateTimeType->#string` — **no root**
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|IndicatorType->#string` — **no root**
