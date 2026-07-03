#!/usr/bin/env bash

# shellcheck source=../../scripts/ca-enroll-helpers.sh
. "${PWD}/scripts/ca-enroll-helpers.sh"

function createIGRPrimary() {
  local org_domain=IGRPrimary.example.com
  local org_home="${PWD}/organizations/peerOrganizations/${org_domain}"
  local ca_cert="${PWD}/organizations/fabric-ca/IGRPrimary/ca-cert.pem"
  local peer0_host="${PEER1_HOST:-10.48.59.70}"
  local peer1_host="${PEER2_HOST:-10.48.59.76}"

  infoln "Enrolling IGRPrimary CA admin"
  rm -rf "${org_home}/msp" "${org_home}/users" "${org_home}/peers" 2>/dev/null || true
  mkdir -p "$org_home"

  export FABRIC_CA_CLIENT_HOME="$org_home"
  fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 \
    --caname ca-IGRPrimary --tls.certfiles "$ca_cert"

  write_node_ous "$org_home"
  copy_org_tls_roots "$ca_cert" "$org_home" "$org_domain"

  infoln "Registering IGRPrimary identities"
  fabric-ca-client register --caname ca-IGRPrimary --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRPrimary --id.name peer1 --id.secret peer1pw --id.type peer --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRPrimary --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRPrimary --id.name igrprimaryadmin --id.secret igrprimaryadminpw --id.type admin --tls.certfiles "$ca_cert" || true

  infoln "Generating peer0 IGRPrimary MSP and TLS"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/peers/peer0.${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/peers/peer0.${org_domain}/tls" --enrollment.profile tls \
    --csr.hosts "peer0.${org_domain}" --csr.hosts "$peer0_host" --csr.hosts localhost \
    --tls.certfiles "$ca_cert"

  infoln "Generating peer1 IGRPrimary MSP and TLS"
  fabric-ca-client enroll -u https://peer1:peer1pw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/peers/peer1.${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://peer1:peer1pw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/peers/peer1.${org_domain}/tls" --enrollment.profile tls \
    --csr.hosts "peer1.${org_domain}" --csr.hosts "$peer1_host" --csr.hosts localhost \
    --tls.certfiles "$ca_cert"

  infoln "Generating IGRPrimary user and admin MSP"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/users/User1@${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://igrprimaryadmin:igrprimaryadminpw@localhost:7054 --caname ca-IGRPrimary \
    -M "${org_home}/users/Admin@${org_domain}/msp" --tls.certfiles "$ca_cert"

  propagate_org_msp_config "$org_home" "$org_domain"
  cp "${org_home}/msp/config.yaml" "${org_home}/users/User1@${org_domain}/msp/config.yaml"

  for P in peer0 peer1; do
    install_peer_tls_files "$org_domain" "$P"
  done
}

function createIGRBank() {
  local org_domain=IGRBank.example.com
  local org_home="${PWD}/organizations/peerOrganizations/${org_domain}"
  local ca_cert="${PWD}/organizations/fabric-ca/IGRBank/ca-cert.pem"
  local peer0_host="${PEER3_HOST:-10.48.59.77}"
  local peer1_host="${PEER5_HOST:-10.48.59.79}"

  infoln "Enrolling IGRBank CA admin"
  rm -rf "${org_home}/msp" "${org_home}/users" "${org_home}/peers" 2>/dev/null || true
  mkdir -p "$org_home"

  export FABRIC_CA_CLIENT_HOME="$org_home"
  fabric-ca-client enroll -u https://admin:adminpw@localhost:8054 \
    --caname ca-IGRBank --tls.certfiles "$ca_cert"

  write_node_ous "$org_home"
  copy_org_tls_roots "$ca_cert" "$org_home" "$org_domain"

  infoln "Registering IGRBank identities"
  fabric-ca-client register --caname ca-IGRBank --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRBank --id.name peer1 --id.secret peer1pw --id.type peer --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRBank --id.name user1 --id.secret user1pw --id.type client --tls.certfiles "$ca_cert" || true
  fabric-ca-client register --caname ca-IGRBank --id.name igrbankadmin --id.secret igrbankadminpw --id.type admin --tls.certfiles "$ca_cert" || true

  infoln "Generating peer0 IGRBank MSP and TLS"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/peers/peer0.${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/peers/peer0.${org_domain}/tls" --enrollment.profile tls \
    --csr.hosts "peer0.${org_domain}" --csr.hosts "$peer0_host" --csr.hosts localhost \
    --tls.certfiles "$ca_cert"

  infoln "Generating peer1 IGRBank MSP and TLS"
  fabric-ca-client enroll -u https://peer1:peer1pw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/peers/peer1.${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://peer1:peer1pw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/peers/peer1.${org_domain}/tls" --enrollment.profile tls \
    --csr.hosts "peer1.${org_domain}" --csr.hosts "$peer1_host" --csr.hosts localhost \
    --tls.certfiles "$ca_cert"

  infoln "Generating IGRBank user and admin MSP"
  fabric-ca-client enroll -u https://user1:user1pw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/users/User1@${org_domain}/msp" --tls.certfiles "$ca_cert"
  fabric-ca-client enroll -u https://igrbankadmin:igrbankadminpw@localhost:8054 --caname ca-IGRBank \
    -M "${org_home}/users/Admin@${org_domain}/msp" --tls.certfiles "$ca_cert"

  propagate_org_msp_config "$org_home" "$org_domain"
  cp "${org_home}/msp/config.yaml" "${org_home}/users/User1@${org_domain}/msp/config.yaml"

  for P in peer0 peer1; do
    install_peer_tls_files "$org_domain" "$P"
  done
}

function createOrderer() {
  local orderer_home="${PWD}/organizations/ordererOrganizations/example.com"
  local ca_cert="${PWD}/organizations/fabric-ca/ordererOrg/ca-cert.pem"

  infoln "Enrolling orderer CA admin"
  mkdir -p "$orderer_home"

  export FABRIC_CA_CLIENT_HOME="$orderer_home"
  fabric-ca-client enroll -u https://admin:adminpw@localhost:9054 \
    --caname ca-orderer --tls.certfiles "$ca_cert"

  write_node_ous "$orderer_home"
  mkdir -p "${orderer_home}/msp/tlscacerts"
  cp "$ca_cert" "${orderer_home}/msp/tlscacerts/tlsca.example.com-cert.pem"
  mkdir -p "${orderer_home}/tlsca"
  cp "$ca_cert" "${orderer_home}/tlsca/tlsca.example.com-cert.pem"

  for ORDERER in orderer orderer2 orderer3 orderer4; do
    infoln "Registering ${ORDERER}"
    fabric-ca-client register --caname ca-orderer --id.name "${ORDERER}" --id.secret "${ORDERER}pw" --id.type orderer --tls.certfiles "$ca_cert" || true

    infoln "Generating the ${ORDERER} MSP"
    fabric-ca-client enroll -u "https://${ORDERER}:${ORDERER}pw@localhost:9054" --caname ca-orderer \
      -M "${orderer_home}/orderers/${ORDERER}.example.com/msp" --tls.certfiles "$ca_cert"

    cp "${orderer_home}/msp/config.yaml" "${orderer_home}/orderers/${ORDERER}.example.com/msp/config.yaml"

    if [ -f "${orderer_home}/orderers/${ORDERER}.example.com/msp/signcerts/cert.pem" ]; then
      mv "${orderer_home}/orderers/${ORDERER}.example.com/msp/signcerts/cert.pem" \
        "${orderer_home}/orderers/${ORDERER}.example.com/msp/signcerts/${ORDERER}.example.com-cert.pem"
    fi

    infoln "Generating the ${ORDERER} TLS certificates"
    fabric-ca-client enroll -u "https://${ORDERER}:${ORDERER}pw@localhost:9054" --caname ca-orderer \
      -M "${orderer_home}/orderers/${ORDERER}.example.com/tls" --enrollment.profile tls \
      --csr.hosts "${ORDERER}.example.com" --csr.hosts orderer.example.com --csr.hosts localhost \
      --csr.hosts "${OPS_HOST:-10.48.59.78}" --tls.certfiles "$ca_cert"

    local tls_dir="${orderer_home}/orderers/${ORDERER}.example.com/tls"
    cp "${tls_dir}/tlscacerts/"* "${tls_dir}/ca.crt"
    cp "${tls_dir}/signcerts/"* "${tls_dir}/server.crt"
    cp "${tls_dir}/keystore/"* "${tls_dir}/server.key"

    mkdir -p "${orderer_home}/orderers/${ORDERER}.example.com/msp/tlscacerts"
    cp "${tls_dir}/tlscacerts/"* "${orderer_home}/orderers/${ORDERER}.example.com/msp/tlscacerts/tlsca.example.com-cert.pem"
  done

  infoln "Registering the orderer admin"
  fabric-ca-client register --caname ca-orderer --id.name ordererAdmin --id.secret ordererAdminpw --id.type admin --tls.certfiles "$ca_cert" || true

  infoln "Generating the orderer admin MSP"
  fabric-ca-client enroll -u https://ordererAdmin:ordererAdminpw@localhost:9054 --caname ca-orderer \
    -M "${orderer_home}/users/Admin@example.com/msp" --tls.certfiles "$ca_cert"

  cp "${orderer_home}/msp/config.yaml" "${orderer_home}/users/Admin@example.com/msp/config.yaml"
}
