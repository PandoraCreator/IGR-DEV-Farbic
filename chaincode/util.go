package main

import (
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// txTimestampRFC3339 returns the transaction timestamp as an RFC3339 UTC string.
// Using the tx timestamp (not wall-clock) keeps the value deterministic across
// endorsing peers.
func txTimestampRFC3339(ctx contractapi.TransactionContextInterface) (string, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return "", fmt.Errorf("failed to get tx timestamp: %v", err)
	}
	return ts.AsTime().UTC().Format(time.RFC3339), nil
}
