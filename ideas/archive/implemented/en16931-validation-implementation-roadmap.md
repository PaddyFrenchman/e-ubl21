# EN 16931 UBL Validation with Schematron

**Project:** UBL-21 Validator  
**Document:** `EN16931-UBL-validation.md`  
**Purpose:** Define the EN 16931 Schematron validation layer and an implementation route from the current queued UBL 2.1 XSD validator.  
**Status:** Implementation roadmap  
**Checked:** 21 August 2026

---

# 1. Objective

The UBL-21 Validator already has the first major validation layer:

```text
client
  |
  v
web service
  |
  v
queue
  |
  v
worker retrieves XML
  |
  v
UBL 2.1 XSD validation
  |
  v
stored/retrievable validation result
```

The next step is to add **EN 16931 semantic validation using Schematron**.

The target architecture becomes:

```text
client
  |
  v
web service
  |
  v
queue
  |
  v
validation worker
  |
  +--> retrieve XML
  |
  +--> detect UBL document type
  |
  +--> UBL 2.1 XSD validation
  |
  +--> EN 16931 validation
  |       |
  |       +--> execute official EN16931 UBL XSLT
  |       |
  |       +--> receive SVRL
  |       |
  |       +--> normalize SVRL into Go validation issues
  |
  +--> persist validation result
  |
  v
existing retrieval web services
```

The EN 16931 stage should be **an additional validation stage inside the existing queued worker pipeline**, not a separate public-facing application.

---

# 2. Scope

EN 16931 is an electronic invoicing semantic standard.

The UBL implementation applies primarily to:

```text
UBL Invoice
UBL CreditNote
```

Therefore EN 16931 must **not** automatically be executed against all 65 UBL 2.1 document types.

The base UBL validator remains generic:

```text
all UBL 2.1 documents
        |
        v
UBL 2.1 XSD validation
```

The EN 16931 profile is narrower:

```text
UBL Invoice / CreditNote
        |
        v
UBL 2.1 XSD
        |
        v
EN 16931 Schematron
```

Future profiles can then be added:

```text
UBL 2.1
   |
   +-- EN 16931
   |      |
   |      +-- Peppol BIS Billing
   |      |
   |      +-- national CIUS
   |
   +-- other UBL-specific profiles
```

---

# 3. Why XSD validation is insufficient

UBL 2.1 XSD validation answers questions such as:

```text
Is this XML structurally valid?
Is this element allowed here?
Is this value compatible with the XSD datatype?
Are required XML elements present?
Are namespaces correct?
```

It does **not** answer business-semantic questions such as:

```text
Do invoice totals reconcile?
Does VAT arithmetic make sense?
Is a payment means code permitted?
Is a required EN 16931 business term present?
Are decimal restrictions satisfied?
Are VAT breakdown values consistent?
Is the invoice type code legal for this profile?
```

Example:

```xml
<cac:LegalMonetaryTotal>
    <cbc:LineExtensionAmount currencyID="EUR">100.00</cbc:LineExtensionAmount>
    <cbc:TaxExclusiveAmount currencyID="EUR">150.00</cbc:TaxExclusiveAmount>
    <cbc:TaxInclusiveAmount currencyID="EUR">173.00</cbc:TaxInclusiveAmount>
    <cbc:PayableAmount currencyID="EUR">173.00</cbc:PayableAmount>
</cac:LegalMonetaryTotal>
```

This can be structurally valid XML and structurally valid UBL.

But if the line-extension total is EUR 100 while the claimed tax-exclusive total is EUR 150 without corresponding charges/allowances, the invoice may fail EN 16931 arithmetic rules.

Therefore:

```text
XSD validity
    !=
semantic validity
```

---

# 4. Current official EN 16931 validation artefacts

The official CEN/TC 434 validation artefacts are maintained in:

- https://github.com/ConnectingEurope/eInvoicing-EN16931

Current UBL/CII release at the time this document was checked:

```text
EN 16931 validation artefacts: 1.3.16
```

Release:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/tag/validation-1.3.16

The repository states that the release supports:

```text
UBL 2.1
CII D16B
```

The release contains, for each supported syntax:

- source Schematron;
- a single preprocessed Schematron;
- precompiled XSLT;
- example documents.

The repository is licensed under **EUPL 1.2**.

Repository README:

- https://github.com/ConnectingEurope/eInvoicing-EN16931

---

# 5. Important architectural distinction

The EN 16931 repository explicitly contains the **base EN 16931 validation rules**.

It does **not** contain CIUS rules such as Peppol BIS Billing.

Therefore these must remain separate layers:

```text
UBL 2.1 XSD
     |
     v
EN 16931
     |
     v
Peppol BIS Billing
```

Do not collapse this into one generic `"Peppol validation"` result.

The API should eventually be able to express:

```json
{
  "ubl21": {
    "valid": true
  },
  "en16931": {
    "valid": true
  },
  "peppol": {
    "valid": false
  }
}
```

That separation is valuable operationally and diagnostically.

---

# 6. Structure of the EN 16931 Schematron implementation

The source project separates several concerns.

Conceptually:

```text
EN 16931 semantic rules
       |
       +--> abstract rule model
       |
       +--> UBL bindings
       |
       +--> UBL syntax restrictions
       |
       +--> code-list rules
       |
       v
preprocessed Schematron
       |
       v
compiled validation XSLT
```

Important source components include:

```text
EN16931-model.sch
EN16931-UBL-model.sch
EN16931-UBL-syntax.sch
EN16931-UBL-codes.sch
```

These are assembled into a single preprocessed Schematron and then compiled to an XSLT validator.

The project itself uses the compiled UBL XSLT for validation.

---

# 7. Runtime artefact to use

For runtime validation, use the official compiled file:

