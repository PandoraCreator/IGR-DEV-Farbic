#!/usr/bin/env bash
# Pack IGR-network and copy to peer-4 (OPS host). Run from your laptop in the repo root:
#   bash scripts/sync-to-peer4.sh
#
# You will be prompted for the SSH password unless sshpass is installed and credentials are sourced.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CREDS="${ROOT}/docs/servers.credentials.local"
if [[ ! -f "$CREDS" ]]; then
  echo "Missing $CREDS — copy from docs/servers.credentials.local.example" >&2
  exit 1
fi
# shellcheck disable=SC1090
source "$CREDS"

TARGET_HOST="${OPS_HOST:-${PEER4_HOST:?set PEER4_HOST in servers.credentials.local}}"
TARGET_USER="${SSH_USER:-igr}"
TARGET_PORT="${SSH_PORT:-5522}"
TARGET_DIR="${IGR_NETWORK:-/opt/igr-network}"

ARCHIVE="/tmp/igr-network-sync-$$.tgz"
ARCHIVE_TAR="/tmp/igr-network-sync-$$.tar"
PARENT="$(cd "$ROOT/.." && pwd)"
trap 'rm -f "$ARCHIVE" "$ARCHIVE_TAR"' EXIT

echo ">>> Building archive (excludes .git, local crypto, channel blocks)..."
tar -C "$ROOT" \
  --exclude './.git' \
  --exclude './organizations/peerOrganizations' \
  --exclude './organizations/ordererOrganizations' \
  --exclude './channel-artifacts/*.block' \
  --exclude './system-genesis-block' \
  --exclude './docs/servers.credentials.local' \
  -cf "$ARCHIVE_TAR" .

if [[ -f "$PARENT/chaincode/go.mod" ]]; then
  echo ">>> Including ../chaincode in archive (→ ${TARGET_DIR}/chaincode on peer-4)"
  tar -C "$PARENT" -rf "$ARCHIVE_TAR" chaincode
elif [[ -f "$ROOT/chaincode/go.mod" ]]; then
  echo ">>> Including chaincode/ in archive"
  tar -C "$ROOT" -rf "$ARCHIVE_TAR" chaincode
else
  echo ">>> WARNING: no chaincode/go.mod found — CCAAS deploy will fail until chaincode is copied" >&2
fi

gzip -c "$ARCHIVE_TAR" > "$ARCHIVE"
rm -f "$ARCHIVE_TAR"

echo ">>> Archive size: $(du -h "$ARCHIVE" | cut -f1)"
echo ">>> Ensuring ${TARGET_DIR} on ${TARGET_USER}@${TARGET_HOST}:${TARGET_PORT}"

SSH_OPTS=(-p "$TARGET_PORT" -o StrictHostKeyChecking=accept-new)
SCP_OPTS=(-P "$TARGET_PORT" -o StrictHostKeyChecking=accept-new)

run_ssh() {
  if command -v sshpass >/dev/null 2>&1 && [[ -n "${PEER4_PASSWORD:-}" ]]; then
    sshpass -p "${PEER4_PASSWORD}" ssh "${SSH_OPTS[@]}" "${TARGET_USER}@${TARGET_HOST}" "$@"
  else
    ssh "${SSH_OPTS[@]}" "${TARGET_USER}@${TARGET_HOST}" "$@"
  fi
}

run_scp() {
  if command -v sshpass >/dev/null 2>&1 && [[ -n "${PEER4_PASSWORD:-}" ]]; then
    sshpass -p "${PEER4_PASSWORD}" scp "${SCP_OPTS[@]}" "$@"
  else
    scp "${SCP_OPTS[@]}" "$@"
  fi
}

run_ssh "echo '${PEER4_PASSWORD}' | sudo -S mkdir -p '${TARGET_DIR}' && echo '${PEER4_PASSWORD}' | sudo -S chown -R ${TARGET_USER}:${TARGET_USER} '${TARGET_DIR}'"

echo ">>> Uploading to ${TARGET_HOST}:${TARGET_DIR}/"
run_scp "$ARCHIVE" "${TARGET_USER}@${TARGET_HOST}:/tmp/igr-network-sync.tgz"

echo ">>> Extracting on peer-4..."
run_ssh "mkdir -p '${TARGET_DIR}' && cd '${TARGET_DIR}' && tar -xzf /tmp/igr-network-sync.tgz && rm -f /tmp/igr-network-sync.tgz && chmod +x scripts/*.sh 2>/dev/null || true"

echo ">>> Uploading docs/servers.credentials.local (gitignored, required on OPS host)..."
run_scp "$CREDS" "${TARGET_USER}@${TARGET_HOST}:${TARGET_DIR}/docs/servers.credentials.local"

# Mirror to /opt/chaincode after extract (deploy script checks both paths)
if [[ -f "$PARENT/chaincode/go.mod" ]] || [[ -f "$ROOT/chaincode/go.mod" ]]; then
  echo ">>> Mirroring chaincode to /opt/chaincode on peer-4..."
  run_ssh "if [ -d '${TARGET_DIR}/chaincode' ]; then \
    echo '${PEER4_PASSWORD}' | sudo -S mkdir -p /opt/chaincode && \
    echo '${PEER4_PASSWORD}' | sudo -S chown -R ${TARGET_USER}:${TARGET_USER} /opt/chaincode && \
    rm -rf /opt/chaincode/* && cp -a '${TARGET_DIR}/chaincode/.' /opt/chaincode/; fi"
fi

echo ">>> Done. SSH and verify:"
echo "    ls ${TARGET_DIR}/chaincode/go.mod /opt/chaincode/go.mod 2>/dev/null"
echo "    ssh -p ${TARGET_PORT} ${TARGET_USER}@${TARGET_HOST}"
echo "    cd ${TARGET_DIR} && grep OPS_PORT_ORDERER docs/servers.credentials.local"
echo "    grep ORDERER_GENERAL_LISTENPORT compose/compose-test-net.yaml | head -1"
