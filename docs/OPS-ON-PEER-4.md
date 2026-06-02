# OPS on peer-4 (peer-5 retired)

**peer-5 (10.48.59.79) is not used.** All orderer, Fabric CA, chaincode, and admin CLI work runs on **peer-4 (10.48.59.78)** using firewall ports **8001–8004**.

| Port | Service |
|------|---------|
| 8001 | Orderer client |
| 8002 | Orderer admin (`osnadmin`) |
| 8003 | Chaincode (CCAAS) |
| 8004 | Orderer operations / metrics |
| 7054, 8054, 9054 | Fabric CAs (localhost only on peer-4) |

**Layout change:** peer-4 does **not** run `peer1.IGRBank`. The network uses **3 peers**:

- peer-1: peer0 IGRPrimary  
- peer-2: peer1 IGRPrimary  
- peer-3: peer0 IGRBank  

`peer1.IGRBank` certs are still created at bootstrap (optional); no container on peer-4.

---

## `/etc/hosts` (all servers)

Point orderer and chaincode to **peer-4**:

```
10.48.59.78  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
```

Remove or ignore `10.48.59.79` entries for orderer.

---

## Step 1 — peer-4: sync updated repo from your laptop

**Easiest** (from repo root on your machine):

```bash
cd /home/encureitlp60/Videos/IGR/IGR-network
bash scripts/sync-to-peer4.sh
```

Enter the `igr` password when prompted (or install `sshpass` to use `PEER4_PASSWORD` from `servers.credentials.local` automatically).

**Manual** equivalent — see [sync commands](#manual-scp-commands) below.

Then SSH and verify:

```bash
ssh -p 5522 igr@10.48.59.78
cd /opt/igr-network
grep ORDERER_GENERAL_LISTENPORT compose/compose-test-net.yaml
# expect: ORDERER_GENERAL_LISTENPORT=8001
```

### Manual SCP commands

On your **laptop** (repo root):

```bash
cd /home/encureitlp60/Videos/IGR/IGR-network

tar --exclude .git \
  --exclude organizations/peerOrganizations \
  --exclude organizations/ordererOrganizations \
  --exclude 'channel-artifacts/*.block' \
  -czf /tmp/igr-network.tgz .

scp -P 5522 /tmp/igr-network.tgz igr@10.48.59.78:/tmp/

scp -P 5522 docs/servers.credentials.local igr@10.48.59.78:/opt/igr-network/docs/

ssh -p 5522 igr@10.48.59.78
sudo mkdir -p /opt/igr-network && sudo chown -R igr:igr /opt/igr-network
cd /opt/igr-network && tar -xzf /tmp/igr-network.tgz
chmod +x scripts/*.sh
rm /tmp/igr-network.tgz
```

---

## Step 2 — peer-4: bootstrap (CAs + certs + orderer + channel)

Run as **igr**, not `sudo` on the script:

```bash
export IGR_NETWORK=/opt/igr-network
cd $IGR_NETWORK
export SUDO_PASS='your-password'
export CHANNEL_NAME=mychannel
export PATH=$HOME/fabric-samples/bin:$PATH

bash scripts/remote-peer5-bootstrap.sh
```

Wait for: `OPS bootstrap complete`

Verify:

```bash
docker ps | grep -E 'ca_|orderer'
nc -zv 127.0.0.1 8002
ls channel-artifacts/mychannel.block
```

If only `osnadmin` failed before, retry:

```bash
bash scripts/remote-peer5-osnadmin-join.sh
```

---

## Step 3 — Copy MSP to peer-1, peer-2, peer-3 only

On peer-4:

```bash
cd /opt/igr-network/organizations
tar czf /tmp/peer-msp.tgz peerOrganizations
```

Copy `/tmp/peer-msp.tgz` to peer-1, peer-2, peer-3 (laptop `scp`). **Do not** need a peer on peer-4.

On each peer-1…3:

```bash
cd /opt/igr-network/organizations && tar xzf /tmp/peer-msp.tgz
```

Copy `compose/docker/peercfg` to each of peer-1…3.

---

## Step 4 — Start peers (peer-1, peer-2, peer-3 only)

| Host | Command |
|------|---------|
| 10.48.59.70 | `PEER_NODE=1 bash scripts/remote-start-peer.sh` |
| 10.48.59.76 | `PEER_NODE=2 bash scripts/remote-start-peer.sh` |
| 10.48.59.77 | `PEER_NODE=3 bash scripts/remote-start-peer.sh` |

**Do not** run `PEER_NODE=4` on peer-4 (OPS-only).

---

## Step 5 — Join channel (peer-4)

```bash
cd /opt/igr-network
export PATH=$HOME/fabric-samples/bin:$PATH
export SUDO_PASS='...'
bash scripts/remote-peer5-channel-join-anchors.sh
```

Joins peer-1, peer-2, peer-3 and sets anchors (skips peer1 IGRBank if `PEER4_ROLE=ops_only`).

If joins succeed but anchor update fails with `orderer.example.com:8001: context deadline exceeded`, sync latest scripts and re-run anchors only (OPS CLI uses `127.0.0.1:8001` with TLS override `orderer.example.com`):

```bash
bash scripts/remote-peer5-set-anchors-only.sh
```

Test:

```bash
export CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:5001
# ... MSP env for IGRPrimary ...
peer channel list
```

---

## Step 6 — Chaincode on peer-4

Sync repo so peer-4 has `/opt/igr-network` **and** `/opt/chaincode` (sibling of `igr-network`, or set `CHAINCODE_PATH`).

Prereqs: `/etc/hosts` on all peers includes `10.48.59.78 chaincode.igr.example.com`; firewall allows **8003** on peer-4.

**peer-4 cannot pull from Docker Hub** — build the image on your laptop, upload, then deploy:

```bash
# Laptop (internet)
cd /path/to/IGR-network
bash scripts/build-ccaas-image-bundle.sh
bash scripts/upload-ccaas-image-to-peer4.sh

# peer-4
cd /opt/igr-network
export PATH=$HOME/fabric-samples/bin:$HOME/bin:$PATH
export SUDO_PASS='...'
bash scripts/remote-peer5-deploy-ccaas.sh
```

The deploy script auto-loads `channel-artifacts/igr_asset_registry_ccaas.tar.gz` if present.

Uses **`chaincode.igr.example.com:8003`** in `connection.json`. Chaincode name default: **`asset_registry`** v1.0.

Quick test after deploy:

```bash
source docs/servers.credentials.local
export FABRIC_CFG_PATH=$PWD/compose/docker/peercfg
export ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export FABRIC_ORDERER_HOST=127.0.0.1
# set CORE_PEER_* for IGRPrimary admin, then:
peer lifecycle chaincode querycommitted -C mychannel --name asset_registry
```

---

## Credentials file

Set on peer-4: `docs/servers.credentials.local` with `PEER4_ROLE=ops_only` and OPS ports 8001–8004 (see template in repo).

---

## Related

- [MANUAL-DEPLOYMENT.md](MANUAL-DEPLOYMENT.md)  
- [PORT-ALIGNMENT-AND-DEPLOYMENT-SUMMARY.md](PORT-ALIGNMENT-AND-DEPLOYMENT-SUMMARY.md)
