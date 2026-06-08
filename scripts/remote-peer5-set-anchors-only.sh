#!/usr/bin/env bash
# Set anchor peers after channel join (OPS host peer-4). Idempotent re-run.
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"
export CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
export TEST_NETWORK_HOME="$IGR_NETWORK"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true
export PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
export PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"

./scripts/setAnchorPeer.sh 1 "${CHANNEL_NAME}"
./scripts/setAnchorPeer.sh 2 "${CHANNEL_NAME}"
echo "Anchor peers OK on ${CHANNEL_NAME}"
