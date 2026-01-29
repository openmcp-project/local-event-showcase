#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_PATH="$1"
YAML_FILE="$2"
TIMEOUT="${3:-300}"

KUBECTL="kubectl --kubeconfig=$KUBECONFIG_PATH"

wait_for_crd() {
  local crd="$1"
  local elapsed=0
  echo "Waiting for CRD '$crd' to exist..."
  while ! $KUBECTL get "crd/$crd" &>/dev/null; do
    if [ "$elapsed" -ge "$TIMEOUT" ]; then
      echo "Timed out waiting for CRD '$crd' to exist"
      exit 1
    fi
    sleep 5
    elapsed=$((elapsed + 5))
  done
  echo "Waiting for CRD '$crd' to be established..."
  if ! $KUBECTL wait --for=condition=Established "crd/$crd" --timeout="${TIMEOUT}s"; then
    echo "Timed out waiting for CRD '$crd' to be established"
    exit 1
  fi
  echo "CRD '$crd' is ready"
}

apply_resources() {
  local kind_filter="$1"
  local tmpfile
  tmpfile=$(mktemp)
  yq "select(.kind == \"$kind_filter\")" "$YAML_FILE" > "$tmpfile"
  if [ -s "$tmpfile" ]; then
    echo "Applying $kind_filter resources..."
    $KUBECTL apply -f "$tmpfile"
  fi
  rm -f "$tmpfile"
}

apply_standard_resources() {
  local tmpfile
  tmpfile=$(mktemp)
  yq 'select(
    .kind == "Namespace" or
    .kind == "ServiceAccount" or
    .kind == "ClusterRoleBinding" or
    .kind == "ClusterRole" or
    .kind == "RoleBinding" or
    .kind == "Role" or
    .kind == "Secret" or
    .kind == "ConfigMap" or
    .kind == "Deployment" or
    .kind == "Service"
  )' "$YAML_FILE" > "$tmpfile"
  if [ -s "$tmpfile" ]; then
    echo "Phase 1: Applying standard Kubernetes resources..."
    $KUBECTL apply -f "$tmpfile"
  fi
  rm -f "$tmpfile"
}

# Phase 1: Apply standard k8s resources (namespace, RBAC, deployment, etc.)
apply_standard_resources

# Phase 2: Wait for operator CRDs, then apply ClusterProvider and ServiceProvider
echo ""
echo "Phase 2: Waiting for operator CRDs..."
wait_for_crd "clusterproviders.openmcp.cloud"
wait_for_crd "serviceproviders.openmcp.cloud"
apply_resources "ClusterProvider"
apply_resources "ServiceProvider"

# Phase 3: Wait for crossplane CRD, then apply ProviderConfig
echo ""
echo "Phase 3: Waiting for crossplane service provider CRDs..."
wait_for_crd "providerconfigs.crossplane.services.openmcp.cloud"
apply_resources "ProviderConfig"

echo ""
echo "All resources applied successfully"
