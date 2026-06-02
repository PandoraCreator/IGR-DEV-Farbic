#!/usr/bin/env bash
# Source from bootstrap / channel scripts. Normalizes OPS host ports (peer-4) vs legacy PEER5.
load_ops_env() {
  local net="${IGR_NETWORK:-/opt/igr-network}"
  local creds="${net}/docs/servers.credentials.local"
  if [[ -f "$creds" ]]; then
    # shellcheck disable=SC1090
    source "$creds"
  fi

  export OPS_HOST="${OPS_HOST:-${PEER4_HOST:-${PEER5_HOST:-}}}"
  export OPS_PORT_ORDERER="${OPS_PORT_ORDERER:-${PEER4_PORT_ORDERER:-${PEER5_PORT_ORDERER:-8001}}}"
  export OPS_PORT_ORDERER_ADMIN="${OPS_PORT_ORDERER_ADMIN:-${PEER4_PORT_ORDERER_ADMIN:-${PEER5_PORT_ORDERER_ADMIN:-8002}}}"
  export OPS_PORT_CHAINCODE="${OPS_PORT_CHAINCODE:-${PEER4_PORT_CHAINCODE:-${PEER5_PORT_CHAINCODE:-8003}}}"
  export OPS_PORT_OPS="${OPS_PORT_OPS:-${PEER4_PORT_OPS:-${PEER5_PORT_OPS:-8004}}}"

  export PEER5_PORT_ORDERER="$OPS_PORT_ORDERER"
  export PEER5_PORT_ORDERER_ADMIN="$OPS_PORT_ORDERER_ADMIN"
  export PEER5_PORT_CHAINCODE="$OPS_PORT_CHAINCODE"
  export PEER5_PORT_OPS="$OPS_PORT_OPS"
  export FABRIC_ORDERER_PORT="$OPS_PORT_ORDERER"
  export FABRIC_ORDERER_ADMIN_PORT="$OPS_PORT_ORDERER_ADMIN"
  # Deliver client on the OPS VM: use loopback (TLS SNI still orderer.example.com).
  export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  export IGR_NETWORK="${IGR_NETWORK:-$(cd "$(dirname "$0")/.." && pwd)}"
  load_ops_env
  echo "OPS_HOST=${OPS_HOST:-unset}"
  echo "ORDERER=${OPS_PORT_ORDERER} ADMIN=${OPS_PORT_ORDERER_ADMIN} CC=${OPS_PORT_CHAINCODE}"
fi
