# e-ubl21 On-Premises Deployment

## Purpose

This document defines the target architecture and installation procedure for running e-ubl21 on a local physical or virtual Linux server instead of AWS or Azure.

The supported baseline is a single server running either:

- Ubuntu Server; or
- Red Hat Enterprise Linux and compatible RHEL-family distributions.

For the initial single-server target, the deployment standard is:

```text
Linux host
   |
   v
K3s
   |
   +--> Traefik ingress
   +--> MetalLB
   +--> cert-manager
   +--> KEDA
   +--> e-ubl21 API
   +--> e-ubl21 workers
   +--> NATS JetStream
   +--> YugabyteDB / YSQL
   +--> SeaweedFS S3-compatible object storage
   +--> Prometheus
   +--> Grafana
   +--> OpenTelemetry Collector
```

The goal is to preserve the same Kubernetes application model used by the cloud targets. The infrastructure beneath Kubernetes changes, but the application containers and future common Helm chart should remain portable.

---

## 1. Recommended server specification

### Minimum development / functional test host

```text
CPU:       4 cores
RAM:       8 GB
Storage:   50 GB SSD
Network:   1 Gbit/s recommended
```

This is enough for a small API + worker + NATS test environment, but leaves little headroom for YugabyteDB, object storage and observability.

### Recommended standalone installation

```text
CPU:       8 cores
RAM:       16 GB
Storage:   250 GB SSD/NVMe
Network:   1 Gbit/s or better
```

### Preferred serious local installation

```text
CPU:       16 cores
RAM:       32 GB
Storage:   500 GB - 1 TB NVMe
Network:   1 Gbit/s or better
```

The validation worker is the most memory-intensive e-ubl21 workload because it runs the Java/Saxon EN 16931 validation runtime. Multiple workers require additional RAM.

A single physical host is not highly available. If the server fails, all workloads on that server stop. Production data must therefore be backed up to another machine or storage system.

---

## 2. Supported operating systems

The installation kit standardizes on **K3s** for the first single-node on-premises target.

K3s supports common Linux distributions including Ubuntu/Debian and RHEL/CentOS/Fedora-family systems.

Official K3s documentation:

- https://docs.k3s.io/quick-start
- https://docs.k3s.io/installation/requirements

For a larger RHEL-centered enterprise installation, **RKE2** is an alternative:

- https://docs.rke2.io/install/quickstart
- https://get.rke2.io

RKE2 is not required for the initial single-server e-ubl21 target.

---

## 3. Cloud-to-on-premises service mapping

| Capability | AWS | Azure | On-premises |
|---|---|---|---|
| Kubernetes | EKS | AKS | K3s |
| Container registry | ECR | ACR | GHCR, Harbor, or local image import |
| Load balancer | AWS Load Balancer | Azure Load Balancer | MetalLB |
| Ingress | ALB / ingress controller | Application Gateway / ingress | Traefik |
| Messaging | SQS / NATS | Service Bus / NATS | NATS JetStream |
| Database | YugabyteDB/YSQL, three-AZ RF-3 production topology | YugabyteDB/YSQL, three-AZ RF-3 production topology | Single-node YugabyteDB/YSQL baseline; three-rack RF-3 for HA |
| Object storage | S3 | Blob Storage | SeaweedFS S3-compatible storage (MinIO bootstrap remains development-only) |
| Secrets | Secrets Manager | Key Vault | Kubernetes Secrets initially; Vault/SOPS later |
| Metrics | Managed Prometheus | Azure Monitor / Managed Prometheus | Prometheus |
| Dashboards | Managed Grafana | Managed Grafana | Grafana |
| Tracing | OpenTelemetry | OpenTelemetry | OpenTelemetry Collector |
| Pod autoscaling | KEDA | KEDA | KEDA |
| Node autoscaling | Karpenter | AKS autoscaler | Not applicable to a single physical host |
| TLS certificates | ACM | Azure certificate services | cert-manager |

---

## 4. Required downloads and repositories

The installation script uses only official or project-maintained distribution points.

### 4.1 Operating-system packages

Installed from the host distribution repositories:

```text
curl
ca-certificates
git
jq
openssl
tar
gzip
```

RHEL 10 additionally requires:

```text
kernel-modules-extra
```

### 4.2 K3s

Installer:

```text
https://get.k3s.io
```

Documentation:

