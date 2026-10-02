# Peppol certification
Date: 2026-10-02
Status: planning document; no accreditation or production readiness is claimed.

## Decision and scope
Operating our own certified Access Point (AP) is an option for a commercial, multi-customer invoice delivery service. It offers control over delivery, customer relationships, incident investigation and provider margins. Whether it is economical depends on customer volume, contracted provider prices and the full recurring cost of certification, security and operations.

Start with a certified provider and preserve a replaceable transport adapter. Reassess ownership when the business case demonstrates sufficient demand and operating capacity. UBL21 remains the independent validation service; generation and routing belong in separate components.

## What certification covers
Peppol combines business-document standards and a governed delivery network. Passing UBL21 validation does not certify an AP, establish participant registration or prove delivery. An AS4 receipt also does not establish business acceptance or payment.

Current verified UBL21 implementation supports UBL Invoice/CreditNote validation through UBL 2.1 XSD, EN16931 and Peppol BIS Billing 3.0.21 using pinned validation artefacts 1.3.16. Billing profiles 01/02 are supported for invoice payload conformity. Separate Invoice Response validation and network operation are outside that claim. A Peppol corpus contains 30 cases (14 valid, 16 invalid); this document does not assert a fresh successful test run.

## Accreditation path
1. Identify the legal entity, countries served and applicable Peppol Authority. Obtain current jurisdiction and service-domain requirements.
2. Join OpenPeppol in the applicable service-provider category. Membership alone does not authorise production service.
3. Sign the Peppol Service Provider Agreement with the responsible Authority and submit required corporate documentation.
4. Obtain Peppol PKI test certificates and set up the test deployment.
5. Implement the applicable current Peppol eDelivery and business specifications.
6. Execute required OpenPeppol Testbed conformance/accreditation tests and any additional Authority checks; retain evidence and resolve failures.
7. Obtain the required approvals and production certificates under the Authority's current process.
8. Establish ongoing compliance, reporting, certificate renewal and upgrade management before onboarding production customers.

Confirm the latest process with the Authority: the linked onboarding guide dates from April 2024 and is a baseline, not a substitute for current instructions.

## Technical platform
Use an established Peppol-compatible AS4 implementation; evaluate it against current Peppol specifications and certification tests. Generic AS4 support is insufficient evidence of Peppol conformity.

Required capabilities:
- Secure sending/receiving using Peppol PKI and applicable AS4 message security.
- Recipient discovery via SML and SMP, using participant, document-type and process identifiers.
- Participant publication/registration through an SMP arrangement. Operating an AP does not require that we build and operate our own SMP.
- Required envelope/message metadata and validation before accepting outgoing documents.
- Durable queues, transactional job creation, bounded concurrency, retry/backoff, duplicate protection and replay controls.
- Signed transport receipts, traceable message identifiers and explicit delivery states.
- Tenant isolation, authenticated customer APIs and secure inbound hand-off.
- Certificate/key protection and renewal, audit records, monitoring, alerting, backup and tested recovery.

Keep the transport engine separate from the Go router and UBL21. Kubernetes can host suitable components, but availability, durable storage and recovery require qualification.

## Operating responsibilities
Budget for service coverage appropriate to contractual availability, incident response, support, interoperability testing, security patching and standards upgrades. Do not assume a blanket 24/7 staffing rule from this document; verify actual Authority and customer obligations.

Maintain customer identity checks, participant-registration authorisation, data protection, retention/deletion policies, access controls and auditable changes. Distinguish successful transport from recipient business acceptance.

## Security certification milestone
The Italian Peppol Authority's published notice states that all Peppol service providers must hold valid active ISO/IEC 27001 certification from 1 July 2027. Confirm the precise scope, transition milestones and any equivalence provisions with our responsible Authority. Plan for an ISMS, independent audit, recurring assurance and coverage of AP/SMP activities as applicable.

## Business case and go-live gates
Compare provider costs with membership/sign-up fees, accreditation, hosting, transport software/support, security audits, ISO certification, operations staffing and insurance/legal obligations. Obtain current quotations; no fixed cost or timeline is claimed here.

Go live only after required accreditation, production certificates, conformance evidence, security controls, tested restore, operational runbooks and tenant onboarding are complete. Review sustained customer demand and contribution margin before committing to ownership.

## Official sources
- [OpenPeppol membership](https://peppol.org/join/membership/)
- [OpenPeppol fees](https://peppol.org/join/fees/)
- [AP onboarding guide, April 2024](https://peppol.org/wp-content/uploads/2024/04/how_to_set-up_a__post-award__peppol_access_point_v2.1.pdf)
- [OpenPeppol Testbed/support](https://peppol.org/support/)
- [Access Point software resources](https://peppol.org/tools-support/links-to-software/)
- [ISO 27001 notice, Italian Peppol Authority](https://peppol.agid.gov.it/en/news/ISO-IEC-27001-Certification-Mandatory-for-Peppol-Service-Providers-from-1-July-2027/)
