# Manual deployment (no automated SSH / sshpass)

Use this when `scripts/deploy-five-servers.sh` fails. You SSH to each VM yourself (`ssh -p 5522 igr@<IP>`).

**Reference:** [servers.credentials.local](servers.credentials.local) (IPs and ports), [5-SERVER-DEPLOYMENT.md](5-SERVER-DEPLOYMENT.md) (details).

| Server | IP | Role |
|--------|-----|------|
| peer-1 | 10.48.59.70 | peer0 IGRPrimary |
| peer-2 | 10.48.59.76 | peer1 IGRPrimary |
| peer-3 | 10.48.59.77 | peer0 IGRBank |
| peer-4 | 10.48.59.78 | peer1 IGRBank |
| peer-5 | 10.48.59.79 | CAs, orderer, admin, chaincode |

---

## Phase 0 — Every server (do once per VM)

SSH to each box, then:

```bash
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-plugin jq curl
sudo usermod -aG docker igr
# Log out and SSH back in so docker group applies
docker --version
```

**`/etc/hosts`** on **all five** (same content everywhere):

```bash
sudo tee -a /etc/hosts <<'EOF'
10.48.59.79  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
10.48.59.78  peer1.IGRBank.example.com
EOF
```

---

## Phase 1 — Copy repo to all servers (from your laptop)

On your laptop (project root):

```bash
cd /path/to/IGR-network
export IGR_NETWORK=/opt/igr-network

# Create dir on each server (enter password when prompted)
for IP in 10.48.59.70 10.48.59.76 10.48.59.77 10.48.59.78 10.48.59.79; do
  ssh -p 5522 igr@$IP "sudo mkdir -p $IGR_NETWORK && sudo chown -R igr:igr $IGR_NETWORK"
done

# Copy project (repeat per IP; or use tar once per host)
tar --exclude .git --exclude 'organizations/peerOrganizations' \
  --exclude 'organizations/ordererOrganizations' \
  -czf /tmp/igr-network.tgz .
scp -P 5522 /tmp/igr-network.tgz igr@10.48.59.79:/opt/
ssh -p 5522 igr@10.48.59.79 "cd /opt/igr-network && tar -xzf ../igr-network.tgz"
```

Repeat `scp` + `tar -xzf` for **10.48.59.70, .76, .77, .78** (or script the loop yourself).

Optional: copy credentials to peer-5 only (not required for scripts; defaults match ESDS ports):

```bash
scp -P 5522 docs/servers.credentials.local igr@10.48.59.79:/opt/igr-network/docs/
```

---

## Phase 2 — peer-5 only: CAs, certificates, orderer, channel

SSH: `ssh -p 5522 igr@10.48.59.79`

```bash
export IGR_NETWORK=/opt/igr-network
cd $IGR_NETWORK
```

### 2.1 Bootstrap (CAs + enroll + orderer + channel join)

`remote-peer5-bootstrap.sh` **installs Fabric binaries automatically** if `fabric-ca-client` is missing (needs `curl` and outbound HTTPS). You can still pre-install manually (optional).

Uses `localhost:7054/8054/9054` for CAs (not in firewall list). Orderer uses **9001/9002**.

```bash
cd $IGR_NETWORK
export CHANNEL_NAME=mychannel
export SUDO_PASS='YOUR_IGR_SUDO_PASSWORD'

bash scripts/remote-peer5-bootstrap.sh
```

Optional — install binaries only (no bootstrap):

```bash
INSTALL_FABRIC_BINARIES=true bash scripts/ensure-fabric-binaries.sh
export PATH=$HOME/fabric-samples/bin:$PATH
fabric-ca-client version
```

To **disable** auto-install and fail fast if CLI is missing:

```bash
INSTALL_FABRIC_BINARIES=false bash scripts/remote-peer5-bootstrap.sh
```

**Success checks:**

```bash
docker ps | grep -E 'ca_|orderer'
ls channel-artifacts/mychannel.block
ls organizations/peerOrganizations/IGRPrimary.example.com/peers/peer0.IGRPrimary.example.com/tls/server.crt
```

If bootstrap fails, run the same steps manually from [5-SERVER-DEPLOYMENT.md](5-SERVER-DEPLOYMENT.md) sections 5.1–5.8.

---

## Phase 3 — Copy certificates to peer-1 … peer-4

Still on **peer-5**, create one tarball of all peer MSPs:

```bash
cd $IGR_NETWORK/organizations
tar czf /tmp/peer-msp.tgz peerOrganizations
ls -la /tmp/peer-msp.tgz
```