```text
ubl/xslt/EN16931-UBL-validation.xslt
```

Pinned version:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/xslt/EN16931-UBL-validation.xslt

The corresponding preprocessed Schematron is:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/schematron/preprocessed/EN16931-UBL-validation-preprocessed.sch

The compiled XSLT is the preferred runtime artefact.

Do **not** compile Schematron on every validation request.

Do **not** implement the Schematron language in Go.

Do **not** translate hundreds of EN 16931 rules manually into Go.

Instead:

```text
official rules
    |
    v
official compiled XSLT
    |
    v
mature XSLT engine
    |
    v
SVRL
    |
    v
Go normalization
```

This keeps the implementation faithful to the published standard and dramatically reduces maintenance.

---

# 8. XSLT processor requirement

The EN 16931 validation pipeline requires an XSLT processor capable of executing the generated stylesheet.

A practical implementation is **SaxonJ HE**.

Official Saxon command-line documentation:

- https://www.saxonica.com/html/documentation12/using-xsl/commandline/

Canonical transformation form:

```text
java net.sf.saxon.Transform \
    -s:source.xml \
    -xsl:stylesheet.xslt \
    -o:output.xml
```

Depending on how Saxon is packaged in the image, the invocation can use an explicit classpath or an executable wrapper.

Example conceptual command:

```bash
java -cp "/opt/saxon/*" net.sf.saxon.Transform \
    -s:/work/input.xml \
    -xsl:/opt/en16931/1.3.16/EN16931-UBL-validation.xslt \
    -o:/work/en16931.svrl.xml
```

The exact Saxon distribution and invocation should be pinned in the Docker build.

---

# 9. Why `xmllint` is not enough

`xmllint` / libxml2 is a good choice for:

```text
XML parsing
XSD validation
XPath 1.0-oriented operations
```

It is not an appropriate engine for the current EN 16931 generated XSLT/Schematron requirements.

Therefore the worker should use two specialized engines:

```text
libxml2 / xmllint
        |
        +--> UBL 2.1 XSD

Saxon
        |
        +--> EN 16931 XSLT
        |
        +--> later Peppol XSLT
```

Go remains the orchestration and API layer.

---

# 10. Schematron output: SVRL

Schematron validation normally produces **SVRL**:

```text
Schematron Validation Report Language
```

Typical structure:

```xml
<svrl:schematron-output
 xmlns:svrl="http://purl.oclc.org/dsdl/svrl">

  <svrl:failed-assert
      id="BR-CO-13"
      flag="fatal"
      test="..."
      location="...">

    <svrl:text>
      ...
    </svrl:text>

  </svrl:failed-assert>

</svrl:schematron-output>
```

The application does not need to understand the internal implementation of every Schematron rule.

It needs to understand SVRL.

This is a much cleaner boundary.

---

# 11. SVRL elements to process

At minimum parse:

```text
svrl:failed-assert
svrl:successful-report
```

For `failed-assert`, extract where available:

```text
id
flag
location
test
text
diagnostic-reference
```

Recommended normalized Go model:

```go
type ValidationIssue struct {
    Layer      string `json:"layer"`
    Rule       string `json:"rule"`
    Severity   string `json:"severity"`
    Location   string `json:"location,omitempty"`
    Test       string `json:"test,omitempty"`
    Message    string `json:"message"`
    Diagnostic string `json:"diagnostic,omitempty"`
}
```

Example normalized result:

```json
{
  "layer": "en16931",
  "rule": "BR-CO-13",
  "severity": "fatal",
  "location": "/ubl:Invoice/cac:LegalMonetaryTotal",
  "message": "Invoice total amount without VAT does not reconcile"
}
```

Keep the original rule identifier.

Do not invent internal rule numbers.

---

# 12. Severity handling

Schematron results can carry severity information such as:

```text
fatal
warning
```

Recommended policy:

```text
fatal failed assertion
    => stage.valid = false

warning only
    => stage.valid = true
       stage.warnings populated

unknown severity
    => preserve original value
       log it
       use configurable fallback policy
```

Do not flatten warnings and fatal errors into one untyped list.

Recommended model:

```json
{
  "profile": "en16931",
  "valid": false,
  "errors": [],
  "warnings": []
}
```

Or use a single ordered `issues[]` array where every item has a severity.

---

# 13. EN 16931 rule families

The validator will report rule IDs belonging to families such as:

```text
BR-*
BR-CO-*
BR-DEC-*
BR-CL-*
BR-S-*
BR-Z-*
BR-E-*
BR-AE-*
BR-G-*
BR-IC-*
BR-O-*
UBL-SR-*
UBL-CR-*
```

Examples of their roles:

| Family | General purpose |
|---|---|
| `BR-*` | Core business rules |
| `BR-CO-*` | Cross-element consistency / calculations |
| `BR-DEC-*` | Decimal precision |
| `BR-CL-*` | Controlled code lists |
| `BR-S-*` | Standard-rated VAT |
| `BR-Z-*` | Zero-rated VAT |
| `BR-E-*` | VAT exempt |
| `BR-AE-*` | Reverse-charge VAT |
| `BR-G-*` | Export-related VAT |
| `BR-IC-*` | Intra-community supply |
| `BR-O-*` | Not subject to VAT |
| `UBL-SR-*` | UBL syntax restrictions |
| `UBL-CR-*` | UBL constructs disallowed by the EN binding |

This means an invoice can fail EN 16931 even when every XML element is perfectly legal under the UBL XSD.

---

# 14. Code-list validation

The EN 16931 rules also validate controlled values.

Examples include:

```text
currency codes
country codes
invoice type codes
credit-note type codes
unit codes
VAT category codes
payment means
identifier schemes
MIME types
```

An XML value such as:

