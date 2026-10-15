# Distinct uncovered schema targets

Source report: `createsamples/samples-structural/coverage.json` (schema version 5; 24,692 fixtures).

There are **50 distinct target IDs**. The category arrays contain 80 entries because targets can belong to multiple coverage categories. Each target ID encodes `namespace | XSD file | kind | schema path`.

## Counts by target kind

| Kind | Distinct targets |
|---|---:|
| `element` | 15 |
| `attribute` | 10 |
| `complexType` | 10 |
| `inheritance` | 8 |
| `any` | 3 |
| `choice` | 2 |
| `choiceAlternative` | 2 |

## Counts by schema file

| XSD file | Distinct targets |
|---|---:|
| `UBL-UnqualifiedDataTypes-2.1.xsd` | 16 |
| `UBL-xmldsig-core-schema-2.1.xsd` | 8 |
| `UBL-CommonAggregateComponents-2.1.xsd` | 7 |
| `UBL-XAdESv132-2.1.xsd` | 7 |
| `CCTS_CCT_SchemaModule-2.1.xsd` | 6 |
| `UBL-XAdESv141-2.1.xsd` | 6 |

## Counts by schema file and target kind

| XSD file | Kind | Distinct targets |
|---|---|---:|
| `CCTS_CCT_SchemaModule-2.1.xsd` | `attribute` | 2 |
| `CCTS_CCT_SchemaModule-2.1.xsd` | `complexType` | 2 |
| `CCTS_CCT_SchemaModule-2.1.xsd` | `inheritance` | 2 |
| `UBL-CommonAggregateComponents-2.1.xsd` | `element` | 7 |
| `UBL-UnqualifiedDataTypes-2.1.xsd` | `attribute` | 4 |
| `UBL-UnqualifiedDataTypes-2.1.xsd` | `complexType` | 6 |
| `UBL-UnqualifiedDataTypes-2.1.xsd` | `inheritance` | 6 |
| `UBL-XAdESv132-2.1.xsd` | `attribute` | 2 |
| `UBL-XAdESv132-2.1.xsd` | `complexType` | 1 |
| `UBL-XAdESv132-2.1.xsd` | `element` | 4 |
| `UBL-XAdESv141-2.1.xsd` | `attribute` | 2 |
| `UBL-XAdESv141-2.1.xsd` | `complexType` | 1 |
| `UBL-XAdESv141-2.1.xsd` | `element` | 3 |
| `UBL-xmldsig-core-schema-2.1.xsd` | `any` | 3 |
| `UBL-xmldsig-core-schema-2.1.xsd` | `choice` | 2 |
| `UBL-xmldsig-core-schema-2.1.xsd` | `choiceAlternative` | 2 |
| `UBL-xmldsig-core-schema-2.1.xsd` | `element` | 1 |

## Distinct target IDs by schema file and kind

### `CCTS_CCT_SchemaModule-2.1.xsd` — `attribute`

- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|DateTimeType/@format`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|attribute|IndicatorType/@format`

### `CCTS_CCT_SchemaModule-2.1.xsd` — `complexType`

- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|DateTimeType`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|complexType|IndicatorType`

### `CCTS_CCT_SchemaModule-2.1.xsd` — `inheritance`

- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|DateTimeType->#string`
- `urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2|CCTS_CCT_SchemaModule-2.1.xsd|inheritance|IndicatorType->#string`

### `UBL-CommonAggregateComponents-2.1.xsd` — `element`

- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Branch/Branch`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Communication/Communication`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Duty/Duty`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/MonetaryTotal/MonetaryTotal`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Payment/Payment`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/Regulation/Regulation`
- `urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2|UBL-CommonAggregateComponents-2.1.xsd|element|global/ServiceFrequency/ServiceFrequency`

### `UBL-UnqualifiedDataTypes-2.1.xsd` — `attribute`

- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|GraphicType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|PictureType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|SoundType/@mimeCode`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|attribute|VideoType/@mimeCode`

### `UBL-UnqualifiedDataTypes-2.1.xsd` — `complexType`

- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|DateTimeType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|GraphicType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|PictureType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|SoundType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|ValueType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|complexType|VideoType`

### `UBL-UnqualifiedDataTypes-2.1.xsd` — `inheritance`

- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|DateTimeType->#dateTime`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|GraphicType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|PictureType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|SoundType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|ValueType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#NumericType`
- `urn:oasis:names:specification:ubl:schema:xsd:UnqualifiedDataTypes-2|UBL-UnqualifiedDataTypes-2.1.xsd|inheritance|VideoType->urn:un:unece:uncefact:data:specification:CoreComponentTypeSchemaModule:2#BinaryObjectType`

### `UBL-XAdESv132-2.1.xsd` — `attribute`

- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|OtherTimeStampType/@Id`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|attribute|XAdESTimeStampType/@Id`

### `UBL-XAdESv132-2.1.xsd` — `complexType`

- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|complexType|QualifierType`

### `UBL-XAdESv132-2.1.xsd` — `element`

- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|OtherTimeStampType/choice[0]/choice[1]/XMLTimeStamp`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|ResponderIDType/choice[1]/ByKey`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SPURI/SPURI`
- `http://uri.etsi.org/01903/v1.3.2#|UBL-XAdESv132-2.1.xsd|element|global/SigningTime/SigningTime`

### `UBL-XAdESv141-2.1.xsd` — `attribute`

- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@Id`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|attribute|ValidationDataType/@UR`

### `UBL-XAdESv141-2.1.xsd` — `complexType`

- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|complexType|ValidationDataType`

### `UBL-XAdESv141-2.1.xsd` — `element`

- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[0]/CertificateValues`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|ValidationDataType/sequence[1]/RevocationValues`
- `http://uri.etsi.org/01903/v1.4.1#|UBL-XAdESv141-2.1.xsd|element|global/TimeStampValidationData/TimeStampValidationData`

### `UBL-xmldsig-core-schema-2.1.xsd` — `any`

- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|CanonicalizationMethodType/sequence`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|PGPDataType/sequence[1]/sequence`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|any|SignatureMethodType/sequence`

### `UBL-xmldsig-core-schema-2.1.xsd` — `choice`

- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|PGPDataType`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choice|SignaturePropertyType`

### `UBL-xmldsig-core-schema-2.1.xsd` — `choiceAlternative`

- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[0]`
- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|choiceAlternative|PGPDataType/sequence[1]`

### `UBL-xmldsig-core-schema-2.1.xsd` — `element`

- `http://www.w3.org/2000/09/xmldsig#|UBL-xmldsig-core-schema-2.1.xsd|element|PGPDataType/sequence[1]/sequence[0]/PGPKeyPacket`
