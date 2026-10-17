# SAP ECC: invoice integration research

Reviewed: 17 October 2026. Status: documentation research, not an implemented or certified connector.

## Official interface and evidence limits
SAP publishes technical information for invoice IDoc basic type `INVOIC02`. The public ERP Help page was located, but its body was not extractable in this review. It is therefore a source locator, not evidence of a complete segment dictionary.

SAP's public configuration guide directs readers to transaction `WE30` for the basic-type definition. The full customer contract must include release, basic type, message type, any extension, segment hierarchy, qualifiers and partner configuration. No complete ECC invoice field schema has been obtained here.

## Connection to e-UBL21 — proposed
Configure the customer's outbound invoice process to deliver an IDoc to integration middleware. The adapter validates the actual IDoc contract, maps the invoice to the shared model and submits to e-UBL21. Transport may be supplied by existing SAP middleware; do not assume a direct REST call from ECC or label an IDoc as UBL.

## Mapping requirements
| Required data | Evidence to obtain |
| --- | --- |
| Invoice number, type, date, currency | Actual header segments and qualifiers |
| Seller/buyer, VAT IDs and addresses | Partner roles and customer extensions |
| Line quantities, units, prices | Item hierarchy and unit code mapping |
| Taxes, discounts and totals | Qualifier meanings and amount/sign conventions |
| References and payment details | Customer-populated segments |

## Remaining evidence
Export the customer IDoc definition and documentation; obtain successful invoice, credit-note, cancellation and mixed-VAT samples. Confirm output trigger, authorisation, delivery acknowledgements, IDoc retries and duplicate handling. SAP Support content may require authorised access. No IDoc receiver or customer connection is implemented or tested.

## Official sources
- [ERP INVOIC02 technical information](https://help.sap.com/docs/SAP_ERP_SPV/349992fb60854a62a264d716ad5c8f54/e9a062543da42357e10000000a44176d.html)
- [SAP configuration guide referring to WE30](https://help.sap.com/doc/2f8f040b351a4354b58f5a3278058d6a/618.28/en-US/loioa3173b1216f04b1290e616bf3891a5ad.pdf) — S/4HANA context; not an ECC release specification.

See the [shared research register](../../INTEGRATION-RESEARCH.md) for common mapping and acceptance requirements.
