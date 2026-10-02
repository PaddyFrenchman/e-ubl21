# ISO/IEC 27001 certification for Peppol operations
Date: 2026-10-02
Status: certification preparation plan; no certification is claimed.

## Purpose
Define the organisational information-security certification work required for a future self-operated Peppol Access Point. This document is separate from [Peppol accreditation](PEPPOL-CERTIFICATION.md) and [provider-based routing](PEPPOL-ROUTE.md).

ISO/IEC 27001 certifies an organisation's information security management system (ISMS) within a defined scope. It does not certify UBL21 source code or replace Peppol technical conformance tests and service-provider accreditation.

## Verified Peppol requirement
The Italian Peppol Authority's notice dated 8 July 2026 states that all Peppol service providers must hold valid active ISO/IEC 27001 certification from **1 July 2027**. It requires the Statement of Applicability to cover relevant Peppol operations and directs organisations without certification to engage an accredited certification body.

The published transition schedule includes:
| Date | Milestone |
| --- | --- |
| 30 June 2026 | Equivalence requests for certificates already held |
| 1 September 2026 | Submission of existing ISO certificates |
| 1 October 2026 | Evidence of ongoing certification projects |
| 1 February 2027 | First progress report |
| 1 May 2027 | Second progress report |
| 1 July 2027 | Certification achievement deadline |

The first three dates have passed as of this document. These are published transition milestones, not an assumption that a new applicant has missed its own admission deadline. Ask the responsible Peppol Authority how onboarding and reporting apply to a new candidate, and confirm current standard edition, scope and accepted certification arrangements.

## Scope to agree with the certification body
Define the legal entity, teams, locations, infrastructure, suppliers and service boundaries. Include the router, AP transport, UBL21 dependencies used in service delivery, production hosting and operational support where relevant.

Cover:
- Access Point operations and SMP services if operated.
- Customer identification and participant-registration authorisation.
- Document handling, confidentiality, integrity and retention.
- Identity/access controls, tenant isolation and privileged administration.
- Certificates, cryptographic keys, secrets and rotation.
- Logging, audit evidence and monitoring.
- Backups, disaster recovery and business continuity.
- Software change management and vulnerability remediation.
- Suppliers, outsourced infrastructure and incident response.

Outsourcing hosting or transport does not by itself establish certification or remove our responsibilities. If using a provider rather than operating an AP, verify which obligations apply to us under that contract; the AP requirement should not automatically be represented as mandatory for every software customer.

## Preparation and certification work
1. Appoint accountable management, an ISMS owner and a budget.
2. Agree scope and conduct a gap assessment with a qualified adviser or certification body.
3. Inventory assets, information flows, dependencies and suppliers.
4. Assess security risks; record risk owners, treatment plans and accepted residual risks.
5. Prepare the Statement of Applicability with justified control selections and exclusions.
6. Implement documented policies, access controls, supplier controls, incident procedures, secure development and continuity arrangements.
7. Operate the controls and retain evidence: access reviews, training, change records, incident exercises, vulnerability remediation and restore tests.
8. Conduct internal audits and management review; resolve identified nonconformities.
9. Undergo the certification body's stage-one and stage-two audits and close required findings.
10. Maintain the ISMS through recurring surveillance, continual improvement and recertification under the body's agreed schedule.

A written policy alone is insufficient: auditors need evidence that controls operate in the agreed scope.

## Deliverables
- Approved scope and security policy.
- Asset/supplier inventory and information-flow descriptions.
- Risk assessment, treatment plan and Statement of Applicability.
- Roles, training and access-review records.
- Incident, backup, recovery and continuity procedures with exercise evidence.
- Secure-development and change-management evidence.
- Internal-audit and management-review records.
- Corrective-action register.
- Valid certificate whose scope matches the Peppol service.
- Authority submissions and ongoing compliance calendar.

## Responsibilities and go-live gates
Management owns scope, resources and risk acceptance. Engineering implements and evidences technical controls. Operations owns monitoring, recovery and incident handling. The ISMS owner coordinates records, audit preparation and improvement. The independent certification body assesses conformity.

Before a self-operated AP launch, obtain the responsible Authority's approval, required Peppol conformance results and production certificates, plus the ISO certification evidence applicable to the launch date. These are separate gates.

## Cost and schedule
Obtain quotations based on entity size, service scope, locations, existing controls and audit effort. Budget internal staff time, independent certification, optional advisory help and recurring maintenance. No fixed cost or guaranteed completion time is established.

## Authoritative source
[Italian Peppol Authority: ISO/IEC 27001 mandatory from 1 July 2027](https://peppol.agid.gov.it/en/news/ISO-IEC-27001-Certification-Mandatory-for-Peppol-Service-Providers-from-1-July-2027/)

The notice links OpenPeppol's dedicated guidance and implementation plan. Confirm applicant-specific requirements directly with the responsible Authority and certification body.
