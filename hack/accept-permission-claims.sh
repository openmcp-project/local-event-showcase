#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors (matching project conventions)
COL='\033[92m'
RED='\033[91m'
YELLOW='\033[93m'
COL_RES='\033[0m'

log() { echo -e "${COL}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }
error() { echo -e "${RED}[$(date '+%H:%M:%S')] ✗ $1 ${COL_RES}"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠️  $1 ${COL_RES}"; }

usage() {
    cat <<EOF
Usage: $(basename "$0") [--dry-run] <apibinding-name>

Accept all permission claims on a kcp APIBinding.

Arguments:
  apibinding-name    Name of the APIBinding to accept claims for

Options:
  --dry-run          Show what would be patched without applying
  --help             Show this help message

Environment variables:
  KCP_KUBECONFIG     Path to kubeconfig for kcp (default: uses current KUBECONFIG)
  KCP_SERVER         KCP server URL / workspace (passed as --server to kubectl)

Examples:
  # Using current kubeconfig context:
  $(basename "$0") openmcp.cloud

  # With explicit KCP kubeconfig and workspace:
  KCP_KUBECONFIG=hack/.secret/kubeconfigs/kcp-admin.kubeconfig \\
  KCP_SERVER=https://localhost:8443/clusters/root:my-workspace \\
    $(basename "$0") openmcp.cloud

  # Dry-run mode:
  $(basename "$0") --dry-run openmcp.cloud
EOF
    exit 0
}

# Parse arguments
DRY_RUN=false
APIBINDING_NAME=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help|-h)
            usage
            ;;
        -*)
            error "Unknown option: $1"
            usage
            ;;
        *)
            if [[ -n "${APIBINDING_NAME}" ]]; then
                error "Unexpected argument: $1"
                usage
            fi
            APIBINDING_NAME="$1"
            shift
            ;;
    esac
done

if [[ -z "${APIBINDING_NAME}" ]]; then
    error "Missing required argument: apibinding-name"
    usage
fi

# Build kubectl arguments
KUBECTL_ARGS=()
if [[ -n "${KCP_KUBECONFIG:-}" ]]; then
    KUBECTL_ARGS+=(--kubeconfig="${KCP_KUBECONFIG}")
fi
if [[ -n "${KCP_SERVER:-}" ]]; then
    KUBECTL_ARGS+=(--server="${KCP_SERVER}")
fi

kctl() {
    kubectl ${KUBECTL_ARGS[@]+"${KUBECTL_ARGS[@]}"} "$@"
}

# Check prerequisites
for cmd in kubectl jq; do
    if ! command -v "${cmd}" &>/dev/null; then
        error "${cmd} is required but not found in PATH"
        exit 1
    fi
done

# Step 1: Read the APIBinding
log "Reading APIBinding '${APIBINDING_NAME}'..."
APIBINDING_JSON=$(kctl get apibinding "${APIBINDING_NAME}" -o json 2>&1) || {
    error "Failed to get APIBinding '${APIBINDING_NAME}'"
    echo "${APIBINDING_JSON}" >&2
    exit 1
}

# Step 2: Get permission claims from status.exportPermissionClaims
CLAIMS=$(echo "${APIBINDING_JSON}" | jq -r '.status.exportPermissionClaims // empty')

if [[ -z "${CLAIMS}" || "${CLAIMS}" == "[]" || "${CLAIMS}" == "null" ]]; then
    warn "No exportPermissionClaims found in APIBinding status, trying APIExport directly..."

    # Resolve the APIExport path from the APIBinding spec
    EXPORT_PATH=$(echo "${APIBINDING_JSON}" | jq -r '.spec.reference.export.path // empty')
    EXPORT_NAME=$(echo "${APIBINDING_JSON}" | jq -r '.spec.reference.export.name // empty')

    if [[ -z "${EXPORT_NAME}" ]]; then
        error "Cannot determine APIExport name from APIBinding spec"
        exit 1
    fi

    if [[ -n "${EXPORT_PATH}" ]]; then
        log "Querying APIExport '${EXPORT_NAME}' at path '${EXPORT_PATH}'..."
        EXPORT_SERVER_ARGS=()
        if [[ -n "${KCP_KUBECONFIG:-}" ]]; then
            EXPORT_SERVER_ARGS+=(--kubeconfig="${KCP_KUBECONFIG}")
        fi
        # Build the server URL from the path
        # KCP paths like "root:providers:openmcp" map to /clusters/root:providers:openmcp
        BASE_SERVER="${KCP_SERVER:-$(kctl config view --minify -o jsonpath='{.clusters[0].cluster.server}')}"
        # Strip any existing /clusters/... suffix to get the base URL
        BASE_URL="${BASE_SERVER%%/clusters/*}"
        EXPORT_SERVER_URL="${BASE_URL}/clusters/${EXPORT_PATH}"
        EXPORT_SERVER_ARGS+=(--server="${EXPORT_SERVER_URL}")

        APIEXPORT_JSON=$(kubectl ${EXPORT_SERVER_ARGS[@]+"${EXPORT_SERVER_ARGS[@]}"} get apiexport "${EXPORT_NAME}" -o json 2>&1) || {
            error "Failed to get APIExport '${EXPORT_NAME}' at '${EXPORT_SERVER_URL}'"
            echo "${APIEXPORT_JSON}" >&2
            exit 1
        }
        CLAIMS=$(echo "${APIEXPORT_JSON}" | jq -r '.spec.permissionClaims // empty')
    else
        error "No export path found in APIBinding spec — cannot resolve APIExport"
        exit 1
    fi
fi

if [[ -z "${CLAIMS}" || "${CLAIMS}" == "[]" || "${CLAIMS}" == "null" ]]; then
    log "No permission claims to accept — nothing to do."
    exit 0
fi

CLAIM_COUNT=$(echo "${CLAIMS}" | jq 'length')
log "Found ${CLAIM_COUNT} permission claim(s) to accept"

# Step 3: Transform claims by adding "state": "Accepted"
ACCEPTED_CLAIMS=$(echo "${CLAIMS}" | jq '[.[] | . + {"state": "Accepted", "selector": {"matchAll": true}}]')

# Step 4: Build and apply the patch
PATCH=$(jq -n --argjson claims "${ACCEPTED_CLAIMS}" '{"spec": {"permissionClaims": $claims}}')

if [[ "${DRY_RUN}" == "true" ]]; then
    log "Dry-run mode — would patch APIBinding '${APIBINDING_NAME}' with:"
    echo "${PATCH}" | jq .
    exit 0
fi

log "Patching APIBinding '${APIBINDING_NAME}' with accepted permission claims..."
kctl patch apibinding "${APIBINDING_NAME}" --type=merge -p "${PATCH}" || {
    error "Failed to patch APIBinding '${APIBINDING_NAME}'"
    exit 1
}

log "Accepted ${CLAIM_COUNT} permission claim(s) on APIBinding '${APIBINDING_NAME}' ✓"
