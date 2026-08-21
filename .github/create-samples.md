# UBL 2.1 Sample Generator Subproject

**Parent project:** UBL-21-validator  
**Subproject:** UBL 2.1 compliant XML sample generation  
**Status:** High-level implementation roadmap

## 1. Purpose

Build a deterministic generator that creates XML documents compliant with the authoritative OASIS UBL 2.1 XSD schemas.

Primary acceptance target:

> Generate at least one XML sample for every UBL 2.1 main document type and prove that every generated sample validates successfully against its corresponding UBL 2.1 XSD.

The first implementation target is therefore **65 generated XML documents**, one for each UBL 2.1 main document schema.

The generator must be reusable and schema-driven. It must not consist of 65 hand-written XML templates. The resulting sample corpus will become the positive integration-test suite for the UBL-21-validator server.

## 2. Scope

The generator must:

- load the complete authoritative UBL 2.1 XSD set;
- resolve `xs:import` and `xs:include` relationships;
- discover the document root element defined by each main schema;
- traverse referenced UBL common components;
- interpret required/optional elements, `xs:sequence`, `xs:choice`, cardinalities and attributes;
- generate legal values for XSD primitive and derived datatypes;
- emit correct namespaces and qualified names;
- generate a deterministic minimal valid document where possible;
- optionally generate richer representative documents;
- validate every generated file with an independent XSD validator;
- produce a machine-readable generation/validation report;
- fail the build if any UBL 2.1 main document type has no valid generated sample.

The initial version guarantees **XSD validity**, not complete business-process validity. Schematron rules, PEPPOL rules, EN 16931, national profiles, business code-list restrictions and digital-signature generation are follow-on concerns.

## 3. Authoritative XSD Inventory

The generator must use the official OASIS UBL 2.1 schema distribution.

The XSD tree contains:

- **65 main document XSDs** under `XSD/maindoc/`;
- **14 shared/common XSDs** under `XSD/common/`;
- **79 XSD files total**.

The 65 main schemas are sample-generation targets. The 14 common schemas are dependencies and are not normally standalone sample targets.

## 4. Main Document XSDs: 65 Generation Targets

1. `UBL-ApplicationResponse-2.1.xsd`
2. `UBL-AttachedDocument-2.1.xsd`
3. `UBL-AwardedNotification-2.1.xsd`
4. `UBL-BillOfLading-2.1.xsd`
5. `UBL-CallForTenders-2.1.xsd`
6. `UBL-Catalogue-2.1.xsd`
7. `UBL-CatalogueDeletion-2.1.xsd`
8. `UBL-CatalogueItemSpecificationUpdate-2.1.xsd`
9. `UBL-CataloguePricingUpdate-2.1.xsd`
10. `UBL-CatalogueRequest-2.1.xsd`
11. `UBL-CertificateOfOrigin-2.1.xsd`
12. `UBL-ContractAwardNotice-2.1.xsd`
13. `UBL-ContractNotice-2.1.xsd`
14. `UBL-CreditNote-2.1.xsd`
15. `UBL-DebitNote-2.1.xsd`
16. `UBL-DespatchAdvice-2.1.xsd`
17. `UBL-DocumentStatus-2.1.xsd`
18. `UBL-DocumentStatusRequest-2.1.xsd`
19. `UBL-ExceptionCriteria-2.1.xsd`
20. `UBL-ExceptionNotification-2.1.xsd`
21. `UBL-Forecast-2.1.xsd`
22. `UBL-ForecastRevision-2.1.xsd`
23. `UBL-ForwardingInstructions-2.1.xsd`
24. `UBL-FreightInvoice-2.1.xsd`
25. `UBL-FulfilmentCancellation-2.1.xsd`
26. `UBL-GoodsItemItinerary-2.1.xsd`
27. `UBL-GuaranteeCertificate-2.1.xsd`
28. `UBL-InstructionForReturns-2.1.xsd`
29. `UBL-InventoryReport-2.1.xsd`
30. `UBL-Invoice-2.1.xsd`
31. `UBL-ItemInformationRequest-2.1.xsd`
32. `UBL-Order-2.1.xsd`
33. `UBL-OrderCancellation-2.1.xsd`
34. `UBL-OrderChange-2.1.xsd`
35. `UBL-OrderResponse-2.1.xsd`
36. `UBL-OrderResponseSimple-2.1.xsd`
37. `UBL-PackingList-2.1.xsd`
38. `UBL-PriorInformationNotice-2.1.xsd`
39. `UBL-ProductActivity-2.1.xsd`
40. `UBL-Quotation-2.1.xsd`
41. `UBL-ReceiptAdvice-2.1.xsd`
42. `UBL-Reminder-2.1.xsd`
43. `UBL-RemittanceAdvice-2.1.xsd`
44. `UBL-RequestForQuotation-2.1.xsd`
45. `UBL-RetailEvent-2.1.xsd`
46. `UBL-SelfBilledCreditNote-2.1.xsd`
47. `UBL-SelfBilledInvoice-2.1.xsd`
48. `UBL-Statement-2.1.xsd`
49. `UBL-StockAvailabilityReport-2.1.xsd`
50. `UBL-Tender-2.1.xsd`
51. `UBL-TenderReceipt-2.1.xsd`
52. `UBL-TendererQualification-2.1.xsd`
53. `UBL-TendererQualificationResponse-2.1.xsd`
54. `UBL-TradeItemLocationProfile-2.1.xsd`
55. `UBL-TransportExecutionPlan-2.1.xsd`
56. `UBL-TransportExecutionPlanRequest-2.1.xsd`
57. `UBL-TransportProgressStatus-2.1.xsd`
58. `UBL-TransportProgressStatusRequest-2.1.xsd`
59. `UBL-TransportServiceDescription-2.1.xsd`
60. `UBL-TransportServiceDescriptionRequest-2.1.xsd`
61. `UBL-TransportationStatus-2.1.xsd`
62. `UBL-TransportationStatusRequest-2.1.xsd`
63. `UBL-UnawardedNotification-2.1.xsd`
64. `UBL-UtilityStatement-2.1.xsd`
65. `UBL-Waybill-2.1.xsd`

