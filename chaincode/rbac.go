package main

import (
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// getMSPID returns the caller's MSP ID.
func getMSPID(ctx contractapi.TransactionContextInterface) (string, error) {
	mspID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("failed to get MSP ID: %v", err)
	}
	return mspID, nil
}

// requireWriterMSP enforces the single authorized writer identity on all state
// changes. There is intentionally no per-role (SRO vs bank) MSP distinction —
// that is expressed via function choice + state guards + payload fields.
func requireWriterMSP(ctx contractapi.TransactionContextInterface) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if mspID != writerMSPID {
		return fmt.Errorf("MSP %s is not authorized to write; requires %s", mspID, writerMSPID)
	}
	return nil
}
