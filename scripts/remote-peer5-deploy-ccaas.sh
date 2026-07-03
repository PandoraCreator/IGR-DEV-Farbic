 #!/usr/bin/env bash
# Build CCAAS chaincode on peer-4, install on 3 remote peers, approve, commit.
# Run on OPS host: /opt/igr-network
# Docker: uses `docker` if allowed; else sudo (set SUDO_PASS like bootstrap).
set -euo pipefail

if [[ "$(id -un)" == "root" && -z "${ALLOW_ROOT_DEPLOY:-}" ]]; then
  echo "Do not run this script with sudo." >&2
  echo "  export SUDO_PASS='...'; bash scripts/remote-peer5-deploy-ccaas.sh" >&2
  exit 1
fi

docker_cmd() {
  if docker info >/dev/null 2>&1; then
    docker "$@"
    return $?
  fi
  if [[ -n "${SUDO_PASS:-}" ]]; then
    echo "${SUDO_PASS}" | sudo -S docker "$@"
  else
    sudo docker "$@"
  fi
}

IGR_NETWORK="${IGR_NETWORK:-/opt/igr-network}"
CC_NAME="${CC_NAME:-igr_anchor}"
CC_VERSION="${CC_VERSION:-1.0}"
CC_SEQUENCE="${CC_SEQUENCE:-1}"
CHANNEL_NAME="${CHANNEL_NAME:-igrchannel}"
CC_IMAGE="${CC_IMAGE:-igr_anchor_ccaas}"
CC_CONTAINER="${CC_CONTAINER:-igr_anchor_ccaas}"

cd "$IGR_NETWORK"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=ensure-fabric-binaries.sh
. "${SCRIPT_DIR}/ensure-fabric-binaries.sh"
_user_home="$(_fabric_bin_user_home 2>/dev/null || true)"
[[ -z "${_user_home}" ]] && _user_home="${HOME}"
export PATH="${_user_home}/fabric-samples/bin:${_user_home}/bin:${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"
ensure_fabric_binaries
export PATH="${_user_home}/fabric-samples/bin:${_user_home}/bin:${PATH}"
export FABRIC_CFG_PATH="$PWD/compose/docker/peercfg"
export TEST_NETWORK_HOME="$IGR_NETWORK"

# shellcheck source=load-ops-env.sh
. "${SCRIPT_DIR}/load-ops-env.sh"
load_ops_env
# shellcheck disable=SC1091
[[ -f "$IGR_NETWORK/docs/servers.credentials.local" ]] && source "$IGR_NETWORK/docs/servers.credentials.local" || true

PEER1_PORT_PEER="${PEER1_PORT_PEER:-5001}"
PEER2_PORT_PEER="${PEER2_PORT_PEER:-6001}"
PEER3_PORT_PEER="${PEER3_PORT_PEER:-7001}"
PEER5_PORT_PEER="${PEER5_PORT_PEER:-9001}"
CC_PORT="${OPS_PORT_CHAINCODE:-8003}"
CC_ADDR="${CC_EXTERNAL_ADDRESS:-chaincode.igr.example.com:${CC_PORT}}"
_resolve_chaincode_path() {
  local c
  for c in \
    "${CHAINCODE_PATH:-}" \
    "${IGR_NETWORK}/chaincode" \
    /opt/chaincode \
    "${IGR_NETWORK}/../chaincode"; do
    [[ -n "$c" && -f "$c/go.mod" && -f "$c/Dockerfile" ]] && { echo "$c"; return 0; }
  done
  return 1
}
if ! CHAINCODE_PATH="$(_resolve_chaincode_path)"; then
  echo "Chaincode not found. Expected one of:" >&2
  echo "  ${IGR_NETWORK}/chaincode  (sync from laptop: bash scripts/sync-to-peer4.sh)" >&2
  echo "  /opt/chaincode" >&2
  echo "Or set: CHAINCODE_PATH=/path/to/chaincode bash $0" >&2
  exit 1
fi
export CHAINCODE_PATH