```xml
<cbc:DocumentCurrencyCode>FROG</cbc:DocumentCurrencyCode>
```

can satisfy an XSD string datatype.

It must still fail semantic/code-list validation because `FROG` is not an accepted currency code.

Supporting artefact registry:

- https://ec.europa.eu/digital-building-blocks/sites/display/DIGITAL/Registry+of+supporting+artefacts+to+implement+EN16931

---

# 15. Proposed validation profiles

The service should make validation depth explicit.

Recommended profiles:

```text
ubl21
en16931
peppol-billing
```

Semantics:

## `ubl21`

```text
XML parsing
+
UBL 2.1 XSD
```

## `en16931`

```text
XML parsing
+
UBL 2.1 XSD
+
EN 16931 UBL validation
```

Only valid for supported invoice document types.

## `peppol-billing`

Future:

```text
XML parsing
+
UBL 2.1 XSD
+
EN 16931
+
Peppol BIS Billing
```

This profile model avoids hidden behavior.

A caller should know what definition of `"valid"` was requested.

---

# 16. Job model extension

The existing queue payload should be extended with a requested validation profile.

Conceptual example:

```json
{
  "job_id": "01J...",
  "document_id": "01J...",
  "validation_profile": "en16931"
}
```

Optional future fields:

```json
{
  "validation_profile": "peppol-billing",
  "profile_version": "3.0.21",
  "country_profile": "IE"
}
```

Do not put the full XML into queue metadata if the existing design already stores/retrieves documents separately.

Prefer:

```text
queue contains document reference
worker retrieves XML
```

This avoids oversized queue messages and keeps document storage authoritative.

---

# 17. Stage model

Treat validation as a pipeline of independent stages.

Recommended stage names:

```text
xml
ubl21-xsd
en16931
peppol
```

Conceptual job state:

```json
{
  "job_id": "01J...",
  "state": "completed",
  "stages": [
    {
      "name": "xml",
      "status": "passed"
    },
    {
      "name": "ubl21-xsd",
      "status": "passed"
    },
    {
      "name": "en16931",
      "status": "failed"
    }
  ]
}
```

This is preferable to one boolean:

```json
{
  "valid": false
}
```

because it tells the caller **where** validation failed.

---

# 18. Pipeline short-circuit rules

Recommended execution rules:

```text
XML malformed
    => stop

unsupported document type
    => stop requested profile
       return profile-not-applicable / unsupported

UBL XSD invalid
    => stop EN16931
       EN16931 status = not-run

UBL XSD valid
    => run EN16931

EN16931 fatal failures
    => EN16931 valid = false

EN16931 warnings only
    => EN16931 valid = true
```

Do not run EN 16931 against malformed or XSD-invalid XML.

That would generate noisy secondary errors and waste CPU.

---

# 19. Document-type applicability

Before executing the EN 16931 stage, confirm the root document is supported.

Initial rule:

```text
Invoice
    => eligible

CreditNote
    => eligible

everything else
    => EN16931 not applicable
```

Use namespace-aware root detection.

Do not identify document type merely by filename.

Example:

```xml
<Invoice
 xmlns="urn:oasis:names:specification:ubl:schema:xsd:Invoice-2">
```

The namespace URI and root local name together identify the UBL document type.

---

# 20. Runtime architecture recommendation

For the first implementation, keep Schematron inside the **existing worker container**.

Recommended:

```text
validation-worker image
|
+-- Go worker
|
+-- xmllint/libxml2
|
+-- Java runtime
|
+-- Saxon
|
+-- pinned EN16931 XSLT
|
+-- pinned UBL 2.1 XSDs
```

Do **not** create a Schematron microservice initially.

The queue already provides:

```text
asynchronous execution
backpressure
retry control
worker scaling
```

Adding another network service at this point would create:

```text
network hop
service discovery
new deployment
new health checks
new failure modes
new version-coordination problem
```

without solving an immediate problem.

Keep the architecture boring until load proves otherwise.

---

# 21. Future scaling option

If transformation throughput later becomes a bottleneck, consider extracting the XSLT engine behind an internal validation service.

Future possibility:

```text
queue worker
    |
    v
internal schematron service
    |
    +-- cached compiled stylesheets
    +-- warm JVM
    +-- controlled concurrency
```

This can eliminate JVM startup costs per validation.

But it should be an optimization based on measurements.

The initial implementation should favor simplicity:

```text
one queued job
    |
    v
one controlled Saxon invocation
```

---

# 22. External-process interface in Go

Create a narrow interface.

Example:

```go
type SemanticValidator interface {
    Validate(ctx context.Context, xmlPath string) (*ValidationResult, error)
}
```

Concrete implementation:

```go
type EN16931Validator struct {
    JavaPath      string
    SaxonClasspath string
    Stylesheet    string
    Timeout       time.Duration
}
```

The orchestration layer should not know Saxon command-line details.

It should call:

```go
result, err := en16931.Validate(ctx, xmlPath)
```

This abstraction makes it possible to replace the implementation later with:

```text
embedded Saxon alternative
internal service
different XSLT processor
test fake
```

without changing queue/business logic.

---

# 23. Suggested Go package boundaries

One possible layout:

```text
internal/
├── validator/
│   ├── validator.go
│   ├── pipeline.go
│   │
│   ├── xsd/
│   │   ├── validator.go
│   │   └── xmllint.go
│   │
│   ├── schematron/
│   │   ├── validator.go
│   │   ├── saxon.go
│   │   └── svrl.go
│   │
│   └── profiles/
│       ├── ubl21.go
│       ├── en16931.go
│       └── peppol.go
```

The exact package names can follow the current repository conventions.

The important separation is:

```text
pipeline
profile selection
XSD engine
XSLT engine
SVRL parser
```

---

# 24. Process invocation requirements

