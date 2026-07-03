#!/usr/bin/env bash
# Verify CCAAS chaincode on peer-4 OPS. Run as igr (not sudo):
#   cd /opt/igr-network && bash scripts/verify-ccaas-deploy.sh
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"

export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"
export TEST_NETWORK_HOME="$PWD"
export CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
export CC_NAME="${CC_NAME:-igr_anchor}"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f docs/servers.credentials.local ]] && source docs/servers.credentials.local || true

# shellcheck source=envVar.sh
. "${SCRIPT_DIR}/envVar.sh"

DOC_REF="${DOC_REF:-MH:PUNE:SR42:2026:991}"

echo "=== 1. CCAAS container ==="
if docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null | grep -E 'igr_anchor|NAMES'; then
  :
elif [[ -n "${SUDO_PASS:-}" ]]; then
  echo "${SUDO_PASS}" | sudo -S docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'igr_anchor|NAMES' || true
else
  sudo docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | grep -E 'igr_anchor|NAMES' || true
fi

echo ""
echo "=== 2. Channel block on OPS host ==="
ls -la "channel-artifacts/${CHANNEL_NAME}.block" 2>/dev/null || echo "MISSING channel-artifacts/${CHANNEL_NAME}.block"

echo ""
echo "=== 3. Lifecycle committed definition (via peer-1) ==="
setGlobals 1
peer lifecycle chaincode querycommitted --channelID "$CHANNEL_NAME" --name "$CC_NAME" || {
  echo "FAILED — set CORE_PEER_MSPCONFIGPATH and query remote peer; see errors above"
}

echo ""
echo "=== 4. Channels joined on peer-1 ==="
peer channel list || echo "FAILED — peer-1 may not have joined ${CHANNEL_NAME}"

echo ""
echo "=== 5. Chaincode query: GetLatestAnchor ==="
peer chaincode query -C "$CHANNEL_NAME" -n "$CC_NAME" \
  -c "{\"function\":\"GetLatestAnchor\",\"Args\":[\"${DOC_REF}\"]}" || {
  echo ""
  echo "Query failed. Common fixes:"
  echo "  - Re-join channel: bash scripts/remote-peer5-channel-join-anchors.sh"
  echo "  - Or join only:     bash scripts/remote-peer5-channel-join-only.sh"
  echo "  - Check /etc/hosts on peer-4 resolves peer0.IGRPrimary.example.com -> 10.48.59.70"
  echo "  - Confirm channel name (CHANNEL_NAME=${CHANNEL_NAME})"
  exit 1
}

echo ""
echo "OK — chaincode ${CC_NAME} on ${CHANNEL_NAME} is queryable."