export PEER1_PORT_PEER PEER2_PORT_PEER PEER3_PORT_PEER PEER5_PORT_PEER
export FABRIC_ORDERER_HOST="${FABRIC_ORDERER_HOST:-127.0.0.1}"
export ORDERER_CA="$IGR_NETWORK/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem"
export CHANNEL_NAME CC_NAME CC_VERSION CC_SEQUENCE
export DELAY="${DELAY:-3}"
export MAX_RETRY="${MAX_RETRY:-5}"
export INIT_REQUIRED=""
# CC_END_POLICY override: pass a full signature policy, e.g.
#   export CC_END_POLICY="--signature-policy OR('IGRPrimaryMSP.peer','IGRBankMSP.peer')"
# Empty = default MAJORITY policy (requires both orgs to endorse).
export CC_END_POLICY="${CC_END_POLICY:-}"
export CC_COLL_CONFIG=""
# Remote peers must receive blocks from orderer; increase wait or disable (see preflight).
export FABRIC_EVENT_TIMEOUT="${FABRIC_EVENT_TIMEOUT:-300s}"
export FABRIC_WAIT_FOR_EVENT="${FABRIC_WAIT_FOR_EVENT:-true}"

echo "==> Chaincode path: $CHAINCODE_PATH"
echo "==> CCAAS address:  $CC_ADDR (port $CC_PORT)"

# --- package ---
LABEL="${CC_NAME}_${CC_VERSION}"
PKG_DIR=$(mktemp -d)
trap 'rm -rf "$PKG_DIR"' EXIT
mkdir -p "$PKG_DIR/src" "$PKG_DIR/pkg"
cat > "$PKG_DIR/src/connection.json" <<EOF
{
  "address": "${CC_ADDR}",
  "dial_timeout": "30s",
  "tls_required": false
}
EOF
cat > "$PKG_DIR/pkg/metadata.json" <<EOF
{"type":"ccaas","label":"${LABEL}"}
EOF
# Deterministic packaging: fixed mtime/owner + gzip -n so the package ID is
# stable across runs (avoids CCID drift when a deploy is re-run).
_DET_TAR=(--sort=name --mtime='UTC 2020-01-01' --owner=0 --group=0 --numeric-owner)
tar "${_DET_TAR[@]}" -C "$PKG_DIR/src" -cf - . | gzip -n > "$PKG_DIR/pkg/code.tar.gz"
tar "${_DET_TAR[@]}" -C "$PKG_DIR/pkg" -cf - metadata.json code.tar.gz | gzip -n > "$IGR_NETWORK/${CC_NAME}.tar.gz"
export PACKAGE_ID
PACKAGE_ID=$(peer lifecycle chaincode calculatepackageid "${CC_NAME}.tar.gz")
echo "==> PACKAGE_ID=$PACKAGE_ID"

# --- docker image (build on peer-4 OR load offline bundle from laptop) ---
CCAAS_IMAGE_TAR="${CCAAS_IMAGE_TAR:-$IGR_NETWORK/channel-artifacts/${CC_IMAGE}.tar.gz}"

_load_ccaas_image_tar() {
  local tar_path="$1" load_input _tmp=""
  if [[ ! -s "$tar_path" ]]; then
    echo "Image bundle is missing or empty: $tar_path" >&2
    return 1
  fi
  echo "==> Bundle size: $(du -h "$tar_path" | cut -f1)"
  if [[ "$tar_path" == *.gz ]]; then
    if ! gzip -t "$tar_path" 2>/dev/null; then
      echo "gzip check failed — re-upload from laptop (file truncated?)" >&2
      return 1
    fi
    _tmp=$(mktemp /tmp/ccaas-docker-load.XXXXXX.tar)
    gunzip -c "$tar_path" > "$_tmp" || { echo "gunzip failed" >&2; rm -f "$_tmp"; return 1; }
    load_input="$_tmp"
  else
    load_input="$tar_path"
  fi
  # Do not pipe into docker_cmd when using sudo -S (password reads stdin and breaks the stream).
  docker_cmd load -i "$load_input"
  [[ -n "$_tmp" ]] && rm -f "$_tmp"
  docker_cmd image inspect "${CC_IMAGE}:latest" >/dev/null
}

