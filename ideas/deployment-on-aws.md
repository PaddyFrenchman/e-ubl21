# Deployment on AWS

**Project:** UBL-21 Validator  
**Document:** `deployment-on-aws.md`  
**Purpose:** Define the recommended script-driven deployment architecture for running the UBL-21 validation platform on AWS using Kubernetes.  
**Target platform:** AWS, Amazon EKS, Terraform-compatible Infrastructure as Code  
**Status:** Deployment architecture and implementation guide

---

# 1. Objective

The UBL-21 platform is intended to become an enterprise-grade managed service hosted on AWS.

All infrastructure and application deployment must be reproducible from source control.

The deployment model should avoid manual configuration in the AWS console except for exceptional bootstrap or break-glass operations.

The desired operating principle is:

```text
Git
 |
 +--> Infrastructure as Code
 |
 +--> Kubernetes configuration
 |
 +--> CI/CD
 |
 v
AWS
```

A complete environment should be reproducible from:

```text
Git repository
+
protected infrastructure state
+
managed secrets
```

The target is not merely "scripted installation".

The target is:

```text
declarative
versioned
reviewable
repeatable
auditable
automatable
```

---

# 2. Recommended deployment stack

The recommended stack is:

| Layer | Recommended tool |
|---|---|
| AWS infrastructure | OpenTofu |
| Terraform-compatible configuration | HCL |
| Kubernetes packaging | Helm |
| Kubernetes deployment | Argo CD |
| Worker autoscaling | KEDA |
| Node autoscaling | Karpenter |
| Container registry | Amazon ECR |
| CI/CD | GitHub Actions or equivalent |
| Command orchestration | Makefile + small shell scripts |
| Secrets | AWS Secrets Manager |
| Metrics | Amazon Managed Service for Prometheus |
| Dashboards | Amazon Managed Grafana |
| Logging | CloudWatch Logs |
| Tracing | OpenTelemetry |

The overall deployment model is:

```text
                 Git
                  |
          +-------+--------+
          |                |
          v                v
     OpenTofu          Helm / Argo CD
          |                |
          v                v
  AWS infrastructure    Kubernetes state
          |                |
          +-------+--------+
                  |
                  v
                 AWS
```

---

# 3. OpenTofu and Terraform

OpenTofu is a Terraform-compatible Infrastructure as Code system.

It uses the same HCL configuration model and a nearly identical command workflow.

Terraform:

```bash
terraform init
terraform validate
terraform plan
terraform apply
```

OpenTofu:

```bash
tofu init
tofu validate
tofu plan
tofu apply
```

The infrastructure configuration remains based on standard Terraform-style files:

```text
main.tf
variables.tf
outputs.tf
providers.tf
versions.tf
```

Example:

```hcl
resource "aws_sqs_queue" "validation" {
  name = "ubl-validation"
}

resource "aws_s3_bucket" "documents" {
  bucket = "ubl-validation-documents"
}
```

For architectural purposes:

```text
Terraform
```

and:

```text
OpenTofu
```

play essentially the same role in this project.

The choice does not materially change the AWS architecture.

---

# 4. Why OpenTofu is the preferred option

For this project, OpenTofu is recommended as the default Infrastructure as Code engine.

Reasons:

- Terraform-compatible workflow;
- declarative infrastructure;
- mature AWS provider ecosystem;
- reusable module ecosystem;
- suitable for large AWS estates;
- reviewable infrastructure plans;
- drift detection;
- open-source governance;
- easy adoption by engineers already familiar with Terraform;
- good fit for Git-based enterprise deployment processes.

The platform will eventually contain enough AWS resources that raw shell scripting would become difficult to maintain.

Expected infrastructure includes:

```text
AWS accounts
VPCs
subnets
security groups
EKS
ECR
SQS
S3
Aurora PostgreSQL
IAM
KMS
WAF
Route 53
Secrets Manager
Managed Prometheus
Managed Grafana
CloudWatch integration
```

OpenTofu is well suited to describing this complete topology declaratively.

---

# 5. Alternative: Terraform

Terraform can be used directly instead of OpenTofu.

The architecture remains:

```text
Terraform
    |
    +--> VPC
    +--> EKS
    +--> SQS
    +--> S3
    +--> Aurora
    +--> IAM
    +--> KMS
    +--> observability
    |
    v
   AWS
```

