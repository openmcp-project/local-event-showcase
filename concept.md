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
| `gardener-local` kind cluster | Runs Gardener and the `gardener-init-operator` (local setup via `hack/setup-gardener-local.sh`) |

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
    participant GardenerLocal as Gardener-Local Cluster

    Admin->>Script: run hack/integrate-openmcp-platform-mesh.sh

    rect rgb(230, 240, 255)
        Note over Script,KCP: Configure KCP workspace hierarchy

        Script->>PlatformMesh: verify platform-mesh resource is Ready
        Script->>KCP: create workspace root:providers
        Script->>KCP: create workspace root:providers:openmcp
        Script->>KCP: create workspace root:providers:gardener
        Script->>PlatformMesh: patch platform-mesh extraDefaultAPIBindings<br/>(openmcp.cloud → root:providers:openmcp, gardener.cloud → root:providers:gardener)
    end

    rect rgb(235, 230, 255)
        Note over Script,KCP: Register OpenMCP as a provider in KCP

        Script->>KCP: lookup identityHash from core.platform-mesh.io APIExport
        Script->>KCP: apply APIExport, RBAC, ProviderMetadata<br/>to root:providers:openmcp workspace
        Script->>KCP: apply gardener.cloud APIExport, RBAC<br/>to root:providers:gardener workspace
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
        Script->>GardenerLocal: create namespace openmcp-system
        Script->>GardenerLocal: create secret kcp-openmfp-system-kubeconfig<br/>(pointing to root:providers:gardener)
        Script->>Script: build gardener-init-operator Docker image
        Script->>GardenerLocal: load image into kind cluster
        Script->>GardenerLocal: helm install gardener-init-operator<br/>(namespace: openmcp-system)
        GardenerLocal-->>Script: operator deployment Ready
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

## 3. Gardener Provisioning Phase (user-driven per account workspace)

```mermaid
sequenceDiagram
    actor User as User
    participant KCP as KCP<br/>(logical workspaces)
    participant Operator as gardener-init-operator<br/>(on gardener-local)
    participant Gardener as Gardener<br/>(gardener-local)

    Note over KCP: APIBinding to gardener.cloud created<br/>via extraDefaultAPIBindings (from root:providers:gardener)
    Note over User,KCP: GardenerProject is NOT auto-created by init-agent.<br/>Users create it explicitly (e.g. via Crossplane compositions or directly).

    User->>KCP: create GardenerProject resource in workspace

    rect rgb(255, 235, 235)
        Note over KCP,Gardener: Gardener Project Provisioning (GardenerProjectReconciler)

        KCP->>Operator: GardenerProject detected

        Note over Operator: CreateGardenerProjectSubroutine
        Operator->>Gardener: create Project (core.gardener.cloud/v1beta1)
        Gardener-->>Operator: Project Ready, namespace garden-<clusterID> exists
        Operator->>Gardener: create ServiceAccount openmcp in garden-<clusterID>
        Operator->>Gardener: create RoleBinding (admin) for SA
        Operator->>KCP: update GardenerProject status (phase: ProjectReady)

        Note over Operator: SetupGardenerAccessSubroutine
        Operator->>Gardener: create SA token Secret
        Gardener-->>Operator: token available
        Operator->>Operator: build kubeconfig (Gardener API + bearer token)
        Operator->>KCP: store Secret gardener-project-kubeconfig in workspace
        Operator->>KCP: update GardenerProject status (phase: Ready)
    end
```

---

## 4. Tool Provisioning Phase (KRO, Flux, OCM Controller)

