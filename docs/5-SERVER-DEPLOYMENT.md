# IGR 5-Server Hyperledger Fabric Deployment

Step-by-step runbook for the ESDS-3709-IGR-Blockchain VMs. Work through sections in order; run commands on the server indicated in each heading.

**Credentials:** SSH user, ports, and passwords are in [servers.credentials.local](servers.credentials.local) (gitignored). Copy from [servers.credentials.local.example](servers.credentials.local.example) on a fresh clone.

**Security:** Passwords are not stored in git. If passwords were shared in chat or email, rotate them on the VMs.

---

## Server map

| VM name | Host | Private IP | Fabric role |
|---------|------|------------|---------------|
| ESDS-3709-IGR-Blockchain peer-1 | peer-1 | `10.48.59.70` | `peer0.IGRPrimary.example.com` + CouchDB |
| ESDS-3709-IGR-Blockchain peer-2 | peer-2 | `10.48.59.76` | `peer1.IGRPrimary.example.com` + CouchDB |
| ESDS-3709-IGR-Blockchain peer-3 | peer-3 | `10.48.59.77` | `peer0.IGRBank.example.com` + CouchDB |
| ESDS-3709-IGR-Blockchain peer-4 | peer-4 | `10.48.59.78` | Orderer + Fabric CAs + CCAAS + admin CLI (OPS) |
| ESDS-3709-IGR-Blockchain peer-5 | peer-5 | `10.48.59.79` | `peer1.IGRBank.example.com` + CouchDB |

| Setting | Value |
|---------|--------|
| SSH user | `igr` |
| SSH port | `5522` |
| OS | Ubuntu 24.04 |
| Default repo path | `/opt/igr-network` |
| Channel name | `igrchannel` |

### Port plan (ESDS firewall — only these ports are open)

Load mapping from credentials: `source docs/servers.credentials.local`

| Role | peer-1 | peer-2 | peer-3 | peer-4 | peer-5 |
|------|--------|--------|--------|--------|--------|
| **Open ports** | 5001–5004 | 6001–6004 | 7001–7004 | 8001–8004 | 9001–9004 |
| Peer (gossip / endorser) | **5001** | **6001** | **7001** | — | **9001** |
| Chaincode listen | **5002** | **6002** | **7002** | — | **9002** |
| CouchDB | **5003** | **6003** | **7003** | — | **9003** |
| Operations / metrics | **5004** | **6004** | **7004** | **8004** | **9004** |
| Orderer client | — | — | — | **8001** | — |
| Orderer admin (osnadmin) | — | — | — | **8002** | — |
| CCAAS chaincode | — | — | — | **8003** | — |
| Fabric CAs | — | — | — | localhost **7054**, **8054**, **9054** | — |

**Convention (per peer host):** `*01` = peer, `*02` = chaincode, `*03` = CouchDB, `*04` = ops.

**Anchor peers (channel config):** IGRPrimary `peer0` → port **5001**; IGRBank `peer0` → port **7001**.

**Orderer address (peers + CLI):** `orderer.example.com:8001`

**CCAAS package address:** `chaincode.igr.example.com:8003`

Fabric CAs on peer-4 are **not** in the open-port list; enroll identities on peer-4 using `localhost:7054` / `8054` / `9054`.

**CouchDB on peer hosts:** publish Couch as `-p ${PEERn_PORT_COUCH}:5984`. If the peer runs in Docker, use `--network host` on peer + Couch, or set `CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS` to the Docker bridge gateway IP and port `${PEERn_PORT_COUCH}` (e.g. `172.17.0.1:5003`).

---

## SSH quick reference

From your admin machine (VPN/network must reach `10.48.59.x`):

```bash
# Load local credentials (not in git)
source docs/servers.credentials.local

ssh -p $SSH_PORT ${SSH_USER}@${PEER1_HOST}   # peer-1
ssh -p $SSH_PORT ${SSH_USER}@${PEER2_HOST}   # peer-2
ssh -p $SSH_PORT ${SSH_USER}@${PEER3_HOST}   # peer-3
ssh -p $SSH_PORT ${SSH_USER}@${PEER4_HOST}   # peer-4 (OPS)
ssh -p $SSH_PORT ${SSH_USER}@${PEER5_HOST}   # peer-5 (peer1 IGRBank)
```

