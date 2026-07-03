#!/usr/bin/env bash
# Fix lifecycle after channel re-join: reinstall package, restart CCAAS, approve+commit at sequence 2.
# Run on peer-4 OPS after remote-peer5-channel-join-only.sh succeeded on all peers.
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
CC_NAME="${CC_NAME:-igr_anchor}"
CC_VERSION="${CC_VERSION:-1.0}"
CC_SEQUENCE="${CC_SEQUENCE:-2}"
CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
CC_CONTAINER="${CC_CONTAINER:-igr_anchor_ccaas}"
CC_IMAGE="${CC_IMAGE:-igr_anchor_ccaas}"

cd "$IGR_NETWORK"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

export SUDO_PASS="${SUDO_PASS:-}"
docker_cmd() {
  if docker info >/dev/null 2>&1; then docker "$@"; return; fi
  if [[ -n "${SUDO_PASS}" ]]; then echo "${SUDO_PASS}" | sudo -S docker "$@"; else sudo docker "$@"; fi
}

# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f docs/servers.credentials.local ]] && source docs/servers.credentials.local || true

PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
PEER2_PORT_PEER="${PEER2_PORT_PEER:-6001}"
PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
CC_PORT="${OPS_PORT_CHAINCODE:-8003}"

[[ -f "${IGR_NETWORK}/${CC_NAME}.tar.gz" ]] || { echo "Missing ${CC_NAME}.tar.gz" >&2; exit 1; }
export PACKAGE_ID
PACKAGE_ID=$(peer lifecycle chaincode calculatepackageid "${IGR_NETWORK}/${CC_NAME}.tar.gz")
echo "PACKAGE_ID=${PACKAGE_ID}"
echo "Using lifecycle sequence ${CC_SEQUENCE} (bump if sequence 1 is corrupted)"

TLS_PRIMARY="${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem"
TLS_BANK="${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem"
MSP_PRIMARY="${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp"
MSP_BANK="${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp"

_install() {
  local msp_id=$1 tls=$2 msp=$3 addr=$4
  export CORE_PEER_LOCALMSPID="$msp_id"
  export CORE_PEER_TLS_ENABLED=true
  export CORE_PEER_TLS_ROOTCERT_FILE="$tls"
  export CORE_PEER_MSPCONFIGPATH="$msp"
  export CORE_PEER_ADDRESS="$addr"
  echo "==> install on $addr"
  peer lifecycle chaincode install "${IGR_NETWORK}/${CC_NAME}.tar.gz"
}

_install IGRPrimaryMSP "$TLS_PRIMARY" "$MSP_PRIMARY" "peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}"
_install IGRPrimaryMSP "$TLS_PRIMARY" "$MSP_PRIMARY" "peer1.IGRPrimary.example.com:${PEER2_PORT_PEER}"
_install IGRBankMSP "$TLS_BANK" "$MSP_BANK" "peer0.IGRBank.example.com:${PEER3_PORT_PEER}"

echo "==> Restart CCAAS with PACKAGE_ID"
docker_cmd rm -f "$CC_CONTAINER" 2>/dev/null || true
docker_cmd run -d --name "$CC_CONTAINER" --restart unless-stopped \
  -p "${CC_PORT}:${CC_PORT}" \
  -e "CHAINCODE_SERVER_ADDRESS=0.0.0.0:${CC_PORT}" \
  -e "CHAINCODE_ID=${PACKAGE_ID}" \
  -e "CORE_CHAINCODE_ID_NAME=${PACKAGE_ID}" \
  "${CC_IMAGE}:latest"

echo "==> Approve + commit (sequence ${CC_SEQUENCE})"
export CC_SEQUENCE CHANNEL_NAME CC_NAME CC_VERSION
export FABRIC_WAIT_FOR_EVENT="${FABRIC_WAIT_FOR_EVENT:-false}"
export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"
bash "${SCRIPT_DIR}/remote-peer5-ccaas-approve-commit.sh"
