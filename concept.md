# OpenMCP + Platform-Mesh Integration — Concept

This document describes the end-to-end flow of the `local-event-showcase` demo project. The demo wires together an OpenMCP onboarding cluster with a [platform-mesh](https://platform-mesh.io) KCP installation so that every new account workspace gets a dedicated MCP instance. Users onboard Crossplane through a dedicated UI (`openmcp-onboarding-ui`) that guides them through activation and configuration.

## Preconditions

The following must be in place before running the integration:

| Component | Description |
|-----------|-------------|
| `platform` kind cluster | Core OpenMCP infrastructure (Flux installed during integration) |
| `onboarding` kind cluster | Hosts the `openmcp-init-operator` |
| `platform-mesh` kind cluster | Runs KCP and the platform portal |
| `platform-mesh` resource | Must be in `Ready` state inside the `platform-mesh` cluster |

---

## 1. Installation Phase (one-time setup)

```mermaid
sequenceDiagram
    actor Admin as Admin/User
    participant Script as Integration Script<br/>(hack/integrate-openmcp-platform-mesh.sh)
    participant PlatformMesh as platform-mesh cluster<br/>(KCP host)
    participant KCP as KCP<br/>(logical workspaces)
    participant Onboarding as Onboarding Cluster
    participant Platform as Platform Cluster

    Admin->>Script: run hack/integrate-openmcp-platform-mesh.sh

    rect rgb(230, 240, 255)
        Note over Script,KCP: Configure KCP workspace hierarchy

        Script->>PlatformMesh: verify platform-mesh resource is Ready
        Script->>KCP: create workspace root:providers
        Script->>KCP: create workspace root:providers:openmcp
        Script->>PlatformMesh: patch platform-mesh extraDefaultAPIBindings<br/>(workspaceType: root:account → export: openmcp.cloud)
    end

    rect rgb(235, 230, 255)
        Note over Script,KCP: Register OpenMCP as a provider in KCP

        Script->>KCP: lookup identityHash from core.platform-mesh.io APIExport
        Script->>KCP: apply APIExport, RBAC, ProviderMetadata<br/>to root:providers:openmcp workspace
    end

    rect rgb(230, 255, 230)
        Note over Script,Platform: Deploy infrastructure & operator

        Script->>Platform: install Flux (source, kustomize, helm, notification controllers)
        Script->>Onboarding: create namespace openmcp-system
        Script->>Onboarding: create secret kcp-openmfp-system-kubeconfig
        Script->>Script: build openmcp-init-operator Docker image
        Script->>Onboarding: load image into kind cluster
        Script->>Onboarding: helm install openmcp-init-operator<br/>(namespace: openmcp-system)
        Onboarding-->>Script: operator deployment Ready
    end
```

---

## 2. Usage Phase (per new account workspace)

```mermaid
sequenceDiagram
    actor User as User
    participant Portal as Platform Portal
    participant UI as openmcp-onboarding-ui
    participant GQL as kubernetes-graphql-gateway
    participant KCP as KCP<br/>(logical workspaces)
    participant InitAgent as init-agent
    participant Operator as openmcp-init-operator
    participant Onboarding as Onboarding Cluster
    participant MCP as MCP Cluster<br/>(provisioned per account)

    User->>Portal: create account
    Portal->>KCP: create account workspace (type: root:account)

    rect rgb(255, 245, 220)
        Note over KCP,InitAgent: Workspace Initialization (init-agent)

        KCP->>InitAgent: LogicalCluster detected (Initializing state)
        Note over KCP: APIBinding to openmcp.cloud already created<br/>via extraDefaultAPIBindings on workspace type.<br/>This enables the ManagedControlPlane and Crossplane APIs in the workspace.
        InitAgent->>KCP: create ManagedControlPlane resource in workspace
        InitAgent->>KCP: remove openmcp initializer from LogicalCluster
        KCP-->>KCP: workspace transitions to Ready
    end

    rect rgb(230, 240, 255)
        Note over KCP,MCP: MCP Provisioning (ManagedControlPlaneReconciler)

        KCP->>Operator: ManagedControlPlane detected

        Note over Operator: CreateMCPSubroutine
        Operator->>Onboarding: create ManagedControlPlaneV2
        Onboarding-->>Operator: MCP cluster provisioned & kubeconfig available
        Operator->>KCP: update ManagedControlPlane status (phase: MCPReady)

        Note over Operator: SetupSyncAgentSubroutine
        Operator->>KCP: create APIExport crossplane.services.openmcp.cloud
        Operator->>MCP: create Secret kcp-kubeconfig
        Operator->>MCP: helm install api-syncagent<br/>(bridges MCP ↔ KCP workspace)
        Operator->>KCP: update ManagedControlPlane status (phase: Ready)
    end

    rect rgb(235, 230, 255)
        Note over User,KCP: Crossplane Onboarding (openmcp-onboarding-ui)

        User->>Portal: navigate to OpenMCP → Crossplane
        Portal->>UI: load openmcp-onboarding-ui
        UI->>GQL: check APIBinding to crossplane.services.openmcp.cloud
        GQL->>KCP: get APIBinding
        KCP-->>GQL: not found
        GQL-->>UI: not found

        UI-->>User: show "Start using Crossplane" button
        User->>UI: click "Start using Crossplane"
        UI->>GQL: create APIBinding to crossplane.services.openmcp.cloud
        GQL->>KCP: create APIBinding
        KCP-->>GQL: created
        GQL-->>UI: Crossplane APIs now available in workspace

        UI->>GQL: check Crossplane resource
        GQL->>KCP: get Crossplane
        KCP-->>GQL: not found
        GQL-->>UI: not found
        UI-->>User: show Crossplane configuration<br/>(version: v1.20.1, provider: provider-kubernetes v0.15.0)
        User->>UI: confirm and save
        UI->>GQL: create Crossplane resource
        GQL->>KCP: create Crossplane
    end

    rect rgb(230, 255, 230)
        Note over KCP,MCP: Crossplane Setup (CrossplaneReconciler)

        KCP->>Operator: Crossplane resource detected in account workspace

        Note over Operator: CreateCrossplaneSubroutine
        Operator->>Onboarding: create Crossplane resource<br/>(copies provider spec from KCP)
        Onboarding-->>Operator: Crossplane Ready

        Note over Operator: InitializePublishedResourcesSubroutine
        Operator->>MCP: create PublishedResource: k8s-ProviderConfig
        Operator->>MCP: create PublishedResource: k8s-Object
        Operator->>MCP: create PublishedResource: k8s-ObservedObjectCollection
        MCP-->>KCP: api-syncagent publishes resources to workspace
        Note over KCP: crossplane.services.openmcp.cloud APIExport<br/>now includes Crossplane resource APIs

        Note over Operator: DeployContentConfigurationsSubroutine
        Operator->>KCP: create ContentConfigurations for published APIs<br/>(renders ProviderConfig, Object, ObservedObjectCollection<br/>via generic resource UI)
    end
```

---

## Key Participants

| Participant | Role |
|-------------|------|
| **Integration Script** | One-time bootstrap: creates KCP workspaces, deploys operator, wires platform-mesh |
| **KCP** | Multi-tenant control plane; hosts logical workspaces, ManagedControlPlane, and Crossplane resources per account |
| **platform-mesh cluster** | Runs KCP and the platform portal; owns the `platform-mesh` resource |
| **init-agent** | Watches LogicalClusters, creates ManagedControlPlane resource per workspace |
| **openmcp-init-operator** | Reconciles ManagedControlPlane and Crossplane resources |
| **openmcp-onboarding-ui** | Luigi micro-frontend: guides users through Crossplane activation and configuration |
| **Onboarding Cluster** | Hosts the `openmcp-init-operator` and `ManagedControlPlaneV2` resources |
| **MCP Cluster** | Provisioned per account; runs Crossplane and the KCP api-syncagent |
| **Platform Cluster** | Core OpenMCP infrastructure; Flux is installed here during Phase 1 |

---

## Notes

- The `init-agent` is the [KCP init-agent](https://github.com/kcp-dev/init-agent), deployed by platform-mesh. It is configured via `InitTemplate` and `InitTarget` resources to create a `ManagedControlPlane` resource in each new account workspace.
- The `openmcp-init-operator` reconciles `ManagedControlPlane` and `Crossplane` resources. It runs on the onboarding cluster.
- `ManagedControlPlane` is the domain resource that triggers MCP provisioning. It carries status phases (`MCPReady`, `Ready`) giving clear visibility into provisioning progress.
- The `openmcp-onboarding-ui` is a Luigi micro-frontend under the OpenMCP → Crossplane navigation node. It detects Crossplane state by checking for the APIBinding to `crossplane.services.openmcp.cloud` and the existence of a `Crossplane` resource. It drives a two-step onboarding: activate Crossplane (creates APIBinding), then configure it (creates Crossplane resource).
- Crossplane onboarding is **user-driven** — the operator does not create Crossplane resources automatically. The UI creates the APIBinding and Crossplane resource based on user choices.
- After Crossplane is ready and PublishedResources are created, the api-syncagent adds the published Crossplane resource APIs (ProviderConfig, Object, ObservedObjectCollection) to the `crossplane.services.openmcp.cloud` APIExport, making them available in the workspace.
- Network routing from MCP clusters to KCP uses `hostAliases` to map `localhost` to the `platform-mesh` Docker container IP, since KCP listens on `localhost:31000` (NodePort) inside the kind network.
- Published resources (`ProviderConfig`, `Object`, `ObservedObjectCollection`) are only initialized once the target Crossplane on the onboarding cluster reports all `*Ready` conditions as `True`.

---

## Implementation Plan

Each phase is independently deployable and verifiable before moving to the next.

### Phase 1 — ManagedControlPlane CRD & Operator Refactor

**Goal:** Replace the APIBinding-triggered reconciliation with a `ManagedControlPlane` custom resource.

**Changes:**
- Define `ManagedControlPlane` CRD in `api/core/v1alpha1/` with spec (empty for now) and status (phase: `Provisioning`, `MCPReady`, `Ready`; conditions)
- Create `ManagedControlPlaneReconciler` replacing `OpenMCPInitReconciler` — watches `ManagedControlPlane` instead of `APIBinding`
- Refactor `CreateMCPSubroutine` and `SetupSyncAgentSubroutine` to work against `ManagedControlPlane` instead of `APIBinding`
- Update status phases on the `ManagedControlPlane` resource after each subroutine completes
- Remove `OpenMCPInitReconciler` and the APIBinding watch logic
- Remove `DeployAPIExportBindingSubroutine` and `SetupFluxSubroutine` (dead code)
- Update `SetupSyncAgentSubroutine`: create APIExport as `crossplane.services.openmcp.cloud`, drop ProviderMetadata and ContentConfiguration creation
- Update Helm chart, RBAC markers, and `task generate`
- Update integration script to apply the new `ManagedControlPlane` APIResourceSchema to the provider workspace

**Validate:**
- Deploy to local kind setup
- Manually create a `ManagedControlPlane` resource in a KCP workspace
- Verify MCP is provisioned, sync agent deployed, status phases progress to `Ready`

---

### Phase 2 — KCP Init-Agent Configuration

**Goal:** Use the [KCP init-agent](https://github.com/kcp-dev/init-agent) (already deployed by platform-mesh) to automatically create `ManagedControlPlane` resources when new account workspaces are initialized.

**Changes:**
- Create `InitTemplate` manifest (`demo/manifests/init-agent/init-template.yaml`) that defines a `ManagedControlPlane` resource to be created in each new workspace
- Create `InitTarget` manifest (`demo/manifests/init-agent/init-target.yaml`) that connects the `root:account` workspace type to the `InitTemplate`
- Remove the `initializer` subcommand from `openmcp-init-operator` (replaced by the KCP init-agent)
- Remove `InitializeWorkspaceSubroutine`, `LogicalClusterReconciler`, and `InitializerConfig` from the operator
- Update integration script to apply init-agent manifests to the provider workspace

**Validate:**
- Deploy to local kind setup
- Create a new account workspace in KCP
- Verify: init-agent creates `ManagedControlPlane`, operator picks it up and provisions MCP

---

### Phase 3 — Onboarding UI (openmcp-onboarding-ui)

**Goal:** Build the Luigi micro-frontend that guides users through Crossplane activation.

**Changes:**
- Create `demo/openmcp-onboarding-ui/` — Luigi micro-frontend project
- Page 1: Check for APIBinding to `crossplane.services.openmcp.cloud` via kubernetes-graphql-gateway
  - Not found → show "Start using Crossplane" button
  - Found → proceed to page 2
- Page 2: Show Crossplane configuration (hardcoded: v1.20.1, provider-kubernetes v0.15.0)
  - Check if `Crossplane` resource exists → show status if yes
  - Not found → show "Confirm and save" → create `Crossplane` resource via graphql gateway
- Register as ContentConfiguration in the provider workspace (OpenMCP → Crossplane nav node)
- Update integration script to deploy the UI ContentConfiguration

**Validate:**
- Deploy to local setup
- Navigate to OpenMCP → Crossplane in the portal
- Click "Start using Crossplane" → verify APIBinding created
- Confirm Crossplane config → verify Crossplane resource created
- Verify operator picks up the Crossplane resource and reconciles it

---

### Phase 4 — ContentConfigurations for Published APIs

**Goal:** After Crossplane is ready, deploy ContentConfigurations so published APIs render in the portal via the generic resource UI.

**Changes:**
- Add `DeployContentConfigurationsSubroutine` to `CrossplaneReconciler`
- After `InitializePublishedResourcesSubroutine` completes, create ContentConfigurations in the KCP workspace for:
  - `k8s-ProviderConfig`
  - `k8s-Object`
  - `k8s-ObservedObjectCollection`
- Each ContentConfiguration points to the generic resource UI with the appropriate API group/version/resource

**Validate:**
- End-to-end: create account → init-agent seeds workspace → operator provisions MCP → user activates Crossplane via UI → operator installs Crossplane → published resources appear → ContentConfigurations deployed → resources visible and manageable in portal
