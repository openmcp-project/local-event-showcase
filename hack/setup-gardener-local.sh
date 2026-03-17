#!/bin/bash

DEBUG=${DEBUG:-false}

if [ "${DEBUG}" = "true" ]; then
  set -x
fi

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors (matching existing scripts)
COL='\033[92m'
RED='\033[91m'
YELLOW='\033[93m'
COL_RES='\033[0m'

log() { echo -e "${COL}[$(date '+%H:%M:%S')] $1 ${COL_RES}"; }
error() { echo -e "${RED}[$(date '+%H:%M:%S')] ✗ $1 ${COL_RES}"; }
warn() { echo -e "${YELLOW}[$(date '+%H:%M:%S')] ⚠️  $1 ${COL_RES}"; }

GARDENER_DIR="${PROJECT_DIR}/demo/external/gardener"

usage() {
  echo "Usage: $0 [--help]"
  echo ""
  echo "Bootstrap a local Gardener environment."
  echo "Clones Gardener into demo/external/gardener (if not present),"
  echo "creates a 'gardener-local' Kind cluster, and starts Gardener."
  echo ""
  echo "Options:"
  echo "  --help    Show this help message"
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --help|-h) usage ;;
    --*) echo "Unknown option: $1" >&2; exit 1 ;;
    *) echo "Ignoring positional arg: $1" ;;
  esac
  shift
done

echo -e "${COL}-------------------------------------${COL_RES}"
echo -e "${COL}[$(date '+%H:%M:%S')] Starting Gardener Local Setup ${COL_RES}"
echo -e "${COL}-------------------------------------${COL_RES}"

# Clone gardener if not present
if [ ! -d "$GARDENER_DIR" ]; then
    log "Cloning Gardener repository into $GARDENER_DIR..."
    git clone https://github.com/gardener/gardener.git "$GARDENER_DIR"
else
    log "Gardener repository already exists at $GARDENER_DIR"
fi

# --- Patches for co-existing with other kind clusters ---
# All patches are applied to the cloned Gardener repo (gitignored via demo/external).

KIND_UP_SCRIPT="$GARDENER_DIR/hack/kind-up.sh"
COMPOSE_FILE="$GARDENER_DIR/dev-setup/infra/docker-compose.yaml"

# 1. Disable Gardener's network setup — we manage the kind network via `task kind-network`.
#    Gardener's setup_kind_network() validates subnet size and options, but rejects our
#    /16 subnet (it expects /24) and ICC option. Instead of patching the validation,
#    we simply skip it since the network is already correctly configured.
if grep -q '^setup_kind_network$' "$KIND_UP_SCRIPT" 2>/dev/null; then
    log "Patching kind-up.sh: disabling setup_kind_network (managed by task kind-network)..."
    sed -i.bak 's|^setup_kind_network$|# setup_kind_network # disabled: network managed by task kind-network|' "$KIND_UP_SCRIPT"
    rm -f "${KIND_UP_SCRIPT}.bak"
fi

# 2. Assign bind9 a static IP (172.18.255.53) on the kind Docker network.
#    On macOS, the loopback IPs on lo0 are not reachable from inside Docker containers.
#    Kind nodes resolve DNS via /etc/resolv.conf pointing to 172.18.255.53.
#    Giving bind9 this IP directly on the bridge makes it reachable without host loopback.
if ! grep -q 'ipv4_address: 172.18.255.53' "$COMPOSE_FILE" 2>/dev/null; then
    log "Patching docker-compose.yaml: bind9 static IP 172.18.255.53..."
    sed -i.bak '/^  bind9:/,/^  [a-z]/{
        s|^\(    networks:\)$|\1|
        s|^\(      kind:\)$|\1\n        ipv4_address: 172.18.255.53|
    }' "$COMPOSE_FILE"
    rm -f "${COMPOSE_FILE}.bak"
fi

# 3. Disable IPv6 port bindings for bind9 — Docker Desktop on macOS cannot bind
#    to fd00:ff::53 on the loopback interface (IPv6 addresses are not mirrored into the VM).
if grep -q '^\s*- "\[fd00:ff::53\]:53:53' "$COMPOSE_FILE" 2>/dev/null; then
    log "Patching docker-compose.yaml: disabling bind9 IPv6 port bindings (unsupported on Docker Desktop macOS)..."
    sed -i.bak 's|^\(\s*\)- "\[fd00:ff::53\]:53:53/\(.*\)"$|\1# - "[fd00:ff::53]:53:53/\2"  # disabled: Docker Desktop macOS|' "$COMPOSE_FILE"
    rm -f "${COMPOSE_FILE}.bak"
fi

# Check if gardener-local kind cluster exists and Gardener is running
if ! kind get clusters 2>/dev/null | grep -q "^gardener-local$"; then
    log "Creating Kind cluster and starting Gardener..."
    pushd "$GARDENER_DIR" > /dev/null
    make kind-up gardener-up
    popd > /dev/null
else
    log "Kind cluster 'gardener-local' already exists"
    # Check if Gardener pods are running; if not, start Gardener
    if ! kubectl --context kind-gardener-local get pods -n garden -l app=gardener-apiserver --no-headers 2>/dev/null | grep -q "Running"; then
        warn "Gardener does not seem to be running. Starting Gardener..."
        pushd "$GARDENER_DIR" > /dev/null
        make gardener-up
        popd > /dev/null
    else
        log "Gardener is already running"
    fi
fi

kubectl config use-context kind-gardener-local || true

echo -e "${COL}-------------------------------------${COL_RES}"
echo -e "${COL}[$(date '+%H:%M:%S')] Gardener Local Setup Complete ${RED}♥${COL} !${COL_RES}"
echo -e "${COL}-------------------------------------${COL_RES}"
echo ""
echo -e "Gardener is running on Kind cluster ${YELLOW}kind-gardener-local${COL_RES}"
echo ""
echo -e "To access the Gardener cluster:"
echo -e "  ${YELLOW}kubectl config use-context kind-gardener-local${COL_RES}"
echo ""

exit 0