External validation must be controlled.

The worker must use:

```text
explicit executable path
explicit stylesheet path
argument arrays
context timeout
captured stdout/stderr
bounded document size
bounded output size
temporary working directory
cleanup after completion
resource/concurrency limits
```

Never execute:

```go
exec.Command("sh", "-c", userControlledString)
```

Instead execute arguments directly.

Conceptually:

```go
exec.CommandContext(
    ctx,
    javaPath,
    "-cp", saxonClasspath,
    "net.sf.saxon.Transform",
    "-s:"+inputPath,
    "-xsl:"+stylesheetPath,
    "-o:"+svrlPath,
)
```

No XML content belongs in a shell command.

---

# 25. Temporary-file strategy

A simple first implementation can use files:

```text
/work/job-UUID/
    input.xml
    en16931.svrl.xml
```

Lifecycle:

```text
create isolated temp directory
        |
        v
write/copy retrieved XML
        |
        v
XSD validation
        |
        v
Saxon transformation
        |
        v
parse SVRL
        |
        v
optionally persist raw SVRL
        |
        v
remove temp directory
```

Do not reuse filenames between concurrent jobs.

---

# 26. SVRL persistence strategy

Store both:

1. normalized validation information;
2. optionally the raw SVRL.

Recommended:

```text
normalized JSON
    => primary API response

raw SVRL
    => diagnostic/audit artefact
```

The normalized result is stable for clients.

The raw SVRL preserves information if the normalization model later evolves.

Example stored metadata:

```json
{
  "validator": "en16931-ubl",
  "validator_version": "1.3.16",
  "engine": "saxon",
  "duration_ms": 84,
  "valid": false,
  "fatal_count": 2,
  "warning_count": 1
}
```

---

# 27. Result API

The existing retrieval web services should expose the new validation layer without forcing clients to parse SVRL.

Suggested shape:

```json
{
  "document_id": "01J...",
  "validation": {
    "profile": "en16931",
    "valid": false,
    "versions": {
      "ubl": "2.1",
      "en16931": "1.3.16"
    },
    "stages": {
      "xml": {
        "valid": true
      },
      "ubl21": {
        "valid": true
      },
      "en16931": {
        "valid": false,
        "issues": [
          {
            "rule": "BR-CO-13",
            "severity": "fatal",
            "message": "...",
            "location": "..."
          }
        ]
      }
    }
  }
}
```

Do not expose Saxon's process exit code as the validation result.

There are two different concepts:

```text
process execution failed
```

and:

```text
document is semantically invalid
```

These must not be confused.

---

# 28. Error taxonomy

At minimum distinguish:

```text
VALIDATION_INVALID
VALIDATION_ENGINE_ERROR
VALIDATION_TIMEOUT
UNSUPPORTED_PROFILE
PROFILE_NOT_APPLICABLE
ARTEFACT_NOT_FOUND
ARTEFACT_VERSION_ERROR
DOCUMENT_NOT_FOUND
INTERNAL_ERROR
```

Examples:

## Invoice fails rule BR-CO-13

```text
job completes normally
EN16931.valid = false
```

This is **not** a worker failure.

## Saxon crashes

```text
job execution failure
VALIDATION_ENGINE_ERROR
```

## Worker exceeds timeout

```text
VALIDATION_TIMEOUT
```

## EN16931 requested for DespatchAdvice

```text
PROFILE_NOT_APPLICABLE
```

This distinction will make queue retries sane.

---

# 29. Queue retry policy

Do not retry semantic failures.

Example:

```text
BR-CO-13 failed
    => deterministic document failure
    => no retry
```

Retry may be appropriate for:

```text
temporary storage failure
worker crash
temporary infrastructure failure
```

Retry is generally not useful for:

```text
malformed XML
XSD failure
EN 16931 rule failure
unsupported document/profile
```

Otherwise the queue will repeatedly process invoices that can never pass.

---

# 30. Version pinning

Never validate against an unpinned moving branch.

Pin:

```text
UBL = 2.1
EN16931 = 1.3.16
Saxon = explicit version
```

Recommended layout:

```text
validation-assets/
├── ubl/
│   └── 2.1/
│       └── ...
│
└── en16931/
    └── 1.3.16/
        ├── EN16931-UBL-validation.xslt
        ├── source-url.txt
        └── sha256.txt
```

Record:

```text
source URL
version/tag
download date
SHA-256
license
```

Validation responses should report the versions used.

---

# 31. Artefact acquisition

The production image should not download validation artefacts on startup.

Preferred build route:

```text
build script
    |
    +--> download pinned release
    |
    +--> verify checksum
    |
    +--> extract only required artefacts
    |
    v
Docker image
```

Alternative:

```text
repository script
    |
    +--> download pinned official artefacts
```

The existing project policy already permits `.sh` scripts whose role is acquiring authoritative specification artefacts.

The runtime should operate offline with respect to standards artefacts.

---

# 32. Docker implementation

The worker Docker image needs:

```text
Go worker binary
libxml2/xmllint
Java runtime
Saxon runtime
UBL 2.1 XSDs
EN16931 1.3.16 XSLT
```

Conceptual image:

```dockerfile
FROM <runtime-base>

# xmllint
RUN ...

# Java runtime
RUN ...

# Saxon
COPY third_party/saxon/ /opt/saxon/

# Validation artefacts
COPY validation-assets/ubl/2.1/ /opt/ubl/2.1/
COPY validation-assets/en16931/1.3.16/ /opt/en16931/1.3.16/

COPY validator /usr/local/bin/validator

ENTRYPOINT ["/usr/local/bin/validator"]
```

Pin package/image versions according to the project's normal reproducibility policy.

---

# 33. Worker concurrency

Saxon execution consumes significantly more resources than a simple XSD check.

