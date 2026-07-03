#!/usr/bin/env bash
# Re-join igrchannel on all remote peers (no anchor update). Run on peer-4 OPS.
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"
CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
BLOCK="${IGR_NETWORK}/channel-artifacts/${CHANNEL_NAME}.block"

[[ -f "$BLOCK" ]] || { echo "Missing $BLOCK — run bootstrap / configtxgen first" >&2; exit 1; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"

# shellcheck disable=SC1091
[[ -f docs/servers.credentials.local ]] && source docs/servers.credentials.local || true
PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
PEER2_PORT_PEER="${PEER2_PORT_PEER:-6001}"
PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
PEER5_PORT_PEER="${PEER5_PORT_PEER:-9001}"

_join() {
  local msp=$1 tls=$2 msppath=$3 addr=$4
  export CORE_PEER_LOCALMSPID="$msp"
  export CORE_PEER_TLS_ENABLED=true
  export CORE_PEER_TLS_ROOTCERT_FILE="$tls"
  export CORE_PEER_MSPCONFIGPATH="$msppath"
  export CORE_PEER_ADDRESS="$addr"
  echo "==> join $addr"
  peer channel join -b "$BLOCK"
  peer channel list
}

TLS_P="${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem"
MSP_P="${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp"
TLS_B="${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem"
MSP_B="${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp"

_join IGRPrimaryMSP "$TLS_P" "$MSP_P" "peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}"
_join IGRPrimaryMSP "$TLS_P" "$MSP_P" "peer1.IGRPrimary.example.com:${PEER2_PORT_PEER}"
_join IGRBankMSP "$TLS_B" "$MSP_B" "peer0.IGRBank.example.com:${PEER3_PORT_PEER}"
_join IGRBankMSP "$TLS_B" "$MSP_B" "peer1.IGRBank.example.com:${PEER5_PORT_PEER}"

echo "Join OK for ${CHANNEL_NAME}"