Recommended generated-file naming:

```text
samples/generated/<DocumentType>/<DocumentType>-minimal.xml
```

## 5. Common XSDs: 14 Required Dependencies

1. `CCTS_CCT_SchemaModule-2.1.xsd`
2. `UBL-CommonAggregateComponents-2.1.xsd`
3. `UBL-CommonBasicComponents-2.1.xsd`
4. `UBL-CommonExtensionComponents-2.1.xsd`
5. `UBL-CommonSignatureComponents-2.1.xsd`
6. `UBL-CoreComponentParameters-2.1.xsd`
7. `UBL-ExtensionContentDataType-2.1.xsd`
8. `UBL-QualifiedDataTypes-2.1.xsd`
9. `UBL-SignatureAggregateComponents-2.1.xsd`
10. `UBL-SignatureBasicComponents-2.1.xsd`
11. `UBL-UnqualifiedDataTypes-2.1.xsd`
12. `UBL-XAdESv132-2.1.xsd`
13. `UBL-XAdESv141-2.1.xsd`
14. `UBL-xmldsig-core-schema-2.1.xsd`

## 6. Implementation Language and CLI

The generator should be written in **Go**, consistent with the parent project.

Suggested executable:

```text
cmd/ubl21-samplegen/
```

Suggested commands:

```bash
ubl21-samplegen inventory --xsd XSD
ubl21-samplegen generate --xsd XSD --output samples/generated
ubl21-samplegen validate --xsd XSD --samples samples/generated
```

The CLI must return non-zero status on schema-loading, generation or validation failure.

## 7. External Tools

### Required

- **Go toolchain**: build and test the generator.
- **Official OASIS UBL 2.1 XSDs**: authoritative schema source.
- **`xmllint` / libxml2**: independent validation oracle for every generated sample.

Example:

```bash
xmllint --noout \
  --schema XSD/maindoc/UBL-Invoice-2.1.xsd \
  samples/generated/Invoice/Invoice-minimal.xml
```

### Optional

- `xmlstarlet` for XPath and namespace diagnostics;
- `jq` for JSON report inspection.

The generator must not consider a sample valid solely because its own internal model accepted it. Validation must be independent.

## 8. Proposed Architecture