_ensure_ccaas_image() {
  if docker_cmd image inspect "${CC_IMAGE}:latest" >/dev/null 2>&1; then
    if [[ "${FORCE_CC_IMAGE:-}" == "1" ]]; then
      echo "==> FORCE_CC_IMAGE=1: removing stale image ${CC_IMAGE}:latest to reload new build"
      docker_cmd rm -f "$CC_CONTAINER" 2>/dev/null || true
      docker_cmd rmi -f "${CC_IMAGE}:latest" 2>/dev/null || true
    else
      echo "==> Docker image ${CC_IMAGE}:latest already loaded (set FORCE_CC_IMAGE=1 to reload new source)"
      return 0
    fi
  fi
  if [[ -f "$CCAAS_IMAGE_TAR" ]]; then
    echo "==> Loading offline image from $CCAAS_IMAGE_TAR"
    _load_ccaas_image_tar "$CCAAS_IMAGE_TAR" || return 1
    return 0
  fi
  if [[ "${SKIP_DOCKER_BUILD:-}" == "1" ]]; then
    echo "SKIP_DOCKER_BUILD=1 but image missing and no bundle at:" >&2
    echo "  $CCAAS_IMAGE_TAR" >&2
    echo "On laptop: bash scripts/build-ccaas-image-bundle.sh && bash scripts/upload-ccaas-image-to-peer4.sh" >&2
    return 1
  fi
  echo "==> Building image ${CC_IMAGE}:latest (needs Docker Hub; use offline bundle if this fails)"
  if ! docker_cmd build -f "$CHAINCODE_PATH/Dockerfile" -t "${CC_IMAGE}:latest" \
    --build-arg "CC_SERVER_PORT=${CC_PORT}" "$CHAINCODE_PATH"; then
    cat >&2 <<EOF

Docker build failed (peer-4 often cannot reach registry-1.docker.io).

On your laptop (with internet):
  cd /path/to/IGR-network
  bash scripts/build-ccaas-image-bundle.sh
  bash scripts/upload-ccaas-image-to-peer4.sh

On peer-4:
  export SUDO_PASS='...'
  bash scripts/remote-peer5-deploy-ccaas.sh

EOF
    return 1
  fi
}

if ! docker info >/dev/null 2>&1 && [[ -z "${SUDO_PASS:-}" ]]; then
  echo "Docker requires root on this host. Run:" >&2
  echo "  export SUDO_PASS='your-password'" >&2
  echo "  bash scripts/remote-peer5-deploy-ccaas.sh" >&2
  exit 1
fi

_ensure_ccaas_image

docker_cmd rm -f "$CC_CONTAINER" 2>/dev/null || true
echo "==> Starting CCAAS container ${CC_CONTAINER}"
docker_cmd run -d --name "$CC_CONTAINER" --restart unless-stopped \
  -p "${CC_PORT}:${CC_PORT}" \
  -e "CHAINCODE_SERVER_ADDRESS=0.0.0.0:${CC_PORT}" \
  -e "CHAINCODE_ID=${PACKAGE_ID}" \
  -e "CORE_CHAINCODE_ID_NAME=${PACKAGE_ID}" \
  "${CC_IMAGE}:latest"

# --- lifecycle helpers ---
install_on_peer() {
  local msp_id=$1 tls_ca=$2 msp_path=$3 peer_addr=$4
  export CORE_PEER_LOCALMSPID="$msp_id"
  export CORE_PEER_TLS_ENABLED=true
  export CORE_PEER_TLS_ROOTCERT_FILE="$tls_ca"
  export CORE_PEER_MSPCONFIGPATH="$msp_path"
  export CORE_PEER_ADDRESS="$peer_addr"
  if peer lifecycle chaincode queryinstalled --output json 2>/dev/null | jq -r '.installed_chaincodes[].package_id' | grep -q "^${PACKAGE_ID}$"; then
    echo "Already installed on $peer_addr"
    return 0
  fi
  peer lifecycle chaincode install "${IGR_NETWORK}/${CC_NAME}.tar.gz"
  echo "Installed on $peer_addr"
}

TLS_PRIMARY="$IGR_NETWORK/organizations/peerOrganizations/IGRPrimary.example.com/tlsca/tlsca.IGRPrimary.example.com-cert.pem"
TLS_BANK="$IGR_NETWORK/organizations/peerOrganizations/IGRBank.example.com/tlsca/tlsca.IGRBank.example.com-cert.pem"
MSP_PRIMARY="$IGR_NETWORK/organizations/peerOrganizations/IGRPrimary.example.com/users/Admin@IGRPrimary.example.com/msp"
MSP_BANK="$IGR_NETWORK/organizations/peerOrganizations/IGRBank.example.com/users/Admin@IGRBank.example.com/msp"

