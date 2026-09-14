# e-ubl21 On-Premises Licensing Guide

## Purpose

This document summarizes the licensing considerations for the e-ubl21 on-premises deployment stack.

It is intended as an engineering and product-planning guide, not as formal legal advice. Before distributing a commercial appliance or customer-installable bundle, the final dependency set and release artefacts should be reviewed for licence compliance.

---

## 1. Licensing objective

The target commercial model is:

```text
e-ubl21
commercial / proprietary licence
        |
        +--> permissively licensed infrastructure
        +--> separately licensed validation components
        +--> third-party notices and SBOM
```

The deployment should avoid unnecessary dependencies whose licences could create reciprocal source-disclosure or network-use obligations for a commercial redistribution.

The preferred policy is therefore:

```text
Prefer: Apache-2.0 / MIT / BSD / PostgreSQL-style licences
Allow with review: MPL-2.0 / EUPL-1.2
Avoid as mandatory bundled components: AGPL-3.0
```

---

## 2. Current on-premises component map

| Component | Licence | Commercial use | Main obligation / consideration |
|---|---|---:|---|
| K3s | Apache-2.0 | Yes | Preserve licence and notices when redistributing |
| Helm | Apache-2.0 | Yes | Preserve licence and notices |
| Traefik Proxy | MIT | Yes | Preserve copyright/licence notice |
| MetalLB | Apache-2.0 | Yes | Preserve licence and notices |
| cert-manager | Apache-2.0 | Yes | Preserve licence and notices |
| KEDA | Apache-2.0 | Yes | Preserve licence and notices |
| NATS Server / JetStream | Apache-2.0 | Yes | Preserve licence and notices |
| YugabyteDB | Apache-2.0 | Yes | Preserve licence and notices; this refers to YugabyteDB, not separately licensed YugabyteDB Anywhere |
| Prometheus | Apache-2.0 | Yes | Preserve licence and notices |
| OpenTelemetry Collector | Apache-2.0 | Yes | Preserve licence and notices |
| Saxon-HE | MPL-2.0 | Yes | File-level copyleft applies to modified MPL-covered files |
| EN 16931 validation artefacts | EUPL-1.2 | Yes | Preserve licence/provenance; modifications require EUPL review |
| MinIO | AGPL-3.0 | Yes, with conditions | High-priority review item for bundled commercial distribution |
| Grafana OSS | AGPL-3.0 | Yes, with conditions | High-priority review item for bundled commercial distribution |

Versions and exact licence texts must be verified against the pinned release used by each e-ubl21 release.

---

## 3. Components with low licensing risk

The following components are suitable as the default foundation for a proprietary/commercial e-ubl21 distribution:

```text
K3s
Helm
Traefik
MetalLB
cert-manager
KEDA
NATS JetStream
YugabyteDB
Prometheus
OpenTelemetry Collector
```

Their licences are permissive and do not require the e-ubl21 application source code to be disclosed merely because these components are used alongside it.

When these components are redistributed, the release package must preserve the applicable copyright notices, licence texts and any NOTICE files required by the upstream projects.

---

## 4. Saxon-HE

Saxon-HE is licensed under MPL-2.0.

MPL-2.0 is file-level copyleft. It does not normally require unrelated proprietary application code to be released simply because the application uses Saxon.

The intended architecture is:

```text
e-ubl21 worker
      |
      +--> Java runtime
      +--> Saxon-HE
      +--> validation XSLT
```

If e-ubl21 distributes an unmodified Saxon-HE binary, the distribution should include the MPL-2.0 licence and required notices.

If Saxon source files are modified and the modified binary is distributed, the modified MPL-covered files must remain available under the MPL terms.

The e-ubl21 source code remains independently licensable provided it is not itself derived from MPL-covered source files.

---

## 5. EN 16931 validation artefacts

The official EN 16931 validation artefacts used by the project are separately licensed and must retain their original licence and provenance.

The project should preserve at least:

```text
ruleset version
source URL
publisher
licence
SHA-256 digest
original filename
modification status
```

The preferred policy is to use the official validation artefacts unchanged and pin their exact version and digest.

