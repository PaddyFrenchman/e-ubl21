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
        +--> CloudNativePG / PostgreSQL
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
sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

The installer can also stop after installing only K3s and Helm:

```bash
INSTALL_PLATFORM=0 sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

## Important limitations

This is a **single-server** architecture. It provides Kubernetes deployment portability but does not provide physical high availability. Failure of the host stops all local workloads.

PostgreSQL, NATS JetStream, object-storage and monitoring data must therefore have off-machine backups before this setup is treated as a serious production installation.

The current repository does not yet contain the final common production Helm chart for e-ubl21. The existing `deploy/minikube/` definitions are regression/test infrastructure and must not be promoted directly into the on-premises production model.

## Related documentation

The project-wide cloud deployment target is documented at:

```text
deployment-target.md
```

The local regression Kubernetes environment remains documented separately at:

```text
deploy/minikube/
```
