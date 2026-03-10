# Crossplane Gardener Shoot Demo

Create Gardener Shoots declaratively via Crossplane Claims on a local Gardener setup (`kind-gardener-local`).

## Prerequisites

- `kind-gardener-local` cluster running with Gardener (`task gardener:local`)
- At least one Gardener Project exists (the `local` project with namespace `garden-local` is created by default)
- `helm`, `kubectl` CLI tools installed

## Architecture

```
GardenerShoot (Claim)
  └─ XGardenerShoot (Composite Resource)
       └─ Object (provider-kubernetes)
            └─ Shoot (core.gardener.cloud/v1beta1)
```

Crossplane's `provider-kubernetes` uses `InjectedIdentity` (in-cluster credentials) to talk to the Gardener API, which is aggregated into the kube-apiserver. A `function-patch-and-transform` pipeline patches Claim parameters into the Shoot spec.

## Directory Structure

```
demo/crossplane/
├── 01-provider/          # Crossplane provider + function setup
│   ├── deployment-runtime-config.yaml   # SA name for deterministic RBAC binding
│   ├── provider-kubernetes.yaml         # provider-kubernetes v0.15.0
│   ├── function-patch-and-transform.yaml # Required for Crossplane v2 pipeline mode
│   └── provider-config.yaml             # InjectedIdentity (in-cluster) credentials
├── 02-rbac/              # RBAC for provider-kubernetes to manage Shoots
│   ├── clusterrole-shoot-admin.yaml       # CRUD on shoots, read on projects/seeds/cloudprofiles
│   └── clusterrolebinding-shoot-admin.yaml # Binds to provider-kubernetes SA
├── 03-xrd/               # XRD + Composition
│   ├── definition.yaml   # Defines GardenerShoot Claim API
│   └── composition.yaml  # Maps Claim params → Gardener Shoot via Object
└── 04-claims/            # Example Claims
    └── example-shoot-claim.yaml
```

## Quick Start

### One-command install

```bash
task crossplane:install
```

This installs everything: Crossplane, provider-kubernetes, function-patch-and-transform, RBAC, XRD, and Composition.

### Create a Shoot

```bash
task crossplane:test-claim
```

Or apply a custom Claim:

```yaml
apiVersion: demo.local/v1alpha1
kind: GardenerShoot
metadata:
  name: my-shoot
  namespace: default
spec:
  parameters:
    projectName: local          # Must be an existing Gardener project
    shootName: my-cluster       # Name for the Shoot resource
    kubernetesVersion: "1.31.1" # Optional, default: 1.31.1
    workerMinimum: 1            # Optional, default: 1
    workerMaximum: 2            # Optional, default: 2
```

```bash
kubectl --context kind-gardener-local apply -f my-claim.yaml
```

### Teardown

```bash
task crossplane:cleanup
```

## Step-by-step Manual Install

If you prefer to apply manifests individually rather than using `task crossplane:install`:

### 1. Install Crossplane

```bash
helm repo add crossplane-stable https://charts.crossplane.io/stable
helm repo update crossplane-stable
helm install crossplane crossplane-stable/crossplane \
  --namespace crossplane-system --create-namespace \
  --kube-context kind-gardener-local --wait
```

### 2. Install Provider + Function

```bash
kubectl --context kind-gardener-local apply -f demo/crossplane/01-provider/deployment-runtime-config.yaml
kubectl --context kind-gardener-local apply -f demo/crossplane/01-provider/provider-kubernetes.yaml
kubectl --context kind-gardener-local apply -f demo/crossplane/01-provider/function-patch-and-transform.yaml

# Wait for both to become healthy
kubectl --context kind-gardener-local wait provider.pkg.crossplane.io/provider-kubernetes \
  --for=condition=Healthy --timeout=180s
kubectl --context kind-gardener-local wait function.pkg.crossplane.io/function-patch-and-transform \
  --for=condition=Healthy --timeout=180s
```

### 3. Configure Provider + RBAC

```bash
kubectl --context kind-gardener-local apply -f demo/crossplane/01-provider/provider-config.yaml
kubectl --context kind-gardener-local apply -f demo/crossplane/02-rbac/
```

### 4. Apply XRD + Composition

```bash
kubectl --context kind-gardener-local apply -f demo/crossplane/03-xrd/definition.yaml
kubectl --context kind-gardener-local wait compositeresourcedefinition xgardenershoots.demo.local \
  --for=condition=Established --timeout=60s
kubectl --context kind-gardener-local apply -f demo/crossplane/03-xrd/composition.yaml
```

### 5. Create a Claim

```bash
kubectl --context kind-gardener-local apply -f demo/crossplane/04-claims/example-shoot-claim.yaml
```

## Verification

Check the full resource chain:

```bash
# Claim
kubectl --context kind-gardener-local get gardenershoots

# Composite resource
kubectl --context kind-gardener-local get xgardenershoots

# provider-kubernetes Object
kubectl --context kind-gardener-local get objects

# Actual Gardener Shoot
kubectl --context kind-gardener-local -n garden-local get shoots
```

All Crossplane resources should show `SYNCED=True` and `READY=True`. The Shoot should show `Create Succeeded (100%)` after a few minutes.

## Claim Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `projectName` | string | yes | — | Gardener project name. Shoot is created in `garden-<projectName>` namespace. |
| `shootName` | string | yes | — | Name for the Shoot resource. |
| `kubernetesVersion` | string | no | `1.31.1` | Kubernetes version (must be in CloudProfile `local`). |
| `workerMinimum` | integer | no | `1` | Minimum worker node count. |
| `workerMaximum` | integer | no | `2` | Maximum worker node count. |

## Hardcoded Values (from local Gardener setup)

These are fixed in the Composition to match the `provider-local` environment:

- CloudProfile: `local`
- Region: `local`
- Provider type: `local`
- CredentialsBinding: `local`
- Networking: `calico`, nodes `10.0.0.0/16`
- Machine type: `local`, CRI: `containerd`

## Troubleshooting

**Claim stays not Ready:**
```bash
kubectl --context kind-gardener-local describe gardenershoot my-test-shoot
kubectl --context kind-gardener-local describe xgardenershoot <composite-name>
kubectl --context kind-gardener-local describe object <object-name>
```

**Shoot not appearing:**
Check provider-kubernetes logs:
```bash
kubectl --context kind-gardener-local -n crossplane-system logs -l pkg.crossplane.io/revision --tail=50
```

**RBAC errors:**
Verify the SA binding:
```bash
kubectl --context kind-gardener-local get clusterrolebinding crossplane-provider-kubernetes-shoot-admin -o yaml
kubectl --context kind-gardener-local auth can-i create shoots.core.gardener.cloud \
  --as=system:serviceaccount:crossplane-system:provider-kubernetes
```
