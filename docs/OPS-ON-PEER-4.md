# OPS on peer-4 (four endorsing peers)

**peer-4 (10.48.59.78)** runs orderer, Fabric CAs, CCAAS chaincode, and admin CLI (ports **8001–8004**).

**peer-5 (10.48.59.79)** runs **`peer1.IGRBank.example.com`** (ports **9001–9004**).

| Port (peer-4) | Service |
|------|---------|
| 8001 | Orderer client |
| 8002 | Orderer admin (`osnadmin`) |
| 8003 | Chaincode (CCAAS) |
| 8004 | Orderer operations / metrics |
| 7054, 8054, 9054 | Fabric CAs (localhost only on peer-4) |

**Endorsing peers:**

- peer-1: peer0 IGRPrimary  
- peer-2: peer1 IGRPrimary  
- peer-3: peer0 IGRBank  
- peer-5: peer1 IGRBank  

**Chaincode naming:** Repo defaults target **`igr_anchor`** on channel **`igrchannel`**. Live POC on servers may still run **`asset_registry`** until MA-04 — see **[CHAINCODE-UAT.md](CHAINCODE-UAT.md)**. Do not redeploy from repo defaults without MA-04 sign-off.

---

## `/etc/hosts` (all servers)

```
10.48.59.78  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
10.48.59.79  peer1.IGRBank.example.com
```

Or run: `bash scripts/install-etc-hosts-five-server.sh` (with sudo).

---

## Step 1 — peer-4: sync updated repo from your laptop

**Easiest** (from repo root on your machine):

```bash
cd /path/to/IGR-network
bash scripts/sync-to-peer4.sh
```

Enter the `igr` password when prompted (or install `sshpass` to use `PEER4_PASSWORD` from `servers.credentials.local` automatically).

Then SSH and verify:

```bash
ssh -p 5522 igr@10.48.59.78
cd /opt/igr-network
grep ORDERER_GENERAL_LISTENPORT compose/compose-test-net.yaml
# expect: ORDERER_GENERAL_LISTENPORT=8001
```

---

## Step 2 — peer-4: bootstrap (CAs + certs + orderer + channel)

Run as **igr**, not `sudo` on the script:

```bash
export IGR_NETWORK=/opt/igr-network
cd $IGR_NETWORK
export SUDO_PASS='your-password'
export CHANNEL_NAME=igrchannel
export PATH=$HOME/fabric-samples/bin:$PATH

bash scripts/remote-peer5-bootstrap.sh
```

Wait for: `OPS bootstrap complete`

Verify:

```bash
docker ps | grep -E 'ca_|orderer'
nc -zv 127.0.0.1 8002
ls channel-artifacts/igrchannel.block
```

If only `osnadmin` failed before, retry:

```bash
bash scripts/remote-peer5-osnadmin-join.sh
```

---

## Step 3 — Copy MSP to peer-1, peer-2, peer-3, peer-5

On peer-4:

```bash
cd /opt/igr-network/organizations
tar czf /tmp/peer-msp.tgz peerOrganizations
```

Copy `/tmp/peer-msp.tgz` to peer-1, peer-2, peer-3, and peer-5.

On each peer host:

```bash
cd /opt/igr-network/organizations && tar xzf /tmp/peer-msp.tgz
```

---

## Step 4 — Start peers

| Host | Command |
|------|---------|
| 10.48.59.70 | `PEER_NODE=1 bash scripts/remote-start-peer.sh` |
| 10.48.59.76 | `PEER_NODE=2 bash scripts/remote-start-peer.sh` |
| 10.48.59.77 | `PEER_NODE=3 bash scripts/remote-start-peer.sh` |
| 10.48.59.79 | `PEER_NODE=5 bash scripts/remote-start-peer.sh` |

**Do not** run `PEER_NODE=4` on peer-4 (OPS-only).

---

## Step 5 — Channel join + anchors (peer-4 OPS)

```bash
export CHANNEL_NAME=igrchannel
bash scripts/remote-peer5-channel-join-anchors.sh
```

---

## Step 6 — Chaincode (CCAAS) — UAT gated on MA-04

Sync repo so peer-4 has `/opt/igr-network` **and** `/opt/chaincode` (sibling of `igr-network`, or set `CHAINCODE_PATH`).

Prereqs: `/etc/hosts` on all peers includes `10.48.59.78 chaincode.igr.example.com`; firewall allows **8003** on peer-4.

**peer-4 cannot pull from Docker Hub** — build the image on your laptop, upload, then deploy:

```bash
# Laptop (internet)
cd /path/to/IGR-network
bash scripts/build-ccaas-image-bundle.sh
bash scripts/upload-ccaas-image-to-peer4.sh

# peer-4 OPS — UAT only after MA-04 (repo defaults: igr_anchor)
cd /opt/igr-network
export PATH=$HOME/fabric-samples/bin:$HOME/bin:$PATH
export SUDO_PASS='...'
bash scripts/remote-peer5-deploy-ccaas.sh
```

The deploy script auto-loads `channel-artifacts/igr_anchor_ccaas.tar.gz` if present.

Uses **`chaincode.igr.example.com:8003`** in `connection.json`. Repo default chaincode name: **`igr_anchor`** v1.0 on **`igrchannel`**.

**POC on live servers:** override `CC_NAME=asset_registry` — see [CHAINCODE-UAT.md](CHAINCODE-UAT.md).

Quick test after deploy:

```bash
source docs/servers.credentials.local
export FABRIC_CFG_PATH=$PWD/compose/docker/peercfg
export ORDERER_CA=$PWD/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
export FABRIC_ORDERER_HOST=127.0.0.1
# set CORE_PEER_* for IGRPrimary admin, then:
peer lifecycle chaincode querycommitted -C igrchannel --name igr_anchor
```

Or: `bash scripts/verify-ccaas-deploy.sh`

---

## Credentials file

Set on peer-4: `docs/servers.credentials.local` with `PEER4_ROLE=ops_only` and OPS ports 8001–8004 (see template in repo).

---

## Related

- [CHAINCODE-UAT.md](CHAINCODE-UAT.md) — POC vs UAT, MA-04 gate  
- [MANUAL-DEPLOYMENT.md](MANUAL-DEPLOYMENT.md)  
- [PORT-ALIGNMENT-AND-DEPLOYMENT-SUMMARY.md](PORT-ALIGNMENT-AND-DEPLOYMENT-SUMMARY.md)
