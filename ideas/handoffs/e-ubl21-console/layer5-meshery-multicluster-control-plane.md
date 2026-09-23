# Idea: Layer5 Meshery as the UBL-21 Multi-Cluster Control Plane

## Status

**Idea / architecture candidate**

This document captures the idea of using **Layer5 Meshery**, with **Kanvas** as its visual design surface, to administer and visualize the Kubernetes clusters used by UBL-21.

The objective is not to make Meshery the permanent source of truth for infrastructure. Instead, Meshery would provide a project-specific visual management plane over a conventional GitOps stack.

---

## Why Meshery fits UBL-21

Meshery is designed to connect to and manage multiple Kubernetes clusters from a central control plane. This maps naturally onto a future UBL-21 deployment topology containing separate development, staging and production clusters, potentially spread across AWS regions.

A possible topology is:

```text
                       UBL-21
                    Meshery Server
                         |
         +---------------+----------------+
         |               |                |
         v               v                v
   Development        Staging         Production
   Environment       Environment      Environment
         |               |                |
         v               v                v
     EKS DEV          EKS STAGE         EKS PROD
```

Meshery concepts can be mapped to UBL-21 roughly as follows:

| Meshery concept | UBL-21 usage |
| --- | --- |
| Workspace | UBL-21 project |
| Environment | Development, integration, staging, production |
| Connection | Kubernetes/EKS cluster or external infrastructure endpoint |
| Model | UBL-21-specific infrastructure vocabulary |
| Component | Validator, document API, worker, database, telemetry component, etc. |
| Kanvas | Visual architecture and deployment design surface |

---

## UBL-21-specific Meshery model

A particularly useful customization would be to define a private **UBL21 Meshery Model**.

Instead of forcing the architecture designer to work only with generic Kubernetes primitives such as `Deployment`, `Service`, `Ingress` and `ConfigMap`, the palette could expose UBL-21 concepts directly:

```text
UBL-21
|
+-- UBL Validator
+-- Invoice API
+-- CreditNote API
+-- Order API
+-- UBL Sample Generator
+-- UBL Database
+-- Tenant Database
+-- OpenTelemetry Collector
+-- API Gateway
+-- UBL Worker
```

These components would represent higher-level application concepts while mapping internally onto Kubernetes resources.

For example:

```text
+----------------------------------+
| UBL Document API                 |
|                                  |
| type: Invoice                    |
| replicas: 4                      |
| autoscaling: 4-20                |
| validation: EN16931              |
| persistence: enabled             |
| tenant-aware: true               |
+----------------------------------+
```

could expand into infrastructure such as:

```text
Deployment
+-- UBL Invoice container
+-- readinessProbe
+-- livenessProbe
+-- resources
+-- OpenTelemetry configuration

Service
Ingress / Gateway API
HPA / KEDA
ConfigMap
Secret references
NetworkPolicy
ServiceMonitor
PodDisruptionBudget
```

The graphical architecture would therefore describe **UBL-21 concepts rather than Kubernetes plumbing**.

---

## Multi-cluster administration

The target architecture could eventually look like:

```text
UBL-21
|
+-- eu-west-1
|   +-- prod-1
|   +-- prod-2
|
+-- eu-central-1
|   +-- prod-1
|
+-- us-east-1
|   +-- prod-1
|
+-- staging
|   +-- stage
|
+-- development
    +-- dev
```

A single Meshery management plane could provide visibility across these clusters rather than requiring a separate administration tool for each one.

Potential UBL-21 control-plane functions include:

- cluster health and inventory
- environment selection
- visual deployment topology
- service health
- replica counts and autoscaling state
- application release visibility
- observability links and metrics
- architecture inspection
- controlled deployment actions

---

## Recommended architectural boundary

Meshery should **not** become the authoritative storage location for UBL-21 infrastructure definitions.

The preferred model is:

```text
                    UBL-21 Meshery
                   visual control plane
                           |
                 architecture/design
                           |
                           v
                         Git
                  source of truth
                           |
              +------------+------------+
              v                         v
         Terraform                   Helm
          AWS/EKS                 Applications
              |                         |
              +------------+------------+
                           v
                       Argo CD
                           |
          +----------------+----------------+
          v                v                v
        DEV              STAGING           PROD
```