echo "==> Installing chaincode package on peers"
install_on_peer IGRPrimaryMSP "$TLS_PRIMARY" "$MSP_PRIMARY" "peer0.IGRPrimary.example.com:${PEER1_PORT_PEER}"
install_on_peer IGRPrimaryMSP "$TLS_PRIMARY" "$MSP_PRIMARY" "peer1.IGRPrimary.example.com:${PEER2_PORT_PEER}"
install_on_peer IGRBankMSP "$TLS_BANK" "$MSP_BANK" "peer0.IGRBank.example.com:${PEER3_PORT_PEER}"
install_on_peer IGRBankMSP "$TLS_BANK" "$MSP_BANK" "peer1.IGRBank.example.com:${PEER5_PORT_PEER}"

# ccutils/envVar use optional vars; avoid set -u errors when sourced
export VERBOSE="${VERBOSE:-false}"
export OVERRIDE_ORG="${OVERRIDE_ORG:-}"
set +u
# shellcheck source=envVar.sh
. "${SCRIPT_DIR}/envVar.sh"
# shellcheck source=ccutils.sh
. "${SCRIPT_DIR}/ccutils.sh"
set -u
# ccutils uses `let rc=0` which returns exit status 1 under `set -e` and would
# silently abort before commit. These functions exit explicitly via fatalln on
# real errors, so disable errexit for the lifecycle steps.
set +e

# Re-approving an org with identical content fails with
# "attempted to redefine uncommitted sequence ... with unchanged content".
# Skip the approve when this org already approved this sequence + package id.
approve_if_needed() {
  local org=$1
  setGlobals "$org"
  local approved
  approved=$(peer lifecycle chaincode queryapproved --channelID "$CHANNEL_NAME" --name "$CC_NAME" 2>/dev/null)
  if echo "$approved" | grep -q "sequence: ${CC_SEQUENCE}," && echo "$approved" | grep -q "${PACKAGE_ID}"; then
    successln "org${org} already approved seq ${CC_SEQUENCE} (${PACKAGE_ID}); skipping approve"
  else
    approveForMyOrg "$org"
  fi
}

# If this version+sequence is already committed, the approve/readiness/commit
# steps would error (e.g. checkcommitreadiness reports "must be sequence N+1").
# In that case the lifecycle is done; only the CCAAS image needed refreshing
# (already handled above), so skip the entire lifecycle block.
setGlobals 1
if peer lifecycle chaincode querycommitted --channelID "$CHANNEL_NAME" --name "$CC_NAME" 2>/dev/null \
    | grep -q "Version: ${CC_VERSION}, Sequence: ${CC_SEQUENCE},"; then
  successln "seq ${CC_SEQUENCE} v${CC_VERSION} already committed on ${CHANNEL_NAME}; skipping approve/commit"
else
  echo "==> Approve for IGRPrimaryMSP"
  approve_if_needed 1
  checkCommitReadiness 1 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": false"
  checkCommitReadiness 2 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": false"

  echo "==> Approve for IGRBankMSP"
  approve_if_needed 2
  checkCommitReadiness 1 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": true"
  checkCommitReadiness 2 "\"IGRPrimaryMSP\": true" "\"IGRBankMSP\": true"

  echo "==> Commit definition"
  commitChaincodeDefinition 1 2
fi

queryCommitted 1
queryCommitted 2

echo ""
echo "Deploy OK: ${CC_NAME} v${CC_VERSION} seq ${CC_SEQUENCE} on ${CHANNEL_NAME}"
echo "PACKAGE_ID=${PACKAGE_ID}"
echo "Test invoke (IGRPrimary admin; run on peer-4 OPS .78):"
echo "  peer chaincode query -C ${CHANNEL_NAME} -n ${CC_NAME} -c '{\"function\":\"GetLatestAnchor\",\"Args\":[\"MH:PUNE:SR42:2026:991\"]}'"
echo "  Or run: bash scripts/verify-ccaas-deploy.sh"