```text
https://docs.k3s.io/
```

The K3s installation also provides Kubernetes runtime components and command-line tools including `kubectl`, `crictl` and `ctr`.

### 4.3 Helm

Official installer:

```text
https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
```

Documentation:

```text
https://helm.sh/docs/
```

### 4.4 MetalLB

Helm repository:

```text
https://metallb.github.io/metallb
```

Documentation:

```text
https://metallb.io/
```

### 4.5 KEDA

Helm repository:

```text
https://kedacore.github.io/charts
```

Documentation:

```text
https://keda.sh/docs/
```

### 4.6 cert-manager

Official OCI Helm chart:

```text
oci://quay.io/jetstack/charts/cert-manager
```

Documentation:

```text
https://cert-manager.io/docs/
```

### 4.7 NATS

Official NATS Kubernetes Helm repository:

```text
https://nats-io.github.io/k8s/helm/charts/
```

Chart:

```text
nats/nats
```

Source/documentation:

```text
https://github.com/nats-io/k8s
https://docs.nats.io/
```

### 4.8 YugabyteDB

Helm repository:

```text
https://charts.yugabyte.com
```

Chart:

```text
yugabytedb/yugabyte
```

Documentation:

```text
https://docs.yugabyte.com/stable/deploy/kubernetes/single-zone/oss/helm-chart/
```

The installer pins chart `2025.2.6` and image
`yugabytedb/yugabyte:2025.2.6.0-b111`. It deploys one YB-Master and one
YB-TServer for the non-HA single-server baseline. The production HA and
multi-region contract is documented in
[`YUGABYTEDB-HA-ARCHITECTURE.md`](../../docs/YUGABYTEDB-HA-ARCHITECTURE.md).

### 4.9 Prometheus and Grafana

Prometheus Community Helm repository:

```text
https://prometheus-community.github.io/helm-charts
```

Chart:

```text
prometheus-community/kube-prometheus-stack
```

This stack supplies Prometheus Operator, Prometheus, Alertmanager, Kubernetes monitoring components and Grafana by default.

### 4.10 OpenTelemetry Collector

Helm repository:

```text
https://open-telemetry.github.io/opentelemetry-helm-charts
```

Chart:

```text
open-telemetry/opentelemetry-collector
```

Documentation:

```text
https://opentelemetry.io/docs/platforms/kubernetes/helm/
```

### 4.11 MinIO-compatible object storage

SeaweedFS is the recommended S3-compatible store for production-grade
on-premises deployments. Configure e-ubl21 with `objectstore-backend=s3` and
the SeaweedFS endpoint, bucket, TLS, and credentials; the application has no
SeaweedFS-specific integration. See
[`docs/OBJECT-STORAGE.md`](../../docs/OBJECT-STORAGE.md) for the configuration,
retention, migration, and recovery contract.

For the initial single-server development target the bootstrap script can
deploy a simple single-node MinIO-compatible service using the MinIO container
image. It is not the recommended production on-prem object-store profile.

Image source:

```text
quay.io/minio/minio
```

For a multi-node or production object-storage installation, use the official MinIO Operator instead:

```text
https://operator.min.io
```

A single-node MinIO deployment is not storage HA and must be backed up externally.

### 4.12 e-ubl21 application image

The e-ubl21 image can be supplied in one of three ways:

1. pull it from a private/public OCI registry such as GHCR;
2. use an on-premises Harbor registry; or
3. build it locally and import it directly into K3s/containerd.

For a locally built image:

```bash
docker save e-ubl21:local -o e-ubl21.tar
sudo k3s ctr images import e-ubl21.tar
```

The final production deployment should use immutable tags or image digests rather than `latest`.

---

## 5. Pre-installation checklist

Before running the installer, verify the machine has:

- a fixed hostname;
- a static LAN IP address or DHCP reservation;
- correct DNS resolution;
- working Internet access for the initial downloads;
- synchronized time using NTP/systemd-timesyncd/chrony;
- enough free disk space;
- SSH access tested from another machine;
- BIOS/firmware virtualization enabled if the host itself is a VM platform;
- backups of any existing data before repartitioning or repurposing the machine.

Example host identity:

```text
hostname: eubl21-server
LAN IP:   192.168.1.50
```

Do not use a frequently changing DHCP address for a permanent installation.

---

## 6. Network planning

### 6.1 Host address

