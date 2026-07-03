#!/usr/bin/env bash
# Run on peer-1 … peer-5 with PEER_NODE=1|2|3|4|5 and SUDO_PASS set.
# Ports match docs/5-SERVER-DEPLOYMENT.md / docs/servers.credentials.local.example (ESDS firewall).
set -euo pipefail

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
SUDO_PASS="${SUDO_PASS:?need SUDO_PASS}"
export PEER_NODE="${PEER_NODE:?need PEER_NODE 1-5}"

# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true

# Defaults = ESDS open ranges (override via servers.credentials.local on the peer)
PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
PEER1_PORT_CHAINCODE="${PEER1_PORT_CHAINCODE:-5002}"
PEER1_PORT_COUCH="${PEER1_PORT_COUCH:-5003}"
PEER1_PORT_OPS="${PEER1_PORT_OPS:-5004}"

PEER2_PORT_PEER="${PEER2_PORT_PEER:-6001}"
PEER2_PORT_CHAINCODE="${PEER2_PORT_CHAINCODE:-6002}"
PEER2_PORT_COUCH="${PEER2_PORT_COUCH:-6003}"
PEER2_PORT_OPS="${PEER2_PORT_OPS:-6004}"

PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
PEER3_PORT_CHAINCODE="${PEER3_PORT_CHAINCODE:-7002}"
PEER3_PORT_COUCH="${PEER3_PORT_COUCH:-7003}"
PEER3_PORT_OPS="${PEER3_PORT_OPS:-7004}"

PEER4_PORT_PEER="${PEER4_PORT_PEER:-8001}"
PEER4_PORT_CHAINCODE="${PEER4_PORT_CHAINCODE:-8002}"
PEER4_PORT_COUCH="${PEER4_PORT_COUCH:-8003}"
PEER4_PORT_OPS="${PEER4_PORT_OPS:-8004}"

PEER5_PORT_PEER="${PEER5_PORT_PEER:-9001}"
PEER5_PORT_CHAINCODE="${PEER5_PORT_CHAINCODE:-9002}"
PEER5_PORT_COUCH="${PEER5_PORT_COUCH:-9003}"
PEER5_PORT_OPS="${PEER5_PORT_OPS:-9004}"

H_OPS="${OPS_HOST:-${PEER4_HOST:-10.48.59.78}}"
H_P0="${PEER1_HOST:-10.48.59.70}"
H_P1="${PEER2_HOST:-10.48.59.76}"
H_P2="${PEER3_HOST:-10.48.59.77}"
H_P5="${PEER5_HOST:-10.48.59.79}"
# Peers must resolve orderer + CCAAS (inside Docker, host /etc/hosts is not enough).
PEER_EXTRA_HOSTS=(
  --add-host "orderer.example.com:${H_OPS}"
  --add-host "chaincode.igr.example.com:${H_OPS}"
  --add-host "peer0.IGRPrimary.example.com:${H_P0}"
  --add-host "peer1.IGRPrimary.example.com:${H_P1}"
  --add-host "peer0.IGRBank.example.com:${H_P2}"
  --add-host "peer1.IGRBank.example.com:${H_P5}"
)

dock() {
  echo "$SUDO_PASS" | sudo -S docker "$@"
}

ensure_peer_docker_net() {
  dock network create igr_peer_net 2>/dev/null || true
}

ensure_peer_ledger_vol() {
  dock volume create "$1" >/dev/null 2>&1 || true
}

cd "$IGR_NETWORK"

dock stop couchdbIGRPrimary couchdbIGRBank \
  peer0.IGRPrimary.example.com peer1.IGRPrimary.example.com \
  peer0.IGRBank.example.com peer1.IGRBank.example.com \
  >/dev/null 2>&1 || true
dock rm couchdbIGRPrimary couchdbIGRBank \
  peer0.IGRPrimary.example.com peer1.IGRPrimary.example.com \
  peer0.IGRBank.example.com peer1.IGRBank.example.com \
  >/dev/null 2>&1 || true