Therefore EN 16931 validation must respect a worker concurrency limit.

Start conservatively.

Example conceptual configuration:

```text
QUEUE_WORKERS=4
SCHEMATRON_MAX_CONCURRENCY=2
SCHEMATRON_TIMEOUT=10s
```

Exact values should come from load testing.

The queue is useful here because it provides natural backpressure.

---

# 34. Resource limits

Protect the service from pathological XML and expensive transformations.

Apply limits to:

```text
maximum XML file size
maximum validation duration
maximum worker concurrency
maximum SVRL output size
temporary disk usage
container memory
container CPU
```

Consider rejecting documents beyond a configured maximum before invoking Saxon.

---

# 35. XML security

The XML handling layer must be defensive.

At minimum:

```text
do not permit arbitrary external entity resolution
do not allow untrusted network retrieval
do not allow document-controlled stylesheet selection
use only server-controlled XSLT
use only server-controlled XSDs
use isolated temporary directories
```

The client must never supply:

```text
stylesheet path
Saxon options
Java options
shell arguments
```

The validation profile maps internally to trusted artefacts.

---

# 36. Logging

Log stage metadata, not full invoices by default.

Recommended log event:

```json
{
  "job_id": "...",
  "document_id": "...",
  "stage": "en16931",
  "version": "1.3.16",
  "duration_ms": 84,
  "valid": false,
  "fatal_count": 2,
  "warning_count": 0
}
```

Invoices can contain commercially sensitive and personal data.

Avoid dumping XML or complete SVRL into ordinary application logs.

---

# 37. Metrics

Add metrics such as:

```text
validation_jobs_total
validation_jobs_active
validation_duration_seconds
validation_failures_total
validation_engine_errors_total
validation_timeouts_total
en16931_fatal_rules_total{rule="BR-CO-13"}
queue_depth
```

Rule-frequency metrics can be particularly useful for discovering common customer integration mistakes.

Be cautious with cardinality.

Do not label metrics with:

```text
document_id
customer_id
invoice_id
```

unless your monitoring architecture is explicitly designed for that cardinality.

---

# 38. Local proof-of-concept

Before touching the queue, prove the transformation manually.

## Step 1

Obtain EN 16931 release 1.3.16:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/tag/validation-1.3.16

## Step 2

Locate:

```text
ubl/xslt/EN16931-UBL-validation.xslt
```

## Step 3

Install/pin a SaxonJ HE runtime.

Documentation:

- https://www.saxonica.com/html/documentation12/using-xsl/commandline/

## Step 4

Run a known UBL Invoice through the stylesheet.

Conceptually:

```bash
java -cp "/opt/saxon/*" net.sf.saxon.Transform \
  -s:invoice.xml \
  -xsl:EN16931-UBL-validation.xslt \
  -o:invoice.svrl.xml
```

## Step 5

Inspect the SVRL.

Acceptance condition:

```text
known valid EN16931 invoice
    => zero fatal failed assertions
```

## Step 6

Deliberately break one business rule.

Examples:

```text
remove required invoice identifier
break total arithmetic
use invalid VAT data
use an invalid controlled code
```

Acceptance condition:

```text
SVRL contains expected failed-assert
with a recognizable rule ID
```

Do this before queue integration.

---

# 39. Implementation route

The implementation should proceed incrementally.

---

## Phase 0: Freeze the contract

Define before coding:

```text
supported profiles
supported document types
version identifiers
normalized validation result format
stage statuses
error taxonomy
```

Initial decisions:

```text
profile ubl21
profile en16931

EN16931:
    Invoice      supported
    CreditNote   supported
    others       not applicable

UBL version:
    2.1

EN16931 artefacts:
    1.3.16
```

### Deliverable

A small design document/API schema defining the validation result contract.

### Acceptance criteria

- result can distinguish XSD invalid from EN16931 invalid;
- process failure is distinct from document invalidity;
- version information is included.

---

## Phase 1: Acquire and pin official artefacts

Add a deterministic acquisition process.

Acquire:

```text
EN16931-UBL-validation.xslt
official EN16931 examples
license information
```

Store metadata:

```text
version
source URL
checksum
```

### Deliverable

Pinned validation-assets directory or download script.

### Acceptance criteria

- clean environment can reproduce the artefact set;
- checksum is verified;
- runtime performs no standards download.

---

## Phase 2: Saxon command-line proof

Run the official stylesheet outside the application.

Test:

```text
known valid invoice
known invalid invoice
known valid credit note
known invalid credit note
```

### Deliverable

Documented command and known SVRL files.

### Acceptance criteria

- Saxon executes stylesheet correctly;
- valid/invalid examples are distinguishable through SVRL;
- rule IDs and locations are present.

---

## Phase 3: Implement SVRL parser in Go

Create:

```text
internal/validator/schematron/svrl.go
```

Responsibilities:

```text
parse SVRL XML
extract failed assertions
extract reports
normalize severity
extract text
extract rule ID
extract location
preserve test expression where useful
```

### Unit tests

Include fixtures for:

```text
zero failures
one fatal
multiple fatal
warning
mixed warning/fatal
missing optional attributes
malformed SVRL
```

### Acceptance criteria

Given a fixed SVRL file, the parser returns deterministic normalized JSON.

---

## Phase 4: Implement Saxon adapter

Create a controlled wrapper around Saxon.

Responsibilities:

```text
construct argument list
create output path
execute with context timeout
capture stderr
check process result
parse generated SVRL
return normalized ValidationResult
```

### Important

The adapter should return two distinct outputs conceptually:

```text
document validation result
execution error
```

Example Go semantics:

```go
result, err := validator.Validate(ctx, xmlPath)
```

Where:

