#!/usr/bin/env bash
# Fail if enrollment scripts use lowercase MSP path casing (must match configtx).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FORBIDDEN=(
  'peerOrganizations/igrprimary'
  'peerOrganizations/igrbank'
  'fabric-ca/igrprimary'
  'fabric-ca/igrbank'
  'ca-igrprimary'
  'ca-igrbank'
  'peer0.igrprimary'
  'peer1.igrprimary'
  'peer0.igrbank'
  'peer1.igrbank'
  'Admin@igrprimary'
  'Admin@igrbank'
  'User1@igrprimary'
  'User1@igrbank'
  'tlsca.igrprimary'
  'tlsca.igrbank'
)

SCAN_DIRS=(
  scripts
  organizations/fabric-ca
  network.sh
)

fail=0
for pat in "${FORBIDDEN[@]}"; do
  hits=$(grep -RInF --exclude='check-msp-path-casing.sh' "$pat" "${SCAN_DIRS[@]}" 2>/dev/null || true)
  if [[ -n "$hits" ]]; then
    echo "FORBIDDEN pattern '$pat':" >&2
    echo "$hits" >&2
    fail=1
  fi
done

if [[ "$fail" -ne 0 ]]; then
  echo "MSP path casing check FAILED. Use IGRPrimary.example.com / IGRBank.example.com." >&2
  exit 1
fi

echo "MSP path casing check OK."
