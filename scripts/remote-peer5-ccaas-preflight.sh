#!/usr/bin/env bash
# Connectivity checks before lifecycle approve/commit (run on peer-4 OPS host).
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true

OPS_IP="${OPS_HOST:-${PEER4_HOST:-10.48.59.78}}"
ORDERER_PORT="${OPS_PORT_ORDERER:-8001}"
CC_PORT="${OPS_PORT_CHAINCODE:-8003}"
P1="${PEER1_HOST:-10.48.59.70}"
P2="${PEER2_HOST:-10.48.59.76}"
P3="${PEER3_HOST:-10.48.59.77}"

check() {
  local label=$1 host=$2 port=$3
  if nc -zv -w 3 "$host" "$port" 2>&1 | grep -q succeeded; then
    echo "OK   $label ($host:$port)"
  else
    echo "FAIL $label ($host:$port)"
  fi
}

echo "=== From peer-4 (this host) ==="
check "orderer" "127.0.0.1" "$ORDERER_PORT"
check "CCAAS" "127.0.0.1" "$CC_PORT"
if getent hosts orderer.example.com >/dev/null 2>&1; then
  check "orderer DNS" "orderer.example.com" "$ORDERER_PORT"
else
  echo "FAIL orderer DNS — add to /etc/hosts: ${OPS_IP} orderer.example.com chaincode.igr.example.com"
fi
if getent hosts chaincode.igr.example.com >/dev/null 2>&1; then
  check "chaincode DNS" "chaincode.igr.example.com" "$CC_PORT"
else
  echo "FAIL chaincode DNS — add to /etc/hosts (see above)"
fi
check "peer0 IGRPrimary" "peer0.IGRPrimary.example.com" "${PEER1_PORT_PEER:-5001}"
check "peer0 IGRBank" "peer0.IGRBank.example.com" "${PEER3_PORT_PEER:-7001}"

echo ""
echo "=== Remote peers must reach OPS (open firewall ${ORDERER_PORT}, ${CC_PORT} on ${OPS_IP}) ==="
check "peer-1 -> orderer" "$OPS_IP" "$ORDERER_PORT"
check "peer-1 -> CCAAS" "$OPS_IP" "$CC_PORT"

echo ""
echo "=== CCAAS container (peer-4) ==="
if command -v docker >/dev/null 2>&1; then
  docker ps --filter name=igr_anchor --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null \
    || sudo docker ps --filter name=igr_anchor --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null \
    || echo "Run: sudo docker ps | grep igr_anchor"
else
  echo "docker not in PATH"
fi

echo ""
echo "On peer-1 (${P1}) run:"
echo "  grep -E 'orderer|chaincode' /etc/hosts"
echo "  nc -zv orderer.example.com ${ORDERER_PORT}"
echo "  nc -zv chaincode.igr.example.com ${CC_PORT}"
echo "  sudo docker logs peer0.IGRPrimary.example.com 2>&1 | tail -30"