```text
result.Valid == false, err == nil
    => invoice is invalid

err != nil
    => validator could not execute correctly
```

### Acceptance criteria

- invalid invoice does not become a Go/process error;
- Saxon failure does;
- timeout is enforced;
- temporary files are cleaned.

---

## Phase 5: Add EN16931 profile to pipeline

Extend the existing validation worker.

Current:

```text
retrieve XML
    |
    v
UBL XSD
    |
    v
persist result
```

New:

```text
retrieve XML
    |
    v
detect document
    |
    v
UBL XSD
    |
    +-- invalid --> persist result
    |
    v
profile == en16931 ?
    |
   yes
    |
    v
document type applicable ?
    |
   yes
    |
    v
EN16931 XSLT
    |
    v
parse SVRL
    |
    v
persist combined result
```

### Acceptance criteria

- existing `ubl21` behavior remains unchanged;
- `en16931` invokes the new stage;
- non-Invoice/CreditNote documents return a defined profile-not-applicable result;
- XSD-invalid documents do not invoke Saxon.

---

## Phase 6: Extend queue payload

Add:

```text
validation_profile
```

to queued validation work.

Default strategy:

```text
if absent:
    use existing ubl21 behavior
```

This protects backward compatibility.

### Acceptance criteria

Old clients continue to receive UBL 2.1 validation behavior.

New clients can request:

```text
en16931
```

---

## Phase 7: Extend persistence

Persist:

```text
requested profile
UBL version
EN16931 version
stage status
normalized issues
validation duration
raw SVRL reference, optionally
```

Avoid tying persistence directly to one Schematron format.

Persist normalized domain objects.

### Acceptance criteria

Results can be retrieved after worker completion without rerunning validation.

---

## Phase 8: Extend retrieval API

Expose stage-specific results.

Example:

```json
{
  "status": "completed",
  "profile": "en16931",
  "valid": false,
  "versions": {
    "ubl": "2.1",
    "en16931": "1.3.16"
  },
  "stages": [
    {
      "name": "ubl21-xsd",
      "status": "passed"
    },
    {
      "name": "en16931",
      "status": "failed",
      "issues": [
        {
          "rule": "BR-CO-13",
          "severity": "fatal",
          "location": "...",
          "message": "..."
        }
      ]
    }
  ]
}
```

### Acceptance criteria

A client can identify every failing rule without knowing SVRL.

---

## Phase 9: Integration tests through the queue

Do not stop at unit testing the validator package.

Test the complete asynchronous route:

```text
HTTP submit
    |
    v
queue
    |
    v
worker
    |
    v
XSD
    |
    v
EN16931
    |
    v
storage
    |
    v
HTTP retrieve result
```

Test cases:

```text
valid UBL + valid EN16931 invoice
valid UBL + invalid EN16931 invoice
invalid UBL invoice
malformed XML
credit note valid
credit note invalid
unsupported UBL document with EN16931 profile
Saxon timeout
Saxon executable unavailable
missing stylesheet
```

### Acceptance criteria

Every failure is classified correctly and queue retry behavior is correct.

---

## Phase 10: Performance baseline

Measure:

```text
XSD validation time
Saxon startup time
XSLT execution time
SVRL parse time
end-to-end queue latency
memory use
CPU
```

Run:

```text
1 concurrent job
2 concurrent jobs
4 concurrent jobs
8 concurrent jobs
```

Choose worker concurrency based on measurements.

### Acceptance criteria

Define a safe production concurrency limit and timeout.

---

## Phase 11: Production hardening

Before declaring EN16931 production-ready:

```text
pin Saxon
pin Java runtime
pin EN16931 artefacts
verify checksums
set size limits
set timeouts
set memory limits
add metrics
add structured logs
add health checks
test worker termination
test retry policy
test corrupted artefact handling
```

---

# 40. Recommended first implementation milestone

The first milestone should deliberately be small:

> **A queued UBL Invoice that already passes UBL 2.1 XSD validation can be passed through the official EN 16931 1.3.16 UBL XSLT; its SVRL output is parsed by Go and returned through the existing validation-result retrieval API.**

Limit milestone 1 to:

```text
Invoice only
EN16931 1.3.16 only
one Saxon invocation per queued job
fatal + warning parsing
normalized results
no Peppol yet
```

Then add:

```text
CreditNote
```

before moving to Peppol.

This creates a narrow, testable vertical slice.

---

# 41. Recommended sequence after milestone 1

```text
Milestone 1
UBL Invoice + EN16931
        |
        v
Milestone 2
UBL CreditNote + EN16931
        |
        v
Milestone 3
versioned validation profiles
        |
        v
Milestone 4
Peppol BIS Billing Schematron
        |
        v
Milestone 5
country-specific validation/profile handling
```

Do not add Peppol until the generic SVRL execution and normalization layer is stable.

Peppol should reuse the same infrastructure:

```text
Saxon runner
SVRL parser
issue normalization
version metadata
profile pipeline
```

Only the stylesheet/profile changes.

---

# 42. Design for Peppol now, implement it later

A useful abstraction is:

```go
type SchematronProfile struct {
    Name         string
    Version      string
    Stylesheet   string
    AppliesTo    []DocumentType
}
```

Examples:

```text
Name: en16931
Version: 1.3.16
Stylesheet: EN16931-UBL-validation.xslt
AppliesTo:
    Invoice
    CreditNote
```

Future:

```text
Name: peppol-billing
Version: 3.0.21
Stylesheet: PEPPOL-EN16931-UBL.xslt
AppliesTo:
    Invoice
    CreditNote
```

The execution engine should be generic.

The business profile supplies configuration.

---

# 43. Generic Schematron engine

Aim for:

```go
type SchematronEngine interface {
    Validate(
        ctx context.Context,
        stylesheet string,
        xmlPath string,
    ) (*SVRLReport, error)
}
```

