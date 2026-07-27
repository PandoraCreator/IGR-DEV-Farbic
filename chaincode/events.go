package main

import (
	"encoding/json"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// emitWriteEvent emits a chaincode event on each write for downstream
// audit/observability. Payload shape: { type, originalDocumentUid,
// certifiedCopyUid?, txnId } (KB 05 §Events). certifiedCopyUid is omitted for
// loan events, which are keyed by originalDocumentUid only.
func emitWriteEvent(ctx contractapi.TransactionContextInterface, eventType, originalDocumentUid, certifiedCopyUid, txnId string) {
	payload := map[string]string{
		"type":                eventType,
		"originalDocumentUid": originalDocumentUid,
		"txnId":               txnId,
	}
	if certifiedCopyUid != "" {
		payload["certifiedCopyUid"] = certifiedCopyUid
	}
	b, _ := json.Marshal(payload)
	_ = ctx.GetStub().SetEvent(eventType, b)
}
