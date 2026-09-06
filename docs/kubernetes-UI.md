# kubernetes-UI

## Purpose

This report evaluates **Meshery** as a candidate visual Kubernetes architecture and operations UI for **e-ubl21**.

The original requirement is broader than a read-only Kubernetes dashboard. The desired tool should help e-ubl21 operators and developers:

- visualize Kubernetes architecture in terms of clusters, namespaces, workloads, pods, containers, services, storage and relationships;
- modify the desired architecture visually rather than editing YAML exclusively by hand;
- inspect the live state of one or more e-ubl21 Kubernetes clusters;
- deploy, validate and operate the resulting configuration;
- keep architecture definitions under version control;
- support local Minikube, on-premises Kubernetes and cloud Kubernetes deployments;
- eventually provide a practical administration surface for multiple customer or environment clusters.

## Executive conclusion

**Meshery is a strong candidate for the e-ubl21 Kubernetes UI and is worth prototyping.**

Its current feature set is substantially closer to the e-ubl21 requirement than a simple topology viewer. Meshery combines a visual infrastructure designer with live cluster operations, multi-cluster management, Git-connected infrastructure design, extensibility and support for Kubernetes resource models.

The recommended direction is **not** to fork Meshery immediately. e-ubl21 should first run Meshery as a separate management-plane component and connect it to a development Minikube cluster. The project should then model the existing e-ubl21 Kubernetes deployment in Meshery and evaluate whether the visual model is accurate enough to become the preferred architecture/operations UI.

If the prototype succeeds, a later phase can customize the UI, schemas, provider integration or adapters specifically for e-ubl21.

## What Meshery is

Meshery describes itself as a self-service engineering platform for the collaborative design and operation of cloud-native infrastructure.

It can run as a collection of containers either:

- on a Docker host, outside the target Kubernetes cluster; or
- inside Kubernetes.

For production use, the Meshery documentation recommends Kubernetes/Helm rather than a single Docker host.

Meshery provides both a web UI and the `mesheryctl` command-line interface.

Primary references:

- https://meshery.io/
- https://docs.meshery.io/
- https://github.com/meshery/meshery

## Licensing

The core Meshery project is open source under the **Apache License 2.0**.

This is favorable for e-ubl21 because Apache 2.0 is permissive and is generally compatible with commercial distribution and internal customization, subject to the usual notice and attribution requirements.

This does not imply that every hosted service, extension or third-party integration in the wider Meshery ecosystem has identical licensing. Any component incorporated into a commercial e-ubl21 distribution should still be reviewed independently.

## Visual architecture design

Meshery's visual designer is the most relevant capability for e-ubl21.

The current Meshery documentation describes a visual designer in which infrastructure can be built by dragging components from a large catalog of versioned Kubernetes and cloud-native models.

The designer supports relationships between resources rather than treating objects as isolated boxes. Examples documented by Meshery include:

- Deployment to ConfigMap relationships;
- Namespace parent/child relationships;
- Pod to PersistentVolumeClaim relationships;
- Ingress to Service relationships;
- Service to Deployment selector relationships;
- ServiceAccount and ClusterRole relationships.

For e-ubl21 this is important because the desired diagram is not merely a drawing. The useful model needs to represent actual Kubernetes semantics.

A Meshery design is a declarative description of desired infrastructure state. Designs can be produced through the visual UI or authored as configuration.

## Kubernetes resource coverage

Meshery currently exposes a Kubernetes model with a broad set of Kubernetes resource types. The published model includes, among others:

- Namespace;
- Service;
- Pod;
- Node;
- Deployment;
- Secret;
- CronJob;
- DaemonSet;
- ClusterRole;
- ClusterRoleBinding;
- APIService;
- Container;
- CustomResourceDefinition;
- storage-related resources;
- policy and authorization resources.

This is sufficient in principle to represent the major e-ubl21 deployment concepts.

## Mapping to e-ubl21

A first e-ubl21 Meshery design should represent the real deployment, not an idealized diagram.

The initial model should include at least the following logical workloads where they exist in the current manifests:

- public API/server workload;
- orchestration components;
- NATS / JetStream;
- XSD validation worker;
- EN 16931 validation worker;
- Peppol validation worker;
- Services and internal network relationships;
- ConfigMaps and Secrets;
- persistent volumes used by stateful components;
- observability components where deployed;
- ingress or external exposure resources;
- autoscaling policies where applicable.

The exact design should be generated from or checked against the repository's actual Kubernetes manifests. Meshery should not become a second, contradictory architectural truth.

## Live cluster operations

Meshery is more useful than a static architecture editor because it also provides an operations mode.

According to current Meshery documentation, operations can include:

- discovering and inspecting live resources;
- validating designs;
- performing dry runs;
- deploying designs;
- cloning deployments;
- undeploying designs;
- streaming logs;
- inspecting live resources;
- opening an interactive terminal into pods and containers.

For e-ubl21 this creates the possibility of using one visual interface to move between the intended architecture and the live system.

