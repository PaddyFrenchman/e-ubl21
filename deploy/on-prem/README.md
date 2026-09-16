# On-Premises Deployment

This directory contains the e-ubl21 single-server on-premises deployment documentation and machine bootstrap tooling.

The initial target is a physical or virtual server running Ubuntu Server or a RHEL-family distribution, with K3s providing the Kubernetes layer.

## Directory contents

```text
deploy/on-prem/
|
+-- README.md
|   This file. Describes the purpose and contents of the on-premises deployment kit.
|
+-- deployment-on-prem.md
|   Architecture, prerequisites, required downloads, hardware/network/storage planning,
|   installation manual, verification procedure, security, backups and scaling guidance.
|
+-- yugabytedb-values.yaml
|   Version-controlled single-server values for the pinned official YugabyteDB chart.
|
+-- scripts/
    |
    +-- install-on-prem.sh
        Privileged bootstrap script for preparing the Linux host and installing the
        Kubernetes platform services used by e-ubl21.
```

## Target stack

```text
Ubuntu / RHEL-family Linux
        |
        v
       K3s
        |
        +--> Traefik
        +--> MetalLB
        +--> cert-manager
        +--> KEDA
        +--> NATS JetStream
        +--> YugabyteDB / YSQL
        +--> MinIO-compatible object storage
        +--> Prometheus
        +--> Grafana
        +--> OpenTelemetry Collector
        +--> e-ubl21 API and workers
```

The e-ubl21 application layer should ultimately use the same OCI images and common Helm chart on AWS/EKS, Azure/AKS and on-premises/K3s.

## Start here

Read the complete manual first:

```text
deploy/on-prem/deployment-on-prem.md
```

Then review the installation script:

```bash
less deploy/on-prem/scripts/install-on-prem.sh
```

Run it only after the server's static IP, storage and MetalLB address range have been planned.

Example:

```bash
METALLB_ADDRESS_POOL=192.168.1.200-192.168.1.220 \
EUBL21_IMAGE=registry.example/e-ubl21@sha256:REPLACE_ME \
sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

`EUBL21_IMAGE` is required because the installer runs the embedded schema
migrations after authenticated YSQL readiness. Supply an immutable registry
digest or an image already imported into K3s.

The installer can also stop after installing only K3s and Helm:

```bash
INSTALL_PLATFORM=0 sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

Validate the chart and installer-owned Jobs statically, or run the clean
two-pass test against the current disposable Kubernetes context:

```bash
make on-prem-database-test
EUBL21_TEST_IMAGE=e-ubl21:local make on-prem-database-integration
make on-prem-database-upgrade-test
```

The integration target owns and removes a uniquely named disposable namespace.
If `EUBL21_TEST_NAMESPACE` names an existing namespace, namespace creation fails
and the existing namespace is left untouched.

### Product-regression scope

This installer currently provisions the on-prem platform services; it does not
deploy the e-ubl21 API or workers because the repository does not yet contain
the final common production Helm chart. Consequently,
`on-prem-database-integration` qualifies YugabyteDB readiness, identity/schema
bootstrap, migrations and rerun safety, but cannot perform an in-namespace
post-install product health check. For issue #123 this known limitation is
paired with the existing deployed-product gate, `./run-regression-tests.sh`,
which exercises API health and the applicable UBL 2.1 and EN 16931 validation
suites in the repository's supported Minikube regression deployment. A future
production chart must add the equivalent health/regression gate against the
on-prem YugabyteDB connection before that chart is declared supported.

## Existing CloudNativePG installations

Do not run the fresh installer through its legacy-deployment safety check.
Follow
[`ON-PREM-POSTGRESQL-TO-YUGABYTEDB-UPGRADE.md`](../../docs/ON-PREM-POSTGRESQL-TO-YUGABYTEDB-UPGRADE.md)
and use `scripts/upgrade-postgresql-to-yugabytedb.sh`. The guarded path inspects
and preserves the old database, installs YugabyteDB without retiring the
source, and separates destructive retirement behind backup verification and
exact operator confirmation.

## Important limitations

This is a **single-server** architecture. It provides Kubernetes deployment portability but does not provide physical high availability. Failure of the host stops all local workloads.

YugabyteDB, NATS JetStream, object-storage and monitoring data must therefore have off-machine backups before this setup is treated as a serious production installation.

The current repository does not yet contain the final common production Helm chart for e-ubl21. The existing `deploy/minikube/` definitions are regression/test infrastructure and must not be promoted directly into the on-premises production model.

## Related documentation

The guarded legacy CloudNativePG upgrade and retirement procedure is:

```text
docs/ON-PREM-POSTGRESQL-TO-YUGABYTEDB-UPGRADE.md
```

The project-wide cloud deployment target is documented at:

```text
deployment-target.md
```

The local regression Kubernetes environment remains documented separately at:

```text
deploy/minikube/
```