Optional `~/.ssh/config` snippet (passwordless keys recommended for production):

```
Host igr-peer1
  HostName 10.48.59.70
  User igr
  Port 5522

Host igr-peer2
  HostName 10.48.59.76
  User igr
  Port 5522

Host igr-peer3
  HostName 10.48.59.77
  User igr
  Port 5522

Host igr-peer4
  HostName 10.48.59.78
  User igr
  Port 5522

Host igr-peer5
  HostName 10.48.59.79
  User igr
  Port 5522
```

Copy repo to all hosts (from machine that has the project):

```bash
source docs/servers.credentials.local
for H in $PEER1_HOST $PEER2_HOST $PEER3_HOST $PEER4_HOST $PEER5_HOST; do
  rsync -avz -e "ssh -p $SSH_PORT" --exclude organizations/peerOrganizations \
    ./ ${SSH_USER}@${H}:${IGR_NETWORK}/
done
```

---

## `/etc/hosts` (all 5 servers)

Run on **each** VM:

```bash
sudo tee -a /etc/hosts <<'EOF'
10.48.59.78  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
10.48.59.79  peer1.IGRBank.example.com
EOF
```

Verify from peer-1:

```bash
ping -c1 orderer.example.com
ping -c1 peer0.IGRBank.example.com
```

---

## Repo fixes (peer-4 OPS, once)

Do these on **peer-4 OPS** before first channel creation. Scripts named `remote-peer5-*` run here via `scripts/load-ops-env.sh` (legacy `PEER5_*` env aliases).

### 1. Add `ChannelUsingRaft` profile

[scripts/createChannel.sh](../scripts/createChannel.sh) expects profile `ChannelUsingRaft`, but [configtx/configtx.yaml](../configtx/configtx.yaml) only defines `TwoRegistryValueGensis`. Append under `Profiles:`:

```yaml
  ChannelUsingRaft:
    <<: *ChannelDefaults
    Orderer:
      <<: *OrdererDefaults
      Organizations:
        - *OrdererOrg
      Capabilities: *OrdererCapabilities
    Application:
      <<: *ApplicationDefaults
      Organizations:
        - *IGRPrimary
        - *IGRBank
      Capabilities: *ApplicationCapabilities
```

### 2. `./network.sh up -ca` (local dev)

`network.sh` CA paths and [organizations/fabric-ca/registerEnroll.sh](../organizations/fabric-ca/registerEnroll.sh) use canonical `IGRPrimary.example.com` / `IGRBank.example.com` MSP directories. For production OPS, prefer [scripts/remote-peer5-bootstrap.sh](../scripts/remote-peer5-bootstrap.sh) on peer-4.

### 3. MSP path casing

Enrollment paths must match [configtx/configtx.yaml](../configtx/configtx.yaml). See [CA-ENROLLMENT.md](CA-ENROLLMENT.md). Run `bash scripts/check-msp-path-casing.sh` before committing enrollment changes.

---

## Step 0 — Prerequisites (all 5 servers)

On **each** peer VM:

```bash
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-plugin jq curl rsync
sudo usermod -aG docker igr
# log out and back in as igr
docker --version
```

On **peer-4 OPS** (admin tools):

```bash
# Install Fabric binaries (adjust version to match fabric-peer image)
curl -sSL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh | bash -s -- binary
export PATH=$HOME/fabric-samples/bin:$PATH
peer version
configtxgen -version
fabric-ca-client version
```