```mermaid
sequenceDiagram
    actor User as User
    participant Portal as Platform Portal
    participant UI as Tool Onboarding UI<br/>(KRO / Flux / OCM)
    participant GQL as kubernetes-graphql-gateway
    participant KCP as KCP<br/>(logical workspaces)
    participant Operator as openmcp-init-operator
    participant MCP as MCP Cluster<br/>(provisioned per account)

    User->>Portal: navigate to OpenMCP → KRO / Flux / OCM Controller
    Portal->>UI: load tool onboarding UI (Angular WebComponent)
    UI->>GQL: check catalog (KROCatalog / FluxCatalog / OCMControllerCatalog)
    GQL->>KCP: get catalog resource (via openmcp-internal.cloud binding)
    KCP-->>GQL: catalog with available versions
    GQL-->>UI: available versions

    UI-->>User: show tool configuration (version selector)
    User->>UI: select version and confirm
    UI->>GQL: create tool enablement resource (KRO / Flux / OCMController)
    GQL->>KCP: create resource in account workspace
    KCP-->>GQL: created
    GQL-->>UI: tool enablement resource created

    rect rgb(230, 255, 230)
        Note over KCP,MCP: Tool Setup (KROReconciler / FluxReconciler / OCMControllerReconciler)

        KCP->>Operator: tool enablement resource detected in account workspace

        Note over Operator: DeployCRDsSubroutine
        Operator->>KCP: deploy tool upstream CRDs into user's KCP workspace
        KCP-->>Operator: CRDs registered in workspace

        Note over Operator: InstallToolSubroutine
        Operator->>MCP: helm install tool<br/>(kubeconfig points to user's KCP workspace)
        MCP-->>Operator: tool controller running, targeting KCP workspace

        Note over Operator: DeployToolContentConfigurationsSubroutine
        Operator->>KCP: create ContentConfigurations for tool APIs<br/>(UI navigation entries in portal)
        Operator->>KCP: update tool resource status (phase: Ready)
    end
```

---

## Key Participants

| Participant | Role |
|-------------|------|
| **Integration Script** | One-time bootstrap: creates KCP workspaces, deploys operator, wires platform-mesh |
| **KCP** | Multi-tenant control plane; hosts logical workspaces, ManagedControlPlane, and Crossplane resources per account |
| **platform-mesh cluster** | Runs KCP and the platform portal; owns the `platform-mesh` resource |
| **init-agent** | Watches LogicalClusters, creates ManagedControlPlane resource per workspace (no longer creates GardenerProject) |
| **openmcp-init-operator** | Reconciles ManagedControlPlane and Crossplane resources |
| **openmcp-onboarding-ui** | Luigi micro-frontend: guides users through Crossplane activation and configuration |
| **KRO / Flux / OCM Onboarding UI** | Angular WebComponent micro-frontends: guide users through KRO, Flux, and OCM Controller activation and version selection |
| **KRO / Flux / OCM Controller** | Tool controllers running on the MCP cluster; reconcile against the user's KCP workspace directly via kubeconfig (no sync-agent) |
| **Onboarding Cluster** | Hosts the `openmcp-init-operator` and `ManagedControlPlaneV2` resources |
| **MCP Cluster** | Provisioned per account; runs Crossplane and the KCP api-syncagent |
| **Platform Cluster** | Core OpenMCP infrastructure; Flux is installed here during Phase 1 |
| **gardener-init-operator** | Reconciles GardenerProject resources: creates Gardener projects, sets up access. Runs on gardener-local cluster. |
| **Gardener (gardener-local)** | Local Gardener installation; provides project-based resource isolation. Hosts the gardener-init-operator. |

---

## Notes