case "$PEER_NODE" in
1)
  PP=$PEER1_PORT_PEER PC=$PEER1_PORT_CHAINCODE PCH=$PEER1_PORT_COUCH PO=$PEER1_PORT_OPS
  COUCH_NAME=couchdbIGRPrimary
  ensure_peer_docker_net
  ensure_peer_ledger_vol peer0_igrprimary_ledger
  dock run -d --name "$COUCH_NAME" --network igr_peer_net --restart unless-stopped \
    -p "${PCH}:5984" \
    -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
    couchdb:3.4.2

  dock run -d --name peer0.IGRPrimary.example.com --network igr_peer_net --restart unless-stopped \
    "${PEER_EXTRA_HOSTS[@]}" \
    -p "${PP}:${PP}" -p "${PC}:${PC}" -p "${PO}:${PO}" \
    -v peer0_igrprimary_ledger:/var/hyperledger/production \
    -v "$IGR_NETWORK/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com:/etc/hyperledger/fabric" \
    -v "$IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg" \
    -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
    -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/fabric/msp \
    -e CORE_PEER_ID=peer0.IGRPrimary.example.com \
    -e CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:${PP} \
    -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PP} \
    -e CORE_PEER_CHAINCODEADDRESS=peer0.IGRPrimary.example.com:${PC} \
    -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PC} \
    -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer0.IGRPrimary.example.com:${PP} \
    -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRPrimary.example.com:${PP} \
    -e CORE_PEER_LOCALMSPID=IGRPrimaryMSP \
    -e CORE_PEER_TLS_ENABLED=true \
    -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
    -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
    -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
    -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=${COUCH_NAME}:5984 \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
    -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PO} \
    -e CORE_METRICS_PROVIDER=prometheus \
    -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer0IGRPrimary"}' \
    hyperledger/fabric-peer:latest peer node start
  ;;
2)
  PP=$PEER2_PORT_PEER PC=$PEER2_PORT_CHAINCODE PCH=$PEER2_PORT_COUCH PO=$PEER2_PORT_OPS
  COUCH_NAME=couchdbIGRPrimary
  ensure_peer_docker_net
  ensure_peer_ledger_vol peer1_igrprimary_ledger
  dock run -d --name "$COUCH_NAME" --network igr_peer_net --restart unless-stopped \
    -p "${PCH}:5984" \
    -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
    couchdb:3.4.2

  dock run -d --name peer1.IGRPrimary.example.com --network igr_peer_net --restart unless-stopped \
    "${PEER_EXTRA_HOSTS[@]}" \
    -p "${PP}:${PP}" -p "${PC}:${PC}" -p "${PO}:${PO}" \
    -v peer1_igrprimary_ledger:/var/hyperledger/production \
    -v "$IGR_NETWORK/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer1.IGRPrimary.example.com:/etc/hyperledger/fabric" \
    -v "$IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg" \
    -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
    -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/fabric/msp \
    -e CORE_PEER_ID=peer1.IGRPrimary.example.com \
    -e CORE_PEER_ADDRESS=peer1.IGRPrimary.example.com:${PP} \
    -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PP} \
    -e CORE_PEER_CHAINCODEADDRESS=peer1.IGRPrimary.example.com:${PC} \
    -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PC} \
    -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer1.IGRPrimary.example.com:${PP} \
    -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER} \
    -e CORE_PEER_LOCALMSPID=IGRPrimaryMSP \
    -e CORE_PEER_TLS_ENABLED=true \
    -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
    -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
    -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
    -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=${COUCH_NAME}:5984 \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
    -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PO} \
    -e CORE_METRICS_PROVIDER=prometheus \
    -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer1IGRPrimary"}' \
    hyperledger/fabric-peer:latest peer node start
  ;;
3)
  PP=$PEER3_PORT_PEER PC=$PEER3_PORT_CHAINCODE PCH=$PEER3_PORT_COUCH PO=$PEER3_PORT_OPS
  COUCH_NAME=couchdbIGRBank
  ensure_peer_docker_net
  ensure_peer_ledger_vol peer0_igrbank_ledger
  dock run -d --name "$COUCH_NAME" --network igr_peer_net --restart unless-stopped \
    -p "${PCH}:5984" \
    -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
    couchdb:3.4.2

  dock run -d --name peer0.IGRBank.example.com --network igr_peer_net --restart unless-stopped \
    "${PEER_EXTRA_HOSTS[@]}" \
    -p "${PP}:${PP}" -p "${PC}:${PC}" -p "${PO}:${PO}" \
    -v peer0_igrbank_ledger:/var/hyperledger/production \
    -v "$IGR_NETWORK/organizations/peerOrganizations/IGRBank.example.com/peers/peer0.IGRBank.example.com:/etc/hyperledger/fabric" \
    -v "$IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg" \
    -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
    -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/fabric/msp \
    -e CORE_PEER_ID=peer0.IGRBank.example.com \
    -e CORE_PEER_ADDRESS=peer0.IGRBank.example.com:${PP} \
    -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PP} \
    -e CORE_PEER_CHAINCODEADDRESS=peer0.IGRBank.example.com:${PC} \
    -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PC} \
    -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer0.IGRBank.example.com:${PP} \
    -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRBank.example.com:${PP} \
    -e CORE_PEER_LOCALMSPID=IGRBankMSP \
    -e CORE_PEER_TLS_ENABLED=true \
    -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
    -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
    -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
    -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=${COUCH_NAME}:5984 \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
    -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PO} \
    -e CORE_METRICS_PROVIDER=prometheus \
    -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer0IGRBank"}' \
    hyperledger/fabric-peer:latest peer node start
  ;;