Firewall: open ports from the [port plan](#port-plan) for that host (ufw/security group).

---

# peer-4 (`10.48.59.78`) — CAs, crypto, orderer, channel, chaincode (OPS)

SSH: `ssh -p 5522 igr@10.48.59.78`

```bash
export IGR_NETWORK=/opt/igr-network
cd $IGR_NETWORK
```

## 5.1 Start Fabric CAs

```bash
docker compose -f compose/compose-ca.yaml up -d
docker ps | grep fabric-ca
```

Wait for:

```bash
test -f organizations/fabric-ca/IGRPrimary/ca-cert.pem && echo OK
test -f organizations/fabric-ca/IGRBank/ca-cert.pem && echo OK
test -f organizations/fabric-ca/ordererOrg/ca-cert.pem && echo OK
```

## 5.2 Enroll identities (IGRPrimary)

```bash
cd $IGR_NETWORK
export FABRIC_CA_CLIENT_HOME=$PWD/organizations/peerOrganizations/IGRPrimary.example.com
CA_PRIMARY=$PWD/organizations/fabric-ca/IGRPrimary/ca-cert.pem

fabric-ca-client enroll -u https://admin:adminpw@localhost:7054 \
  --caname ca-IGRPrimary --tls.certfiles "$CA_PRIMARY"

fabric-ca-client register --caname ca-IGRPrimary --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "$CA_PRIMARY" || true
fabric-ca-client register --caname ca-IGRPrimary --id.name peer1 --id.secret peer1pw --id.type peer --tls.certfiles "$CA_PRIMARY" || true
fabric-ca-client register --caname ca-IGRPrimary --id.name igrprimaryadmin --id.secret igrprimaryadminpw --id.type admin --tls.certfiles "$CA_PRIMARY" || true

fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-IGRPrimary \
  -M "$PWD/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com/msp" \
  --tls.certfiles "$CA_PRIMARY"

fabric-ca-client enroll -u https://peer0:peer0pw@localhost:7054 --caname ca-IGRPrimary \
  -M "$PWD/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com/tls" \
  --enrollment.profile tls \
  --csr.hosts peer0.IGRPrimary.example.com --csr.hosts 10.48.59.70 \
  --tls.certfiles "$CA_PRIMARY"

fabric-ca-client enroll -u https://peer1:peer1pw@localhost:7054 --caname ca-IGRPrimary \
  -M "$PWD/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer1.IGRPrimary.example.com/msp" \
  --tls.certfiles "$CA_PRIMARY"

fabric-ca-client enroll -u https://peer1:peer1pw@localhost:7054 --caname ca-IGRPrimary \
  -M "$PWD/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer1.IGRPrimary.example.com/tls" \
  --enrollment.profile tls \
  --csr.hosts peer1.IGRPrimary.example.com --csr.hosts 10.48.59.76 \
  --tls.certfiles "$CA_PRIMARY"

fabric-ca-client enroll -u https://igrprimaryadmin:igrprimaryadminpw@localhost:7054 --caname ca-IGRPrimary \
  -M "$PWD/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp" \
  --tls.certfiles "$CA_PRIMARY"

for P in peer0 peer1; do
  TLS=$PWD/organizations/peerOrganizations/IGRPrimary.example.com/peers/${P}.IGRPrimary.example.com/tls
  cp $TLS/tlscacerts/* $TLS/ca.crt
  cp $TLS/signcerts/* $TLS/server.crt
  cp $TLS/keystore/* $TLS/server.key
done
```

## 5.3 Enroll identities (IGRBank)

```bash
export FABRIC_CA_CLIENT_HOME=$PWD/organizations/peerOrganizations/IGRBank.example.com
CA_BANK=$PWD/organizations/fabric-ca/IGRBank/ca-cert.pem

fabric-ca-client enroll -u https://admin:adminpw@localhost:8054 --caname ca-IGRBank --tls.certfiles "$CA_BANK"

fabric-ca-client register --caname ca-IGRBank --id.name peer0 --id.secret peer0pw --id.type peer --tls.certfiles "$CA_BANK" || true
fabric-ca-client register --caname ca-IGRBank --id.name peer1 --id.secret peer1pw --id.type peer --tls.certfiles "$CA_BANK" || true
fabric-ca-client register --caname ca-IGRBank --id.name igrbankadmin --id.secret igrbankadminpw --id.type admin --tls.certfiles "$CA_BANK" || true

fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-IGRBank \
  -M "$PWD/organizations/peerOrganizations/IGRBank.example.com/peers/peer0.IGRBank.example.com/msp" \
  --tls.certfiles "$CA_BANK"

fabric-ca-client enroll -u https://peer0:peer0pw@localhost:8054 --caname ca-IGRBank \
  -M "$PWD/organizations/peerOrganizations/IGRBank.example.com/peers/peer0.IGRBank.example.com/tls" \
  --enrollment.profile tls --csr.hosts peer0.IGRBank.example.com --csr.hosts 10.48.59.77 \
  --tls.certfiles "$CA_BANK"

fabric-ca-client enroll -u https://peer1:peer1pw@localhost:8054 --caname ca-IGRBank \
  -M "$PWD/organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com/msp" \
  --tls.certfiles "$CA_BANK"

fabric-ca-client enroll -u https://peer1:peer1pw@localhost:8054 --caname ca-IGRBank \
  -M "$PWD/organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com/tls" \
  --enrollment.profile tls --csr.hosts peer1.IGRBank.example.com --csr.hosts 10.48.59.79 \
  --tls.certfiles "$CA_BANK"

fabric-ca-client enroll -u https://igrbankadmin:igrbankadminpw@localhost:8054 --caname ca-IGRBank \
  -M "$PWD/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp" \
  --tls.certfiles "$CA_BANK"

for P in peer0 peer1; do
  TLS=$PWD/organizations/peerOrganizations/IGRBank.example.com/peers/${P}.IGRBank.example.com/tls
  cp $TLS/tlscacerts/* $TLS/ca.crt
  cp $TLS/signcerts/* $TLS/server.crt
  cp $TLS/keystore/* $TLS/server.key
done
```

## 5.4 Enroll orderer

```bash
cd $IGR_NETWORK
source scripts/ca-enroll-helpers.sh
source organizations/fabric-ca/registerEnroll.sh
createOrderer
```

If `createOrderer` fails on paths, enroll only `orderer` (single node) under `organizations/ordererOrganizations/example.com/orderers/orderer.example.com/`.

## 5.5 Distribute peer crypto to peer-1 … peer-4

From **peer-4 OPS**:

```bash
source docs/servers.credentials.local
IGR_NETWORK=/opt/igr-network

rsync -avz -e "ssh -p $SSH_PORT" \
  organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com/ \
  ${SSH_USER}@${PEER1_HOST}:${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com/

rsync -avz -e "ssh -p $SSH_PORT" \
  organizations/peerOrganizations/IGRPrimary.example.com/peers/peer1.IGRPrimary.example.com/ \
  ${SSH_USER}@${PEER2_HOST}:${IGR_NETWORK}/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer1.IGRPrimary.example.com/

rsync -avz -e "ssh -p $SSH_PORT" \
  organizations/peerOrganizations/IGRBank.example.com/peers/peer0.IGRBank.example.com/ \
  ${SSH_USER}@${PEER3_HOST}:${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/peers/peer0.IGRBank.example.com/

rsync -avz -e "ssh -p $SSH_PORT" \
  organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com/ \
  ${SSH_USER}@${PEER4_HOST}:${IGR_NETWORK}/organizations/peerOrganizations/IGRBank.example.com/peers/peer1.IGRBank.example.com/

# peercfg on all peers
for H in $PEER1_HOST $PEER2_HOST $PEER3_HOST $PEER4_HOST; do
  rsync -avz -e "ssh -p $SSH_PORT" compose/docker/peercfg/ \
    ${SSH_USER}@${H}:${IGR_NETWORK}/compose/docker/peercfg/
done
```

## 5.6 Start orderer

Repo [compose/compose-test-net.yaml](../compose/compose-test-net.yaml) runs the orderer with **matching ESDS ports inside the container** (client **9001**, admin **9002**, operations **9004**) so `/etc/hosts` peers and Docker-network peers resolve the same address.

```bash
cd $IGR_NETWORK

docker compose -f compose/compose-test-net.yaml up -d orderer.example.com

docker logs -f orderer.example.com
```

[configtx/configtx.yaml](../configtx/configtx.yaml) `OrdererEndpoints` and Raft consenters use `orderer.example.com:9001` to match the plan.

## 5.7 Chaincode container (CCAAS)

```bash
# Replace CC_PATH with your chaincode directory (must contain Dockerfile)
export CC_PATH=/path/to/chaincode
source docs/servers.credentials.local
docker build -f $CC_PATH/Dockerfile -t igr_ccaas:1.0 --build-arg CC_SERVER_PORT=${PEER5_PORT_CHAINCODE} $CC_PATH

docker run -d --name igr_ccaas --restart unless-stopped \
  -p ${PEER5_PORT_CHAINCODE}:${PEER5_PORT_CHAINCODE} \
  -e CHAINCODE_SERVER_ADDRESS=0.0.0.0:${PEER5_PORT_CHAINCODE} \
  igr_ccaas:1.0
```

Test from peer-1: `nc -zv chaincode.igr.example.com ${PEER5_PORT_CHAINCODE}` (port **9003**)

## 5.8 Create channel and join orderer

```bash
cd $IGR_NETWORK
export PATH=$HOME/fabric-samples/bin:$PATH
export FABRIC_CFG_PATH=$PWD/configtx
mkdir -p channel-artifacts
export CHANNEL_NAME=igrchannel

configtxgen -profile ChannelUsingRaft \
  -outputBlock ./channel-artifacts/${CHANNEL_NAME}.block \
  -channelID ${CHANNEL_NAME}

export ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export ORDERER_ADMIN_TLS_SIGN_CERT=$PWD/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.crt
export ORDERER_ADMIN_TLS_PRIVATE_KEY=$PWD/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/tls/server.key

source docs/servers.credentials.local
osnadmin channel join \
  --channelID ${CHANNEL_NAME} \
  --config-block ./channel-artifacts/${CHANNEL_NAME}.block \
  -o localhost:${PEER5_PORT_ORDERER_ADMIN} \
  --ca-file "$ORDERER_CA" \
  --client-cert "$ORDERER_ADMIN_TLS_SIGN_CERT" \
  --client-key "$ORDERER_ADMIN_TLS_PRIVATE_KEY"
```

## 5.9 Join all four peers (remote addresses)

```bash
export ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export FABRIC_CFG_PATH=$PWD/configtx
BLOCK=./channel-artifacts/${CHANNEL_NAME}.block

# IGRPrimary peer0
export CORE_PEER_LOCALMSPID=IGRPrimaryMSP
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_TLS_ROOTCERT_FILE=$PWD/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=$PWD/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp
source docs/servers.credentials.local
export CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}
peer channel join -b $BLOCK

# IGRPrimary peer1
export CORE_PEER_ADDRESS=peer1.IGRPrimary.example.com:${PEER2_PORT_PEER}
peer channel join -b $BLOCK

# IGRBank peer0
export CORE_PEER_LOCALMSPID=IGRBankMSP
export CORE_PEER_TLS_ROOTCERT_FILE=$PWD/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=$PWD/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp
export CORE_PEER_ADDRESS=peer0.IGRBank.example.com:${PEER3_PORT_PEER}
peer channel join -b $BLOCK

# IGRBank peer1
export CORE_PEER_ADDRESS=peer1.IGRBank.example.com:${PEER4_PORT_PEER}
peer channel join -b $BLOCK
```

## 5.10 Anchor peers

[scripts/setAnchorPeer.sh](../scripts/setAnchorPeer.sh) uses `FABRIC_ORDERER_PORT` (default **9001**) and anchor ports from `PEER1_PORT_PEER` / `PEER3_PORT_PEER` (**5001** / **7001** on five-server; export them or use `servers.credentials.local`).

Then:

```bash
./scripts/setAnchorPeer.sh 1 ${CHANNEL_NAME}
./scripts/setAnchorPeer.sh 2 ${CHANNEL_NAME}
```

## 5.11 Chaincode lifecycle (CCAAS)

```bash
cd $IGR_NETWORK
cat > /tmp/connection.json <<'EOF'
{
  "address": "chaincode.igr.example.com:9003",
  "dial_timeout": "10s",
  "tls_required": false
}
EOF

mkdir -p /tmp/ccaas-pkg/src /tmp/ccaas-pkg/pkg
cp /tmp/connection.json /tmp/ccaas-pkg/src/
echo '{"type":"ccaas","label":"igr_cc_1.0"}' > /tmp/ccaas-pkg/pkg/metadata.json
tar -C /tmp/ccaas-pkg/src -czf /tmp/ccaas-pkg/pkg/code.tar.gz .
tar -C /tmp/ccaas-pkg/pkg -czf igr_cc.tar.gz metadata.json code.tar.gz

export PACKAGE_ID=$(peer lifecycle chaincode calculatepackageid igr_cc.tar.gz)
```

Install on **each** peer (change `CORE_PEER_ADDRESS` four times):

```bash
peer lifecycle chaincode install igr_cc.tar.gz
```

Approve (peer0 per org) and commit — see [CHAINCODE_AS_A_SERVICE_TUTORIAL.md](../CHAINCODE_AS_A_SERVICE_TUTORIAL.md). Example commit:

```bash
source docs/servers.credentials.local
peer lifecycle chaincode commit \
  -o orderer.example.com:${PEER5_PORT_ORDERER} --ordererTLSHostnameOverride orderer.example.com \
  --tls --cafile "$ORDERER_CA" \
  --channelID ${CHANNEL_NAME} --name igr_cc --version 1.0 --sequence 1 \
  --peerAddresses peer0.IGRPrimary.example.com:${PEER1_PORT_PEER} \
  --tlsRootCertFiles $PWD/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem \
  --peerAddresses peer0.IGRBank.example.com:${PEER3_PORT_PEER} \
  --tlsRootCertFiles $PWD/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem
```

---

# peer-1 (`10.48.59.70`) — peer0 IGRPrimary

SSH: `ssh -p 5522 igr@10.48.59.70`

```bash
export IGR_NETWORK=/opt/igr-network
cd $IGR_NETWORK

source $IGR_NETWORK/docs/servers.credentials.local 2>/dev/null || source docs/servers.credentials.local

docker network create igrnet1 2>/dev/null || true

docker run -d --name couchdbIGRPrimary --restart unless-stopped \
  --network igrnet1 \
  -p ${PEER1_PORT_COUCH}:5984 \
  -e COUCHDB_USER=admin -e COUCHDB_PASSWORD=adminpw \
  couchdb:3.4.2

docker run -d --name peer0.IGRPrimary.example.com --restart unless-stopped \
  --network igrnet1 \
  -p ${PEER1_PORT_PEER}:${PEER1_PORT_PEER} \
  -p ${PEER1_PORT_CHAINCODE}:${PEER1_PORT_CHAINCODE} \
  -p ${PEER1_PORT_OPS}:${PEER1_PORT_OPS} \
  -v $IGR_NETWORK/organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com:/etc/hyperledger/fabric \
  -v $IGR_NETWORK/compose/docker/peercfg:/etc/hyperledger/peercfg \
  -e FABRIC_CFG_PATH=/etc/hyperledger/peercfg \
  -e CORE_PEER_ID=peer0.IGRPrimary.example.com \
  -e CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER} \
  -e CORE_PEER_LISTENADDRESS=0.0.0.0:${PEER1_PORT_PEER} \
  -e CORE_PEER_CHAINCODEADDRESS=peer0.IGRPrimary.example.com:${PEER1_PORT_CHAINCODE} \
  -e CORE_PEER_CHAINCODELISTENADDRESS=0.0.0.0:${PEER1_PORT_CHAINCODE} \
  -e CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER} \
  -e CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRPrimary.example.com:${PEER1_PORT_PEER} \
  -e CORE_OPERATIONS_LISTENADDRESS=0.0.0.0:${PEER1_PORT_OPS} \
  -e CORE_PEER_LOCALMSPID=IGRPrimaryMSP \
  -e CORE_PEER_TLS_ENABLED=true \
  -e CORE_PEER_TLS_CERT_FILE=/etc/hyperledger/fabric/tls/server.crt \
  -e CORE_PEER_TLS_KEY_FILE=/etc/hyperledger/fabric/tls/server.key \
  -e CORE_PEER_TLS_ROOTCERT_FILE=/etc/hyperledger/fabric/tls/ca.crt \
  -e CORE_LEDGER_STATE_STATEDATABASE=CouchDB \
  -e CORE_LEDGER_STATE_COUCHDBCONFIG_COUCHDBADDRESS=couchdbIGRPrimary:5984 \
  -e CORE_LEDGER_STATE_COUCHDBCONFIG_USERNAME=admin \
  -e CORE_LEDGER_STATE_COUCHDBCONFIG_PASSWORD=adminpw \
  -e CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer0IGRPrimary"}' \
  hyperledger/fabric-peer:latest peer node start

docker logs -f peer0.IGRPrimary.example.com
```

---

# peer-2 (`10.48.59.76`) — peer1 IGRPrimary

SSH: `ssh -p 5522 igr@10.48.59.76`

Same as peer-1 with credentials `PEER2_*` ports (**6001–6004**):

- Volumes: `peer1.IGRPrimary.example.com`
- Publish `-p ${PEER2_PORT_PEER}:${PEER2_PORT_PEER}` etc.
- `CORE_PEER_ADDRESS=peer1.IGRPrimary.example.com:6001`
- `CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRPrimary.example.com:5001`
- `CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer1.IGRPrimary.example.com:6001`
- Couch: `127.0.0.1:6003` with `-p 6003:5984`

---

# peer-3 (`10.48.59.77`) — peer0 IGRBank

SSH: `ssh -p 5522 igr@10.48.59.77`

Same pattern as peer-1 with **IGRBank** paths:

- Credentials `PEER3_*` ports (**7001–7004**)
- `CORE_PEER_ADDRESS=peer0.IGRBank.example.com:7001`
- Volume: `.../peer0.IGRBank.example.com`
- `CHAINCODE_AS_A_SERVICE_BUILDER_CONFIG='{"peername":"peer0IGRBank"}'`

---

# peer-4 (`10.48.59.78`) — OPS (orderer, CAs, CCAAS, admin CLI)

This host runs orderer, Fabric CAs, CCAAS chaincode (`:8003`), and admin CLI. It is **not** an endorsing peer.

---

# peer-5 (`10.48.59.79`) — peer1 IGRBank

SSH: `ssh -p 5522 igr@10.48.59.79`

- Credentials `PEER5_*` ports (**9001–9004**)
- `CORE_PEER_ADDRESS=peer1.IGRBank.example.com:9001`
- `CORE_PEER_GOSSIP_BOOTSTRAP=peer0.IGRBank.example.com:7001`
- `CORE_PEER_GOSSIP_EXTERNALENDPOINT=peer1.IGRBank.example.com:9001`

---

## Execution order (checklist)

| # | Where | Task |
|---|--------|------|
| 1 | All | `/etc/hosts`, Docker, firewall |
| 2 | peer-4 OPS | Repo fixes (`ChannelUsingRaft`) |
| 3 | peer-4 OPS | Start CAs → enroll → orderer |
| 4 | peer-4 OPS | Rsync crypto to peer-1…3, peer-5 |
| 5 | peer-1…3, peer-5 | CouchDB + peer containers |
| 6 | peer-4 OPS | Create channel, join 4 peers, anchors |
| 7 | peer-4 OPS | Chaincode image + lifecycle |
| 8 | All | Verification (below) |

---

## Verification

| Check | Command (where) |
|-------|------------------|
| Orderer | `docker logs orderer.example.com` (peer-4 OPS) |
| Peers running | `docker ps` (peer-1, peer-2, peer-3, peer-5) |
| Channel | `peer channel list` with each `CORE_PEER_ADDRESS` (peer-4 OPS CLI) |
| CCAAS port | `nc -zv chaincode.igr.example.com 8003` (from any peer host) |
| Open ports | `nc -zv peer-host <port>` for each of 5001–5004 / 6001–6004 / etc. |
| Chaincode | `peer lifecycle chaincode querycommitted -C igrchannel` (peer-4 OPS) |

---

## Related docs

- [README.md](../README.md) — test network overview
- [CHAINCODE_AS_A_SERVICE_TUTORIAL.md](../CHAINCODE_AS_A_SERVICE_TUTORIAL.md) — CCAAS details
- [servers.credentials.local.example](servers.credentials.local.example) — credential template