## Multi-cluster support

Multi-cluster management is one of Meshery's core capabilities.

An out-of-cluster Meshery Server can connect to multiple Kubernetes contexts from kubeconfig files. Multiple kubeconfig files can also be imported through the UI.

For managed connections Meshery can deploy per-cluster components such as Meshery Operator, MeshSync and a broker. Meshery also supports an embedded MeshSync mode in which these additional components are not necessarily installed in every cluster.

This fits the long-term e-ubl21 requirement well because a single management plane could potentially visualize and administer:

- developer Minikube clusters;
- test/staging clusters;
- production cloud clusters;
- customer on-premises clusters;
- multiple geographic or tenant-specific deployments.

A production design must determine carefully which clusters are allowed to be centrally managed and what credentials and permissions the management plane receives.

## Deployment approaches for e-ubl21

### Option A - Meshery on the developer workstation

Meshery runs using Docker and connects to Minikube through the developer's kubeconfig.

Advantages:

- simplest prototype;
- minimal changes to e-ubl21;
- low risk;
- no management-plane workload inside the e-ubl21 cluster.

This is the recommended first experiment.

### Option B - Dedicated Meshery management cluster

Meshery runs on a separate Kubernetes cluster and manages multiple e-ubl21 clusters.

Advantages:

- clean separation between application and management plane;
- suited to multi-cluster production operations;
- management UI remains available even if one target cluster fails.

This is the preferred long-term topology if Meshery becomes an operational tool.

### Option C - Meshery inside each e-ubl21 cluster

Meshery is installed alongside e-ubl21 workloads.

This is technically possible but is not the preferred general architecture because it duplicates the management plane and couples its availability to each individual e-ubl21 cluster.

## GitOps and source control

Meshery supports Git-connected infrastructure workflows and visual GitOps concepts.

This is particularly attractive for e-ubl21 because visual editing should not bypass the repository.

The desired e-ubl21 workflow should be:

1. Git remains the authoritative desired-state source.
2. Meshery imports or represents that desired state visually.
3. A proposed visual modification produces a reviewable configuration change.
4. The change is committed through the normal repository / pull-request workflow.
5. CI validates the generated Kubernetes configuration.
6. Deployment occurs through the approved deployment mechanism.
7. Meshery visualizes the resulting live cluster and identifies drift.

Meshery should therefore be treated as an **Infrastructure-as-Design interface over Infrastructure-as-Code**, not as an uncontrolled imperative configuration console.

## Extensibility

Meshery is explicitly designed for extension.

Current documented extension mechanisms include:

- UI extension points;
- APIs;
- authorization integration;
- providers;
- adapters;
- models;
- schemas and vendor extensions;
- build-time extensions.

This provides several possible future e-ubl21 integrations.

### e-ubl21-specific model

A higher-level model could eventually represent e-ubl21 concepts such as:

- API service;
- validation stage;
- XSD worker;
- EN worker;
- Peppol worker;
- NATS stream;
- validation profile;
- validation-asset release;
- deployment environment.

Such components could visually map onto lower-level Kubernetes resources.

### e-ubl21 provider or extension

A future extension could expose application-specific information that Kubernetes alone does not know, for example:

- installed UBL release;
- EN 16931 ruleset release;
- Peppol BIS release;
- validation-asset checksums;
- queue depths;
- worker throughput;
- validation-stage failure rates;
- current validation pipeline state;
- active deployment version.

This could make Meshery not only a Kubernetes UI but eventually an **e-ubl21 operations UI**.

## Observability integration

Meshery should complement, not replace, the project's observability stack.

Kubernetes architecture and live-resource inspection belong naturally in Meshery. Historical metrics, alerting, traces and log analytics should remain in purpose-built observability components.

A practical split would be:

- **Meshery**: architecture, topology, desired state, deployment and live Kubernetes operations;
- **Prometheus-compatible metrics stack**: measurements and alerts;
- **OpenTelemetry**: instrumentation and telemetry transport;
- **log/tracing backend**: historical debugging and analysis.

Links from the Meshery interface to deeper observability views could be considered later.

## Security considerations

Meshery becomes a high-value administrative component because it may have permissions to alter Kubernetes infrastructure.

A production adoption must therefore address:

- least-privilege Kubernetes RBAC;
- separation between read-only and deployment-capable users;
- authentication and identity-provider integration;
- credential storage;
- auditability;
- cluster isolation;
- network exposure of the Meshery UI;
- TLS;
- whether shell/terminal access to pods should be permitted;
- whether customer clusters may be centrally managed;
- backup and recovery of Meshery configuration and state.

For customer on-premises deployments, a read-only or locally controlled Meshery instance may be preferable to granting a central e-ubl21 service administrative access to customer clusters.

## Potential concerns

### Meshery is a substantial platform

Meshery is not a tiny Kubernetes diagramming library. Adopting it introduces another operational platform with its own lifecycle, releases and security surface.

It should only be adopted if its operational value justifies this additional component.

### Do not fork too early