Give the server a stable address, for example:

```text
192.168.1.50
```

### 6.2 MetalLB address pool

Reserve one or more unused LAN addresses outside the DHCP allocation range, for example:

```text
192.168.1.200-192.168.1.220
```

Do not configure a MetalLB address pool until those addresses have been reserved and verified unused.

The installation script accepts:

```bash
METALLB_ADDRESS_POOL=192.168.1.200-192.168.1.220
```

If no pool is provided, MetalLB is installed but left unconfigured.

### 6.3 Kubernetes internal networks

K3s defaults include the following internal networks:

```text
Pod CIDR:      10.42.0.0/16
Service CIDR:  10.43.0.0/16
```

Ensure these networks do not overlap with the physical LAN, VPNs or corporate routes.

### 6.4 Firewall

At minimum, plan for:

```text
22/tcp      SSH administration
80/tcp      HTTP ingress if used
443/tcp     HTTPS ingress
6443/tcp    Kubernetes API, preferably restricted to administration networks
```

On RHEL systems using `firewalld`, K3s also requires the pod and service CIDRs to be allowed/trusted according to the K3s requirements documentation.

Do not expose the Kubernetes API (`6443`) directly to the public Internet.

---

## 7. Storage planning

K3s includes the local-path storage provisioner. This is suitable for a single-node deployment and creates PersistentVolumes on the local server.

Persistent data will include at least:

```text
NATS JetStream
YugabyteDB master and tablet-server data
MinIO/object storage
Prometheus
Grafana state
```

For the first host, use SSD/NVMe storage. If possible, keep application data on a separate filesystem or physical device from the OS.

Suggested capacity allocation for a 500 GB machine:

```text
OS + K3s/container images:    80 GB
YugabyteDB:                 110 GB
Object storage:              200 GB
NATS JetStream:               30 GB
Prometheus/Grafana:           50 GB
Free/reserve:                 40 GB
```

Exact sizes depend on retained document volume, validation throughput and monitoring retention.

A future multi-node deployment should move persistent workloads to replicated storage such as Longhorn, Ceph, SAN/NAS-backed storage, or another suitable CSI provider.

---

## 8. Machine installation procedure

### Step 1 - Install the operating system

Install a minimal supported server OS.

Recommended starting points:

```text
Ubuntu Server LTS
or
RHEL / compatible RHEL-family server
```

During OS installation:

- configure the primary NIC;
- set the hostname;
- enable SSH;
- create a named administrator account;
- use SSH keys where possible;
- apply all OS security updates.

### Step 2 - Update the host

Ubuntu:

```bash
sudo apt-get update
sudo apt-get upgrade -y
sudo reboot
```

RHEL-family:

```bash
sudo dnf upgrade -y
sudo reboot
```

### Step 3 - Clone e-ubl21

```bash
git clone <e-ubl21-repository-url>
cd e-ubl21
```

### Step 4 - Review the installer

Never execute a privileged installation script without reading it first.

```bash
less deploy/on-prem/scripts/install-on-prem.sh
```

The installer downloads the official K3s and Helm installer scripts to temporary files before executing them.

### Step 5 - Choose the MetalLB pool

If the server must expose Kubernetes `LoadBalancer` services on the LAN, reserve an address pool and pass it to the installer.

Example:

```bash
export METALLB_ADDRESS_POOL=192.168.1.200-192.168.1.220
```

If you do not yet know the safe range, omit this variable. The pool can be configured later.

### Step 6 - Run the installation