Then profile code becomes:

```text
EN16931 profile
    -> stylesheet A

Peppol profile
    -> stylesheet B

future CIUS
    -> stylesheet C
```

This avoids creating:

```text
EN16931-specific Saxon code
Peppol-specific Saxon code
Ireland-specific Saxon code
...
```

The engine executes XSLT.

Profiles determine which XSLT to execute.

---

# 44. Recommended internal pipeline abstraction

Conceptually:

```go
type Stage interface {
    Name() string
    Execute(ctx context.Context, input *DocumentContext) StageResult
}
```

Pipeline:

```text
XMLStage
    |
UBLXSDStage
    |
EN16931Stage
    |
PeppolStage
```

Profile selection builds the stage list:

```text
ubl21:
    XML
    UBLXSD

en16931:
    XML
    UBLXSD
    EN16931

peppol-billing:
    XML
    UBLXSD
    EN16931
    Peppol
```

That scales better than a growing `if/else` chain.

---

# 45. Validation semantics

Define these three concepts separately:

## Stage execution status

```text
not-run
running
completed
engine-error
timeout
```

## Compliance result

```text
valid
invalid
not-applicable
unknown
```

## Issue severity

```text
fatal
warning
information
unknown
```

This avoids overloading one field called `"status"`.

Example:

```json
{
  "execution_status": "completed",
  "compliance": "invalid",
  "issues": [
    {
      "severity": "fatal",
      "rule": "BR-CO-13"
    }
  ]
}
```

---

# 46. Test corpus strategy

Use three sources.

## 46.1 Official EN 16931 examples

Release:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/tag/validation-1.3.16

Use official examples as reference fixtures.

## 46.2 Existing UBL-21 generated XSD samples

The project's generated UBL samples are valuable for proving:

```text
UBL-valid
does not imply
EN16931-valid
```

Many generic UBL samples should legitimately fail the EN 16931 profile.

That is expected.

## 46.3 Mutation tests

Take a known-valid EN 16931 invoice and mutate one rule at a time.

Examples:

```text
delete invoice ID
delete issue date
change currency to invalid value
change tax total
change payable total
break VAT breakdown
exceed permitted decimal precision
remove seller information
remove buyer information
```

For each mutation assert:

```text
expected rule ID appears
```

This is much stronger than merely asserting `"invalid"`.

---

# 47. Regression test rule

Pin expected validator version with fixtures.

Example:

```text
fixture:
    invoice-valid-001.xml

expected:
    en16931_version = 1.3.16
    fatal_count = 0
```

On upgrade to a new EN 16931 release:

```text
1. install new artefacts
2. rerun corpus
3. compare results
4. inspect differences
5. explicitly approve upgrade
```

Never silently replace the stylesheet in production.

---

# 48. EN 16931 upgrade process

Treat rule upgrades like application releases.

Recommended workflow:

```text
new EN16931 release discovered
        |
        v
download to new version directory
        |
        v
verify checksum
        |
        v
run official examples
        |
        v
run project regression corpus
        |
        v
diff rule results
        |
        v
review changes
        |
        v
release new worker
```

Keep the previous version available during testing.

---

# 49. Failure examples

## Case A: malformed XML

```text
XML       fail
UBL XSD   not run
EN16931   not run
```

## Case B: valid XML but invalid UBL

```text
XML       pass
UBL XSD   fail
EN16931   not run
```

## Case C: valid UBL but invalid EN 16931

```text
XML       pass
UBL XSD   pass
EN16931   fail
```

This is the new case this implementation adds.

## Case D: EN 16931 engine crashes

```text
XML       pass
UBL XSD   pass
EN16931   engine-error
```

This is **not** the same as EN 16931 non-compliance.

---

# 50. Recommended configuration

Conceptual configuration:

```text
VALIDATION_UBL21_ENABLED=true

EN16931_ENABLED=true
EN16931_VERSION=1.3.16
EN16931_XSLT=/opt/en16931/1.3.16/EN16931-UBL-validation.xslt

SAXON_JAVA=/usr/bin/java
SAXON_CLASSPATH=/opt/saxon/*
SAXON_TIMEOUT=10s

MAX_XML_SIZE=10MB
SCHEMATRON_MAX_CONCURRENCY=2
```

Configuration names should follow existing project conventions.

Paths should be server controlled.

---

# 51. Health checks

A basic process health check is not enough.

At startup, verify:

```text
EN16931 XSLT exists
file is readable
configured version matches metadata
Saxon can start
Java can start
```

Optionally perform a tiny known validation as a readiness check.

Do not run a heavyweight invoice validation on every liveness probe.

---

# 52. Deployment strategy

Recommended rollout:

## Deployment 1

Ship Saxon and artefacts but leave profile disabled.

Verify:

```text
image size
Java runtime
file paths
health checks
```

## Deployment 2

Enable EN16931 in non-production.

Run integration corpus.

## Deployment 3

Expose `en16931` profile to selected clients.

## Deployment 4

Make generally available.

Keep existing `ubl21` behavior as the backward-compatible default unless the API contract intentionally changes.

---

# 53. What not to do

Avoid the following approaches.

## Do not manually reimplement EN 16931 rules in Go

Hundreds of rules evolve over time.

You would create a second, unofficial standard implementation.

## Do not compile Schematron per request

Use the official precompiled XSLT.

## Do not merge XSD and semantic failures

They mean different things.

## Do not retry invalid invoices

Compliance failure is deterministic.

## Do not run EN 16931 on every UBL document type

It is an invoice semantic profile.

## Do not download rules at runtime

Production behavior must be reproducible.

## Do not make clients parse SVRL

Normalize it behind the API.

## Do not begin with a separate Schematron microservice