If the organization already standardizes on Terraform, there is no architectural requirement to move to OpenTofu.

The project should avoid supporting both at the same time unless there is a compelling reason.

Choose one IaC engine as the standard.

Recommended default:

```text
OpenTofu
```

Enterprise/customer requirement:

```text
Terraform acceptable
```

---

# 6. Alternative: AWS CDK in Go

AWS CDK is the strongest alternative if the project prefers infrastructure expressed in Go.

Conceptually:

```go
queue := sqs.NewQueue(stack, jsii.String("ValidationQueue"), &sqs.QueueProps{
    VisibilityTimeout: awscdk.Duration_Seconds(jsii.Number(120)),
})
```

Deployment:

```bash
cdk synth
cdk diff
cdk deploy
```

Architecture:

```text
Go source
   |
   v
AWS CDK
   |
   v
CloudFormation
   |
   v
AWS
```

Advantages:

- infrastructure expressed using Go;
- strong AWS-native support;
- reusable software abstractions;
- useful for teams that prefer programming constructs over HCL.

Disadvantages:

- more tightly coupled to AWS;
- infrastructure logic can become more procedural;
- reviewing topology can be less direct than HCL;
- CloudFormation remains the execution backend.

For UBL-21:

```text
OpenTofu > AWS CDK
```

unless the team explicitly wants to standardize infrastructure on Go.

---

# 7. Alternative: CloudFormation

AWS CloudFormation can provision the entire environment.

Example:

```yaml
Resources:
  ValidationQueue:
    Type: AWS::SQS::Queue
```

Deployment:

```bash
aws cloudformation deploy ...
```

Advantages:

- native AWS technology;
- mature;
- strong support for AWS resources;
- built-in rollback and drift detection.

Disadvantages:

- large YAML/JSON templates become difficult to maintain;
- less portable;
- more verbose than OpenTofu/Terraform.

CloudFormation is therefore not the preferred authoring layer for this project.

---

# 8. Alternative: Pulumi

Pulumi is another Infrastructure as Code option and supports Go.

It allows infrastructure definitions using ordinary programming languages.

It is technically suitable, but the preferred choices remain:

```text
OpenTofu
```

or:

```text
Terraform
```

for declarative infrastructure,

and:

```text
AWS CDK Go
```

if programming-language infrastructure is explicitly desired.

---

# 9. Division of responsibility

A very important deployment rule is to keep a clean boundary between AWS infrastructure and Kubernetes application state.

Recommended ownership:

```text
OpenTofu
    |
    +--> AWS accounts
    +--> VPC
    +--> subnets
    +--> EKS
    +--> ECR
    +--> SQS
    +--> S3
    +--> Aurora
    +--> IAM
    +--> KMS
    +--> Secrets Manager
    +--> Route 53
    +--> WAF
    +--> Prometheus
    +--> Grafana
```

Then:

```text
Helm + Argo CD
    |
    +--> API Deployment
    +--> worker Deployment
    +--> Services
    +--> ConfigMaps
    +--> KEDA
    +--> Karpenter configuration
    +--> PodDisruptionBudgets
    +--> NetworkPolicies
    +--> ServiceAccounts
    +--> OpenTelemetry agents
    +--> Kubernetes configuration
```

Do not make OpenTofu manage every Kubernetes Deployment, Service, and ConfigMap individually.

That creates unnecessary coupling between infrastructure state and application release cycles.

---

# 10. Kubernetes packaging with Helm

Use Helm to package the UBL-21 Kubernetes applications.

Example repository layout:

```text
deploy/
└── helm/
    └── ubl-validator/
        ├── Chart.yaml
        ├── values.yaml
        ├── templates/
        │   ├── api-deployment.yaml
        │   ├── worker-deployment.yaml
        │   ├── api-service.yaml
        │   ├── keda.yaml
        │   ├── pdb.yaml
        │   ├── service-account.yaml
        │   └── network-policy.yaml
        │
        └── values/
            ├── dev.yaml
            ├── staging.yaml
            └── production.yaml
```

Manual/scripted deployment remains possible:

```bash
helm upgrade \
  --install ubl-validator \
  ./deploy/helm/ubl-validator \
  -f ./deploy/helm/ubl-validator/values/production.yaml \
  --namespace validation \
  --create-namespace
```