```text
OASIS UBL 2.1 XSD tree
          │
          ▼
   Schema Repository
          │
          ▼
    XSD Model Graph
          │
    ┌─────┴─────┐
    ▼           ▼
Generation   Datatype
 Planner      Rules
    └─────┬─────┘
          ▼
   XML Sample Builder
          │
          ▼
  Namespace Renderer
          │
          ▼
 Generated XML file
          │
          ▼
 xmllint / libxml2
    ┌─────┴─────┐
    ▼           ▼
   PASS        FAIL
    │           │
    ▼           ▼
  corpus   diagnostics
```

The schema parser, generation planner, value generator, XML renderer, validator and report writer should remain separate components.

## 9. Internal XSD Model

At minimum, the internal model must support:

- target namespaces and namespace aliases;
- global elements and attributes;
- named and anonymous complex/simple types;
- element and attribute references;
- `xs:sequence`, `xs:choice`, and `xs:all` if encountered;
- `minOccurs` and `maxOccurs`;
- simple type restrictions;
- enumerations;
- patterns and length constraints;
- numeric bounds;
- simple/complex content extension and restriction;
- imports/includes;
- cycle-safe reference resolution.

Unsupported XSD constructs must be reported explicitly rather than silently ignored.

## 10. Generation Policy

### Minimal mode

Rules:

- emit every element with `minOccurs > 0`;
- omit optional elements by default;
- select one deterministic valid branch for `xs:choice`;
- emit the minimum required repetition count;
- emit all required attributes;
- generate the simplest legal value for each datatype;
- emit stable namespaces/prefixes;
- keep output deterministic for reproducible tests.

### Representative mode

After 65/65 minimal generation succeeds, add a richer profile:

```bash
ubl21-samplegen generate --profile representative
```

Representative samples should exercise optional aggregates, repeated elements, XML attributes, monetary values, quantities, identifiers and nested structures. These samples will be especially useful for XML/JSON round-trip, OpenAPI and persistence testing.

### Stress mode

A later optional mode may generate larger legal documents for performance testing.

## 11. Value Generation

The value generator must create deterministic legal values for XML Schema primitive and UBL-derived datatypes.

Examples:

| Type | Default policy |
|---|---|
| `xs:string` | `Sample` |
| `xs:boolean` | `true` |
| integer | `1` |
| decimal | `1.00` |
| date | `2026-01-01` |
| time | `12:00:00` |
| dateTime | `2026-01-01T12:00:00Z` |
| URI | `https://example.invalid/resource` |
| language | `en` |
| enumeration | first declared legal value |

Restrictions always override generic defaults.

## 12. Namespace Handling

Namespace handling is a first-class requirement.

The generator must correctly emit the document root namespace and all referenced UBL common namespaces, including `cac`, `cbc`, extension and signature namespaces where required.

Prefixes should be stable and deterministic so generated sample diffs remain readable.

## 13. Validation Strategy

For every generated sample:

1. identify its corresponding main schema;
2. execute `xmllint` against that schema;
3. capture stdout, stderr and exit status;
4. mark the sample pass/fail;
5. retain diagnostics for failures;
6. produce an aggregate machine-readable report.

Acceptance gate:

```text
generated == 65
valid     == 65
invalid   == 0
missing   == 0
```

Suggested report:

```json
{
  "ubl_version": "2.1",
  "main_schemas": 65,
  "generated": 65,
  "valid": 65,
  "invalid": 0,
  "missing": 0
}
```

# 14. Implementation Roadmap

## Phase SG-0: Schema inventory and baseline

Deliver:

- inventory of 65 main schemas and 14 common schemas;
- schema existence checker;
- import/include dependency scanner;
- root-element discovery;
- `xmllint` wrapper;
- inventory report.

**Exit:** exactly 65 generation targets are discovered and the full local schema tree resolves.

## Phase SG-1: XSD parser and dependency graph

Deliver:

- XSD loader;
- namespace registry;
- import/include resolver;
- global element/type registry;
- complex/simple type parser;
- occurrence constraints;
- sequence/choice model;
- attribute model;
- restriction model;
- cycle-safe dependency graph.

**Exit:** every main-document root and required type graph can be traversed without unresolved references.

## Phase SG-2: Datatype/value generator

