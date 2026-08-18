For ordinary XML work, Go is **quite good**. For standards-heavy XML work such as UBL validation, Go’s **standard library alone is not enough**.

The built-in `encoding/xml` package handles parsing, streaming tokens, marshaling/unmarshaling to structs, attributes, comments, CDATA, processing instructions, and namespace URIs. It also supports custom `MarshalXML` / `UnmarshalXML` hooks and streaming through `Decoder.Token()`. ([pkg.go.dev][1])

A rough scorecard for your project:

| XML capability                              | Go stdlib support |
| ------------------------------------------- | ----------------- |
| XML parsing                                 | Excellent         |
| Streaming parsing                           | Excellent         |
| Marshal/unmarshal structs                   | Excellent         |
| Attributes / CDATA / comments               | Good              |
| Namespace URI handling                      | Good              |
| Exact namespace prefix preservation/control | Weak              |
| XML syntax checking                         | Good              |
| XSD validation                              | **None**          |
| XPath                                       | **None**          |
| XSLT                                        | **None**          |
| Schematron                                  | **None**          |
| XML canonicalization / C14N                 | **None**          |
| XML signatures                              | **None**          |

The namespace point matters for UBL. `encoding/xml` resolves names to their namespace URI and local name, which is generally what you want for parsing. But prefix handling and round-tripping have long-standing limitations in the Go XML package. The Go issue tracker still documents limitations around controlling or preserving prefixes. ([GitHub][2])

For example, this is perfectly comfortable in Go:

```go
type Invoice struct {
    XMLName xml.Name `xml:"urn:oasis:names:specification:ubl:schema:xsd:Invoice-2 Invoice"`

    ID        string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ID"`
    IssueDate string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 IssueDate"`
}
```

and:

```go
var invoice Invoice

err := xml.Unmarshal(data, &invoice)
if err != nil {
    return err
}
```

Go matches on the actual namespace URI, not merely whether the document uses `cbc:` or some other prefix. The standard decoder exposes namespace names via `xml.Name.Space`. ([pkg.go.dev][1])

That is excellent for your validator.

The problem starts here:

```text
invoice.xml
      │
      ▼
 XML parsing       ← Go is excellent
      │
      ▼
 XSD validation    ← Go stdlib has nothing
      │
      ▼
 Schematron        ← Go stdlib has nothing
      │
      ▼
 XSLT              ← Go stdlib has nothing
```

For `UBL-21-validator`, I would therefore split the responsibilities.

```text
Go
│
├── HTTP/API layer
├── document detection
├── XML streaming/parser
├── namespace detection
├── schema selection
├── result normalization
├── caching
├── concurrency
└── orchestration

XML engine
│
├── XSD validation
├── XPath
├── XSLT
└── Schematron execution
```

For XSD, one established route is **libxml2** behind Go. For example, `krolaw/xsd` is explicitly a Go wrapper around libxml2 for XML Schema validation. ([GitHub][3])

There is also `lestrrat-go/libxml2`, which exposes libxml2 functionality including XSD validation. However, that particular project was archived in March 2026, and its author points users toward a newer replacement, so I would not base a fresh commercial product on it without evaluating the successor first. ([GitHub][4])

For our project, I would probably start even more conservatively:

```text
Go service
   │
   ├── encoding/xml
   │
   └── exec / library boundary
          │
          ▼
       libxml2
```

The first implementation could literally invoke:

```bash
xmllint \
  --noout \
  --schema XSD/maindoc/UBL-Invoice-2.1.xsd \
  invoice.xml
```

from Go.

Something like:

```go
cmd := exec.Command(
    "xmllint",
    "--noout",
    "--schema",
    "XSD/maindoc/UBL-Invoice-2.1.xsd",
    filename,
)

output, err := cmd.CombinedOutput()
```

That may look unsophisticated, but for the MVP it has several advantages:

```text
known XML engine
+
known XSD behavior
+
no cgo complexity
+
easy debugging
+
easy Docker deployment
```

Once the validator works correctly, we can decide whether process startup becomes expensive enough to justify linking directly against libxml2.

For **EN 16931 Schematron**, the situation becomes more important. The official validation artefacts rely on XSLT processing, and there are historical compatibility notes in the official project around XSLT versions and processor choice. ([GitHub][5])

So I would not try to turn Go into an XML standards engine.

I would make Go the **conductor**, not every instrument in the orchestra:

```text
                       Go API
                         │
             ┌───────────┼───────────┐
             │           │           │
             ▼           ▼           ▼
        encoding/xml   libxml2     XSLT engine
             │           │           │
             ▼           ▼           ▼
         metadata       XSD       Schematron
         detection   validation    EN16931
```

That architecture is actually very strong.

Go gives you exactly what you want for the SaaS layer:

* high concurrency
* low memory usage
* simple deployment
* static-ish binaries
* excellent HTTP support
* straightforward worker pools
* easy streaming uploads

And you delegate the gnarly twenty-year-old XML standards jungle to engines built specifically for it.

For **UBL-21-validator**, my recommendation would therefore be:

```text
encoding/xml          document inspection/parsing
libxml2/xmllint       UBL XSD validation
Saxon or equivalent   EN16931 / Schematron XSLT
Go                    orchestration + REST API
```

That would be a much safer architecture than trying to implement XSD, XPath and Schematron natively in Go.

[1]: https://pkg.go.dev/encoding/xml "xml package - encoding/xml - Go Packages"
[2]: https://github.com/golang/go/issues/11496?utm_source=chatgpt.com "encoding/xml: Serializing XML with namespace prefix #11496 - GitHub"
[3]: https://github.com/krolaw/xsd?utm_source=chatgpt.com "krolaw/xsd: XML Schema Validator in Go that wraps libxml2."
[4]: https://github.com/lestrrat-go/libxml2?utm_source=chatgpt.com "lestrrat-go/libxml2: Interface to libxml2, with DOM interface"
[5]: https://github.com/ConnectingEurope/eInvoicing-EN16931/issues/50?utm_source=chatgpt.com "Rules with attribute as context (UBL) · Issue #50"