However, direct Helm commands should eventually be replaced by GitOps for production.

---

# 11. Argo CD for Kubernetes deployment

Argo CD should manage application deployment into EKS.

Desired workflow:

```text
Git
 |
 | desired Kubernetes state
 v
Argo CD
 |
 v
Amazon EKS
```

Benefits:

- automatic synchronization;
- drift detection;
- self-healing;
- rollback;
- environment promotion;
- deployment history;
- strong auditability;
- support for Helm;
- support for Kustomize;
- support for plain Kubernetes YAML.

Production deployment becomes:

```text
modify values-production.yaml
        |
        v
Git pull request
        |
        v
review
        |
        v
merge
        |
        v
Argo CD
        |
        v
production EKS
```

This removes the need for administrators to manually invoke `kubectl apply` or `helm upgrade` against production.

---

# 12. Kustomize

Kustomize may be used where simple environment overlays are useful.

Example:

```text
base/
    deployment.yaml
    service.yaml

overlays/
    dev/
    staging/
    production/
```

However, the UBL-21 platform has many configurable parameters:

```text
replica limits
queue URLs
resource requests
resource limits
KEDA thresholds
Saxon limits
validation versions
observability configuration
```

Helm is therefore preferred as the main packaging system.

Kustomize can still be used selectively.

---

# 13. KEDA

KEDA should manage autoscaling of validation workers based on SQS demand.

Architecture:

```text
Amazon SQS
     |
     v
KEDA
     |
     v
desired worker replicas
     |
     v
Kubernetes worker Deployment
```

KEDA converts queue backlog into Kubernetes replica demand.

Typical inputs:

```text
queue depth
target messages per worker
minimum replicas
maximum replicas
```

Example concept:

```text
queue depth = 100
target = 5 messages per worker

desired workers ≈ 20
```

KEDA configuration belongs in Helm/Argo CD, not in OpenTofu.

---

# 14. Karpenter

Karpenter should manage EC2 capacity for EKS worker nodes.

Architecture:

```text
KEDA increases worker pods
        |
        v
pods become Pending
        |
        v
Karpenter detects unschedulable pods
        |
        v
EC2 nodes are provisioned
        |
        v
pods schedule
```

This creates the desired autoscaling chain:

```text
SQS
 ↓
KEDA
 ↓
Pods
 ↓
Karpenter
 ↓
EC2
```

Karpenter configuration should be managed declaratively and deployed through the Kubernetes/GitOps layer.

---

# 15. Script-driven does not mean Bash-driven

Shell scripts should remain small orchestration wrappers.

Good:

```bash
#!/usr/bin/env bash
set -euo pipefail

tofu init
tofu plan -var-file=production.tfvars
```

Bad:

```bash
aws ec2 create-vpc ...
aws ec2 create-subnet ...
aws eks create-cluster ...
aws sqs create-queue ...
aws rds create-db-cluster ...
```

The second approach creates infrastructure imperatively.

It is difficult to:

- detect drift;
- review changes;
- safely re-run;
- destroy/recreate environments;
- understand current desired state.

Use declarative IaC as the real deployment engine.

Use scripts only as a human-friendly command interface.

---

# 16. Makefile as the operational interface

A top-level Makefile can expose standard commands.

Example:

```makefile
infra-init:
	cd infrastructure/tofu/environments/$(ENV) && tofu init

infra-plan:
	cd infrastructure/tofu/environments/$(ENV) && tofu plan

infra-apply:
	cd infrastructure/tofu/environments/$(ENV) && tofu apply

build:
	docker build -t $(IMAGE) .

push:
	docker push $(IMAGE)

deploy:
	helm upgrade --install ...
```

Usage:

```bash
make infra-plan ENV=production
make infra-apply ENV=production

make build
make push

make deploy ENV=staging
```

When Argo CD is introduced, `make deploy` should normally update or promote Git configuration rather than directly mutate production.

---

# 17. Recommended repository structure

