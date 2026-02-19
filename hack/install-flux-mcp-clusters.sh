#!/bin/bash
set -euo pipefail

COL='\033[92m'
RED='\033[91m'
YELLOW='\033[93m'
COL_RES='\033[0m'

log() { echo -e "${COL}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }
error() { echo -e "${RED}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }

MCP_CLUSTERS=$(kind get clusters 2>/dev/null | grep -E "^mcp-worker-" || true)

if [[ -z "${MCP_CLUSTERS}" ]]; then
    warn "No mcp-worker-* kind clusters found"
    exit 0
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

for CLUSTER in ${MCP_CLUSTERS}; do
    log "Installing Flux on ${CLUSTER}..."
    KUBECONFIG_FILE="${TMPDIR}/${CLUSTER}.kubeconfig"
    kind get kubeconfig --name "${CLUSTER}" > "${KUBECONFIG_FILE}"
    KUBECONFIG="${KUBECONFIG_FILE}" flux install --components=source-controller,kustomize-controller,helm-controller,notification-controller
    log "Flux installed on ${CLUSTER} ✓"
done

log "Flux installed on all MCP clusters"
