Yes. For **XSD → Go structs/code generation**, there are several options.

For your UBL 2.1 project, I would test **`xuri/xgen` first**. It is actively maintained, written in Go, and can generate Go declarations from XSD files. Its current documentation says it supports Go generation directly from XML Schema. ([GitHub][1])

Install:

```bash
go install github.com/xuri/xgen/cmd/xgen@latest
```

Then something along these lines:

```bash
xgen \
  -i XSD/maindoc/UBL-Invoice-2.1.xsd \
  -l Go \
  -o generated/
```

The exact CLI flags have changed across releases, so I would check:

```bash
xgen --help
```

before wiring it into your build.

### Other candidates

**`xsdgen` from `droyo/go-xml`** is specifically designed to generate Go type declarations plus XML marshal/unmarshal methods from XSD. It was also designed to work nicely with `go generate`. ([GitHub][2])

Typical model:

```bash
xsdgen \
  -o invoice.go \
  -pkg ubl \
  XSD/maindoc/UBL-Invoice-2.1.xsd
```

There is also **GoComply `xsd2go`**, whose purpose is explicitly to generate Go XML parsing models from XSD. ([GitHub][3])

And `goxsd` exists, but its own documentation says it does **not implement the full XSD specification**, so I would not choose it for something as complex as UBL. ([GitHub][4])

## The catch with UBL

UBL is not a toy schema.

Your file:

```text
UBL-Invoice-2.1.xsd
```

imports:

```text
UBL-CommonAggregateComponents-2.1.xsd
UBL-CommonBasicComponents-2.1.xsd
UBL-CommonExtensionComponents-2.1.xsd
...
```

and those schemas contain a very large number of shared types.

A generator may therefore produce something enormous:

```text
generated/
├── invoice.go
├── commonaggregatecomponents.go
├── commonbasiccomponents.go
├── commondatatypes.go
├── extensions.go
└── ...
```

Potentially tens of thousands of lines.

That is not necessarily bad, but I would avoid generating the same shared UBL types once per document.

### Better structure

For `UBL-21-validator`, I would aim for something like:

```text
internal/
└── ubl/
    ├── common/
    │   ├── aggregate.go
    │   ├── basic.go
    │   └── datatypes.go
    │
    ├── invoice/
    │   └── invoice.go
    │
    ├── creditnote/
    │   └── creditnote.go
    │
    ├── debitnote/
    │   └── debitnote.go
    │
    └── ...
```

Or, if the generator behaves better with everything together:

```text
internal/
└── ubl21/
    ├── generated.go
    └── helpers.go
```

### But there is another important point

For a **validator**, you may not actually need generated Go structs for every UBL element.

You primarily need:

```text
XML
 │
 ├── detect document type
 ├── detect namespace
 ├── detect profile
 └── pass document to XSD/Schematron validation
```

You can detect those with `encoding/xml` without unmarshalling the complete document.

For example:

```go
type DocumentHeader struct {
    XMLName xml.Name

    UBLVersionID string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 UBLVersionID"`

    CustomizationID string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 CustomizationID"`

    ProfileID string `xml:"urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2 ProfileID"`
}
```

Then:

```go
var header DocumentHeader

if err := xml.Unmarshal(data, &header); err != nil {
    return err
}
```

Now:

```go
header.XMLName.Local
```

might give:

```text
Invoice
CreditNote
DebitNote
```

That may be all the Go layer initially needs.

## Where generated structs become valuable

They become much more useful when we implement the later feature:

```text
JSON
 ↓
Go structures
 ↓
UBL XML
```

For example:

```go
invoice := ubl.Invoice{
    ID: "INV-2026-001",
    IssueDate: "2026-08-18",
    ...
}
```

then:

```go
xml.Marshal(invoice)
```

At that point, having a complete generated UBL object model becomes extremely valuable.

So I would separate the requirements:

```text
VALIDATOR
    → generated structs optional

GENERATOR
    → generated structs very useful

CONVERTER
    → generated structs extremely useful
```

### What I would do next

Before committing to a generator, take our downloaded UBL tree and run **both `xgen` and `xsdgen` against `UBL-Invoice-2.1.xsd`**.

Then compare:

```text
Does it resolve ../common imports?
Does generated code compile?
Are namespaces correct?
Does xml.Unmarshal() work on a real UBL invoice?
Does xml.Marshal() produce valid UBL?
How large is the generated code?
```

That five-test bake-off will tell us much more than README marketing.

For this particular project, **`xgen` is the one I would test first**. ([GitHub][1])

[1]: https://github.com/xuri/xgen?utm_source=chatgpt.com "xuri/xgen: XSD (XML Schema Definition) parser and Go/C ..."
[2]: https://github.com/droyo/go-xml?utm_source=chatgpt.com "droyo/go-xml: utility and code-generation libraries for XML"
[3]: https://github.com/gocomply/xsd2go?utm_source=chatgpt.com "XSD2Go - Automatically generate golang xml parser based ..."
[4]: https://github.com/ivarg/goxsd?utm_source=chatgpt.com "ivarg/goxsd: Generate XML decoding structs from an XSD ..."