Deliver:

- primitive value providers;
- restriction-aware generation;
- enumeration support;
- numeric bounds;
- string length/pattern strategy;
- date/time generation;
- required attribute generation;
- UBL datatype adapters.

**Exit:** every simple type encountered in required UBL generation paths can produce a legal value or a clear unsupported-feature error.

## Phase SG-3: Structural document generator

Deliver:

- recursive element builder;
- sequence handling;
- choice handling;
- cardinality handling;
- complex-content handling;
- attributes;
- deterministic namespaces;
- recursion/depth/node safeguards.

Suggested early milestones:

1. `ApplicationResponse`;
2. `DocumentStatusRequest`;
3. `Invoice`;
4. `Order`;
5. `BillOfLading`.

**Exit:** at least five structurally different UBL document types are generated and accepted by `xmllint`.

## Phase SG-4: Validation-driven completion

Iterate across all 65 schemas:

```text
generate → validate → PASS
                  ↘ FAIL → classify construct → improve generic generator
```

Fix failures by improving generic schema-driven logic wherever possible. Avoid document-specific templates.

**Exit:** all 65 main document schemas have at least one independently validated sample.

## Phase SG-5: Corpus packaging and CI gate

Recommended layout:

```text
samples/
├── generated/
│   ├── ApplicationResponse/
│   ├── AttachedDocument/
│   ├── ...
│   └── Waybill/
├── reports/
│   ├── inventory.json
│   └── validation.json
└── README.md
```

CI must provision XSDs, build the generator, regenerate the corpus, validate all outputs, enforce the 65-document count and fail on any invalid/missing sample.

**Exit:** a clean checkout can reproduce the complete valid sample corpus automatically.

## Phase SG-6: Representative samples

Add richer samples exercising optional components, repeated elements, attributes, nested aggregates, monetary values, quantities and identifiers.

These become primary fixtures for XML↔JSON conversion, OpenAPI and database tests.

## Phase SG-7: Negative corpus

Generate deliberate invalid variants under:

```text
samples/invalid/
```

Examples:

- missing required element;
- illegal datatype;
- illegal enumeration value;
- incorrect namespace;
- wrong element order;
- unexpected element;
- invalid required attribute;
- malformed XML.

Each invalid fixture must document why it is expected to fail.

## 15. Proposed Go Package Structure

```text
cmd/
└── ubl21-samplegen/
    └── main.go

internal/
└── samplegen/
    ├── schema/
    │   ├── loader.go
    │   ├── model.go
    │   ├── resolver.go
    │   └── inventory.go
    ├── generate/
    │   ├── planner.go
    │   ├── document.go
    │   ├── value.go
    │   └── namespace.go
    ├── validate/
    │   └── xmllint.go
    └── report/
        └── report.go
```

## 16. Non-Functional Requirements

The generator must be:

- deterministic by default;
- reproducible;
- safe against recursive schema loops;
- bounded in document size and recursion depth;
- independent of network access after XSD provisioning;
- diagnostic-friendly;
- scriptable in CI;
- capable of machine-readable reports;
- free from Node.js dependencies;
- suitable for Linux command-line execution.

The generator must never modify the authoritative XSD files.

## 17. Definition of Done

The initial subproject is complete when:

1. all 65 main XSDs are discovered automatically;
2. all 14 common XSDs are available and resolvable;
3. at least one XML sample is generated for every main schema;
4. all 65 samples validate successfully with `xmllint/libxml2`;
5. no sample relies on a hand-written full-document template;
6. generation is deterministic;
7. generation and validation can be run from a single command or CI target;
8. the aggregate report proves `65/65` coverage;
9. the resulting corpus is directly usable by UBL-21-validator integration tests.

## 18. Strategic Follow-On

Once the corpus reaches 65/65, it becomes shared regression infrastructure for the broader project:

```text
XSD validation
      ↓
XML → canonical model
      ↓
canonical model → JSON
      ↓
JSON → canonical model
      ↓
canonical model → XML
      ↓
XSD re-validation
      ↓
database persistence / retrieval
```

This lets a single authoritative sample corpus support Phase 1 validation, Phase 2 XML↔JSON/OpenAPI work, and Phase 3 persistence.
