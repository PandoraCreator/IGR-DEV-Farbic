#!/usr/bin/env bash
#
# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

# import utils
# test network home var targets to test network folder
# the reason we use a var here is considering with org3 specific folder
# when invoking this for org3 as test-network/scripts/org3-scripts
# the value is changed from default as $PWD(test-network)
# to .. as relative path to make the import works
TEST_NETWORK_HOME=${TEST_NETWORK_HOME:-${PWD}}
. ${TEST_NETWORK_HOME}/scripts/configUpdate.sh

# Anchor peer ports: local compose 7051/9051; five-server uses PEER1_PORT_PEER / PEER3_PORT_PEER from credentials.
if [[ "${LOCAL_FABRIC_NETWORK:-}" == "1" ]]; then
  ANCHOR_PRIMARY_PORT=7051
  ANCHOR_BANK_PORT=9051
else
  ANCHOR_PRIMARY_PORT="${PEER1_PORT_PEER:-7051}"
  ANCHOR_BANK_PORT="${PEER3_PORT_PEER:-9051}"
fi

# NOTE: This requires jq and configtxlator for execution.
createAnchorPeerUpdate() {
  infoln "Fetching channel config for channel $CHANNEL_NAME"
  fetchChannelConfig $ORG $CHANNEL_NAME ${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}config.json

  infoln "Generating anchor peer update transaction for Org${ORG} on channel $CHANNEL_NAME"

  if [ $ORG -eq 1 ]; then
    HOST="peer0.IGRPrimary.example.com"
    PORT="${ANCHOR_PRIMARY_PORT}"
  elif [ $ORG -eq 2 ]; then
    HOST="peer0.IGRBank.example.com"
    PORT="${ANCHOR_BANK_PORT}"
  elif [ $ORG -eq 3 ]; then
    HOST="peer0.org3.example.com"
    PORT=11051
  else
    errorln "Org${ORG} unknown"
  fi

  local cfg="${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}config.json"
  local current_host current_port
  current_host=$(jq -r --arg msp "$CORE_PEER_LOCALMSPID" \
    '.channel_group.groups.Application.groups[$msp].values.AnchorPeers.value.anchor_peers[0].host // empty' \
    "$cfg" 2>/dev/null || true)
  current_port=$(jq -r --arg msp "$CORE_PEER_LOCALMSPID" \
    '.channel_group.groups.Application.groups[$msp].values.AnchorPeers.value.anchor_peers[0].port // empty' \
    "$cfg" 2>/dev/null || true)
  if [[ "$current_host" == "$HOST" && "$current_port" == "$PORT" ]]; then
    successln "Anchor peer already set for '$CORE_PEER_LOCALMSPID' on channel '$CHANNEL_NAME': ${HOST}:${PORT}"
    ANCHOR_ALREADY_SET=true
    return 0
  fi

  set -x
  # Set or replace anchor peer for this org
  jq --arg msp "$CORE_PEER_LOCALMSPID" --arg host "$HOST" --argjson port "$PORT" \
    '.channel_group.groups.Application.groups[$msp].values.AnchorPeers = {
      "mod_policy": "Admins",
      "value": {"anchor_peers": [{"host": $host, "port": $port}]},
      "version": "0"
    }' "$cfg" > "${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}modified_config.json"
  res=$?
  { set +x; } 2>/dev/null
  verifyResult $res "Channel configuration update for anchor peer failed, make sure you have jq installed"
  

  # Compute a config update, based on the differences between 
  # {orgmsp}config.json and {orgmsp}modified_config.json, write
  # it as a transaction to {orgmsp}anchors.tx
  createConfigUpdate ${CHANNEL_NAME} ${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}config.json ${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}modified_config.json ${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}anchors.tx
}

updateAnchorPeer() {
  peer channel update -o ${FABRIC_ORDERER_HOST:-localhost}:${FABRIC_ORDERER_PORT} --ordererTLSHostnameOverride orderer.example.com -c $CHANNEL_NAME -f ${TEST_NETWORK_HOME}/channel-artifacts/${CORE_PEER_LOCALMSPID}anchors.tx --tls --cafile "$ORDERER_CA" >&log.txt
  res=$?
  cat log.txt
  verifyResult $res "Anchor peer update failed"
  successln "Anchor peer set for org '$CORE_PEER_LOCALMSPID' on channel '$CHANNEL_NAME'"
}

ORG=$1
CHANNEL_NAME=$2

setGlobals $ORG

ANCHOR_ALREADY_SET=false
createAnchorPeerUpdate

if [ "${ANCHOR_ALREADY_SET}" = "true" ]; then
  exit 0
fi

updateAnchorPeer
