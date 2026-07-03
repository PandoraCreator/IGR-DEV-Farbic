#!/usr/bin/env bash
#
# Copyright IBM Corp All Rights Reserved
#
# SPDX-License-Identifier: Apache-2.0
#

# This is a collection of bash functions used by different scripts

# imports
# test network home var targets to test-network folder
# the reason we use a var here is to accommodate scenarios
# where execution occurs from folders outside of default as $PWD, such as the test-network/addOrg3 folder.
# For setting environment variables, simple relative paths like ".." could lead to unintended references
# due to how they interact with FABRIC_CFG_PATH. It's advised to specify paths more explicitly,
# such as using "../${PWD}", to ensure that Fabric's environment variables are pointing to the correct paths.
TEST_NETWORK_HOME=${TEST_NETWORK_HOME:-${PWD}}
. ${TEST_NETWORK_HOME}/scripts/utils.sh

# Optional prod credentials — orderer listening on host mapped port (five-server firewall)
if [[ -f "${TEST_NETWORK_HOME}/docs/servers.credentials.local" ]]; then
  # shellcheck disable=SC1091
  source "${TEST_NETWORK_HOME}/docs/servers.credentials.local"
fi

# Host ports mapped in compose — see compose/compose-test-net.yaml orderer.service
if [[ -f "${TEST_NETWORK_HOME}/scripts/load-ops-env.sh" ]]; then
  # shellcheck source=scripts/load-ops-env.sh
  . "${TEST_NETWORK_HOME}/scripts/load-ops-env.sh"
  load_ops_env
else
  export FABRIC_ORDERER_PORT="${PEER5_PORT_ORDERER:-8001}"
  export FABRIC_ORDERER_ADMIN_PORT="${PEER5_PORT_ORDERER_ADMIN:-8002}"
fi

# Local network.sh uses docker compose on localhost (7051/9051), not prod VM hostnames.
local_fabric_network() {
  [[ "${LOCAL_FABRIC_NETWORK:-}" == "1" ]]
}

export CORE_PEER_TLS_ENABLED=true
export ORDERER_CA=${TEST_NETWORK_HOME}/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export PEER0_ORG1_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem
export PEER0_ORG2_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem
export PEER0_ORG3_CA=${TEST_NETWORK_HOME}/organizations/peerOrganizations/org3.example.com/tlsca/tlsca.org3.example.com-cert.pem

# Set environment variables for the peer org
setGlobals() {
  local USING_ORG=""
  if [ -z "${OVERRIDE_ORG:-}" ]; then
    USING_ORG=$1
  else
    USING_ORG="${OVERRIDE_ORG}"
  fi
  infoln "Using organization ${USING_ORG}"
  if [ $USING_ORG -eq 1 ]; then
    export CORE_PEER_LOCALMSPID=IGRPrimaryMSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_ORG1_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp
    if local_fabric_network; then
      export CORE_PEER_ADDRESS=localhost:7051
      export CORE_PEER_TLS_SERVERHOSTOVERRIDE=peer0.IGRPrimary.example.com
    elif [ -n "${PEER1_PORT_PEER:-}" ]; then
      export CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}
      unset CORE_PEER_TLS_SERVERHOSTOVERRIDE
    else
      export CORE_PEER_ADDRESS=localhost:7051
      export CORE_PEER_TLS_SERVERHOSTOVERRIDE=peer0.IGRPrimary.example.com
    fi
  elif [ $USING_ORG -eq 2 ]; then
    export CORE_PEER_LOCALMSPID=IGRBankMSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_ORG2_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp
    if local_fabric_network; then
      export CORE_PEER_ADDRESS=localhost:9051
      export CORE_PEER_TLS_SERVERHOSTOVERRIDE=peer0.IGRBank.example.com
    elif [ -n "${PEER3_PORT_PEER:-}" ]; then
      export CORE_PEER_ADDRESS=peer0.IGRBank.example.com:${PEER3_PORT_PEER}
      unset CORE_PEER_TLS_SERVERHOSTOVERRIDE
    else
      export CORE_PEER_ADDRESS=localhost:9051
      export CORE_PEER_TLS_SERVERHOSTOVERRIDE=peer0.IGRBank.example.com
    fi
  elif [ $USING_ORG -eq 3 ]; then
    export CORE_PEER_LOCALMSPID=Org3MSP
    export CORE_PEER_TLS_ROOTCERT_FILE=$PEER0_ORG3_CA
    export CORE_PEER_MSPCONFIGPATH=${TEST_NETWORK_HOME}/organizations/peerOrganizations/org3.example.com/users/Admin@org3.example.com/msp
    export CORE_PEER_ADDRESS=localhost:11051
  else
    errorln "ORG Unknown"
  fi

  if [ "${VERBOSE:-}" = "true" ]; then
    env | grep CORE
  fi
}

# parsePeerConnectionParameters $@
# Helper function that sets the peer connection parameters for a chaincode
# operation
parsePeerConnectionParameters() {
  PEER_CONN_PARMS=()
  PEERS=""
  while [ "$#" -gt 0 ]; do
    setGlobals $1
    PEER="peer0.org$1"
    ## Set peer addresses
    if [ -z "${PEERS:-}" ]
    then
	PEERS="$PEER"
    else
	PEERS="$PEERS $PEER"
    fi
    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" --peerAddresses $CORE_PEER_ADDRESS)
    ## Set path to TLS certificate
    CA=PEER0_ORG$1_CA
    TLSINFO=(--tlsRootCertFiles "${!CA}")
    PEER_CONN_PARMS=("${PEER_CONN_PARMS[@]}" "${TLSINFO[@]}")
    # shift by one to get to the next organization
    shift
  done
}

verifyResult() {
  if [ $1 -ne 0 ]; then
    fatalln "$2"
  fi
}