If the project modifies the official validation rules, the modification must be reviewed against the applicable EUPL-1.2 obligations before redistribution.

Do not represent modified validation artefacts as unchanged official upstream rules.

---

## 6. UBL 2.1 schemas

UBL 2.1 schemas are authoritative OASIS standard artefacts.

The project should treat downloaded UBL XSDs as read-only validation assets:

```text
OASIS UBL 2.1 XSD
        |
        v
validation input
        |
        v
e-ubl21
```

The recommended practice is:

- retain OASIS attribution and provenance;
- record the exact UBL release used;
- retain original filenames and checksums;
- avoid modifying authoritative XSDs;
- if extensions are required, keep them clearly separate from official OASIS schemas.

---

## 7. MinIO licensing risk

MinIO is licensed under AGPL-3.0.

This does not automatically make e-ubl21 AGPL merely because e-ubl21 communicates with a separate MinIO service using the S3 protocol.

However, AGPL introduces additional obligations and commercial-distribution complexity, particularly when software is modified, bundled, redistributed, or provided as a network service.

For a customer-installable commercial e-ubl21 appliance, MinIO should therefore not be considered a neutral dependency without legal review.

### Recommended policy

MinIO should be one of the following:

1. **optional and externally supplied by the customer**;
2. replaced with a permissively licensed S3-compatible storage implementation;
3. covered by a suitable commercial licence if that becomes the preferred storage product.

The preferred architecture is to make object storage an interface/configuration choice rather than a hard dependency:

```text
e-ubl21
   |
   +--> S3-compatible API
           |
           +--> AWS S3
           +--> Azure-compatible adapter
           +--> customer-managed S3-compatible storage
           +--> approved on-prem implementation
```

---

## 8. Grafana OSS licensing risk

Grafana OSS is licensed under AGPL-3.0.

As with MinIO, simply running Grafana as an independent service does not automatically relicense the e-ubl21 application.

However, bundling Grafana into a redistributable commercial appliance creates avoidable licence-compliance work.

### Recommended policy

Grafana should be treated as:

- optional; or
- customer-supplied; or
- replaced by a permissively licensed visualization layer; or
- covered by an appropriate commercial agreement if Grafana becomes a core product feature.

Prometheus itself may remain part of the default stack because it is Apache-2.0 licensed.

---

## 9. Operating-system licensing

### Ubuntu Server

Ubuntu Server can be installed and used commercially without purchasing a per-server Ubuntu licence.

Optional commercial services such as Ubuntu Pro may be purchased independently for support, extended security maintenance, FIPS and compliance requirements.

The recommended e-ubl21 product model is that the customer provides and maintains the operating system.

### Red Hat Enterprise Linux

RHEL usage is governed by Red Hat's subscription model and customer agreements.

The e-ubl21 distribution should not bundle or sublicense RHEL.

Instead, e-ubl21 should state that RHEL is a supported host operating system where the customer maintains the required Red Hat subscriptions.

A suitable support statement is:

```text
Supported host operating systems:
- Ubuntu Server
- compatible RHEL-family Linux distributions
- Red Hat Enterprise Linux where appropriately licensed by the customer
```

---

## 10. e-ubl21 licence independence

Third-party components must remain clearly separated from the e-ubl21 application's own licence.

A commercial distribution should have a structure similar to:

```text
LICENSE
THIRD-PARTY-NOTICES.md
licenses/
    Apache-2.0.txt
    MIT.txt
    MPL-2.0.txt
    EUPL-1.2.txt
SBOM.spdx.json
```

The e-ubl21 application licence should not be inferred from the licences of separate infrastructure components.

The project may therefore use a proprietary or dual-licence commercial model provided its own source and dependencies permit that model.

---

## 11. Required release compliance artefacts

Every production release intended for redistribution should generate and retain:

### 11.1 Software Bill of Materials

Generate an SBOM in a standard machine-readable format such as:

```text
SPDX JSON
CycloneDX JSON
```

The SBOM should include:

```text
container images
Go modules
Java libraries
Helm charts
Kubernetes operators
validation artefacts
operating-system packages included in application images
```

### 11.2 Third-party notices

Generate or maintain:

