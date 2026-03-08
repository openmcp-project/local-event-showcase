#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HACK_DIR="${SCRIPT_DIR}"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors (matching platform-mesh local setup)
COL='\033[92m'
RED='\033[91m'
YELLOW='\033[93m'
COL_RES='\033[0m'

log() { echo -e "${COL}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }
error() { echo -e "${RED}[$(date '+%H:%M:%S')] ✗ $1 ${COL_RES}"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠️  $1 ${COL_RES}"; }

# Load environment variables (optional)
if [[ -f "${HACK_DIR}/.env" ]]; then
    source "${HACK_DIR}/.env"
fi

# Default PLATFORM_MESH_DIR to the project-local checkout
PLATFORM_MESH_DIR="${PLATFORM_MESH_DIR:-${PROJECT_DIR}/demo/external/platform-mesh/helm-charts}"

if [[ ! -d "${PLATFORM_MESH_DIR}" ]]; then
    error "PLATFORM_MESH_DIR does not exist: ${PLATFORM_MESH_DIR}"
    exit 1
fi

log "Using PLATFORM_MESH_DIR: ${PLATFORM_MESH_DIR}"

# Check if kind cluster 'platform-mesh' exists
if ! kind get clusters 2>/dev/null | grep -q "^platform-mesh$"; then
    error "kind cluster 'platform-mesh' does not exist."
    warn "Please follow the platform-mesh local setup guide: https://platform-mesh.io/release-0.2/getting-started/"
    exit 1
fi

log "Kind cluster 'platform-mesh' found ✓"

# Export kubeconfig for platform-mesh cluster
KUBECONFIGS_DIR="${HACK_DIR}/.secret/kubeconfigs"
mkdir -p "${KUBECONFIGS_DIR}"
kind get kubeconfig --name platform-mesh > "${KUBECONFIGS_DIR}/platform-mesh.kubeconfig"
log "Exported kubeconfig to ${KUBECONFIGS_DIR}/platform-mesh.kubeconfig ✓"

# Check if platform-mesh resource is ready
PLATFORM_MESH_KUBECONFIG="${KUBECONFIGS_DIR}/platform-mesh.kubeconfig"
if ! KUBECONFIG="${PLATFORM_MESH_KUBECONFIG}" kubectl get platformmesh platform-mesh -n platform-mesh-system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q "True"; then
    error "platform-mesh resource is not ready."
    warn "Please ensure platform-mesh is fully deployed: https://platform-mesh.io/release-0.2/getting-started/"
    exit 1
fi

log "platform-mesh resource is ready ✓"

# Patch platform-mesh resource with extraDefaultAPIBindings for openmcp
log "Patching platform-mesh with extraDefaultAPIBindings..."
KUBECONFIG="${PLATFORM_MESH_KUBECONFIG}" kubectl patch platformmesh platform-mesh -n platform-mesh-system --type=merge -p '
{
  "spec": {
    "kcp": {
      "extraDefaultAPIBindings": [
        {
          "workspaceTypePath": "root:account",
          "export": "openmcp.cloud",
          "path": "root:providers:openmcp"
        }
      ]
    }
  }
}'
log "Patched platform-mesh with extraDefaultAPIBindings ✓"

# Copy KCP admin kubeconfig from platform-mesh
KCP_ADMIN_KUBECONFIG="${PLATFORM_MESH_DIR}/.secret/kcp/admin.kubeconfig"
if [[ ! -f "${KCP_ADMIN_KUBECONFIG}" ]]; then
    error "KCP admin kubeconfig not found at ${KCP_ADMIN_KUBECONFIG}"
    exit 1
fi
cp "${KCP_ADMIN_KUBECONFIG}" "${KUBECONFIGS_DIR}/kcp-admin.kubeconfig"
log "Copied KCP admin kubeconfig to ${KUBECONFIGS_DIR}/kcp-admin.kubeconfig ✓"

# Find and export the first onboarding.* kind cluster kubeconfig
ONBOARDING_CLUSTER=$(kind get clusters 2>/dev/null | grep -E "^onboarding" | head -n 1 || true)
if [[ -z "${ONBOARDING_CLUSTER}" ]]; then
    warn "No onboarding.* kind cluster found. Available clusters:"
    kind get clusters 2>/dev/null || echo "  (none)"
    warn "Skipping onboarding kubeconfig export"
else
    log "Found onboarding cluster: ${ONBOARDING_CLUSTER}"
    kind get kubeconfig --name "${ONBOARDING_CLUSTER}" > "${KUBECONFIGS_DIR}/onboarding.kubeconfig"
    log "Exported onboarding kubeconfig to ${KUBECONFIGS_DIR}/onboarding.kubeconfig ✓"
fi

# Generate operator manifests
OPERATOR_DIR="${SCRIPT_DIR}/../demo/openmcp-init-operator"
log "Generating operator manifests..."
(cd "${OPERATOR_DIR}" && task generate)
log "Generated operator manifests ✓"

# Prepare provider workspace
KCP_KUBECONFIG="${KUBECONFIGS_DIR}/kcp-admin.kubeconfig"
KUBECONFIG="${KCP_KUBECONFIG}" kubectl create-workspace providers --type=root:providers --ignore-existing --server="https://localhost:8443/clusters/root"
KUBECONFIG="${KCP_KUBECONFIG}" kubectl create-workspace openmcp --type=root:provider --ignore-existing --server="https://localhost:8443/clusters/root:providers"
log "Created provider workspaces ✓"

# Copy operator API resources to demo manifests directory for deployment
MANIFESTS_DIR="${SCRIPT_DIR}/../demo/manifests"
OPENMCP_MANIFESTS_DIR="${MANIFESTS_DIR}/providers/openmcp"
log "Copying API resources to ${OPENMCP_MANIFESTS_DIR}..."
cp "${OPERATOR_DIR}/config/resources/"*.yaml "${OPENMCP_MANIFESTS_DIR}/"
log "Copied API resources ✓"

# Lookup identityHash from core.platform-mesh.io APIExport
log "Looking up identityHash from core.platform-mesh.io APIExport..."
PLATFORM_MESH_SYSTEM_URL="https://localhost:8443/clusters/root:platform-mesh-system"
CONTENT_CONFIG_IDENTITY_HASH=$(KUBECONFIG="${KCP_KUBECONFIG}" kubectl get apiexport core.platform-mesh.io \
    --server="${PLATFORM_MESH_SYSTEM_URL}" \
    -o jsonpath='{.status.identityHash}')
log "Found identityHash: ${CONTENT_CONFIG_IDENTITY_HASH}"

# Patch copied APIExport file with identityHash
APIEXPORT_FILE="${OPENMCP_MANIFESTS_DIR}/apiexport-openmcp.cloud.yaml"
log "Patching APIExport with contentconfigurations identityHash..."
yq -i "(.spec.permissionClaims[] | select(.resource == \"contentconfigurations\")).identityHash = \"${CONTENT_CONFIG_IDENTITY_HASH}\"" "${APIEXPORT_FILE}"
log "Patched APIExport file ✓"

# Apply all provider manifests to the openmcp provider workspace
OPENMCP_WORKSPACE_URL="https://localhost:8443/clusters/root:providers:openmcp"
log "Applying provider manifests to openmcp provider workspace..."
KUBECONFIG="${KCP_KUBECONFIG}" kubectl apply -f "${OPENMCP_MANIFESTS_DIR}" --server="${OPENMCP_WORKSPACE_URL}" --server-side --force-conflicts
log "Applied provider manifests to ${OPENMCP_WORKSPACE_URL} ✓"

# Apply openmcp-init WorkspaceType to root workspace
# This type carries initializer: true so the init-agent can watch for new account workspaces
INIT_AGENT_MANIFESTS_DIR="${MANIFESTS_DIR}/init-agent"
ROOT_WORKSPACE_URL="https://localhost:8443/clusters/root"
log "Applying openmcp-init WorkspaceType to root workspace..."
KUBECONFIG="${KCP_KUBECONFIG}" kubectl apply -f "${INIT_AGENT_MANIFESTS_DIR}/workspace-type-openmcp-init.yaml" \
    --server="${ROOT_WORKSPACE_URL}"
log "Applied openmcp-init WorkspaceType ✓"

# Patch account WorkspaceType to extend from openmcp-init
# This ensures new account workspaces get the root:openmcp-init initializer via transitive extend.with
log "Patching account WorkspaceType to extend from openmcp-init..."
KUBECONFIG="${KCP_KUBECONFIG}" kubectl patch workspacetype account \
    --server="${ROOT_WORKSPACE_URL}" \
    --type=json \
    -p '[{"op": "add", "path": "/spec/extend/with/-", "value": {"name": "openmcp-init", "path": "root"}}]'
log "Patched account WorkspaceType ✓"

# Apply init-agent manifests (InitTemplate + InitTarget) to the config workspace
# The KCP init-agent reads these to know what resources to create in new workspaces
log "Applying init-agent manifests to platform-mesh-system workspace..."
KUBECONFIG="${KCP_KUBECONFIG}" kubectl apply -f "${INIT_AGENT_MANIFESTS_DIR}" --server="${PLATFORM_MESH_SYSTEM_URL}"
log "Applied init-agent manifests to ${PLATFORM_MESH_SYSTEM_URL} ✓"

# Create operator kubeconfig pointing to the openmcp workspace
OPERATOR_KUBECONFIG="${KUBECONFIGS_DIR}/operator.kubeconfig"

# Extract credentials from the admin kubeconfig (using kcp-admin user)
CLIENT_CERT=$(KUBECONFIG="${KCP_KUBECONFIG}" kubectl config view --raw -o jsonpath='{.users[?(@.name=="kcp-admin")].user.client-certificate-data}')
CLIENT_KEY=$(KUBECONFIG="${KCP_KUBECONFIG}" kubectl config view --raw -o jsonpath='{.users[?(@.name=="kcp-admin")].user.client-key-data}')
CA_DATA=$(KUBECONFIG="${KCP_KUBECONFIG}" kubectl config view --raw -o jsonpath='{.clusters[?(@.name=="workspace.kcp.io/current")].cluster.certificate-authority-data}')

# Create a clean kubeconfig with single cluster/context pointing to openmcp workspace
cat > "${OPERATOR_KUBECONFIG}" <<EOF
apiVersion: v1
kind: Config
clusters:
- name: openmcp
  cluster:
    server: ${OPENMCP_WORKSPACE_URL}
    certificate-authority-data: ${CA_DATA}
contexts:
- name: openmcp
  context:
    cluster: openmcp
    user: openmcp
current-context: openmcp
users:
- name: openmcp
  user:
    client-certificate-data: ${CLIENT_CERT}
    client-key-data: ${CLIENT_KEY}
EOF
chmod 600 "${OPERATOR_KUBECONFIG}"
log "Created operator kubeconfig at ${OPERATOR_KUBECONFIG} ✓"

# Get platform-mesh control plane Docker IP for cross-cluster access
# The sync agent in MCP clusters needs this IP to reach KCP via hostAliases
PLATFORM_MESH_IP=$(docker inspect platform-mesh-control-plane --format '{{.NetworkSettings.Networks.kind.IPAddress}}')
log "Platform-mesh control plane IP: ${PLATFORM_MESH_IP}"

# Save the IP for the operator to use in hostAliases (maps localhost -> platform-mesh IP)
echo "${PLATFORM_MESH_IP}" > "${KUBECONFIGS_DIR}/platform-mesh-ip.txt"
log "Saved platform-mesh IP to ${KUBECONFIGS_DIR}/platform-mesh-ip.txt ✓"

# Build and deploy the openmcp-init-operator
if [[ -z "${ONBOARDING_CLUSTER}" ]]; then
    error "Cannot deploy operator: no onboarding cluster found"
    exit 1
fi

ONBOARDING_KUBECONFIG="${KUBECONFIGS_DIR}/onboarding.kubeconfig"

# Install Flux on the platform cluster (openmcp prerequisite)
PLATFORM_KUBECONFIG="${KUBECONFIGS_DIR}/platform.kubeconfig"
kind get kubeconfig --name platform > "${PLATFORM_KUBECONFIG}"
log "Installing Flux on platform cluster..."
KUBECONFIG="${PLATFORM_KUBECONFIG}" flux install --components=source-controller,kustomize-controller,helm-controller,notification-controller
log "Flux installed on platform cluster ✓"

OPERATOR_IMAGE="openmcp-init-operator:local-$(date +%s)"

# Build Docker image
log "Building openmcp-init-operator Docker image..."
docker build -t "${OPERATOR_IMAGE}" "${OPERATOR_DIR}"
log "Built Docker image ${OPERATOR_IMAGE} ✓"

# Load image into the onboarding kind cluster
log "Loading Docker image into ${ONBOARDING_CLUSTER} cluster..."
kind load docker-image "${OPERATOR_IMAGE}" --name "${ONBOARDING_CLUSTER}"
log "Loaded Docker image into ${ONBOARDING_CLUSTER} ✓"

# Create namespace for the operator
OPERATOR_NAMESPACE="openmcp-system"
KUBECONFIG="${ONBOARDING_KUBECONFIG}" kubectl create namespace "${OPERATOR_NAMESPACE}" --dry-run=client -o yaml | \
    KUBECONFIG="${ONBOARDING_KUBECONFIG}" kubectl apply -f -
log "Created namespace ${OPERATOR_NAMESPACE} ✓"

# Create KCP kubeconfig secret
# Keep localhost in the kubeconfig - hostAliases will map localhost to platform-mesh IP
KCP_OPERATOR_KUBECONFIG=$(sed "s|https://localhost:8443|https://localhost:31000|g" "${OPERATOR_KUBECONFIG}")
KUBECONFIG="${ONBOARDING_KUBECONFIG}" kubectl create secret generic kcp-openmfp-system-kubeconfig \
    --namespace="${OPERATOR_NAMESPACE}" \
    --from-literal=kubeconfig="${KCP_OPERATOR_KUBECONFIG}" \
    --dry-run=client -o yaml | \
    KUBECONFIG="${ONBOARDING_KUBECONFIG}" kubectl apply -f -
log "Created KCP kubeconfig secret ✓"

# Deploy the operator using Helm
log "Deploying openmcp-init-operator to ${ONBOARDING_CLUSTER}..."
helm upgrade --install openmcp-init-operator "${OPERATOR_DIR}/chart" \
    --kubeconfig="${ONBOARDING_KUBECONFIG}" \
    --namespace="${OPERATOR_NAMESPACE}" \
    --set image.name="${OPERATOR_IMAGE%:*}" \
    --set image.tag="${OPERATOR_IMAGE#*:}" \
    --set image.imagePullSecret="" \
    --set image.pullPolicy="Never" \
    --set kcp.apiExportEndpointSliceName="openmcp.cloud" \
    --set kcp.platformMeshIP="${PLATFORM_MESH_IP}" \
    --set kcp.hostOverride="localhost:31000" \
    --set runtime.namespace="default" \
    --set log.level="debug" \
    --set log.noJson=true
log "Deployed openmcp-init-operator ✓"

# Wait for operator to be ready
log "Waiting for operator deployment to be ready..."
KUBECONFIG="${ONBOARDING_KUBECONFIG}" kubectl rollout status deployment/openmcp-init-operator \
    --namespace="${OPERATOR_NAMESPACE}" --timeout=120s
log "Operator is ready ✓"

# Build and deploy the onboarding UI to the platform-mesh cluster
UI_DIR="${PROJECT_DIR}/demo/openmcp-onboarding-ui"
UI_IMAGE="openmcp-onboarding-ui:local-$(date +%s)"

log "Building openmcp-onboarding-ui Docker image..."
docker build -t "${UI_IMAGE}" "${UI_DIR}"
log "Built Docker image ${UI_IMAGE} ✓"

log "Loading Docker image into platform-mesh cluster..."
kind load docker-image "${UI_IMAGE}" --name platform-mesh
log "Loaded Docker image into platform-mesh ✓"

log "Deploying openmcp-onboarding-ui to platform-mesh..."
helm upgrade --install openmcp-onboarding-ui "${UI_DIR}/chart" \
    --kubeconfig="${PLATFORM_MESH_KUBECONFIG}" \
    --namespace=platform-mesh-system \
    --set image.name="${UI_IMAGE%:*}" \
    --set image.tag="${UI_IMAGE#*:}" \
    --set image.imagePullSecret="" \
    --set image.pullPolicy=Never
log "Deployed openmcp-onboarding-ui ✓"

log "Waiting for UI deployment to be ready..."
KUBECONFIG="${PLATFORM_MESH_KUBECONFIG}" kubectl rollout status deployment/openmcp-onboarding-ui \
    --namespace=platform-mesh-system --timeout=120s
log "Onboarding UI is ready ✓"

log "Integration complete!"
log "Portal URL: https://portal.localhost:8443/"