```text
UBL-21-validator/
│
├── cmd/
│
├── internal/
│
├── Dockerfile
├── Makefile
│
├── scripts/
│   ├── bootstrap.sh
│   ├── deploy-dev.sh
│   ├── deploy-staging.sh
│   ├── deploy-prod.sh
│   ├── infra-plan.sh
│   └── infra-apply.sh
│
├── infrastructure/
│   └── tofu/
│       ├── modules/
│       │   ├── network/
│       │   ├── eks/
│       │   ├── ecr/
│       │   ├── queue/
│       │   ├── storage/
│       │   ├── database/
│       │   ├── security/
│       │   └── observability/
│       │
│       └── environments/
│           ├── dev/
│           │   ├── main.tf
│           │   ├── variables.tf
│           │   └── terraform.tfvars
│           │
│           ├── staging/
│           │   ├── main.tf
│           │   ├── variables.tf
│           │   └── terraform.tfvars
│           │
│           └── production/
│               ├── main.tf
│               ├── variables.tf
│               └── terraform.tfvars
│
├── deploy/
│   ├── helm/
│   │   └── ubl-validator/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       ├── templates/
│   │       └── values/
│   │           ├── dev.yaml
│   │           ├── staging.yaml
│   │           └── production.yaml
│   │
│   └── argocd/
│       ├── dev.yaml
│       ├── staging.yaml
│       └── production.yaml
│
└── .github/
    └── workflows/
        ├── infrastructure-plan.yml
        ├── infrastructure-apply.yml
        ├── build.yml
        ├── test.yml
        └── release.yml
```

---

# 18. OpenTofu module structure

Reusable modules should describe logical infrastructure components.

Example:

```text
infrastructure/tofu/modules/
|
+-- network
+-- eks
+-- ecr
+-- queue
+-- storage
+-- database
+-- security
+-- observability
```

Environment configuration then composes modules.

Example conceptual `main.tf`:

```hcl
module "network" {
  source = "../../modules/network"

  environment = var.environment
  vpc_cidr    = var.vpc_cidr
}

module "eks" {
  source = "../../modules/eks"

  environment = var.environment
  vpc_id      = module.network.vpc_id
  subnet_ids  = module.network.private_subnet_ids
}

module "queue" {
  source = "../../modules/queue"

  environment = var.environment
}

module "storage" {
  source = "../../modules/storage"

  environment = var.environment
}
```

---

# 19. Environment separation

Use separate environments:

```text
dev
staging
production
```

Each should have:

- independent IaC state;
- independent configuration;
- independent EKS namespace or cluster strategy;
- independent AWS resources where appropriate;
- separate secrets;
- controlled promotion.

Production must never share infrastructure state with development.

---

# 20. IaC state

OpenTofu/Terraform state must be stored remotely.

Recommended:

```text
Amazon S3
+
encryption
+
versioning
+
state locking
```

Conceptual backend:

```hcl
terraform {
  backend "s3" {
    bucket       = "ubl21-infrastructure-state"
    key          = "production/aws.tfstate"
    region       = "eu-west-1"
    encrypt      = true
    use_lockfile = true
  }
}
```

Use separate state keys for:

```text
dev
staging
production
```

Example:

```text
dev/aws.tfstate
staging/aws.tfstate
production/aws.tfstate
```

State buckets must be protected against accidental deletion.

---

# 21. Bootstrap problem

Infrastructure state storage itself must exist before OpenTofu can use it.

Handle this with a minimal bootstrap process.

Example:

```text
scripts/bootstrap.sh
```

Responsibilities:

```text
create state S3 bucket
enable versioning
enable encryption
create required bootstrap IAM role
```

After bootstrap, normal infrastructure management should be performed exclusively through OpenTofu.

Keep bootstrap logic minimal.

---

# 22. Container build and registry

Application images should be built through CI.

Flow:

```text
Git commit
   |
   v
tests
   |
   v
container build
   |
   v
security scan
   |
   v
ECR push
   |
   v
image digest
   |
   v
deployment configuration update
```

Use Amazon ECR as the container registry.

Do not deploy production images using mutable tags such as:

```text
latest
```

Prefer:

```text
version tag
+
image digest
```

---

# 23. CI/CD pipeline

Recommended stages:

```text
Pull Request
    |
    +--> Go tests
    +--> lint
    +--> security scanning
    +--> OpenTofu validate
    +--> OpenTofu plan
    +--> Helm lint
    +--> Kubernetes manifest validation
    |
    v
Review
    |
    v
Merge
    |
    +--> build containers
    +--> push to ECR
    +--> deploy/publish staging
    |
    v
Staging verification
    |
    v
Production approval
    |
    v
Production deployment
```

---

# 24. Infrastructure plan in pull requests