Although the project is extensible and open source, maintaining a private fork would create long-term merge and upgrade costs.

The first implementation should use the upstream product with configuration and supported extension mechanisms wherever possible.

### Git must remain authoritative

Direct graphical deployment can be convenient but may create configuration drift if the visual editor becomes independent of Git.

For e-ubl21, significant production architecture changes should continue through source control and CI.

### Need to validate import/export fidelity

Before adopting Meshery as the architecture editor, the project must test whether importing the existing e-ubl21 manifests and exporting/editing them preserves all relevant Kubernetes details.

This is the most important technical proof-of-concept question.

## Suggested proof of concept

The first experiment should be deliberately small.

### Phase 1 - local installation

Install Meshery on the development machine using Docker and connect it to the existing Minikube cluster.

### Phase 2 - discovery

Allow Meshery to discover the running e-ubl21 namespace and inspect how accurately it represents:

- deployments;
- pods;
- services;
- ConfigMaps;
- Secrets without exposing secret values;
- persistent volumes;
- NATS;
- validation workers;
- relationships between components.

### Phase 3 - import the desired architecture

Import the Kubernetes manifests or Helm/Kustomize output used by e-ubl21 and create an e-ubl21 Meshery Design.

Compare the design to the live topology.

### Phase 4 - controlled visual modification

Make a harmless visual configuration change such as changing a worker replica count or resource request.

Inspect the resulting configuration difference before applying it.

The key question is whether the resulting diff is sufficiently deterministic and clean to use in normal code review.

### Phase 5 - Git workflow

Store the resulting Meshery design in source control and determine whether the design can coexist cleanly with the current Kubernetes manifests without creating two competing desired-state formats.

### Phase 6 - operational test

Use Meshery to:

- inspect a worker pod;
- view its logs;
- observe a rollout;
- inspect resource relationships;
- switch between Minikube and another test cluster if available.

## Proposed repository organization

If the proof of concept succeeds, consider adding:

```text
deploy/
  meshery/
    README.md
    values.yaml
    designs/
      e-ubl21.yaml
```

The precise structure should follow the repository's deployment organization at the time of implementation.

The Meshery server itself should not initially be embedded in the primary e-ubl21 application image.

## Recommended architectural role

Meshery should be positioned as an optional **management-plane UI**, not as part of the validation request path.

Conceptually:

```text
                     GitHub / Git
                         |
                         v
               +--------------------+
               |      Meshery       |
               | visual design + UI |
               +---------+----------+
                         |
              Kubernetes API / kubeconfig
                         |
        +----------------+----------------+
        |                |                |
        v                v                v
   Minikube/dev      staging         production
        |                |                |
        +---------- e-ubl21 clusters ----+
```

Meshery must have **no dependency relationship with validation availability**. If Meshery is unavailable, e-ubl21 validation must continue normally.

## Evaluation scorecard

| Requirement | Meshery fit | Notes |
|---|---|---|
| Visual Kubernetes topology | Strong | Core capability |
| Visual architecture editing | Strong | Designer supports declarative infrastructure design |
| Live cluster visualization | Strong | Resource discovery and operations |
| Modify Kubernetes architecture | Strong | Validate/dry-run/deploy workflows available |
| Multi-cluster | Strong | Central management is a core use case |
| Minikube support | Strong | Kubernetes/kubeconfig connection model fits |
| On-prem support | Strong | Can run locally or on Kubernetes |
| Cloud Kubernetes support | Strong | Multi-cloud/multi-cluster design |
| Git integration | Strong | Git-connected visual design/GitOps is supported |
| Open-source suitability | Strong | Core project Apache 2.0 |
| Extensibility for e-ubl21 | Strong | UI, provider, adapters, models and schemas |
| Lightweight dependency | Weak/Medium | Meshery is a substantial platform |
| Application-specific e-ubl21 knowledge | Not built in | Would require extension/custom model |
| Safe production change governance | Conditional | Must preserve Git/PR/CI as authority |

## Recommendation

Proceed with a **Meshery proof of concept** for e-ubl21.

Do not make Meshery mandatory for the current validation pipeline and do not fork the project at this stage.

The first acceptance test should answer one concrete question:

> Can Meshery import/discover the real e-ubl21 Kubernetes architecture, display it accurately, allow a controlled visual change, and produce a clean configuration change suitable for Git review without losing Kubernetes semantics?

If the answer is yes, Meshery is a credible foundation for the project's Kubernetes UI and could later evolve into a broader e-ubl21 cluster administration interface.

If the answer is no, the project should retain Meshery only as an optional visualization/inspection tool and evaluate a more focused Kubernetes UI or a custom topology editor.

## Sources checked

Current review performed 2026-09-06 against:

- Meshery project overview and documentation;
- Meshery Kubernetes model documentation;
- Meshery multi-cluster and multi-cloud documentation;
- Meshery installation and production-deployment documentation;
- Meshery design/configuration-management documentation;
- Meshery adapter and extensibility documentation;
- current Meshery GitHub organization/repositories.
