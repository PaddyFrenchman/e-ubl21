# Deployment Target

## Objective

Define the target automation stack for deploying e-ubl21 on AWS Kubernetes, using Amazon EKS as the Kubernetes platform.

The deployment approach should be reproducible, version-controlled, automatable, and suitable for both local execution and CI/CD pipelines.

## Recommended Stack

The recommended deployment stack is:

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
   +--> RDS / Aurora
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

## Target Deployment Flow

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

## Decision

The preferred target for a Go-centric e-ubl21 deployment is:

```text
Pulumi + Go + Amazon EKS + Helm
```

Terraform/OpenTofu remains the strongest alternative where declarative infrastructure and organizational standardization take priority over expressing infrastructure in Go.
