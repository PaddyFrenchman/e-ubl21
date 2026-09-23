# Ideas catalogue

This directory contains proposals that may shape e-ubl21 after the current Version 1 release boundary. Placement indicates lifecycle and ownership; it does not by itself approve implementation.

## Lifecycle

- **Active plan** — accepted work currently tracked by implementation issues.
- **Product direction** — candidate product capability requiring prioritization and issue decomposition.
- **Architecture candidate** — technical option that requires an architectural decision before implementation.
- **Reference** — explanatory or standards material, not an implementation backlog.
- **Implemented archive** — retained decision and implementation history for completed work.
- **Incubator** — valuable concept retained in this repository but outside the e-ubl21 product roadmap.
- **Handoff** — concept whose implementation belongs to a related repository.

## One-line overview

| Status | Document | One-line overview | Next action |
| --- | --- | --- | --- |
| Architecture candidate | [AWS Kubernetes scalability](architecture/aws-kubernetes-scalability.md) | Enterprise EKS scalability, multi-tenancy, autoscaling, resilience and observability model. | Reconcile with YugabyteDB, Pulumi/Go and the current deployment target before adoption. |
| Architecture candidate | [AWS deployment](architecture/deployment-on-aws.md) | Reproducible AWS/EKS deployment architecture covering IaC, Helm, GitOps, autoscaling and operations. | Consolidate with the scalability document and resolve obsolete OpenTofu/PostgreSQL assumptions. |
| Architecture candidate | [Message-bus abstraction](architecture/message-bus-abstraction.md) | Narrow Go abstraction for NATS/JetStream, Kafka, RabbitMQ and in-memory test adapters. | Keep deferred until a concrete second-broker requirement justifies implementation. |
| Active plan | [PostgreSQL-to-YugabyteDB migration](active-plans/postgresql-to-yugabytedb-migration-plan.md) | Issue-by-issue migration from PostgreSQL/CloudNativePG assumptions to YugabyteDB. | Complete #129, #149 and #130, then move this plan to the implemented archive. |
| Product direction | [Invoice lifecycle platform](product/invoice-lifecycle-platform.md) | Evolves validation into conversion, routing, exchange, reconciliation and later payment infrastructure. | Use as the principal long-term product vision and split accepted phases into roadmap issues. |
| Product direction | [Third-party integration](product/third-party-integration.md) | Maps Sage, Xero, QuickBooks, SAP, NetSuite and custom inputs through a canonical billing model. | Prioritize a canonical-model foundation and one narrowly scoped adapter proof of concept. |
| Product direction | [Client and party master data](product/yugabytedb-client-party-master-data-schema.md) | Defines owner-scoped clients, parties, integrations, provenance and authorization in YugabyteDB. | Convert into an implementation plan after the database migration release gate is complete. |
| Reference | [UBL 2.1, EN 16931 and Peppol overview](reference/ubl21-en16931-peppol-overview.md) | Explains how structural UBL validation, EN semantics and Peppol constraints form layered validation. | Maintain as product documentation and keep aligned with supported profiles. |
| Reference | [Peppol BIS Billing specification](reference/peppol-bis-billing-spec.md) | Records the pinned Peppol release, identifiers, rule artefacts, code lists and official sources. | Review and update whenever the supported Peppol ruleset changes. |
| Implemented archive | [EN 16931 validation roadmap](archive/implemented/en16931-validation-implementation-roadmap.md) | Historical design and phased implementation plan for the Schematron validation stage. | Retain as architectural history; current behavior is authoritative in code and active documentation. |
| Implemented archive | [EN 16931 sample-generator plan](archive/implemented/en16931-samples-generator-plan.md) | Historical plan for deterministic positive and rule-targeted negative EN 16931 corpora. | Retain as implementation history; active corpus contracts live with the regression tooling. |
| Implemented archive | [Podman-to-Docker conversion](archive/implemented/podman-to-docker-conversion.md) | Completed workstation/runtime migration plan tracked by issue #133. | No further action unless the container-runtime policy changes. |
| Incubator | [SiteSentinel micro-SaaS](incubator/sitesentinel-micro-saas.md) | Independent agency-focused website health, accessibility, DNS, TLS and reporting SaaS concept. | Preserve for possible future extraction into its own repository; it is not an e-ubl21 feature. |
| Handoff | [Meshery multi-cluster control plane](handoffs/e-ubl21-console/layer5-meshery-multicluster-control-plane.md) | Visual multi-cluster operations concept using Meshery/Kanvas above the GitOps source of truth. | Continue only in the separate e-ubl21-console lifecycle. |

## Highest-value post-V1 capability sequence

1. Establish the canonical invoice business model and generation API.
2. Implement client and party master data in YugabyteDB.
3. Prove one third-party adapter end to end.
4. Add bidirectional conversion and profile-aware enrichment.
5. Add routing, delivery and inbound exchange while keeping validation state separate.
6. Extend to national profiles, reconciliation and payment workflows only after the exchange core is stable.

## Catalogue rules

1. Every new idea must declare its lifecycle status, product owner and related GitHub issue when one exists.
2. Accepted implementation work moves to `active-plans/`; completed plans move to `archive/implemented/`.
3. Standards references belong in `reference/` and must state their checked or pinned version.
4. Concepts outside the e-ubl21 roadmap belong in `incubator/`, not mixed with the product backlog.
5. Cross-repository concepts belong in `handoffs/<repository>/`.
6. Runtime and released behavior is defined by code, tests, ADRs and active documentation—not by archived idea documents.
