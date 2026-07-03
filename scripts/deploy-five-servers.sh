#!/usr/bin/env bash
# Orchestrate docs/5-SERVER-DEPLOYMENT.md from your admin machine.
# Usage: source docs/servers.credentials.local (or it is sourced here); run from anywhere:
#   bash scripts/deploy-five-servers.sh

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! test -f docs/servers.credentials.local; then
  echo "Missing docs/servers.credentials.local — copy from docs/servers.credentials.local.example" >&2
  exit 1
fi
# shellcheck source=/dev/null
source docs/servers.credentials.local

if ! command -v sshpass >/dev/null 2>&1; then
  echo "Install sshpass on this machine." >&2
  exit 1
fi
igr_ssh() {
  local n=$1
  shift
  eval "local H=\${PEER${n}_HOST}; local P=\${PEER${n}_PASSWORD}"
  sshpass -p "$P" ssh -o StrictHostKeyChecking=accept-new -p "$SSH_PORT" "${SSH_USER}@${H}" "$@"
}

echo ">>> Phase 1: prepare all VMs (/opt dir, deps, /etc/hosts)"
for n in 1 2 3 4 5; do
  eval "P=\${PEER${n}_PASSWORD}"
  igr_ssh "$n" bash -s "$(printf '%q' "$P")" "${IGR_NETWORK}" "${SSH_USER}" <<'REMOTEPREP'
set -euo pipefail
PW=$1
IGR_NET=$2
U=$3
if ! echo "$PW" | sudo -S true 2>/dev/null; then
  echo "sudo failed" >&2
  exit 1
fi
echo "$PW" | sudo -S mkdir -p "$IGR_NET"
echo "$PW" | sudo -S chown -R "$U:$U" "$IGR_NET"
if ! grep -q 'peer0.IGRPrimary.example.com' /etc/hosts 2>/dev/null; then
  echo "$PW" | sudo -S tee -a /etc/hosts >/dev/null <<'EOS'
10.48.59.78  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
10.48.59.79  peer1.IGRBank.example.com
EOS
fi
echo "$PW" | sudo -S docker --version || true
REMOTEPREP
done

echo ">>> Phase 2: sync repo to all nodes (tar-over-ssh; no rsync on servers)"
TAR_EX=(
  --exclude './.git'
  --exclude './organizations/ordererOrganizations'
  --exclude './organizations/peerOrganizations'
  --exclude './channel-artifacts/*.block'
  --exclude './system-genesis-block/*.block'
  --exclude './docs/servers.credentials.local'
  --exclude './docs/Untitled'
)
for n in 1 2 3 4 5; do
  eval "H=\${PEER${n}_HOST} P=\${PEER${n}_PASSWORD}"
  echo "    tarball -> peer$n $H"
  tar -C "$ROOT" "${TAR_EX[@]}" -czf - . \
    | sshpass -p "$P" ssh -p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new "${SSH_USER}@${H}" \
      "mkdir -p '${IGR_NETWORK}' && cd '${IGR_NETWORK}' && tar -xzf -"
done

echo ">>> Phase 3: install Fabric binaries on peer-4 OPS (download)"
eval "PW4=\${PEER4_PASSWORD}"
igr_ssh 4 "$(printf '%s\n' "
set -e
export DEBIAN_FRONTEND=noninteractive
if ! command -v peer >/dev/null 2>&1; then
  curl -sSL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh | bash -s -- binary
fi
echo PATH=\\\$HOME/bin:\\\$HOME/fabric-samples/bin:\\\$PATH >> ~/.bashrc || true
export PATH=\$HOME/bin:\$HOME/fabric-samples/bin:\$PATH
peer version || true
configtxgen -version
fabric-ca-client version
")"

chmod +x "$ROOT/scripts/remote-peer5-bootstrap.sh" "$ROOT/scripts/remote-start-peer.sh" "$ROOT/scripts/remote-peer5-channel-join-anchors.sh" || true

echo ">>> Phase 4: peer-4 Fabric CAs / enroll / orderer / OSN channel join"
PW4_ESC=$(printf '%q' "$PW4")
igr_ssh 4 "export SUDO_PASS=${PW4_ESC}; export CHANNEL_NAME=${CHANNEL_NAME:-igrchannel}; export PATH=\$HOME/bin:\$HOME/fabric-samples/bin:\$PATH; bash ${IGR_NETWORK}/scripts/remote-peer5-bootstrap.sh"

echo ">>> Phase 5: copy MSP to peer-1..3 and peer-5"
TMPMSP=$(mktemp -d)
trap 'rm -rf "$TMPMSP"' EXIT
mkdir -p "$TMPMSP/organizations"
eval "H4=\$PEER4_HOST PW4=\$PEER4_PASSWORD"
sshpass -p "$PW4" ssh -p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new "${SSH_USER}@${H4}" \
  "cd '${IGR_NETWORK}/organizations' && tar czf - peerOrganizations" \
  | tar xz -C "$TMPMSP/organizations"
for n in 1 2 3 5; do
  eval "Pn=\${PEER${n}_PASSWORD} Hn=\${PEER${n}_HOST}"
  tar czf - -C "$TMPMSP/organizations" peerOrganizations \
    | sshpass -p "$Pn" ssh -p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new "${SSH_USER}@${Hn}" \
      "mkdir -p '${IGR_NETWORK}/organizations' && cd '${IGR_NETWORK}/organizations' && tar -xzf -"
  tar czf - -C "$ROOT/compose/docker" peercfg \
    | sshpass -p "$Pn" ssh -p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new "${SSH_USER}@${Hn}" \
      "mkdir -p '${IGR_NETWORK}/compose/docker' && cd '${IGR_NETWORK}/compose/docker' && tar -xzf -"
done

echo ">>> Phase 6: start CouchDB + peer on peer-1, peer-2, peer-3, peer-5"
for n in 1 2 3 5; do
  eval "Pn=\${PEER${n}_PASSWORD}"
  igr_ssh "$n" "export SUDO_PASS=$(printf '%q' "$Pn"); export PEER_NODE=$n; export IGR_NETWORK=${IGR_NETWORK}; bash ${IGR_NETWORK}/scripts/remote-start-peer.sh"
done

echo ">>> Waiting for peers (20s)"
sleep 20

echo ">>> Phase 7: peer channel join + anchors (peer-4 OPS)"
eval "PW4=\${PEER4_PASSWORD}"
PW4_ESC=$(printf '%q' "$PW4")
igr_ssh 4 "export CHANNEL_NAME=${CHANNEL_NAME:-igrchannel}; export PATH=\$HOME/bin:\$HOME/fabric-samples/bin:\$PATH; bash ${IGR_NETWORK}/scripts/remote-peer5-channel-join-anchors.sh"

echo ">>> Done. Verify on peer-4 OPS:"
echo "    export PATH=\\\$HOME/bin:\\\$HOME/fabric-samples/bin:\\\$PATH; cd ${IGR_NETWORK}"
echo "    peer channel list (after setting CORE_PEER_* per org)"
