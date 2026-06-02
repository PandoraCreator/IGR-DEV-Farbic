#!/usr/bin/env bash
# After four remote peers are up: join channel + anchors (runs on peer-5).
# Peer ports must match docs/5-SERVER-DEPLOYMENT.md (defaults match servers.credentials.local.example).
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${IGR_NETWORK}/../bin:${PATH}"
# peer CLI needs core.yaml (not configtx/). configtxgen uses configtx/ separately.
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"
export CHANNEL_NAME="${CHANNEL_NAME:-mychannel}"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true
PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
PEER2_PORT_PEER="${PEER2_PORT_PEER:-6001}"
PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
PEER4_PORT_PEER="${PEER4_PORT_PEER:-8001}"
export PEER1_PORT_PEER PEER2_PORT_PEER PEER3_PORT_PEER PEER4_PORT_PEER

ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
BLOCK=./channel-artifacts/${CHANNEL_NAME}.block

# IGRPrimary peer0
export CORE_PEER_LOCALMSPID=IGRPrimaryMSP
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_TLS_ROOTCERT_FILE=$PWD/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=$PWD/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp
export CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}
peer channel join -b "$BLOCK"

# IGRPrimary peer1
export CORE_PEER_ADDRESS=peer1.IGRPrimary.example.com:${PEER2_PORT_PEER}
peer channel join -b "$BLOCK"

# IGRBank peer0
export CORE_PEER_LOCALMSPID=IGRBankMSP
export CORE_PEER_TLS_ROOTCERT_FILE=$PWD/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=$PWD/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp
export CORE_PEER_ADDRESS=peer0.IGRBank.example.com:${PEER3_PORT_PEER}
peer channel join -b "$BLOCK"

# IGRBank peer1 (skip when peer-4 is OPS-only — no peer container on .78)
if [[ "${PEER4_ROLE:-}" != "ops_only" ]]; then
  export CORE_PEER_ADDRESS=peer1.IGRBank.example.com:${PEER4_PORT_PEER}
  peer channel join -b "$BLOCK"
else
  echo "Skipping peer1.IGRBank join (peer-4 is OPS-only; 3-peer layout)."
fi

export CHANNEL_NAME="$CHANNEL_NAME"
export TEST_NETWORK_HOME="$IGR_NETWORK"
export PEER4_ROLE="${PEER4_ROLE:-ops_only}"
export PEER1_PORT_PEER PEER3_PORT_PEER
export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"

./scripts/setAnchorPeer.sh 1 "${CHANNEL_NAME}"
./scripts/setAnchorPeer.sh 2 "${CHANNEL_NAME}"

echo "Join + anchors OK for channel ${CHANNEL_NAME}"
