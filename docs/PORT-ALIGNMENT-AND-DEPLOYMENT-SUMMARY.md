# Port alignment and deployment summary

This document summarizes how repository settings match `docs/5-SERVER-DEPLOYMENT.md` and the ESDS firewall port plan, and what remains for you to do on the VMs.

## Port alignment (vs `servers.credentials.local.example`)

| Layer | What was aligned |
|--------|------------------|
| **`scripts/remote-start-peer.sh`** | Peers 1–4 use **5001–5004**, **6001–6004**, **7001–7004**, **8001–8004** (peer / chaincode / CouchDB / ops). Gossip bootstraps use **5001** and **7001**. Optionally sources `docs/servers.credentials.local` if present on a peer. |
| **`compose/compose-test-net.yaml`** (orderer only) | Orderer listens **inside** the container on **9001** (client), **9002** (admin), **9004** (operations), with the same host publish so `/etc/hosts` peers and Docker-network peers use the same ports. |
| **`configtx/configtx.yaml`** | `OrdererEndpoints`, `Addresses`, and Raft **Consenters** use **`orderer.example.com:9001`**. |
| **`scripts/envVar.sh`** | Optionally sources `servers.credentials.local` and sets **`FABRIC_ORDERER_PORT`** / **`FABRIC_ORDERER_ADMIN_PORT`** (defaults **9001** / **9002** from `PEER5_PORT_ORDERER` / `PEER5_PORT_ORDERER_ADMIN`). |
| **`scripts/setAnchorPeer.sh`** | Orderer: **`orderer.example.com:${FABRIC_ORDERER_PORT}`**. Anchor ports default to **7051** / **9051** (single-machine Compose). **`remote-peer5-channel-join-anchors.sh`** exports **5001** / **7001** for the five-server case. |
| **`scripts/configUpdate.sh`**, **`scripts/ccutils.sh`**, **`scripts/orderer.sh`**, **org3 scripts** | Orderer client uses **`FABRIC_ORDERER_PORT`**; **osnadmin** uses **`FABRIC_ORDERER_ADMIN_PORT`**. |
| **`scripts/remote-peer5-bootstrap.sh`** | **osnadmin** uses **`localhost:${PEER5_PORT_ORDERER_ADMIN:-9002}`** (optional credentials). |
| **`scripts/remote-peer5-channel-join-anchors.sh`** | Channel join uses **5001 / 6001 / 7001 / 8001**; exports those for anchor updates. |
| **`prometheus-grafana/prometheus/prometheus.yml`** | Orderer scrape target **`orderer.example.com:9004`**. |
| **`docs/5-SERVER-DEPLOYMENT.md`** | Section 5.6 and anchor notes updated for the above. |

**Note:** Peer services in **`compose/compose-test-net.yaml`** are still **7051 / 9051** for all-in-one local Compose (`localhost:7051` in `envVar.sh`). The distributed five-server path uses **5xxx / 6xxx / 7xxx / 8xxx**.

## What is already reflected in the repo (not the same as “done on VMs”)

- Firewall port layout in **`docs/servers.credentials.local.example`** and the scripts above.
- **`ChannelUsingRaft`** profile in **`configtx/configtx.yaml`**.
- Orchestration via **`scripts/deploy-five-servers.sh`** (host prep, tar sync, peer-5 bootstrap, MSP copy, `remote-start-peer.sh`, join + anchors).
- Runbook and known caveats in **`docs/5-SERVER-DEPLOYMENT.md`** (manual CA enrolment, path casing, etc.).

Nothing is confirmed **executed on the VMs** until you run installs and Docker there.

## What you still need to do (operational)

1. **Network / security** — VPN to `10.48.59.x`, ESDS firewall rules for the published ranges; rotate passwords if they were exposed.
2. **Credentials** — `cp docs/servers.credentials.local.example docs/servers.credentials.local` and fill values. Required on the **admin machine** for **`deploy-five-servers.sh`**. For script defaults on **peer-5**, place a copy at **`$IGR_NETWORK/docs/servers.credentials.local`** (this file is **not** included in the deploy tarball). Peers 1–4 only need it if you change ports from the defaults.
3. **Each VM** — `/etc/hosts` as in the plan, Docker + `igr` in the `docker` group, firewall open only for the planned ports.
4. **Deploy** — Run **`bash scripts/deploy-five-servers.sh`** (with `sshpass` and `servers.credentials.local`), or follow **`docs/MANUAL-DEPLOYMENT.md`** if automated SSH fails, or **`docs/5-SERVER-DEPLOYMENT.md`** for full detail.
5. **CCAAS** — Build/run chaincode on **peer-5** on **9003** (`chaincode.igr.example.com:9003`); complete install / approve / commit.
6. **Verification** — `peer channel list`, connectivity checks, `querycommitted`, etc., as in the deployment doc.
7. **If a channel was already created with orderer port 7050** — Regenerate the channel block with the current **`configtx.yaml`** and redo channel creation/participation (or plan an explicit config migration). Old artifacts still encode the previous orderer endpoint.

## Related files

- Full runbook: `docs/5-SERVER-DEPLOYMENT.md`
- Credential template: `docs/servers.credentials.local.example`
