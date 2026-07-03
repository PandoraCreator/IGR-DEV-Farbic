#!/usr/bin/env bash
# Add IGR hostnames (run on each VM with sudo). Idempotent-ish via grep check.
set -euo pipefail

HOSTS_FILE="${HOSTS_FILE:-/etc/hosts}"
MARKER="# igr-network-five-server"

block="${MARKER}
10.48.59.78  orderer.example.com chaincode.igr.example.com
10.48.59.70  peer0.IGRPrimary.example.com
10.48.59.76  peer1.IGRPrimary.example.com
10.48.59.77  peer0.IGRBank.example.com
10.48.59.79  peer1.IGRBank.example.com
"

if grep -q "$MARKER" "$HOSTS_FILE" 2>/dev/null; then
  echo "Already present in $HOSTS_FILE"
else
  echo "$block" | sudo tee -a "$HOSTS_FILE" >/dev/null
  echo "Added IGR hostnames to $HOSTS_FILE"
fi

getent hosts orderer.example.com chaincode.igr.example.com peer0.IGRPrimary.example.com peer1.IGRBank.example.com