Responsibilities should remain clearly separated:

| Layer | Responsibility |
| --- | --- |
| Meshery / Kanvas | Design, visualization, administration and inspection |
| Git | Authoritative configuration and change history |
| Terraform / OpenTofu | AWS and EKS infrastructure |
| Helm | UBL-21 application definitions |
| Argo CD / GitOps | Continuous deployment and reconciliation |
| Kubernetes | Runtime execution |
| OpenTelemetry | Telemetry generation and collection |
| Prometheus / Grafana | Metrics and operational visualization |

This avoids turning the visual tool into opaque click-ops. The visual model becomes an engineering interface while Git remains the durable, auditable source of truth.

---

## Possible implementation phases

### Phase 1: proof of concept

1. Deploy Meshery locally or into a management Kubernetes cluster.
2. Connect one development UBL-21 Kubernetes cluster.
3. Import or represent the current UBL-21 Kubernetes architecture in Kanvas.
4. Verify resource discovery, topology visualization and deployment inspection.
5. Validate that generated/exported artifacts remain compatible with the existing Git workflow.

### Phase 2: UBL-21 model

1. Create a private `UBL21` Meshery Model.
2. Define higher-level components such as `UBLValidator`, `UBLDocumentAPI`, `UBLWorker` and `UBLDatabase`.
3. Define relationships between those components.
4. Map components onto standard Kubernetes, Helm and observability resources.
5. Create reusable visual deployment patterns.

### Phase 3: multi-environment administration

1. Connect development, staging and production clusters.
2. Introduce UBL-21 workspace/environment conventions.
3. Add environment-level status views.
4. Integrate Prometheus, Grafana and OpenTelemetry data.
5. Add operational links and safe management actions.

### Phase 4: GitOps integration

1. Treat Git as the only authoritative desired-state store.
2. Generate or export Helm/Kubernetes artifacts into controlled repository locations.
3. Use pull requests for infrastructure changes.
4. Use Argo CD or Flux to reconcile the clusters.
5. Prevent ad-hoc production changes from bypassing GitOps unless explicitly required for emergency operations.

---

## Security considerations

Meshery should be treated as privileged infrastructure software because it may have access to multiple Kubernetes clusters.

Important controls include:

- least-privilege Kubernetes RBAC
- separate credentials per environment
- stronger restrictions for production clusters
- audit logging
- authenticated access to the Meshery control plane
- careful review of private extensions and custom components
- no credentials embedded in designs or repository files
- GitOps approval requirements for production mutations

Custom Meshery extensions should be considered trusted infrastructure code and reviewed accordingly.

---

## Why this is preferable to a generic Kubernetes dashboard

Tools such as Lens are excellent for examining Kubernetes resources, but the goal here is higher-level abstraction.

The desired progression is:

```text
Generic Kubernetes dashboard
        |
        v
Pods / Deployments / Services

versus

UBL-21 Meshery model
        |
        v
Invoice API / Validator / Worker / Tenant Database
        |
        v
Generated or mapped Kubernetes resources
```

The second approach allows the operational view to use the language of the application domain rather than exposing every Kubernetes implementation detail to the operator.

---

## Recommendation

Meshery is worth prototyping for UBL-21.

The principal value is not merely graphical Kubernetes design. The stronger proposition is the possibility of defining a **UBL-21-specific infrastructure vocabulary**, manipulating that architecture visually, associating it with multiple EKS/Kubernetes clusters, and retaining Git + Terraform + Helm + Argo CD underneath as the durable deployment mechanism.

The first experiment should therefore be deliberately small: connect one development cluster, model the validator deployment in Kanvas, and determine how naturally the visual model can coexist with the repository's existing Git workflow.

---

## References

- Meshery documentation: https://docs.meshery.io/
- Multi-cluster and multi-cloud: https://docs.meshery.io/installation/production/multi-cluster-and-multi-cloud/
- Meshery Models: https://docs.meshery.io/concepts/logical/models/
- Meshery Workspaces: https://docs.meshery.io/concepts/logical/workspaces/
- Extensibility: https://docs.meshery.io/reference/extensibility/
- Security hardening: https://docs.meshery.io/installation/production/security-hardening/
- Layer5 Kanvas: https://docs.layer5.io/kanvas/