The existing queue worker is already the correct execution boundary.

---

# 54. Proposed definition of done

EN 16931 support is complete for the first production increment when:

- [ ] official EN16931 1.3.16 artefacts are pinned;
- [ ] Saxon runtime is pinned;
- [ ] Invoice is supported;
- [ ] CreditNote is supported;
- [ ] UBL XSD must pass before EN16931 runs;
- [ ] SVRL is parsed in Go;
- [ ] fatal and warning severities are preserved;
- [ ] rule IDs are preserved;
- [ ] XPath/location is preserved where available;
- [ ] process errors are separate from compliance failures;
- [ ] validation results include versions;
- [ ] queue payload supports `en16931`;
- [ ] old UBL-only clients remain compatible;
- [ ] results are persisted;
- [ ] results are retrievable through existing web services;
- [ ] official example corpus is exercised;
- [ ] mutation tests verify specific rule IDs;
- [ ] timeout and concurrency limits are enforced;
- [ ] raw invoice content is not dumped to ordinary logs;
- [ ] production image does not download standards artefacts at runtime.

---

# 55. Immediate next engineering tasks

The recommended next sprint is:

1. **Download and pin EN16931 validation release 1.3.16.**
2. **Add Saxon to a development/container environment.**
3. **Run `EN16931-UBL-validation.xslt` manually against one Invoice.**
4. **Capture the SVRL output.**
5. **Write the Go SVRL parser.**
6. **Write a generic Saxon/XSLT adapter.**
7. **Introduce the `en16931` validation profile.**
8. **Insert the EN16931 stage immediately after successful UBL XSD validation.**
9. **Persist normalized rule failures.**
10. **Expose them through the existing result retrieval API.**
11. **Add Invoice integration tests through the queue.**
12. **Add CreditNote.**
13. **Only then start Peppol BIS Billing integration.**

This sequence minimizes architectural churn and produces usable value early.

---

# 56. Final target architecture

```text
                           HTTP API
                              |
                              v
                         queue request
                              |
                              v
                     validation queue
                              |
                              v
                     validation worker
                              |
                    +---------+---------+
                    |                   |
                    v                   |
               retrieve XML             |
                    |                   |
                    v                   |
               XML parser               |
                    |                   |
                    v                   |
             detect UBL type            |
                    |                   |
                    v                   |
              UBL 2.1 XSD               |
                    |                   |
             invalid|                   |
                    +-------> persist result
                    |
                  valid
                    |
                    v
             requested profile
                    |
          +---------+----------+
          |                    |
       ubl21                en16931
          |                    |
          |                    v
          |             applicable type?
          |                    |
          |                    v
          |            Saxon + official
          |            EN16931 UBL XSLT
          |                    |
          |                    v
          |                   SVRL
          |                    |
          |                    v
          |              Go SVRL parser
          |                    |
          +---------+----------+
                    |
                    v
          normalized validation result
                    |
                    v
                 storage
                    |
                    v
           result retrieval API
```

Later:

```text
EN16931 stage
      |
      v
Peppol BIS Billing stage
      |
      v
national profile stage
```

The infrastructure introduced for EN 16931 should become the generic foundation for every later Schematron-based profile.

---

# 57. Relevant links

## Official EN 16931 validation artefacts

Repository:

- https://github.com/ConnectingEurope/eInvoicing-EN16931

Release 1.3.16:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/releases/tag/validation-1.3.16

Pinned UBL validation XSLT:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/xslt/EN16931-UBL-validation.xslt

Pinned preprocessed UBL Schematron:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/ubl/schematron/preprocessed/EN16931-UBL-validation-preprocessed.sch

UBL source Schematron directory:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/tree/validation-1.3.16/ubl/schematron

UBL examples:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/tree/validation-1.3.16/ubl/examples

Build/validation configuration demonstrating use of the compiled XSLT:

- https://github.com/ConnectingEurope/eInvoicing-EN16931/blob/validation-1.3.16/pom-validate.xml

## EN 16931 supporting artefacts

European Commission registry:

- https://ec.europa.eu/digital-building-blocks/sites/display/DIGITAL/Registry+of+supporting+artefacts+to+implement+EN16931

European eInvoicing information:

- https://ec.europa.eu/digital-building-blocks/sites/display/DIGITAL/eInvoicing

## Saxon

Saxon documentation:

- https://www.saxonica.com/documentation/

Saxon command-line XSLT documentation:

- https://www.saxonica.com/html/documentation12/using-xsl/commandline/

## UBL 2.1

OASIS UBL 2.1 specification:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/UBL-2.1.html

UBL 2.1 XSDs:

- https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/

## Peppol next stage

OpenPeppol post-award specifications:

- https://peppol.org/documentation/technical-documentation/post-award-documentation/

Peppol BIS Billing 3.0:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/

Peppol Billing rules:

- https://docs.peppol.eu/poacc/billing/3.0/upcoming/rules/

---

# 58. Conclusion

The next architectural step is not to make the Go application understand Schematron.

It is to add a **generic XSLT/SVRL validation capability** to the existing queued worker.

The responsibility split should be:

```text
Go
    orchestration
    queue handling
    profile selection
    process control
    SVRL parsing
    normalization
    persistence
    API

libxml2 / xmllint
    UBL 2.1 XSD validation

Saxon
    execute standards-provided XSLT

EN16931 official XSLT
    semantic business rules
```

That gives the project a clean path:

```text
current
UBL 2.1 XSD validator
        |
        v
next
EN 16931 semantic validator
        |
        v
then
Peppol BIS Billing validator
        |
        v
later
national CIUS / profile validation
```

The queue and retrieval architecture you already have remains intact. Schematron simply becomes the next deterministic validation stage in the worker pipeline.