**Option A — Laptop as relay**

```bash
# On laptop
scp -P 5522 igr@10.48.59.79:/tmp/peer-msp.tgz /tmp/
scp -P 5522 /tmp/peer-msp.tgz igr@10.48.59.70:/tmp/
# repeat for .76, .77, .78
```

On **each peer-1…4**:

```bash
cd /opt/igr-network/organizations
tar xzf /tmp/peer-msp.tgz
```

**Option B — peer-5 → peer direct** (if SSH between VMs works)

```bash
# On peer-5
scp -P 5522 /tmp/peer-msp.tgz igr@10.48.59.70:/tmp/
# repeat for .76, .77, .78
```

Also copy **peercfg** to each peer (from laptop or peer-5):

```bash
# From laptop
scp -P 5522 -r compose/docker/peercfg igr@10.48.59.70:/opt/igr-network/compose/docker/
# repeat for .76, .77, .78
```

---

## Phase 4 — Start peers (peer-1 … peer-4)

SSH to **each** server and run **one** command (`PEER_NODE` = 1 on peer-1, 2 on peer-2, etc.):

**peer-1** (`10.48.59.70`):

```bash
export IGR_NETWORK=/opt/igr-network
export PEER_NODE=1
export SUDO_PASS='YOUR_PASSWORD'
bash /opt/igr-network/scripts/remote-start-peer.sh
docker ps
```

**peer-2** (`10.48.59.76`): `export PEER_NODE=2` — same script.

**peer-3** (`10.48.59.77`): `export PEER_NODE=3`

**peer-4** (`10.48.59.78`): `export PEER_NODE=4`

Wait ~30 seconds, then from **peer-1** test orderer:

```bash
nc -zv orderer.example.com 9001
nc -zv peer0.IGRBank.example.com 7001
```

---

## Phase 5 — Join channel + anchors (peer-5)

SSH to **peer-5**:

```bash
cd /opt/igr-network
export PATH=$HOME/fabric-samples/bin:$PATH
export CHANNEL_NAME=mychannel
bash scripts/remote-peer5-channel-join-anchors.sh
```

Verify (peer-5):

```bash
export PATH=$HOME/fabric-samples/bin:$PATH
export FABRIC_CFG_PATH=/opt/igr-network/configtx
export CORE_PEER_LOCALMSPID=IGRPrimaryMSP
export CORE_PEER_TLS_ENABLED=true
export CORE_PEER_TLS_ROOTCERT_FILE=/opt/igr-network/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem
export CORE_PEER_MSPCONFIGPATH=/opt/igr-network/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp
export CORE_PEER_ADDRESS=peer0.IGRPrimary.example.com:5001
peer channel list
```

Repeat `CORE_PEER_ADDRESS` for 6001, 7001, 8001 on the other peers.

---

## Phase 6 — Chaincode on peer-5 (later)

1. Build/run CCAAS on port **9003** ([5-SERVER-DEPLOYMENT.md](5-SERVER-DEPLOYMENT.md) §5.7).
2. Lifecycle install/approve/commit from peer-5 (§5.11).
3. `connection.json` address: `chaincode.igr.example.com:9003`

---

## Manual checklist

| Done | Step | Where |
|------|------|--------|
| [ ] | Docker + `/etc/hosts` | All 5 |
| [ ] | Repo at `/opt/igr-network` | All 5 |
| [ ] | Fabric binaries | peer-5 |
| [ ] | `remote-peer5-bootstrap.sh` | peer-5 |
| [ ] | `peer-msp.tgz` extracted | peer-1…4 |
| [ ] | `peercfg` copied | peer-1…4 |
| [ ] | `remote-start-peer.sh` | peer-1…4 |
| [ ] | `remote-peer5-channel-join-anchors.sh` | peer-5 |
| [ ] | `peer channel list` on all 4 | peer-5 CLI |
| [ ] | CCAAS + lifecycle | peer-5 |

---

## Common manual SSH issues

| Problem | Fix |
|---------|-----|
| `sshpass` / deploy script fails | Use this doc instead of `deploy-five-servers.sh` |
| `sudo` asks password | Set `export SUDO_PASS='...'` before bootstrap/start scripts |
| `fabric-ca-client: command not found` | Re-run bootstrap (auto-installs) or `INSTALL_FABRIC_BINARIES=true bash scripts/ensure-fabric-binaries.sh` |
| Peer cannot reach orderer | Check `/etc/hosts` and firewall **9001** on peer-5 |
| Join fails TLS | Re-check enroll used correct hostname + IP in `--csr.hosts` |