Infrastructure changes should produce a plan before merge.

Example:

```bash
tofu init
tofu validate
tofu plan
```

The plan should be attached to or summarized in the pull request.

Reviewers should be able to see:

```text
resources created
resources modified
resources destroyed
```

before the change is applied.

---

# 25. Production apply

Production infrastructure changes should be explicit.

Example policy:

```text
PR merged
    |
    v
approved CI job
    |
    v
tofu apply saved-plan
```

Avoid unattended production `apply` from arbitrary branches.

Use protected environments and approval controls in the CI system.

---

# 26. Kubernetes GitOps workflow

Production application deployment should eventually be entirely Git-driven.

Example:

```text
application build
    |
    v
ECR image
    |
    v
update Helm values
    |
    v
Git commit / pull request
    |
    v
merge
    |
    v
Argo CD detects change
    |
    v
EKS rollout
```

This creates a complete deployment audit trail.

---

# 27. Argo CD application model

Suggested:

```text
deploy/argocd/
|
+-- dev.yaml
+-- staging.yaml
+-- production.yaml
```

Each Argo CD Application points at:

```text
Helm chart
+
environment values
```

Example concept:

```yaml
source:
  repoURL: <repository>
  path: deploy/helm/ubl-validator

  helm:
    valueFiles:
      - values/production.yaml
```

---

# 28. Secrets

Secrets must not live in:

```text
terraform.tfvars committed to Git
Helm values committed to Git
container images
shell scripts
```

Use:

```text
AWS Secrets Manager
```

with workload identity.

The application retrieves or receives secrets through controlled Kubernetes/AWS integration.

---

# 29. AWS identity

Use:

```text
EKS Pod Identity
```

or the approved AWS mechanism for IAM-backed Kubernetes service accounts.

Example permissions:

```text
API service account
    SQS SendMessage
    S3 Put/Get
    Secrets access as required

Worker service account
    SQS Receive/Delete
    S3 Get/Put
    database secret access
```

Do not deploy long-lived AWS access keys into pods.

---

# 30. Deployment ownership boundary

The cleanest ownership model is:

```text
                OpenTofu
                    |
          AWS infrastructure
                    |
        +-----------+-----------+
        |                       |
       EKS                  SQS/S3/Aurora
        |
        | Kubernetes boundary
        v
      Argo CD
        |
        v
 Kubernetes applications
        |
        +-- API
        +-- workers
        +-- KEDA
        +-- Karpenter configuration
        +-- observability agents
```

This boundary should be documented and enforced.

---

# 31. Do not duplicate ownership

Avoid situations such as:

```text
OpenTofu manages Deployment X
AND
Argo CD manages Deployment X
```

or:

```text
Terraform manages ServiceAccount Y
AND
Helm manages ServiceAccount Y
```

Every resource should have one authoritative owner.

Recommended:

```text
AWS resource:
    OpenTofu

Kubernetes application resource:
    Helm / Argo CD
```

---

# 32. Deployment commands

Developer/operator-facing commands should be simple.

Example:

```bash
make infra-init ENV=dev
make infra-plan ENV=dev
make infra-apply ENV=dev

make build
make test
make push

make deploy ENV=staging
```

Behind these commands:

```text
OpenTofu
Docker
Helm
GitOps
```

remain the authoritative mechanisms.

---

# 33. Development environment

Development infrastructure can be cheaper and simpler.

Possible:

```text
smaller Aurora/RDS
smaller EKS capacity
lower KEDA minimum
no Spot requirement
shorter retention
reduced observability retention
```

But deployment mechanisms should remain identical to production.

Do not create a completely different deployment model for development.

---

# 34. Staging environment

Staging should resemble production closely.

It should exercise:

```text
EKS
SQS
S3
database
KEDA
Karpenter
Saxon
UBL artefacts
EN16931 artefacts
Peppol artefacts
observability
```

It can use smaller capacities but should preserve architectural equivalence.

---

# 35. Production environment

Production should require:

```text
multiple Availability Zones
minimum API replicas
warm worker capacity
KEDA
Karpenter
PDBs
resource requests
resource limits
managed observability
encrypted storage
remote state
protected CI approvals
immutable image versions
```

---

# 36. Infrastructure versioning

Pin:

