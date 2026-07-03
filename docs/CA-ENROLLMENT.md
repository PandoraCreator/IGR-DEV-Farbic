# CA enrollment and MSP paths

Fabric MSP directories are **case-sensitive** and must match [configtx/configtx.yaml](../configtx/configtx.yaml):

- `organizations/peerOrganizations/IGRPrimary.example.com`
- `organizations/peerOrganizations/IGRBank.example.com`

## Canonical enrollment

All `fabric-ca-client` enroll/register paths live in:

- [organizations/fabric-ca/registerEnroll.sh](../organizations/fabric-ca/registerEnroll.sh) — `createIGRPrimary`, `createIGRBank`, `createOrderer`
- [scripts/ca-enroll-helpers.sh](../scripts/ca-enroll-helpers.sh) — shared NodeOU and TLS helpers

Production OPS bootstrap ([scripts/remote-peer5-bootstrap.sh](../scripts/remote-peer5-bootstrap.sh)) calls the same functions after starting CAs via [compose/compose-ca.yaml](../compose/compose-ca.yaml).

Local `./network.sh up -ca` also sources `registerEnroll.sh` (CRYPTO=`Certificate Authorities`).

## Regression check

Before committing enrollment changes:

```bash
cd IGR-network
bash scripts/check-msp-path-casing.sh
```

This fails on lowercase paths such as `igrprimary.example.com`, `fabric-ca/igrprimary`, or `--caname ca-igrprimary`.

## Identity usernames (lowercase OK)

CA login names `igrprimaryadmin` / `igrbankadmin` are enrollment **usernames**, not filesystem paths — those remain lowercase.

## UAT validation (after MA-04)

Fresh enroll on Dev/UAT, then verify admin MSP works with a chaincode invoke. Runtime re-enroll on servers is out of scope for repo-only changes; see [CHAINCODE-UAT.md](CHAINCODE-UAT.md).
