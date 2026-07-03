#!/usr/bin/env bash
# Retry channel block + osnadmin join after orderer is healthy (peer-5, user igr).
set -euo pipefail

export IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${IGR_NETWORK}/../bin:${PATH}"

export CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env

if ! command -v nc >/dev/null || ! nc -z 127.0.0.1 "$OPS_PORT_ORDERER_ADMIN" 2>/dev/null; then
  echo "Port ${OPS_PORT_ORDERER_ADMIN} not open. Start orderer first:" >&2
  echo "  export SUDO_PASS='...'" >&2
  echo "  sudo docker compose -f compose/compose-test-net.yaml up -d orderer.example.com" >&2
  echo "  sudo docker logs orderer.example.com" >&2
  exit 1
fi

mkdir -p channel-artifacts
FABRIC_CFG_PATH=$PWD/configtx configtxgen -profile ChannelUsingRaft \
  -outputBlock "./channel-artifacts/${CHANNEL_NAME}.block" \
  -channelID "${CHANNEL_NAME}"

ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
ORDERER_ADMIN_TLS_SIGN_CERT=$PWD/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt
ORDERER_ADMIN_TLS_PRIVATE_KEY=$PWD/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.key

osnadmin channel join \
  --channelID "${CHANNEL_NAME}" \
  --config-block "./channel-artifacts/${CHANNEL_NAME}.block" \
  -o "localhost:${OPS_PORT_ORDERER_ADMIN}" \
  --ca-file "$ORDERER_CA" \
  --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" \
  --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"

echo "Channel ${CHANNEL_NAME} joined on orderer (osnadmin OK)."