```text
OpenTofu version
AWS provider version
Helm version
Argo CD version
KEDA version
Karpenter version
Kubernetes/EKS version
```

Do not allow production tooling to drift automatically.

Example:

```hcl
terraform {
  required_version = "~> 1.x"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.x"
    }
  }
}
```

Actual version numbers should be selected and updated deliberately.

---

# 37. Application release versioning

Every deployment should identify:

```text
application version
Git commit
container digest
UBL version
EN16931 version
Peppol version
Saxon version
```

Example:

```text
ubl-validator:1.8.0
git: abcdef12
ubl: 2.1
en16931: 1.3.16
peppol: 3.0.21
```

This is essential for enterprise reproducibility.

---

# 38. Destroy/rebuild capability

A non-production environment should be testable with:

```bash
tofu destroy
```

followed by:

```bash
tofu apply
```

and Kubernetes reconciliation through Argo CD.

The objective is:

```text
destroy environment
    |
    v
recreate infrastructure
    |
    v
Argo CD restores applications
    |
    v
environment returns to known state
```

This is one of the strongest tests of Infrastructure as Code quality.

---

# 39. Drift detection

AWS console changes should be treated as drift.

Operational rule:

```text
No normal production infrastructure changes in AWS Console.
```

Emergency console changes must later be:

```text
captured in IaC
or
reverted
```

OpenTofu/Terraform plan output should regularly be checked for unexpected drift.

Argo CD performs the same role inside Kubernetes.

---

# 40. Disaster recovery benefit

Script-driven infrastructure improves disaster recovery.

If a cluster is lost:

```text
OpenTofu
    |
    v
rebuild AWS/EKS foundation
```

then:

```text
Argo CD
    |
    v
restore Kubernetes workloads
```

Persistent data remains in managed services according to backup policy.

This is much safer than attempting to reconstruct infrastructure from manual documentation.

---

# 41. Security review benefit

Enterprise customers may request evidence of:

```text
network design
IAM permissions
encryption
storage policy
autoscaling rules
deployment controls
```

Infrastructure as Code provides an auditable source of truth.

A security review can inspect:

```text
Git
+
OpenTofu modules
+
Helm charts
+
Argo CD configuration
```

rather than relying solely on screenshots of AWS Console configuration.

---

# 42. Recommended implementation order

## Stage 1

Introduce OpenTofu locally.

Create modules for:

```text
network
S3
SQS
ECR
```

Do not begin with the entire platform.

---

## Stage 2

Add EKS provisioning.

Create:

```text
VPC
private subnets
EKS
IAM
Pod Identity
```

---

## Stage 3

Package current API and worker with Helm.

Deploy manually to non-production using:

```bash
helm upgrade --install
```

---

## Stage 4

Add Aurora PostgreSQL and Secrets Manager through OpenTofu.

---

## Stage 5

Add KEDA to the Helm chart.

Validate queue-driven worker scaling.

---

## Stage 6

Add Karpenter.

Validate node autoscaling.

---

## Stage 7

Add managed observability through OpenTofu.

Deploy application telemetry components through Helm.

---

## Stage 8

Introduce Argo CD.

Move Kubernetes deployment ownership from manual Helm commands to GitOps.

---

## Stage 9

Add CI infrastructure planning.

Every infrastructure pull request should run:

```bash
tofu validate
tofu plan
```

---

## Stage 10

Add protected production apply.

Production changes require:

```text
review
merge
approval
apply
```

---

## Stage 11

Add automated environment bootstrap documentation.

A new AWS environment should be reproducible using documented commands.

---

# 43. Minimum command set

The project should converge on a small operational command surface.

Suggested:

```bash
make bootstrap ENV=dev

make infra-init ENV=dev
make infra-validate ENV=dev
make infra-plan ENV=dev
make infra-apply ENV=dev

make test
make build
make push

make deploy ENV=staging

make status ENV=production
```

Production application deployment should eventually be triggered by Git/Argo CD rather than `make deploy`.

---

# 44. Recommended CI workflows

```text
.github/workflows/
|
+-- test.yml
+-- build.yml
+-- infrastructure-plan.yml
+-- infrastructure-apply.yml
+-- helm-lint.yml
+-- release.yml
```

### `infrastructure-plan.yml`

Runs:

```text
tofu fmt -check
tofu validate
tofu plan
```

### `build.yml`

Runs:

