package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// ---------------------------------------------------------------------------
// Composite key builders
// ---------------------------------------------------------------------------

func docCompositeKey(stub shim.ChaincodeStubInterface, originalDocumentUid string) (string, error) {
	return stub.CreateCompositeKey(docObjectType, []string{originalDocumentUid})
}

func uidCompositeKey(stub shim.ChaincodeStubInterface, certifiedCopyUid string) (string, error) {
	return stub.CreateCompositeKey(uidObjectType, []string{certifiedCopyUid})
}

func loanCompositeKey(stub shim.ChaincodeStubInterface, originalDocumentUid string) (string, error) {
	return stub.CreateCompositeKey(loanObjectType, []string{originalDocumentUid})
}

// loanHistCompositeKey zero-pads the sequence so lexicographic range order
// matches numeric order for partial-composite-key iteration.
func loanHistCompositeKey(stub shim.ChaincodeStubInterface, originalDocumentUid string, seq int) (string, error) {
	return stub.CreateCompositeKey(loanHistObjectType, []string{originalDocumentUid, fmt.Sprintf("%012d", seq)})
}

// ---------------------------------------------------------------------------
// Anchor record (UID~)
// ---------------------------------------------------------------------------

func loadAnchor(ctx contractapi.TransactionContextInterface, certifiedCopyUid string) (*AnchorRecord, bool, error) {
	key, err := uidCompositeKey(ctx.GetStub(), certifiedCopyUid)
	if err != nil {
		return nil, false, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read anchor record: %v", err)
	}
	if data == nil {
		return nil, false, nil
	}
	var record AnchorRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal anchor record: %v", err)
	}
	return &record, true, nil
}

func saveAnchor(ctx contractapi.TransactionContextInterface, record *AnchorRecord) error {
	key, err := uidCompositeKey(ctx.GetStub(), record.CertifiedCopyUid)
	if err != nil {
		return err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal anchor record: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

// ---------------------------------------------------------------------------
// Document pointer (DOC~)
// ---------------------------------------------------------------------------

func loadDocPointer(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (*DocPointer, bool, error) {
	key, err := docCompositeKey(ctx.GetStub(), originalDocumentUid)
	if err != nil {
		return nil, false, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read doc pointer: %v", err)
	}
	if data == nil {
		return nil, false, nil
	}
	var pointer DocPointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal doc pointer: %v", err)
	}
	return &pointer, true, nil
}

func saveDocPointer(ctx contractapi.TransactionContextInterface, pointer *DocPointer) error {
	key, err := docCompositeKey(ctx.GetStub(), pointer.OriginalDocumentUid)
	if err != nil {
		return err
	}
	b, err := json.Marshal(pointer)
	if err != nil {
		return fmt.Errorf("failed to marshal doc pointer: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

// ---------------------------------------------------------------------------
// Loan state (LOAN~)
// ---------------------------------------------------------------------------

func loadLoanState(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (*LoanState, bool, error) {
	key, err := loanCompositeKey(ctx.GetStub(), originalDocumentUid)
	if err != nil {
		return nil, false, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read loan state: %v", err)
	}
	if data == nil {
		return nil, false, nil
	}
	var state LoanState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal loan state: %v", err)
	}
	return &state, true, nil
}

func saveLoanState(ctx contractapi.TransactionContextInterface, state *LoanState) error {
	key, err := loanCompositeKey(ctx.GetStub(), state.OriginalDocumentUid)
	if err != nil {
		return err
	}
	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal loan state: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

// ---------------------------------------------------------------------------
// Loan history (LOANHIST~)
// ---------------------------------------------------------------------------

// appendLoanHistory writes one transition entry keyed by its sequence number.
func appendLoanHistory(ctx contractapi.TransactionContextInterface, originalDocumentUid string, entry *LoanHistoryEntry) error {
	key, err := loanHistCompositeKey(ctx.GetStub(), originalDocumentUid, entry.Seq)
	if err != nil {
		return err
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal loan history entry: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

// iterateLoanHistory returns all transition entries for a document in ascending
// sequence order (guaranteed by the zero-padded key suffix).
func iterateLoanHistory(ctx contractapi.TransactionContextInterface, originalDocumentUid string) ([]LoanHistoryEntry, error) {
	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(loanHistObjectType, []string{originalDocumentUid})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate loan history: %v", err)
	}
	defer iter.Close()

	entries := []LoanHistoryEntry{}
	for iter.HasNext() {
		kv, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read loan history entry: %v", err)
		}
		var entry LoanHistoryEntry
		if err := json.Unmarshal(kv.Value, &entry); err != nil {
			return nil, fmt.Errorf("failed to unmarshal loan history entry: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
