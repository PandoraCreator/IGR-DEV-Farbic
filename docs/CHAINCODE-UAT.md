# Chaincode deployment: POC vs UAT (MA-04)

Repo defaults target **UAT** chaincode naming and the **igr-anchor KB model**. **Do not redeploy to production/UAT servers until ops sign-off.**

## Naming

| Context | Channel | Chaincode name | Notes |
|---------|---------|----------------|-------|
| **Repo / UAT target** | `igrchannel` | `igr_anchor` | `network.config`, deploy scripts, CCAAS image `igr_anchor_ccaas` |
| **Live POC (servers today)** | `igrchannel` | `asset_registry` v1.1 | Legacy SignDoc flow — not UAT-ready |

## MA-04 complete (repo)

The repo now implements the igr-anchor KB model:

| Function | Purpose |
|----------|---------|
| `CreateAnchorVersion` | Immutable anchor write (`UID~` + `DOC~` pointer) |
| `GetLatestAnchor` | Read latest version for a docRef |
| `GetAnchorByUid` | Read one version record |
| `VerifyDocHash` | Compare normalized SHA-256 hash — returns `{status: MATCH\|MISMATCH\|NOT_FOUND}` |
| `GetLoanState` | Loan encumbrance for docRef — `ACTIVE`, `RELEASED`, or `NOT_FOUND` |
| `MarkLoanActive` | Set `LOAN~` to ACTIVE (IGRBankMSP / IGRPrimaryMSP) |
| `ReleaseLoan` | Set `LOAN~` to RELEASED |

**Test vectors:** docRef `MH:PUNE:SR42:2026:991`, uid `:V1`/`:V2`, hash `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.

**Verification gate (local):**

```bash
cd chaincode && go test ./... -count=1
cd chaincode && go build -o /dev/null .
cd igr-api && python -m pytest tests/ -q
bash IGR-network/scripts/check-msp-path-casing.sh
```

## Roadmap (post MA-04)

| Story | Change |
|-------|--------|
| MA-05 | Idempotent `CreateAnchorVersion` — **done in repo** |
| MA-06 | `VerifyDocHash` four-state — **done in repo** |
| MA-07 | Integrity field trim audit (optional extra tests) |
| MA-08 | `LOAN~` + loan API routes — **done in repo** |

## POC re-deploy (servers only when explicitly approved)

To interact with the **existing** POC chaincode on live servers, override env vars — do not use repo defaults:

```bash
export CC_NAME=asset_registry
export CC_VERSION=1.1
export CC_SEQUENCE=2   # match committed sequence on igrchannel
export CC_IMAGE=igr_asset_registry_ccaas   # if container already named on OPS host
export CC_CONTAINER=igr_asset_registry_ccaas
bash scripts/remote-peer5-deploy-ccaas.sh   # runs on peer-4 OPS (.78)
```

Verify against POC:

```bash
export CC_NAME=asset_registry
bash scripts/verify-ccaas-deploy.sh
```

## UAT deploy (after MA-04 ops sign-off)

From laptop (build image):

```bash
cd IGR-network
bash scripts/build-ccaas-image-bundle.sh
bash scripts/upload-ccaas-image-to-peer4.sh
```

On **peer-4 OPS** (`.78`, port `:8003`; script name `remote-peer5-*` is a legacy alias):

```bash
cd /opt/igr-network
export SUDO_PASS='...'
bash scripts/remote-peer5-deploy-ccaas.sh
bash scripts/verify-ccaas-deploy.sh
```

Smoke query (expects anchor data or structured not-found after at least one anchor):

```bash
peer chaincode query -C igrchannel -n igr_anchor \
  -c '{"function":"GetLatestAnchor","Args":["MH:PUNE:SR42:2026:991"]}'
```

API smoke (post-deploy, from laptop with Fabric running):

```bash
curl -s "http://localhost:8080/blockchain/latest/MH%3APUNE%3ASR42%3A2026%3A991"
```

## Opaque artifacts

CCAAS Docker image bundles (`channel-artifacts/*_ccaas.tar.gz`) are **gitignored**. Build locally and upload via `scripts/upload-ccaas-image-to-peer4.sh`; never commit them.

## CA enrollment paths

MSP directory casing must match configtx — see [CA-ENROLLMENT.md](CA-ENROLLMENT.md). Run `bash scripts/check-msp-path-casing.sh` when changing enrollment scripts.

## Related

- Topology: [KB/01.md](../../KB/01.md) — peer-4 = OPS, peer-5 = peer1 IGRBank
- Milestones: [KB/milestone.md](../../KB/milestone.md) — MA-04 through MA-08
