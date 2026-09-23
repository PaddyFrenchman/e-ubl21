# Deployment Target

## Objective

Define the target automation stacks for deploying e-ubl21 on managed Kubernetes in the major cloud environments.

The initial targets are:

- **AWS**, using Amazon EKS;
- **Microsoft Azure**, using Azure Kubernetes Service (AKS).

The deployment approach should be reproducible, version-controlled, automatable, portable where practical, and suitable for both local execution and CI/CD pipelines.

# AWS Deployment Target

## Recommended Stack

The recommended AWS deployment stack is:

- **Pulumi with Go** for AWS infrastructure provisioning and orchestration;
- **Amazon EKS** as the Kubernetes runtime;
- **Helm** for packaging and deploying e-ubl21 workloads into Kubernetes;
- **AWS ECR** for container images;
- small shell scripts or a Makefile only as thin command wrappers around the main tooling.

The intended split of responsibility is:

```text
Go source
   |
   v
Pulumi
   |
   +--> VPC
   +--> EKS
   +--> IAM
   +--> ECR
   +--> S3
   +--> YugabyteDB/YSQL (three-AZ RF-3)
   +--> Load Balancing
   +--> other AWS services
   |
   v
AWS

Helm
   |
   v
Kubernetes / EKS
   |
   +--> e-ubl21 API
   +--> validation workers
   +--> messaging / queue integration
   +--> services
   +--> configuration
   +--> observability components
```

## Why Pulumi

Pulumi is a strong fit for e-ubl21 because infrastructure can be expressed directly in **Go**, the primary implementation language of the project.

This provides:

- strongly typed infrastructure definitions;
- normal Go abstractions and reusable packages;
- access to standard testing and refactoring techniques;
- easier sharing of constants and deployment conventions;
- less dependence on large shell-script orchestration layers;
- support for AWS and Kubernetes resources from the same codebase.

Conceptually, an EKS cluster can be created from Go with Pulumi:

```go
package main

import (
    "github.com/pulumi/pulumi-eks/sdk/v4/go/eks"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        cluster, err := eks.NewCluster(ctx, "e-ubl21", &eks.ClusterArgs{})
        if err != nil {
            return err
        }

        ctx.Export("kubeconfig", cluster.Kubeconfig)
        return nil
    })
}
```

## Kubernetes Packaging

Pulumi should provision the AWS infrastructure, while **Helm** should remain responsible for the Kubernetes application layer.

A possible repository layout is:

```text
deployment/
|
+-- infrastructure/
|   +-- main.go
|   +-- network.go
|   +-- eks.go
|   +-- database.go
|   +-- storage.go
|   +-- iam.go
|   +-- observability.go
|
+-- kubernetes/
|   +-- helm/
|       +-- e-ubl21/
|           +-- Chart.yaml
|           +-- values.yaml
|           +-- templates/
|
+-- scripts/
    +-- deploy.sh
    +-- destroy.sh
    +-- status.sh
```

Shell scripts should remain small wrappers. Infrastructure creation itself should not be implemented as long sequences of imperative `aws` CLI commands.

## Alternative Engines

Several other deployment engines remain technically valid.

| Tool | Style | Appropriate use |
|---|---|---|
| Pulumi | Go / Python / TypeScript / C# / Java | Programmable infrastructure; preferred when infrastructure should be expressed as software |
| Terraform / OpenTofu | HCL | Declarative, industry-standard infrastructure as code |
| AWS CDK | Go and other languages | AWS-native programmable infrastructure |
| eksctl | YAML + CLI | Straightforward EKS cluster creation and administration |
| Helm | YAML/templates | Kubernetes application packaging and deployment |
| Kustomize | YAML overlays | Environment-specific Kubernetes configuration |
| Argo CD / Flux | GitOps | Continuous reconciliation and production deployment |
| Bash + AWS CLI | Shell | Thin orchestration and utility tasks only |

## Terraform / OpenTofu Alternative

Terraform or OpenTofu should be considered when the primary goal is conventional declarative infrastructure with broad industry adoption and minimal coupling to an application programming language.

In that model:

```text
Terraform / OpenTofu
        |
        v
AWS infrastructure
        |
        v
Amazon EKS
        |
        v
Helm
        |
        v
e-ubl21 workloads
```

This remains a valid alternative to Pulumi.

## eksctl

`eksctl` can be used for rapid or lightweight EKS cluster creation.

Example:

```yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: e-ubl21
  region: eu-west-1

managedNodeGroups:
  - name: workers
    instanceType: t3.medium
    minSize: 2
    maxSize: 10
```

Deployment:

```bash
eksctl create cluster -f cluster.yaml
```

For the long-term e-ubl21 platform, however, Pulumi or Terraform/OpenTofu provides a more complete infrastructure model than `eksctl` alone.

## AWS Target Deployment Flow