- The `init-agent` is the [KCP init-agent](https://github.com/kcp-dev/init-agent), deployed by platform-mesh. It is configured via `InitTemplate` and `InitTarget` resources to create a `ManagedControlPlane` resource in each new account workspace. It does **not** create GardenerProject resources — those are user-driven.
- The `openmcp-init-operator` reconciles `ManagedControlPlane` and `Crossplane` resources. It runs on the onboarding cluster.
- The `gardener-init-operator` reconciles `GardenerProject` resources. It uses `unstructured.Unstructured` to interact with the Gardener API to avoid pulling in the massive Gardener Go dependency tree. It runs on the **gardener-local** cluster (not the onboarding cluster), giving it direct access to the Gardener API via in-cluster config.
- Gardener is an **independent provider** with its own KCP workspace (`root:providers:gardener`) and APIExport (`gardener.cloud`). This decouples Gardener from the OpenMCP provider workspace.
- `ManagedControlPlane` is the domain resource that triggers MCP provisioning. It carries status phases (`MCPReady`, `Ready`) giving clear visibility into provisioning progress.
- The `openmcp-onboarding-ui` is a Luigi micro-frontend under the OpenMCP → Crossplane navigation node. It detects Crossplane state by checking for the APIBinding to `crossplane.services.openmcp.cloud` and the existence of a `Crossplane` resource. It drives a two-step onboarding: activate Crossplane (creates APIBinding), then configure it (creates Crossplane resource).
- Crossplane onboarding is **user-driven** — the operator does not create Crossplane resources automatically. The UI creates the APIBinding and Crossplane resource based on user choices.
- After Crossplane is ready and PublishedResources are created, the api-syncagent adds the published Crossplane resource APIs (ProviderConfig, Object, ObservedObjectCollection) to the `crossplane.services.openmcp.cloud` APIExport, making them available in the workspace.
- Network routing from MCP clusters to KCP uses `hostAliases` to map `localhost` to the `platform-mesh` Docker container IP, since KCP listens on `localhost:31000` (NodePort) inside the kind network.
- Published resources (`ProviderConfig`, `Object`, `ObservedObjectCollection`) are only initialized once the target Crossplane on the onboarding cluster reports all `*Ready` conditions as `True`.
- **Crossplane vs. KRO/Flux/OCM architecture**: Crossplane uses a sync-agent bridge — the `api-syncagent` runs on the MCP cluster and bridges resources between KCP and MCP via a dynamic `crossplane.services.openmcp.cloud` APIExport. KRO, Flux, and OCM Controller use a simpler direct pattern: the operator deploys the tool's upstream CRDs directly into the user's KCP workspace, then installs the tool controller on the MCP cluster with a kubeconfig that points at that KCP workspace. The tool controller reconciles against KCP directly, with no sync-agent in between.
- Catalog resources (`KROCatalog`, `FluxCatalog`, `OCMControllerCatalog`) are published via the `openmcp-internal.cloud` APIExport and are intended to be replicated via `CachedResource` (read-only, provider-managed). CachedResources are currently non-functional; catalog data is hardcoded in the onboarding UI assets until the feature is fixed.
- Each new tool API group is a separate KCP service domain: `kro.services.openmcp.cloud`, `flux.services.openmcp.cloud`, and `ocm.services.openmcp.cloud`. All three are published via the existing `openmcp.cloud` APIExport (enablement resources) and `openmcp-internal.cloud` APIExport (catalog resources).

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

---

### Phase 5 — GardenerProject + gardener-init-operator (independent provider)

**Goal:** Gardener is an independent provider with its own KCP workspace (`root:providers:gardener`). The `gardener-init-operator` runs on the `gardener-local` cluster and reconciles `GardenerProject` resources, creating Gardener Projects, ServiceAccounts, and kubeconfig Secrets. GardenerProject resources are **user-driven** — not auto-created by the init-agent.

**Changes:**
- Define `GardenerProject` CRD in `gardener-init-operator/api/v1alpha1/` with status phases (`Provisioning`, `ProjectReady`, `Ready`)
- Create `gardener-init-operator` — new Go operator (`demo/gardener-init-operator/`)
- `CreateGardenerProjectSubroutine`: creates Gardener Project (unstructured), ServiceAccount, RoleBinding
- `SetupGardenerAccessSubroutine`: creates SA token Secret, builds kubeconfig, stores in KCP workspace
- Register `gardener.cloud` APIExport with GardenerProject APIResourceSchema in the `root:providers:gardener` workspace (separate from openmcp)
- No `GardenerProject` InitTemplate — users create GardenerProject explicitly (e.g., via Crossplane compositions or directly)
- Update integration script to:
  - Create `root:providers:gardener` workspace
  - Apply gardener manifests to the gardener workspace (not openmcp)
  - Deploy gardener-init-operator to `gardener-local` cluster (not onboarding)
  - Create KCP kubeconfig secret pointing to `root:providers:gardener`
- Migrate both operators to new golang-commons config pattern (no mapstructure/viper)

**Validate:**
- `gardener.cloud` APIExport exists in `root:providers:gardener` workspace (not in openmcp)
- New account workspace has APIBindings to both `openmcp.cloud` (from `root:providers:openmcp`) and `gardener.cloud` (from `root:providers:gardener`)
- Init-agent creates only `ManagedControlPlane` (no auto-created GardenerProject)
- Manually create a `GardenerProject` in an account workspace — gardener-init-operator (on gardener-local) picks it up
- gardener-init-operator provisions Gardener Project, SA, kubeconfig
- `kubectl get gardenerprojects` → `Phase: Ready`
- `kubectl get secret gardener-project-kubeconfig` exists in workspace

---

### Phase 6 — KRO, Flux, OCM Controller Tools

**Goal:** Extend the demo with three new tools — KRO, Flux, and OCM Controller — following a direct CRD + controller-on-MCP pattern (no sync-agent). Each tool has a user-facing onboarding UI, a catalog resource for version discovery, and three operator subroutines.

**Changes:**
- Define new API groups in `openmcp-init-operator/api/`:
  - `kro.services.openmcp.cloud`: `KRO` (enablement), `KROCatalog` (version catalog)
  - `flux.services.openmcp.cloud`: `Flux` (enablement), `FluxCatalog` (version catalog)
  - `ocm.services.openmcp.cloud`: `OCMController` (enablement), `OCMControllerCatalog` (version catalog)
- Add `KROReconciler`, `FluxReconciler`, `OCMControllerReconciler` to `openmcp-init-operator`; each reconciler runs three subroutines:
  - `DeployCRDsSubroutine`: deploy the tool's upstream CRDs directly into the user's KCP workspace
  - `InstallToolSubroutine`: Helm install the tool controller on the MCP cluster with a kubeconfig targeting the user's KCP workspace
  - `DeployToolContentConfigurationsSubroutine`: create ContentConfiguration resources in the KCP workspace for portal navigation
- Register new `APIResourceSchemas` for all six types in the `root:providers:openmcp` workspace; publish enablement types via the existing `openmcp.cloud` APIExport and catalog types via `openmcp-internal.cloud` APIExport
- Create `CachedResource` manifests for catalog types (replicate catalog data to KCP cache; currently non-functional — catalog data is hardcoded in UI assets as a workaround)
- Create catalog instance manifests (`KROCatalog`, `FluxCatalog`, `OCMControllerCatalog`) in the provider workspace
- Add Angular WebComponent onboarding UIs (`kro-onboarding`, `flux-onboarding`, `ocm-onboarding`) to `openmcp-onboarding-ui`:
  - Read catalog resource to populate version selector
  - Create enablement resource (`KRO` / `Flux` / `OCMController`) on confirmation
- Update integration script to apply new manifests and register new ContentConfigurations

**Validate:**
- New `APIResourceSchemas` and updated `openmcp.cloud` / `openmcp-internal.cloud` APIExports are present in `root:providers:openmcp`
- Navigate to portal → OpenMCP → KRO (/ Flux / OCM Controller) → onboarding UI loads, version selector populated
- Confirm selection → enablement resource created in account workspace
- Operator picks up resource → DeployCRDs runs (CRDs visible in KCP workspace) → InstallTool runs (tool controller running on MCP, targeting KCP) → DeployToolContentConfigurations runs (ContentConfigurations visible in workspace)
- Tool resource status reaches `Phase: Ready`
- Portal navigation entries for tool APIs appear
