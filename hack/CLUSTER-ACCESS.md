# Cluster Access

## Gardener Local (kind-gardener-local)

Kind automatically merges the kubeconfig into `~/.kube/config`, so the `kind-gardener-local` context is available immediately.

### kubectl

```bash
kubectl config use-context kind-gardener-local
kubectl get pods -A
```

### k9s

```bash
k9s --context kind-gardener-local
```

### Export kubeconfig to a separate file

```bash
kind get kubeconfig --name gardener-local > /tmp/gardener-local.kubeconfig
k9s --kubeconfig /tmp/gardener-local.kubeconfig
```

## OpenMCP Platform (kind-platform)

```bash
k9s --context kind-platform
```

Or use the exported kubeconfig:

```bash
k9s --kubeconfig .tmp/openmcp/kubeconfigs/platform-ext.kubeconfig
```