The desired high-level workflow is:

```text
Source repository
      |
      +--> Go application build
      |
      +--> container image
      |       |
      |       v
      |      ECR
      |
      +--> Pulumi deployment
      |       |
      |       v
      |      AWS / EKS
      |
      +--> Helm deployment
              |
              v
          Kubernetes
              |
              +--> API pods
              +--> worker pods
              +--> services
              +--> autoscaling
              +--> observability
```

## AWS Decision

The preferred AWS target for a Go-centric e-ubl21 deployment is:

```text
Pulumi + Go + Amazon EKS + Helm
```

Terraform/OpenTofu remains the strongest alternative where declarative infrastructure and organizational standardization take priority over expressing infrastructure in Go.

# Azure Deployment Target

## Objective

Provide an Azure deployment target with the same operational capabilities as the AWS target while keeping the Kubernetes application layer as cloud-neutral as practical.

The application containers and Helm charts should remain common across AWS and Azure. Cloud-specific differences should be isolated primarily in infrastructure modules, identity configuration, storage/database bindings, ingress, secrets, and observability integration.

## Recommended Azure Stack

The recommended Azure deployment stack is:

- **Pulumi with Go** using the **Azure Native provider** for Azure infrastructure provisioning;
- **Azure Kubernetes Service (AKS)** as the Kubernetes runtime;
- **Helm** for packaging and deploying the same e-ubl21 Kubernetes workloads used on AWS;
- **Azure Container Registry (ACR)** for container images;
- **Azure Virtual Network (VNet)** for private networking;
- **Microsoft Entra ID**, **Managed Identities**, and **AKS Workload Identity** for service identity;
- **Azure Key Vault** for secrets and keys;
- **Azure Blob Storage** for durable object storage;
- **YugabyteDB/YSQL** in a three-AZ RF-3 Kubernetes topology;
- **Azure Service Bus** when a managed Azure queue is preferred, while retaining the option to run NATS JetStream for cross-cloud runtime consistency;
- **Azure Load Balancer** and/or **Application Gateway** for ingress and external traffic;
- **Azure DNS** for public or private DNS;
- **Azure Monitor**, **Log Analytics**, **Managed Prometheus**, and **Azure Managed Grafana** for platform observability;
- **OpenTelemetry** for application traces, metrics, and logs;
- **KEDA** for workload/event-driven autoscaling;
- AKS node autoscaling or node auto-provisioning for cluster capacity;
- small shell scripts or a Makefile only as thin orchestration wrappers.

## AWS-to-Azure Service Mapping

| Capability | AWS target | Azure target |
|---|---|---|
| Managed Kubernetes | Amazon EKS | Azure Kubernetes Service (AKS) |
| Container registry | Amazon ECR | Azure Container Registry (ACR) |
| Network | VPC | Virtual Network (VNet) |
| Object storage | Amazon S3 | Azure Blob Storage |
| Relational database | YugabyteDB/YSQL (three-AZ RF-3) | YugabyteDB/YSQL (three-AZ RF-3) |
| Queue / messaging | Amazon SQS or NATS | Azure Service Bus or NATS |
| Identity | AWS IAM | Microsoft Entra ID + Managed Identities |
| Workload identity | IAM Roles for Service Accounts / Pod Identity | AKS Workload Identity |
| Secrets | AWS Secrets Manager | Azure Key Vault |
| Keys | AWS KMS | Azure Key Vault Keys / Managed HSM where required |
| Load balancing | ELB / ALB / NLB | Azure Load Balancer / Application Gateway |
| DNS | Route 53 | Azure DNS |
| Metrics | Amazon Managed Service for Prometheus | Azure Monitor managed service for Prometheus |
| Dashboards | Amazon Managed Grafana | Azure Managed Grafana |
| Logs | CloudWatch Logs | Azure Monitor Logs / Log Analytics |
| Tracing | OpenTelemetry | OpenTelemetry + Azure Monitor |
| Pod autoscaling | KEDA / HPA | KEDA / HPA |
| Node autoscaling | Karpenter / EKS autoscaling | AKS cluster autoscaler / node auto-provisioning |

## Azure Architecture

The intended split of responsibility is:

```text
Go source
   |
   v
Pulumi Azure Native
   |
   +--> Resource Groups
   +--> VNet
   +--> Subnets
   +--> Network Security Groups
   +--> AKS
   +--> ACR
   +--> Managed Identities
   +--> Key Vault
   +--> Blob Storage
   +--> YugabyteDB/YSQL (three-AZ RF-3)
   +--> Service Bus
   +--> Load Balancer / Application Gateway
   +--> Azure DNS
   +--> Azure Monitor
   +--> Managed Prometheus
   +--> Azure Managed Grafana
   |
   v
Microsoft Azure

Helm
   |
   v
Kubernetes / AKS
   |
   +--> e-ubl21 API
   +--> validation workers
   +--> services
   +--> queue integration
   +--> configuration
   +--> autoscaling
   +--> OpenTelemetry
   +--> Kubernetes policies
```