```text
THIRD-PARTY-NOTICES.md
```

For every redistributed component record:

```text
name
version
source
licence
copyright
modification status
```

### 11.3 Licence texts

Preserve the complete upstream licence text for redistributed dependencies when required.

### 11.4 Source-offer obligations

For licences that require source availability for covered modifications, ensure the corresponding source is retained and made available in the manner required by the licence.

---

## 12. Pinning requirement

The current on-premises installer must evolve from floating dependencies to pinned releases before being used as a commercial release mechanism.

Do not rely on values such as:

```text
latest
unversioned Helm repository state
unversioned installer scripts
```

A commercial release should pin:

```text
K3s version
Helm version
Helm chart versions
container image tags
container image digests
Saxon version and digest
EN 16931 ruleset version and digest
UBL 2.1 asset digests
```

This is necessary for both reproducibility and licence compliance.

A release must be able to answer:

> Exactly which third-party software and licence terms were shipped to this customer?

---

## 13. Container-image policy

Every container image used by the official on-premises distribution should be approved and pinned.

For each image record:

```text
repository
version tag
immutable digest
licence
upstream source
SBOM
signature status
```

Production manifests should prefer immutable image digests:

```text
image: example/image@sha256:<digest>
```

rather than floating tags.

---

## 14. Recommended commercial dependency policy

The target dependency profile should be:

```text
                         e-ubl21
                    commercial licence
                           |
          +----------------+----------------+
          |                |                |
    Apache/MIT/BSD      MPL-2.0          EUPL-1.2
    infrastructure      Saxon-HE          EN16931
          |                |                |
       preferred        acceptable       acceptable
```

Mandatory AGPL dependencies should be avoided where practical:

```text
AGPL-3.0
   |
   +--> MinIO      avoid as mandatory bundled dependency
   +--> Grafana    avoid as mandatory bundled dependency
```

This minimizes the legal and operational surface of a proprietary customer-installable product.

---

## 15. Product architecture requirement

The on-premises design should maintain clean process and service boundaries between e-ubl21 and third-party infrastructure.

Preferred pattern:

```text
e-ubl21 API / workers
        |
        +--> YSQL/PostgreSQL wire protocol
        +--> NATS protocol
        +--> S3-compatible protocol
        +--> OpenTelemetry protocol
        +--> Prometheus metrics endpoint
```

Avoid embedding or copying third-party server source code directly into the e-ubl21 application unless there is a specific engineering requirement and the licence has been reviewed.

This separation improves:

- licence clarity;
- upgrade independence;
- customer choice;
- portability;
- security patching;
- commercial packaging.

---

## 16. Recommended actions before first commercial on-premises release

Before declaring the on-premises package commercially distributable:

1. Pin every Helm chart and container image to an exact version and digest.
2. Replace or make optional the mandatory MinIO dependency.
3. Replace, make optional, or separately license Grafana OSS.
4. Generate an SBOM for every application image and the complete deployment bundle.
5. Create `THIRD-PARTY-NOTICES.md` automatically from the pinned dependency inventory.
6. Archive all applicable third-party licence texts with each release.
7. Record exact UBL, EN 16931, Saxon and XML Resolver versions and hashes.
8. Record whether any third-party component has been modified.
9. Perform a licence-compliance review before publishing or delivering the first commercial bundle.
10. Repeat the licence scan for every release because dependencies and upstream licence terms can change.

---

## 17. Summary

The core on-premises e-ubl21 architecture is compatible with a proprietary/commercial e-ubl21 licence.

Most of the platform uses permissive open-source licences and does not impose source-disclosure obligations on e-ubl21 itself.

The primary components requiring special treatment are:

```text
MinIO       AGPL-3.0
Grafana OSS AGPL-3.0
Saxon-HE    MPL-2.0
EN 16931    EUPL-1.2
RHEL        customer subscription model
```

The recommended commercial posture is:

```text
permissive infrastructure
+
clearly separated MPL/EUPL validation assets
+
no mandatory AGPL components
+
pinned dependencies
+
SBOM
+
third-party notices
```

This keeps the on-premises deployment technically portable while preserving a clean commercial licensing boundary around e-ubl21.
