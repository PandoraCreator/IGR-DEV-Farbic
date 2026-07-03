#!/usr/bin/env bash
# Run on OPS host (peer-4, 10.48.59.78) as user igr. IGR_NETWORK=/opt/igr-network.
# Orderer/CAs/chaincode use firewall ports 8001-8004 on peer-4 (peer-5 retired).
# Requires: Fabric CLI, Docker, docker compose plugin.
#
# Do NOT run: sudo scripts/remote-peer5-bootstrap.sh
# Use:        export SUDO_PASS='...'; bash scripts/remote-peer5-bootstrap.sh
set -euo pipefail

if [[ "$(id -u)" -eq 0 ]] && [[ "${ALLOW_ROOT_BOOTSTRAP:-}" != "1" ]]; then
  echo "Run as user igr, not root/sudo on the whole script." >&2
  echo "  export SUDO_PASS='your-password'" >&2
  echo "  bash scripts/remote-peer5-bootstrap.sh" >&2
  exit 1
fi

export IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
cd "$IGR_NETWORK"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
ensure_fabric_binaries
export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${IGR_NETWORK}/../bin:${PATH}"

export TEST_NETWORK_HOME="$IGR_NETWORK"
# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
echo "==> OPS host ports: orderer=${OPS_PORT_ORDERER} admin=${OPS_PORT_ORDERER_ADMIN} chaincode=${OPS_PORT_CHAINCODE}"
# shellcheck source=/dev/null
. "${IGR_NETWORK}/scripts/utils.sh"

echo "==> Stop any legacy Fabric stack on this host (frees ports)"
echo "${SUDO_PASS:-}" | sudo -S docker stop peer0.IGRPrimary.example.com peer0.IGRBank.example.com peer1.IGRPrimary.example.com peer1.IGRBank.example.com couchdbIGRPrimary couchdbIGRBank orderer.example.com ca_IGRPrimary ca_IGRBank ca_orderer 2>/dev/null || true
echo "${SUDO_PASS:-}" | sudo -S docker rm peer0.IGRPrimary.example.com peer0.IGRBank.example.com peer1.IGRPrimary.example.com peer1.IGRBank.example.com couchdbIGRPrimary couchdbIGRBank orderer.example.com ca_IGRPrimary ca_IGRBank ca_orderer 2>/dev/null || true

echo "==> Start Fabric CAs"
echo "${SUDO_PASS:-}" | sudo -S docker compose -f compose/compose-ca.yaml up -d

for i in $(seq 1 60); do
  if test -f organizations/fabric-ca/IGRPrimary/ca-cert.pem \
    && test -f organizations/fabric-ca/IGRBank/ca-cert.pem \
    && test -f organizations/fabric-ca/ordererOrg/ca-cert.pem; then
    echo "CA TLS roots present."
    break
  fi
  sleep 2
  if [ "$i" -eq 60 ]; then
    echo "Timeout waiting for CA cert PEMs." >&2
    exit 1
  fi
done

# shellcheck source=ca-enroll-helpers.sh
. "${SCRIPT_DIR}/ca-enroll-helpers.sh"
# shellcheck source=/dev/null
. "${IGR_NETWORK}/organizations/fabric-ca/registerEnroll.sh"

# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true

echo "==> Enroll IGRPrimary"
createIGRPrimary

echo "==> Enroll IGRBank"
createIGRBank

echo "==> Enroll orderers"
createOrderer

wait_for_orderer_admin() {
  local port="${OPS_PORT_ORDERER_ADMIN:-8002}"
  local i
  for i in $(seq 1 45); do
    if command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 "$port" 2>/dev/null; then
      echo "Orderer admin port ${port} is accepting connections."
      return 0
    fi
    sleep 2
  done
  echo "Orderer admin port ${port} not ready after 90s." >&2
  echo "${SUDO_PASS:-}" | sudo -S docker ps -a --filter name=orderer.example.com >&2 || true
  echo "--- orderer.example.com logs (last 40 lines) ---" >&2
  echo "${SUDO_PASS:-}" | sudo -S docker logs orderer.example.com 2>&1 | tail -40 >&2 || true
  return 1
}

echo "==> Start orderer (ports ${OPS_PORT_ORDERER} client, ${OPS_PORT_ORDERER_ADMIN} admin — compose/compose-test-net.yaml)"
echo "${SUDO_PASS:-}" | sudo -S docker compose -f compose/compose-test-net.yaml up -d orderer.example.com

if ! echo "${SUDO_PASS:-}" | sudo -S docker ps --filter name=orderer.example.com --filter status=running -q | grep -q .; then
  echo "orderer.example.com is not running. Check: sudo docker logs orderer.example.com" >&2
  exit 1
fi

wait_for_orderer_admin

echo "==> Create channel block + OSN admin join"
export CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
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

echo "OPS bootstrap complete on $(hostname) (orderer/CAs/channel join)."