## Pulumi with Azure Native

Pulumi's Azure Native provider allows the Azure deployment layer to remain written in Go, preserving the same Infrastructure-as-Software approach used by the AWS target.

The infrastructure code should be separated by provider-specific package while sharing project-level abstractions where useful.

A conceptual AKS deployment begins with Azure resource groups and Azure Native resources:

```go
package main

import (
    "github.com/pulumi/pulumi-azure-native/sdk/v3/go/azure/containerservice"
    "github.com/pulumi/pulumi-azure-native/sdk/v3/go/azure/resources"
    "github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
    pulumi.Run(func(ctx *pulumi.Context) error {
        resourceGroup, err := resources.NewResourceGroup(ctx, "e-ubl21", nil)
        if err != nil {
            return err
        }

        _, err = containerservice.NewManagedCluster(ctx, "e-ubl21-aks", &containerservice.ManagedClusterArgs{
            ResourceGroupName: resourceGroup.Name,
            DnsPrefix:         pulumi.String("e-ubl21"),
        })
        return err
    })
}
```

The production implementation must additionally define networking, node pools, identity, RBAC, workload identity, observability, security policies, private endpoints where appropriate, and upgrade policies.

## Container Registry

The same e-ubl21 application images used for AWS should be built without cloud-specific assumptions and pushed to ACR for Azure deployments.

```text
Source
  |
  v
Docker / OCI build
  |
  v
Azure Container Registry
  |
  v
AKS
```

AKS should authenticate to ACR through Azure identity rather than static registry credentials.

The image topology remains unchanged:

```text
e-ubl21 application image
   |
   +--> serve   -> API pods
   |
   +--> worker  -> validation-worker pods
```

Additional infrastructure images, such as NATS, OpenTelemetry Collector, ingress components, or other platform services, should be independently version-pinned.

## Storage

Azure Blob Storage is the natural equivalent of Amazon S3 for durable document/object storage.

Expected uses include:

- uploaded UBL documents;
- validation artifacts;
- generated test corpora where durable storage is required;
- regression evidence bundles;
- export files;
- backups or intermediate artifacts where object storage is appropriate.

Applications should access Blob Storage through managed workload identity rather than embedded storage-account keys wherever practical.

## Database

The managed relational database target is:

```text
YugabyteDB/YSQL deployed across three availability zones
```

It provides the same YSQL contract as AWS. See
[`docs/YUGABYTEDB-HA-ARCHITECTURE.md`](docs/YUGABYTEDB-HA-ARCHITECTURE.md)
for placement, TLS, credentials, scaling and multi-region policy.

The same logical e-ubl21 database model should be used on both clouds, including the project's multi-tenant customer identifier strategy.

Cloud-specific details such as endpoints, TLS certificates, private networking, backup policy, high availability, scaling, and maintenance configuration belong in the Azure infrastructure layer rather than application code.

## Messaging

Two supported Azure patterns should be considered.

### Portable Runtime Model

Retain NATS JetStream as the application queue:

```text
API
 |
 v
NATS JetStream
 |
 v
Validation workers
```

This maximizes runtime consistency between local Minikube, AWS, Azure, and other Kubernetes platforms.

### Azure-Managed Model

Use Azure Service Bus:

```text
API
 |
 v
Azure Service Bus
 |
 v
Validation workers
```

This reduces responsibility for operating the messaging infrastructure but introduces an Azure-specific transport dependency.

The application should therefore keep the queue boundary abstract enough that the transport can be replaced without changing validation logic.

## Identity and Secrets

Azure deployments should avoid static service credentials wherever possible.

Preferred model:

```text
Kubernetes ServiceAccount
        |
        v
AKS Workload Identity
        |
        v
Microsoft Entra ID
        |
        +--> Blob Storage
        +--> Key Vault
        +--> Service Bus
        +--> YugabyteDB/YSQL access
        +--> other Azure services
```

Secrets that cannot be replaced by identity-based access should reside in **Azure Key Vault**.

Kubernetes Secrets should contain only the minimum material required by the workload and should preferably be populated or mounted from Key Vault rather than becoming the authoritative secret store.

## Networking

The Azure networking model should provide the same isolation goals as the AWS VPC design.

Expected topology:

```text
Azure VNet
   |
   +--> AKS subnet
   +--> ingress subnet where required
   +--> database/private endpoint connectivity
   +--> storage/private endpoint connectivity
   +--> monitoring connectivity
```

Production deployments should support private endpoints and restricted public exposure for YugabyteDB/YSQL, Blob Storage, Key Vault, ACR, and other managed services where appropriate.

