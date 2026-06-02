#!/usr/bin/env bash
# Alias: run OPS bootstrap on peer-4 (same as remote-peer5-bootstrap.sh).
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/remote-peer5-bootstrap.sh" "$@"
