# Competitor API specifications

Downloaded 30 September 2026. Original vendor payloads are preserved for internal evaluation.

| File | Source | Declared specification | Paths | SHA-256 |
|---|---|---|---|---|
| [storecove-openapi.json](storecove-openapi.json) | https://api.storecove.com/api/v2/openapi.json | Swagger / OpenAPI 2.0 | 31 | 98cc538ea188e709f41f0a9f897168d9c9b2140f04b35fe50147d60d0904b374 |
| [b2brouter-openapi-2026-06-26.yaml](b2brouter-openapi-2026-06-26.yaml) | https://app.b2brouter.net/api/v20260626/bundled/openapi.yaml | OpenAPI 3.0.1; API release 2026-06-26 | 60 | a9d7effa0537cb5fd360df860dedf345ce8d1c5225b252974fd97dd4ebf89b63 |

Documentation: [Storecove](https://www.storecove.com/docs/), [B2Brouter download links](https://docs.b2brouter.net/en/developers/start-using-our-api/introduction/).

The versioned B2Brouter download succeeds; the previous uncertainty concerned an older unversioned URL.

## Confirmed checks

Both files parse as structured JSON/YAML and contain a specification version and paths. This is a preliminary inspection, not a complete OpenAPI validation or a test of live API conformance.

## Evaluation steps

1. Validate each file using a validator supporting its declared OpenAPI version.
2. Resolve all references and check schemas, parameters, request bodies, responses and security definitions.
3. Check coverage, examples, error models and whether client generation succeeds.
4. Compare documented requests and responses with authorized sandbox behaviour.
5. Record specification defects separately from implementation mismatches.

Specification correctness, documentation completeness and live API conformance are distinct measures. Path count alone is not a quality score.

No authenticated API calls were made. No specification was downloaded for Sovos or ecosio: a suitable public specification was not established in the research so far. Free web validators and Java validation libraries in MARKET-COMPETITORS.md are not assumed to expose OpenAPI APIs.

Vendor materials retain their original ownership and terms; downloading does not establish an open-source licence.
