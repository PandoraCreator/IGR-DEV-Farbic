#!/usr/bin/env bash
# Shared Fabric CA enrollment helpers (canonical IGRPrimary/IGRBank paths).
# Source from registerEnroll.sh and remote-peer5-bootstrap.sh.

write_node_ous() {
  local org_home=$1
  local pem base
  pem=$(ls -1 "${org_home}/msp/cacerts/"*.pem | head -1)
  base=$(basename "$pem")
  cat >"${org_home}/msp/config.yaml" <<YAML
NodeOUs:
  Enable: true
  ClientOUIdentifier:
    Certificate: cacerts/${base}
    OrganizationalUnitIdentifier: client
  PeerOUIdentifier:
    Certificate: cacerts/${base}
    OrganizationalUnitIdentifier: peer
  AdminOUIdentifier:
    Certificate: cacerts/${base}
    OrganizationalUnitIdentifier: admin
  OrdererOUIdentifier:
    Certificate: cacerts/${base}
    OrganizationalUnitIdentifier: orderer
YAML
}

copy_org_tls_roots() {
  local fabric_ca_cert=$1 org_home=$2 name=$3
  mkdir -p "${org_home}/msp/tlscacerts"
  cp "$fabric_ca_cert" "${org_home}/msp/tlscacerts/ca.crt"
  mkdir -p "${org_home}/tlsca"
  cp "$fabric_ca_cert" "${org_home}/tlsca/tlsca.${name}-cert.pem"
  mkdir -p "${org_home}/ca"
  cp "$fabric_ca_cert" "${org_home}/ca/ca.${name}-cert.pem"
}

install_peer_tls_files() {
  local org_domain=$1 peer_name=$2
  local tls_dir="${PWD}/organizations/peerOrganizations/${org_domain}/peers/${peer_name}.${org_domain}/tls"
  cp "${tls_dir}/tlscacerts/"* "${tls_dir}/ca.crt"
  cp "${tls_dir}/signcerts/"* "${tls_dir}/server.crt"
  cp "${tls_dir}/keystore/"* "${tls_dir}/server.key"
}

propagate_org_msp_config() {
  local org_home=$1 org_domain=$2
  local cfg="${org_home}/msp/config.yaml"
  cp "$cfg" "${org_home}/peers/peer0.${org_domain}/msp/config.yaml"
  cp "$cfg" "${org_home}/peers/peer1.${org_domain}/msp/config.yaml"
  cp "$cfg" "${org_home}/users/Admin@${org_domain}/msp/config.yaml"
}