```text
Go tests
container build
container scan
ECR push
```

### `release.yml`

Updates deployment configuration or creates a release PR.

---

# 45. Infrastructure state must not contain secrets unnecessarily

Terraform/OpenTofu state can contain sensitive values.

Therefore:

- encrypt the state bucket;
- restrict bucket access;
- enable versioning;
- enable logging/auditing where appropriate;
- avoid storing secret values in state where possible;
- use Secrets Manager for application secrets.

Treat state as sensitive infrastructure data.

---

# 46. Terraform/OpenTofu modules versus monolithic configuration

Do not build one enormous `main.tf`.

Use modules:

```text
module.network
module.eks
module.queue
module.storage
module.database
module.observability
module.security
```

Advantages:

- testability;
- reuse;
- clearer ownership;
- smaller plans;
- easier future dedicated-customer environments.

---

# 47. Support for dedicated enterprise deployments

The same modules should be reusable for customer-dedicated environments.

Example:

```text
environments/
|
+-- production-shared
|
+-- customer-acme
|
+-- customer-bank-x
```

Each environment composes the same modules with different parameters.

That allows the commercial platform to offer:

```text
shared SaaS
```

and:

```text
dedicated AWS deployment
```

without maintaining two architectures.

---

# 48. Infrastructure outputs

OpenTofu should expose outputs used by downstream configuration.

Examples:

```text
EKS cluster name
SQS queue URL
S3 bucket name
Aurora endpoint
Prometheus workspace ID
Grafana workspace URL
ECR repository URL
```

Do not copy these values manually into Helm configuration.

Use an automated handoff mechanism.

---

# 49. Avoid manual parameter copying

Bad workflow:

```text
tofu apply
    |
operator copies SQS URL
    |
operator edits values.yaml
```

Better:

```text
OpenTofu output
    |
CI / deployment pipeline
    |
generated environment configuration
```

or use stable references discoverable through AWS APIs/parameters.

Manual copying creates configuration drift.

---

# 50. Configuration separation

Separate:

```text
infrastructure configuration
application configuration
secret configuration
```

Example:

```text
OpenTofu:
    AWS resources

Helm:
    application parameters

Secrets Manager:
    credentials/secrets
```

This separation keeps each tool focused on the problem it solves best.

---

# 51. Recommended final architecture

```text
                       Git repository
                            |
            +---------------+---------------+
            |                               |
            v                               v
        OpenTofu                       Helm Charts
            |                               |
            |                               v
            |                            Argo CD
            |                               |
            v                               v
     AWS infrastructure                 Amazon EKS
            |                               |
    +-------+---------+          +----------+-----------+
    |       |         |          |          |           |
    v       v         v          v          v           v
   VPC     SQS       S3         API       Workers      KEDA
    |                  \                     |
    v                   \                    v
   EKS                   \               Karpenter
    |                     \                  |
    v                      +-----------------+
 Aurora / IAM / KMS / Observability
```

---

# 52. Final recommendation

For UBL-21, standardize on:

```text
OpenTofu
    for AWS Infrastructure as Code

Helm
    for Kubernetes packaging

Argo CD
    for Kubernetes deployment and drift control

KEDA
    for SQS-driven worker autoscaling

Karpenter
    for EC2/EKS node autoscaling

Makefile + small shell scripts
    for a simple command interface

GitHub Actions or equivalent
    for CI/CD
```

The core operating model becomes:

```text
OpenTofu:
    creates the AWS/EKS foundation

Argo CD:
    owns everything inside Kubernetes

Git:
    is the authoritative desired state
```

This provides a deployment system suitable for enterprise operation because it is:

```text
repeatable
reviewable
auditable
automatable
recoverable
scalable
```

Most importantly, the infrastructure can be reconstructed from source instead of depending on undocumented manual AWS configuration.

---

# 53. Practical first step

The first infrastructure commit should introduce:

```text
infrastructure/tofu/
    modules/
        network/
        queue/
        storage/
        ecr/

    environments/
        dev/
```

Then implement:

```bash
make infra-init ENV=dev
make infra-plan ENV=dev
make infra-apply ENV=dev
```

Once that works reliably, add:

```text
EKS
Helm
KEDA
Karpenter
Aurora
Observability
Argo CD
```

incrementally.

This avoids attempting to design and deploy the entire production platform in one infrastructure change.
