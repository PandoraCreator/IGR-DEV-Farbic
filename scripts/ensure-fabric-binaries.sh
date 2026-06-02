#!/usr/bin/env bash
# Source from peer-5 scripts:  . "$(dirname "$0")/ensure-fabric-binaries.sh"
# Adds Fabric CLI tools to PATH; installs via install-fabric.sh if missing.
#
# Env:
#   IGR_NETWORK          repo root (default /opt/igr-network)
#   INSTALL_FABRIC_BINARIES  auto | true | false  (default: auto)
#   FABRIC_TAG           e.g. 2.5.12 (optional, passed to install-fabric.sh)

# Home directory whose ~/fabric-samples/bin to use (igr when invoked via sudo).
_fabric_bin_user_home() {
  if [[ -n "${FABRIC_BIN_USER_HOME:-}" && -d "${FABRIC_BIN_USER_HOME}" ]]; then
    echo "${FABRIC_BIN_USER_HOME}"
    return
  fi
  if [[ -n "${SUDO_USER:-}" && "${SUDO_USER}" != "root" ]]; then
    echo "/home/${SUDO_USER}"
    return
  fi
  if [[ "$(id -un)" == "root" && -d /home/igr/fabric-samples/bin ]]; then
    echo /home/igr
    return
  fi
  echo "${HOME}"
}

ensure_fabric_binaries() {
  local net="${IGR_NETWORK:-/opt/igr-network}"
  local install_mode="${INSTALL_FABRIC_BINARIES:-auto}"
  local user_home
  user_home="$(_fabric_bin_user_home)"
  local d

  for d in \
    "${net}/../bin" \
    "${net}/bin" \
    "${user_home}/fabric-samples/bin" \
    "${user_home}/bin" \
    "${HOME}/fabric-samples/bin" \
    "${HOME}/bin" \
    "/usr/local/bin"; do
    if [[ -d "$d" ]]; then
      export PATH="$d:$PATH"
    fi
  done

  if command -v fabric-ca-client >/dev/null 2>&1 \
    && command -v configtxgen >/dev/null 2>&1 \
    && command -v peer >/dev/null 2>&1 \
    && command -v osnadmin >/dev/null 2>&1; then
    echo "Fabric binaries OK: $(command -v fabric-ca-client)"
    fabric-ca-client version 2>/dev/null | head -1 || true
    return 0
  fi

  if [[ "$install_mode" == "false" ]]; then
    cat >&2 <<EOF
Fabric CLI not found (need fabric-ca-client, configtxgen, peer, osnadmin).

Install manually on peer-5:
  curl -sSL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh | bash -s -- binary
  export PATH=\$HOME/fabric-samples/bin:\$PATH

Or re-run with auto-install:
  INSTALL_FABRIC_BINARIES=true bash scripts/remote-peer5-bootstrap.sh
EOF
    return 1
  fi

  if [[ "$(id -un)" == "root" && -n "${SUDO_USER:-}" ]]; then
    cat >&2 <<EOF
Fabric CLI not found in PATH for root, and auto-install from GitHub is disabled when using sudo.

Run as user ${SUDO_USER} (no sudo):
  bash scripts/remote-peer5-deploy-ccaas.sh

Or point to existing binaries:
  export PATH=${user_home}/fabric-samples/bin:\$PATH
  INSTALL_FABRIC_BINARIES=false bash scripts/remote-peer5-deploy-ccaas.sh
EOF
    return 1
  fi

  if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required to download Fabric binaries." >&2
    return 1
  fi

  if ! curl -sSL --connect-timeout 5 -o /dev/null https://raw.githubusercontent.com 2>/dev/null; then
    cat >&2 <<EOF
Cannot reach raw.githubusercontent.com (DNS/firewall). Install Fabric CLI offline:

  # on a machine with internet, then copy to peer-4:
  #   ~/fabric-samples/bin/{peer,configtxgen,osnadmin,fabric-ca-client}

On peer-4, then:
  export PATH=${user_home}/fabric-samples/bin:\$PATH
  INSTALL_FABRIC_BINARIES=false bash scripts/remote-peer5-deploy-ccaas.sh
EOF
    return 1
  fi

  echo "==> Installing Hyperledger Fabric binaries (this may take a few minutes)..."
  local install_args=(binary)
  if [[ -n "${FABRIC_TAG:-}" ]]; then
    install_args=("${FABRIC_TAG}" binary)
  fi

  (
    cd "${HOME}" || exit 1
    curl -sSL https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh \
      | bash -s -- "${install_args[@]}"
  )

  export PATH="${HOME}/fabric-samples/bin:${HOME}/bin:${PATH}"

  for d in "${net}/../bin" "${HOME}/fabric-samples/bin" "${HOME}/bin"; do
    [[ -d "$d" ]] && export PATH="$d:$PATH"
  done

  if ! command -v fabric-ca-client >/dev/null 2>&1; then
    echo "Install finished but fabric-ca-client still not in PATH." >&2
    echo "Check ~/fabric-samples/bin and add to PATH:" >&2
    ls -la "${HOME}/fabric-samples/bin" 2>/dev/null || ls -la "${HOME}/bin" 2>/dev/null || true
    return 1
  fi

  echo "==> Installed:"
  fabric-ca-client version | head -1
  configtxgen -version 2>&1 | head -1 || true
  peer version | head -1
  return 0
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  export IGR_NETWORK="${IGR_NETWORK:-$(cd "$(dirname "$0")/.." && pwd)}"
  ensure_fabric_binaries
fi