Network Security Groups, Kubernetes NetworkPolicies, ingress policy, and Azure firewalling should form complementary layers rather than substitutes for one another.

## Ingress and External Access

Public API traffic can be exposed using Azure-native load-balancing services while keeping the Kubernetes service model portable.

Possible architecture:

```text
Internet
   |
   v
Azure Application Gateway / Load Balancer
   |
   v
Kubernetes ingress / service
   |
   v
e-ubl21 API pods
```

TLS termination, WAF policy, certificate management, rate limiting, and public/private endpoint strategy should be selectable per environment.

## Autoscaling

The Azure target should preserve the same two-level scaling model as AWS.

### Pod Scaling

Use Kubernetes HPA for resource-based scaling and **KEDA** for event-driven worker scaling.

For queue-backed validation workers:

```text
Queue depth
    |
    v
KEDA
    |
    v
Desired worker replicas
    |
    v
AKS worker pods
```

This supports either Azure Service Bus or another KEDA-supported queue source.

### Node Scaling

When increased pod demand exceeds available capacity:

```text
KEDA / HPA
    |
    v
More pods requested
    |
    v
Insufficient node capacity
    |
    v
AKS node autoscaling / node auto-provisioning
    |
    v
Additional compute capacity
```

Pod scaling and node scaling should remain independent concerns.

## Observability

The Azure observability target should expose the same signals as the AWS environment:

```text
metrics
logs
traces
health
queue depth
worker throughput
validation latency
validation failures
pod/node capacity
```

Recommended stack:

```text
e-ubl21 applications
      |
      v
OpenTelemetry
      |
      +--> Azure Monitor
      +--> Managed Prometheus
      +--> Log Analytics
      +--> Azure Managed Grafana
```

Application instrumentation should remain based on OpenTelemetry rather than Azure-specific instrumentation wherever practical. This keeps telemetry portable while allowing Azure-native storage, dashboards, alerts, and operations tooling.

## Helm Portability

The preferred model is to maintain **one core Helm chart** for e-ubl21 rather than independent AWS and Azure charts.

Cloud differences should be expressed through values and optional templates:

```text
deployment/
|
+-- kubernetes/
|   +-- helm/
|       +-- e-ubl21/
|           +-- Chart.yaml
|           +-- values.yaml
|           +-- values-aws.yaml
|           +-- values-azure.yaml
|           +-- templates/
|
+-- infrastructure/
    +-- aws/
    |   +-- main.go
    |   +-- eks.go
    |   +-- network.go
    |   +-- storage.go
    |   +-- database.go
    |
    +-- azure/
        +-- main.go
        +-- aks.go
        +-- network.go
        +-- storage.go
        +-- database.go
        +-- identity.go
```

The boundary should be:

```text
cloud-specific infrastructure
          |
          v
well-defined configuration outputs
          |
          v
common Helm deployment
          |
          v
common e-ubl21 containers
```

This minimizes cloud-specific application logic.

## Azure Target Deployment Flow

The high-level Azure workflow is:

```text
Source repository
      |
      +--> Go application build
      |
      +--> container image
      |       |
      |       v
      |      ACR
      |
      +--> Pulumi Azure deployment
      |       |
      |       v
      |      Azure / AKS
      |
      +--> Helm deployment
              |
              v
          Kubernetes
              |
              +--> API pods
              +--> worker pods
              +--> messaging
              +--> services
              +--> autoscaling
              +--> observability
```

## Azure Alternatives

Terraform/OpenTofu is the primary alternative to Pulumi for Azure infrastructure.

Azure-native alternatives include:

- **Bicep** for declarative Azure Resource Manager deployment;
- **ARM templates** where required by an existing Azure estate;
- **Azure CLI** for bootstrap and thin orchestration only.

The same rule used for AWS applies to Azure: avoid implementing the cloud estate as a long imperative shell script containing hundreds of `az` commands.

## Azure Decision

The preferred Azure target is:

```text
Pulumi + Go + Azure Native + AKS + Helm
```

The design objective is not to make AWS and Azure identical. The objective is to keep the **application runtime identical** while isolating cloud-specific infrastructure behind explicit deployment modules.

# Multi-Cloud Deployment Principle

The long-term deployment architecture should therefore converge on:

```text
                         e-ubl21 source
                               |
                        common OCI images
                               |
                         common Helm chart
                               |
                 +-------------+-------------+
                 |                           |
                 v                           v
          Pulumi AWS modules           Pulumi Azure modules
                 |                           |
                 v                           v
             Amazon EKS                 Azure AKS
                 |                           |
                 +-------------+-------------+
                               |
                               v
                    same application runtime
```

Cloud-specific managed services are allowed and desirable when they improve reliability or operations, but the validation engine, API behavior, worker behavior, document model, regression suite, container images, and core Kubernetes packaging should remain cloud-neutral.
