# Milestone 1 Completion: EN 16931 and Peppol BIS Billing Validation

**Project:** e-UBL21  
**Date:** 2026-10-16  
**Status:** Completed for the tested local validation scope  
**Evidence:** `compliance.log` and `images.log`, supplied after local execution on 2026-10-16  
**Commit SHA:** Not captured in the supplied logs; associate this evidence with the tested revision before using it as a permanent release or PR gate.

## Executive summary

All four requested validation gates passed locally. The implemented UBL 2.1 XSD → EN 16931 → Peppol BIS Billing pipeline is verified for the scenarios exercised by these suites. No failures in the supplied logs block starting invoice-generation development.

This milestone **does not** assert full national conformity, Peppol network interoperability, Access Point accreditation, hosted CI success, or production readiness.

## Validation gate results

| Command | Result | Evidence |
| --- | --- | --- |
| `make test-en16931-compliance` | **PASS** | Server suite: 75.514 seconds |
| `make test-en16931-image` | **PASS** | Offline packaged-runtime check passed |
| `make test-peppol-compliance` | **PASS** | Release baseline passed; server suite: 108.809 seconds |
| `make test-peppol-image` | **PASS** | Offline packaged Peppol runtime check passed |

### EN 16931 scenarios exercised

- Valid Invoice and CreditNote passing UBL XSD and EN 16931.
- XSD-invalid Invoice short-circuiting the EN stage.
- XSD-valid but semantically invalid Invoice producing EN rule diagnostics.
- Non-Invoice UBL document correctly limited to XSD validation.
- Operational failure, dead-letter/replay, and crash/restart recovery.
- Targeted negative cases including VAT rates/exemptions, identifiers, party details, totals and payment information.
- Positive fixtures, including generated VAT variants and official guide examples.
- Concurrent duplicate replay claim resulting in one permanent result.

### Peppol scenarios exercised

- Pinned normalized release baseline verification.
- Public asynchronous Peppol compliance scenarios.
- XSD and EN failures correctly short-circuiting later stages.
- Invalid endpoint scheme, missing/invalid `ProfileID`, and invalid `CustomizationID`.
- EN-only backward compatibility without an incorrect Peppol-valid claim.
- Operational failure/replay and crash/restart/duplicate handling.

### Runtime and artefact verification

| Component | Verified version |
| --- | --- |
| EN 16931 validation artefacts | 1.3.16 |
| Peppol BIS Billing | 3.0.21 |
| Saxon-HE | 12.7 |
| XML Resolver | 6.0.17 |
| SchXslt compiler | 1.10.1 |

Both image builds verified pinned checksums, reproducibly compiled the Peppol Schematron, checked the separately packaged SBDH envelope stylesheet, and passed offline packaged-runtime checks. The intentional NATS-boundary failure at the end of each image test is expected.

The EN compliance log records a NATS key-revision mismatch during a concurrent duplicate replay test. The duplicate was discarded and the test passed; this is an expected optimistic-concurrency race, not a failure of this run.

## Completed scope

- UBL 2.1 XSD, EN 16931 semantic, and Peppol BIS Billing payload-validation stages implemented.
- Local compliance and image verification completed successfully for the four gates above.
- Explicit distinction between standards-invalid documents and operational processing failures.
- Durable stage evidence and ordered short-circuiting verified in the tested scenarios.
- Version-pinned validation runtime and assets verified.

## Remaining work

| Workstream | Next action | Priority |
| --- | --- | --- |
| SBDH envelope, issue #82 | Complete remaining realistic fixtures, identifier cases, and integration qualification | Medium |
| Structural corpus, issue #167 | Resolve outstanding corpus qualification and evidence issues; do not infer closure from this milestone | High |
| Ireland/UK conformity | Define and verify market-specific identifier, VAT, and business-rule scenarios | High |
| Invoice generation | Build canonical invoice model and UBL 2.1 generator; require generated outputs to pass applicable EN/Peppol profiles | High |
| Accounting adapters | Define and implement mappings for Sage, SAP, Xero and other selected systems | High |
| Peppol network routing | Access Point provider integration, delivery jobs, receipts, reconciliation, and inbound processing | Deferred |
| Production operations | HA, backup/restore, deployment hardening, hosted CI and operational qualification | Deferred |

## Recommended Milestone 2

**Invoice generation for Irish and UK markets.**

Proposed pipeline:

```text
Accounting-system adapters (Sage / SAP / Xero / others)
                         |
                         v
              Canonical invoice model
                         |
                         v
                UBL 2.1 generator
                         |
                         v
             Existing validation pipeline
          XSD -> EN 16931 -> Peppol BIS
                         |
                         v
           Validated XML + rule evidence
```

Initial acceptance criterion: representative Irish and UK invoices, including differing VAT treatments, can be generated and pass the explicitly requested applicable validation profile. Retain generated fixtures and rule evidence in regression tests.

## Evidence and limitations

The uploaded logs demonstrate **local test success**, not the exact tested Git commit, a fresh hosted GitHub Actions result, comprehensive country-specific coverage, a Peppol transport exchange, or production accreditation. The separate open work for #82 and #167 remains unresolved by these four gates.

Related project documents:
- [EN 16931 compliance gate](../docs/EN16931-COMPLIANCE-GATE.md)
- [Peppol validation contract](../docs/PEPPOL-VALIDATION-CONTRACT.md)
- [Peppol certification scope](../docs/PEPPOL-CERTIFICATION.md)
- [Peppol routing plan](../docs/PEPPOL-ROUTE.md)