```bash
sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

The default run performs the following operations:

1. validates the Linux distribution;
2. installs base OS tools;
3. disables swap;
4. installs K3s;
5. disables K3s ServiceLB so MetalLB can own `LoadBalancer` addresses;
6. installs Helm;
7. creates the `e-ubl21` namespace;
8. installs MetalLB;
9. optionally configures the MetalLB address pool;
10. installs cert-manager;
11. installs KEDA;
12. installs one YB-Master and one YB-TServer from the pinned YugabyteDB chart and image;
13. creates the authenticated `eubl21` YSQL user and database;
14. verifies the exact YugabyteDB version and authenticated YSQL usability;
15. runs the embedded e-ubl21 schema migrations from `EUBL21_IMAGE`;
16. installs NATS with JetStream and persistent storage;
17. deploys a single-node MinIO-compatible object store;
18. installs kube-prometheus-stack, including Prometheus and Grafana;
19. installs an OpenTelemetry Collector;
20. records installed component information under `/var/lib/e-ubl21-onprem/`.

The script is designed to be rerunnable. It uses `helm upgrade --install` and preserves generated Kubernetes secrets when they already exist.

The current installer stops at platform provisioning: the repository does not
yet contain the final common production Helm chart for the e-ubl21 API and
workers. The disposable on-prem qualification therefore proves YugabyteDB
readiness, the application identity/database, migrations and idempotency, while
the repository's existing `./run-regression-tests.sh` Minikube deployment
provides the API health and UBL 2.1/EN 16931 product-regression gate. This is an
explicit issue #123 scope limitation, not evidence that the application has
been deployed into the on-prem qualification namespace. The future production
chart must add that connected post-install gate before it is supported.

### Step 7 - Verify Kubernetes

```bash
kubectl get nodes -o wide
kubectl get pods -A
kubectl get svc -A
kubectl get pvc -A
```

Expected node state:

```text
Ready
```

### Step 8 - Verify NATS

```bash
kubectl get pods -n e-ubl21
kubectl get pvc -n e-ubl21
```

NATS must have JetStream enabled and its file store backed by a PVC.
Set the NATS server/account `max_payload` to at least `2097152` bytes so every
V1 validation `StageResult`, including worst-case JSON-escaped diagnostics,
fits the normative 2 MiB result stream ceiling.

### Step 9 - Verify YugabyteDB/YSQL

```bash
kubectl get statefulset yb-master yb-tserver -n e-ubl21
kubectl get job eubl21-yugabytedb-bootstrap eubl21-database-migrate -n e-ubl21
kubectl logs job/eubl21-yugabytedb-bootstrap -n e-ubl21
kubectl logs job/eubl21-database-migrate -n e-ubl21
```

The bootstrap Job checks the pinned YugabyteDB version, then creates or updates
the following YSQL identity without granting superuser privileges:

```text
database: eubl21
user:     eubl21
```

Database credentials remain in the generic `eubl21-db-auth` Kubernetes Secret
and are not printed to the console. The installer preserves an existing
`eubl21` application username/password pair and the separate
`eubl21_migrator` credential. The migration Job uses `migration-database-url`,
not the application `database-url`. If a legacy `e-ubl21-postgres` resource exists, the fresh
installer stops without deleting it. Existing installations must use the
guarded procedure in
[`ON-PREM-POSTGRESQL-TO-YUGABYTEDB-UPGRADE.md`](../../docs/ON-PREM-POSTGRESQL-TO-YUGABYTEDB-UPGRADE.md);
the old cluster is never retired as part of YugabyteDB installation.

Retrieve them only when required:

```bash
kubectl get secret eubl21-db-auth -n e-ubl21 -o jsonpath='{.data.username}' | base64 -d; echo
kubectl get secret eubl21-db-auth -n e-ubl21 -o jsonpath='{.data.password}' | base64 -d; echo
```

### Step 10 - Verify object storage

```bash
kubectl get statefulset,svc,pvc -n e-ubl21 | grep minio
```

The bootstrap MinIO service is internal to Kubernetes by default. Do not expose its administrative console publicly. Production SeaweedFS administration endpoints must likewise remain private.

### Step 11 - Verify monitoring

```bash
kubectl get pods -n monitoring
```

For local administration, Grafana can initially be accessed with port forwarding rather than exposing it publicly:

```bash
kubectl port-forward -n monitoring svc/monitoring-grafana 3000:80
```

Then browse to:

```text
http://127.0.0.1:3000
```

### Step 12 - Verify OpenTelemetry

```bash
kubectl get pods -n observability
```

The initial collector installation establishes the collector runtime. Export pipelines must be aligned with the final e-ubl21 telemetry configuration before production use.

---

## 9. Deploying the e-ubl21 application

The Kubernetes infrastructure should not contain cloud-specific application assumptions.

The target application packaging remains:

```text
same e-ubl21 OCI image
       +
same e-ubl21 Helm chart
       |
       +--> AWS / EKS
       +--> Azure / AKS
       +--> local / K3s
