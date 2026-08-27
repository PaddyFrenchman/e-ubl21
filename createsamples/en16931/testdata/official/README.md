# Official EN 16931 UBL example fixtures

**Purpose:** byte-pinned reference fixtures used by unit tests and imported by
the unified EN 16931 positive-corpus generator. The generated corpus preserves
their official-import provenance and copies their bytes without transformation.

See [ideas/EN16931-samples-generator-plan.md](/home/a/e-ubl21/ideas/EN16931-samples-generator-plan.md)
§2 and Phase EN-0.

## Provenance

| File | Source | SHA-256 |
|---|---|---|
| `guide-example1.xml` | https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/examples/guide-example1.xml | `8cf298f82ccc0b1e0a3fd57d07dd39f4c174ee6739e147020f22c8aebb364ca5` |
| `guide-example2.xml` | https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/examples/guide-example2.xml | `7c073be56039d35d1caad07ecc407ff85910925b206a59a92fa5e3048bb891b5` |

- Repository: `ConnectingEurope/eInvoicing-EN16931`
- Tag: `validation-1.3.16`
- Fetched: 22 August 2026
- Verified well-formed with `xmllint --noout`.

## License

Both files carry the following header, preserved verbatim from the source
repository:

```
Licensed under European Union Public Licence (EUPL) version 1.2.
```

The `eInvoicing-EN16931` repository itself is licensed under EUPL 1.2 (see
its `README.md`). These two files are vendored and redistributed unmodified as
regression fixtures under those terms.
