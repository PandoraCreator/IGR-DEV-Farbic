#!/usr/bin/env bash
# Build CCAAS image on a machine with Docker Hub access (your laptop), save for peer-4.
# Output: channel-artifacts/igr_anchor_ccaas.tar.gz
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PARENT="$(cd "$ROOT/.." && pwd)"
CC_IMAGE="${CC_IMAGE:-igr_anchor_ccaas}"
CC_PORT="${CC_PORT:-8003}"
OUT="${OUT:-$ROOT/channel-artifacts/${CC_IMAGE}.tar.gz}"

if [[ -f "$PARENT/chaincode/go.mod" ]]; then
  CHAINCODE_PATH="$PARENT/chaincode"
elif [[ -f "$ROOT/chaincode/go.mod" ]]; then
  CHAINCODE_PATH="$ROOT/chaincode"
else
  echo "chaincode/ not found next to IGR-network" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUT")"
echo "==> Building ${CC_IMAGE}:latest from $CHAINCODE_PATH"
docker build -f "$CHAINCODE_PATH/Dockerfile" -t "${CC_IMAGE}:latest" \
  --build-arg "CC_SERVER_PORT=${CC_PORT}" "$CHAINCODE_PATH"

echo "==> Saving to $OUT"
docker save "${CC_IMAGE}:latest" | gzip > "$OUT"
gzip -t "$OUT"
echo "==> Verified: $(du -h "$OUT" | cut -f1)"
echo "Done. Upload to peer-4:"
echo "  scp -P 5522 $OUT igr@10.48.59.78:/opt/igr-network/channel-artifacts/"
echo "Then on peer-4:"
echo "  export SUDO_PASS='...'"
echo "  bash scripts/remote-peer5-deploy-ccaas.sh"