```

The repository does not currently contain the final common production Helm chart. Until that chart is implemented, do not treat the test-only Minikube manifests as the production on-premises deployment definition.

The production Helm chart should deploy at least:

```text
e-ubl21 API Deployment
e-ubl21 worker Deployment
Services
Ingress
ConfigMaps
Secrets references
KEDA ScaledObjects
PodDisruptionBudgets where applicable
NetworkPolicies
ServiceAccounts
OpenTelemetry configuration
```

The application image should be supplied using an immutable version or digest.

For a locally built image on the server:

```bash
docker build -t e-ubl21:local .
docker save e-ubl21:local -o /tmp/e-ubl21.tar
sudo k3s ctr images import /tmp/e-ubl21.tar
```

A future Helm deployment can then use:

```text
image: e-ubl21:local
imagePullPolicy: IfNotPresent
```

For repeatable production deployment, a registry such as GHCR or Harbor is preferable.

---

## 10. TLS and DNS

For LAN-only deployment, create an internal DNS record such as:

```text
eubl21.internal.example -> MetalLB ingress address
```

For an Internet-facing deployment:

```text
public DNS
   |
   v
router/firewall NAT
   |
   v
MetalLB / Traefik
   |
   v
e-ubl21 Ingress
```

Use cert-manager for certificate lifecycle management.

Do not create a Let's Encrypt issuer until the required DNS name and public reachability are confirmed.

---

## 11. Backups

A single server is a single failure domain.

Minimum backup policy:

- YugabyteDB backups copied off-host using a procedure qualified by issue #128;
- object-storage data copied or replicated off-host;
- e-ubl21 configuration and Helm values retained in Git;
- Kubernetes Secrets backed up securely or generated from an external secret-management system;
- NATS JetStream data backed up if queued work cannot simply be regenerated;
- periodic full-host recovery testing.

Do not consider a second disk in the same server to be a complete backup.

---

## 12. Security baseline

For a serious installation:

- use SSH keys;
- disable direct root SSH login;
- restrict the Kubernetes API to trusted management networks;
- expose only required ingress ports;
- keep the OS, K3s and Helm-managed components patched;
- pin container images by version/digest;
- use Kubernetes NetworkPolicies;
- move long-lived secrets to Vault/SOPS/external secret management when the deployment matures;
- do not expose Grafana, MinIO Console, YSQL or NATS directly to the Internet;
- use TLS for public and cross-network traffic;
- audit backup restoration regularly.

---

## 13. Scaling model

On a single physical server, KEDA can scale worker pod count according to NATS backlog, but it cannot create additional CPU or RAM.

```text
NATS backlog
    |
    v
KEDA
    |
    v
worker replicas
    |
    v
bounded by physical server resources
```

Node autoscaling such as Karpenter is not meaningful on a one-node bare-metal installation.

When more capacity is required, add Kubernetes nodes and migrate the single-node design toward a proper multi-node cluster with replicated storage and redundant ingress.

---

## 14. Target operating model

The desired deployment matrix is:

```text
AWS
Pulumi + Go + EKS + Helm

Azure
Pulumi + Go + AKS + Helm

On-premises Ubuntu / RHEL
Linux bootstrap + K3s + Helm
```

The portability boundary is Kubernetes:

```text
                e-ubl21 source
                     |
                     v
                 OCI image
                     +
                 Helm chart
                     |
          +----------+-----------+
          |          |           |
          v          v           v
         EKS        AKS         K3s
          |          |           |
          v          v           v
         AWS       Azure      On-prem
```

The application should remain unaware of the infrastructure provider whenever practical.

---

## 15. Installation script

The machine bootstrap script is stored at:

```text
deploy/on-prem/scripts/install-on-prem.sh
```

Usage:

```bash
sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

The platform installation requires an immutable e-ubl21 application image so
the schema migration Job can run. The image must be pullable by K3s or imported
into its containerd image store before installation.

Useful overrides:

```bash
METALLB_ADDRESS_POOL=192.168.1.200-192.168.1.220 \
NATS_STORAGE_SIZE=20Gi \
YB_MASTER_STORAGE_SIZE=10Gi \
YB_TSERVER_STORAGE_SIZE=50Gi \
YB_STORAGE_CLASS=local-path \
MINIO_STORAGE_SIZE=100Gi \
EUBL21_IMAGE=registry.example/e-ubl21@sha256:REPLACE_ME \
sudo -E bash deploy/on-prem/scripts/install-on-prem.sh
```

Review the script and this document before using it on the target machine.
