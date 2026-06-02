#!/usr/bin/env bash
# Upload pre-built CCAAS image bundle to peer-4 (after build-ccaas-image-bundle.sh).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CC_IMAGE="${CC_IMAGE:-igr_asset_registry_ccaas}"
BUNDLE="${BUNDLE:-$ROOT/channel-artifacts/${CC_IMAGE}.tar.gz}"

CREDS="${ROOT}/docs/servers.credentials.local"
[[ -f "$CREDS" ]] || { echo "Missing $CREDS" >&2; exit 1; }
# shellcheck disable=SC1090
source "$CREDS"

[[ -f "$BUNDLE" ]] || { echo "Missing bundle: $BUNDLE — run scripts/build-ccaas-image-bundle.sh first" >&2; exit 1; }

HOST="${OPS_HOST:-${PEER4_HOST:?}}"
PORT="${SSH_PORT:-5522}"
USER="${SSH_USER:-igr}"
DIR="${IGR_NETWORK:-/opt/igr-network}"

echo ">>> Uploading $(du -h "$BUNDLE" | cut -f1) to ${USER}@${HOST}:${DIR}/channel-artifacts/"
ssh -p "$PORT" -o StrictHostKeyChecking=accept-new "${USER}@${HOST}" \
  "mkdir -p '${DIR}/channel-artifacts'"

if command -v sshpass >/dev/null 2>&1 && [[ -n "${PEER4_PASSWORD:-}" ]]; then
  sshpass -p "${PEER4_PASSWORD}" scp -P "$PORT" -o StrictHostKeyChecking=accept-new \
    "$BUNDLE" "${USER}@${HOST}:${DIR}/channel-artifacts/"
else
  scp -P "$PORT" -o StrictHostKeyChecking=accept-new \
    "$BUNDLE" "${USER}@${HOST}:${DIR}/channel-artifacts/"
fi

echo ">>> On peer-4 run:"
echo "    export SUDO_PASS='...'"
echo "    cd ${DIR} && bash scripts/remote-peer5-deploy-ccaas.sh"