4)
  if [[ "${PEER4_ROLE:-}" == "ops_only" ]]; then
    echo "peer-4 (${PEER4_HOST:-10.48.59.78}) is OPS-only: orderer, CAs, chaincode." >&2
    echo "Do not start a peer here. Use PEER_NODE=5 on peer-5 for peer1.IGRBank." >&2
    exit 0
  fi
  PP=$PEER4_PORT_PEER PC=$PEER4_PORT_CHAINCODE PCH=$PEER4_PORT_COUCH PO=$PEER4_PORT_OPS
  COUCH_NAME=couchdbIGRBank
  ensure_peer_docker_net
  ensure_peer_ledger_vol peer1_igrbank_ledger
  dock run -d --name "$COUCH_NAME" --network igr_peer_net --restart unless-stopped \
    -p "${PCH}:5984" \
    -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
    couchdb:3.4.2

  dock run -d --name peer1.IGRBank.example.com --network igr_peer_net --restart unless-stopped \
    "${PEER_EXTRA_HOSTS[@]}" \
    -p "${PP}:${PP}" -p "${PC}:${PC}" -p "${PO}:${PO}" \
    -v peer1_igrbank_ledger:/var/hyperledger/production \
    -v "$IGR_NETWORK/organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com:/etc/hyperledger/fabric" \
    -v "$IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg" \
    -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
    -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/fabric/msp \
    -e CORE_PEER_ID=peer1.IGRBank.example.com \
    -e CORE_PEER_ADDRESS=peer1.IGRBank.example.com:${PP} \
    -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PP} \
    -e CORE_PEER_CHAINCODEADDRESS=peer1.IGRBank.example.com:${PC} \
    -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PC} \
    -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer1.IGRBank.example.com:${PP} \
    -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRBank.example.com:${PEER3_PORT_PEER} \
    -e CORE_PEER_LOCALMSPID=IGRBankMSP \
    -e CORE_PEER_TLS_ENABLED=true \
    -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
    -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
    -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
    -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=${COUCH_NAME}:5984 \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
    -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PO} \
    -e CORE_METRICS_PROVIDER=prometheus \
    -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer1IGRBank"}' \
    hyperledger/fabric-peer:latest peer node start
  ;;
5)
  PP=$PEER5_PORT_PEER PC=$PEER5_PORT_CHAINCODE PCH=$PEER5_PORT_COUCH PO=$PEER5_PORT_OPS
  COUCH_NAME=couchdbIGRBank
  ensure_peer_docker_net
  ensure_peer_ledger_vol peer1_igrbank_ledger
  dock run -d --name "$COUCH_NAME" --network igr_peer_net --restart unless-stopped \
    -p "${PCH}:5984" \
    -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
    couchdb:3.4.2

  dock run -d --name peer1.IGRBank.example.com --network igr_peer_net --restart unless-stopped \
    "${PEER_EXTRA_HOSTS[@]}" \
    -p "${PP}:${PP}" -p "${PC}:${PC}" -p "${PO}:${PO}" \
    -v peer1_igrbank_ledger:/var/hyperledger/production \
    -v "$IGR_NETWORK/organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com:/etc/hyperledger/fabric" \
    -v "$IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg" \
    -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
    -e CORE_PEER_MSPCONFIGPATH=/etc/hyperledger/fabric/msp \
    -e CORE_PEER_ID=peer1.IGRBank.example.com \
    -e CORE_PEER_ADDRESS=peer1.IGRBank.example.com:${PP} \
    -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PP} \
    -e CORE_PEER_CHAINCODEADDRESS=peer1.IGRBank.example.com:${PC} \
    -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PC} \
    -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer1.IGRBank.example.com:${PP} \
    -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRBank.example.com:${PEER3_PORT_PEER} \
    -e CORE_PEER_LOCALMSPID=IGRBankMSP \
    -e CORE_PEER_TLS_ENABLED=true \
    -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
    -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
    -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
    -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=${COUCH_NAME}:5984 \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
    -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
    -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PO} \
    -e CORE_METRICS_PROVIDER=prometheus \
    -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer1IGRBank"}' \
    hyperledger/fabric-peer:latest peer node start
  ;;
*)
  echo "PEER_NODE must be 1-5" >&2
  exit 1
  ;;
esac

echo "Docker ps for this peer:"
dock ps --format '{{.Names}}'
