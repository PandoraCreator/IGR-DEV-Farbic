#!/usr/bin/env bash
# Finish lifecycle after install + CCAAS container (approve both orgs, commit).
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
CC_NAME="${CC_NAME:-igr_anchor}"
CC_VERSION="${CC_VERSION:-1.0}"
CC_SEQUENCE="${CC_SEQUENCE:-1}"
CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"

cd "$IGR_NETWORK"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
_user_home="$(_fabric_bin_user_home 2>/dev/null || true)"
[[ -z "${_user_home}" ]] && _user_home="${HOME}"
export PATH="${_user_home}/fabric-samples/bin:${_user_home}/bin:${PATH}"
ensure_fabric_binaries
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"
export TEST_NETWORK_HOME="$IGR_NETWORK"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true

export PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
export PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"
export ORDERER_CA="$IGR_NETWORK/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem"
export CHANNEL_NAME CC_NAME CC_VERSION CC_SEQUENCE
export DELAY="${DELAY:-5}"
export MAX_RETRY="${MAX_RETRY:-10}"
export INIT_REQUIRED=""
export CC_END_POLICY=""
export CC_COLL_CONFIG=""
export VERBOSE="${VERBOSE:-false}"
export OVERRIDE_ORG="${OVERRIDE_ORG:-}"
# Remote OPS: do not wait on peer deliver (peers pull blocks from orderer separately).
export FABRIC_WAIT_FOR_EVENT="${FABRIC_WAIT_FOR_EVENT:-false}"

if [[ ! -f "${IGR_NETWORK}/${CC_NAME}.tar.gz" ]]; then
  echo "Missing ${IGR_NETWORK}/${CC_NAME}.tar.gz" >&2
  exit 1
fi
export PACKAGE_ID
PACKAGE_ID=$(peer lifecycle chaincode calculatepackageid "${IGR_NETWORK}/${CC_NAME}.tar.gz")
echo "PACKAGE_ID=${PACKAGE_ID}"

set +u
# shellcheck source=envVar.sh
. "${SCRIPT_DIR}/envVar.sh"
# shellcheck source=ccutils.sh
. "${SCRIPT_DIR}/ccutils.sh"
set -u

if ! getent hosts orderer.example.com >/dev/null 2>&1; then
  echo "WARN: /etc/hosts missing orderer.example.com on this host (peers need it too)" >&2
fi

_run_approve() {
  local org=$1
  setGlobals "$org"
  _lifecycle_wait_flags
  echo "Approving org ${org} via ${CORE_PEER_ADDRESS} ..."
  if timeout 120 peer lifecycle chaincode approveformyorg \
    -o "${FABRIC_ORDERER_HOST:-127.0.0.1}:${FABRIC_ORDERER_PORT:-8001}" \
    --ordererTLSHostnameOverride orderer.example.com --tls --cafile "$ORDERER_CA" \
    --channelID "$CHANNEL_NAME" --name "$CC_NAME" --version "$CC_VERSION" \
    --package-id "$PACKAGE_ID" --sequence "$CC_SEQUENCE" \
    "${LIFECYCLE_WAIT_FLAGS[@]}" ${INIT_REQUIRED:-} ${CC_END_POLICY:-} ${CC_COLL_CONFIG:-} \
    >log.txt 2>&1; then
    cat log.txt
    successln "Approved org ${org}"
  else
    cat log.txt
    echo "Approve org ${org} failed or timed out (120s). Check peer logs on ${CORE_PEER_ADDRESS}" >&2
    return 1
  fi
}

echo "==> Approve IGRPrimaryMSP (org 1)"
_run_approve 1
echo "==> Approve IGRBankMSP (org 2)"
_run_approve 2

checkCommitReadiness 1 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": true"
checkCommitReadiness 2 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": true"

echo "==> Commit"
commitChaincodeDefinition 1 2
queryCommitted 1
queryCommitted 2
echo "Done."
