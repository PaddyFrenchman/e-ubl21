UBL 2.1 defines **65 top-level document types**. The authoritative list is the OASIS `xsd/maindoc/` directory. ([OASIS Open Docs][1])

For your e-invoicing project, the most important ones are **Invoice, CreditNote, DebitNote, SelfBilledInvoice, SelfBilledCreditNote, FreightInvoice, Reminder, RemittanceAdvice, Statement, and UtilityStatement**. ([OASIS Open Docs][1])

Here is the complete list, using the exact UBL/XSD names:

```text
ApplicationResponse
AttachedDocument
AwardedNotification
BillOfLading
CallForTenders
Catalogue
CatalogueDeletion
CatalogueItemSpecificationUpdate
CataloguePricingUpdate
CatalogueRequest
CertificateOfOrigin
ContractAwardNotice
ContractNotice
CreditNote
DebitNote
DespatchAdvice
DocumentStatus
DocumentStatusRequest
ExceptionCriteria
ExceptionNotification
Forecast
ForecastRevision
ForwardingInstructions
FreightInvoice
FulfilmentCancellation
GoodsItemItinerary
GuaranteeCertificate
InstructionForReturns
InventoryReport
Invoice
ItemInformationRequest
Order
OrderCancellation
OrderChange
OrderResponse
OrderResponseSimple
PackingList
PriorInformationNotice
ProductActivity
Quotation
ReceiptAdvice
Reminder
RemittanceAdvice
RequestForQuotation
RetailEvent
SelfBilledCreditNote
SelfBilledInvoice
Statement
StockAvailabilityReport
Tender
TenderReceipt
TendererQualification
TendererQualificationResponse
TradeItemLocationProfile
TransportExecutionPlan
TransportExecutionPlanRequest
TransportProgressStatus
TransportProgressStatusRequest
TransportServiceDescription
TransportServiceDescriptionRequest
TransportationStatus
TransportationStatusRequest
UnawardedNotification
UtilityStatement
Waybill
```

These correspond directly to files such as:

```text
UBL-Invoice-2.1.xsd
UBL-CreditNote-2.1.xsd
UBL-DebitNote-2.1.xsd
UBL-Order-2.1.xsd
UBL-DespatchAdvice-2.1.xsd
UBL-RemittanceAdvice-2.1.xsd
...
```

The official directory shows all 65 schemas, from `UBL-ApplicationResponse-2.1.xsd` through `UBL-Waybill-2.1.xsd`. ([OASIS Open Docs][1])

### For the validator we are building

I would **not download and support all 65 initially**. Your first product should probably concentrate on:

```text
Invoice
CreditNote
DebitNote
SelfBilledInvoice
SelfBilledCreditNote
FreightInvoice
```

Then possibly:

```text
Reminder
RemittanceAdvice
Statement
UtilityStatement
```

The first six map most naturally to an **e-invoice validation API**. The others start pushing the product toward a broader financial-document validator.

Your architecture could eventually detect the root namespace and automatically select the correct XSD:

```go
switch documentType {
case "Invoice":
    schema = "UBL-Invoice-2.1.xsd"

case "CreditNote":
    schema = "UBL-CreditNote-2.1.xsd"

case "DebitNote":
    schema = "UBL-DebitNote-2.1.xsd"

case "SelfBilledInvoice":
    schema = "UBL-SelfBilledInvoice-2.1.xsd"
}
```

A useful next step would be to make **one downloader script that fetches all 65 `maindoc` schemas plus the common schemas once**, rather than maintaining separate Invoice and CreditNote download scripts.

[1]: https://docs.oasis-open.org/ubl/os-UBL-2.1/xsd/maindoc/ "Index of /ubl/os-UBL-2.1/xsd/maindoc/"
