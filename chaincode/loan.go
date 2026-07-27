package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// currentLoanStatus returns the current loan status for a document, treating an
// absent LOAN~ key as NOT_FOUND.
func currentLoanStatus(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (string, error) {
	state, exists, err := loadLoanState(ctx, originalDocumentUid)
	if err != nil {
		return "", err
	}
	if !exists {
		return loanStatusNotFound, nil
	}
	return state.Status, nil
}

// nextLoanSeq returns the next 1-based loan-history sequence number.
func nextLoanSeq(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (int, error) {
	entries, err := iterateLoanHistory(ctx, originalDocumentUid)
	if err != nil {
		return 0, err
	}
	return len(entries) + 1, nil
}

// RecordNOIApproval records SRO approval of an NOI, setting the loan to
// ACTIVE_LOAN and appending a history entry.
//
// Guard (MA-26): rejects if the loan is already ACTIVE_LOAN.
func (cc *IgrAnchorChaincode) RecordNOIApproval(ctx contractapi.TransactionContextInterface, payloadJSON string) (*LoanWriteResponse, error) {
	var p NOIApprovalPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, fmt.Errorf("invalid NOI approval payload: %v", err)
	}

	if err := requireWriterMSP(ctx); err != nil {
		return nil, err
	}
	if err := validateOriginalDocumentUid(p.OriginalDocumentUid); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("noiApplicationId", p.NoiApplicationId); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("bankId", p.BankId); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("approvedBySroRef", p.ApprovedBySroRef); err != nil {
		return nil, err
	}
	if p.LoanState != "" && p.LoanState != loanStatusActive {
		return nil, fmt.Errorf("loanState must be %s for NOI approval", loanStatusActive)
	}
	if p.NoiDocumentHash != "" {
		if _, err := validateSha256Hash(p.NoiDocumentHash); err != nil {
			return nil, fmt.Errorf("noiDocumentHash: %v", err)
		}
	}
	for _, c := range []struct {
		field string
		val   string
		max   int
	}{
		{"noiApplicationId", p.NoiApplicationId, maxNoiAppIdLen},
		{"bankId", p.BankId, maxBankIdLen},
		{"approvedBySroRef", p.ApprovedBySroRef, maxRefLen},
		{"approvalTimestamp", p.ApprovalTimestamp, maxTimestampLen},
	} {
		if err := checkLen(c.field, c.val, c.max); err != nil {
			return nil, err
		}
	}

	fromState, err := currentLoanStatus(ctx, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	if fromState == loanStatusActive {
		return nil, fmt.Errorf("loan for %q is already ACTIVE_LOAN", p.OriginalDocumentUid)
	}

	now, err := txTimestampRFC3339(ctx)
	if err != nil {
		return nil, err
	}
	txID := ctx.GetStub().GetTxID()

	state := &LoanState{
		OriginalDocumentUid: p.OriginalDocumentUid,
		Status:              loanStatusActive,
		NoiApplicationId:    p.NoiApplicationId,
		BankId:              p.BankId,
		ApprovedBySroRef:    p.ApprovedBySroRef,
		UpdatedAt:           now,
		LatestTxId:          txID,
	}
	if err := saveLoanState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to store loan state: %v", err)
	}

	seq, err := nextLoanSeq(ctx, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	entry := &LoanHistoryEntry{
		FromState:        fromState,
		ToState:          loanStatusActive,
		NoiApplicationId: p.NoiApplicationId,
		BankId:           p.BankId,
		ApprovedBySroRef: p.ApprovedBySroRef,
		Timestamp:        now,
		TxnId:            txID,
		Seq:              seq,
	}
	if err := appendLoanHistory(ctx, p.OriginalDocumentUid, entry); err != nil {
		return nil, fmt.Errorf("failed to append loan history: %v", err)
	}

	emitWriteEvent(ctx, eventNOIApproved, p.OriginalDocumentUid, "", txID)

	return &LoanWriteResponse{
		Status:              loanStatusActive,
		OriginalDocumentUid: p.OriginalDocumentUid,
		TxnId:               txID,
	}, nil
}

// RecordLoanSatisfaction records a bank NOC / loan finish (no SRO validation),
// setting the loan to NO_LOAN and appending a history entry.
//
// Guard (MA-26): rejects unless the loan is currently ACTIVE_LOAN.
func (cc *IgrAnchorChaincode) RecordLoanSatisfaction(ctx contractapi.TransactionContextInterface, payloadJSON string) (*LoanWriteResponse, error) {
	var p LoanFinishPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, fmt.Errorf("invalid loan finish payload: %v", err)
	}

	if err := requireWriterMSP(ctx); err != nil {
		return nil, err
	}
	if err := validateOriginalDocumentUid(p.OriginalDocumentUid); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("noiApplicationId", p.NoiApplicationId); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("bankId", p.BankId); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("nocReferenceId", p.NocReferenceId); err != nil {
		return nil, err
	}
	if p.LoanState != "" && p.LoanState != loanStatusNoLoan {
		return nil, fmt.Errorf("loanState must be %s for loan finish", loanStatusNoLoan)
	}
	for _, c := range []struct {
		field string
		val   string
		max   int
	}{
		{"noiApplicationId", p.NoiApplicationId, maxNoiAppIdLen},
		{"bankId", p.BankId, maxBankIdLen},
		{"nocReferenceId", p.NocReferenceId, maxNocRefLen},
		{"submittedTimestamp", p.SubmittedTimestamp, maxTimestampLen},
	} {
		if err := checkLen(c.field, c.val, c.max); err != nil {
			return nil, err
		}
	}

	fromState, err := currentLoanStatus(ctx, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	if fromState != loanStatusActive {
		return nil, fmt.Errorf("loan for %q is not ACTIVE_LOAN (current: %s); cannot record satisfaction", p.OriginalDocumentUid, fromState)
	}

	now, err := txTimestampRFC3339(ctx)
	if err != nil {
		return nil, err
	}
	txID := ctx.GetStub().GetTxID()

	state := &LoanState{
		OriginalDocumentUid: p.OriginalDocumentUid,
		Status:              loanStatusNoLoan,
		NoiApplicationId:    p.NoiApplicationId,
		BankId:              p.BankId,
		NocReferenceId:      p.NocReferenceId,
		UpdatedAt:           now,
		LatestTxId:          txID,
	}
	if err := saveLoanState(ctx, state); err != nil {
		return nil, fmt.Errorf("failed to store loan state: %v", err)
	}

	seq, err := nextLoanSeq(ctx, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	entry := &LoanHistoryEntry{
		FromState:        fromState,
		ToState:          loanStatusNoLoan,
		NoiApplicationId: p.NoiApplicationId,
		BankId:           p.BankId,
		NocReferenceId:   p.NocReferenceId,
		Timestamp:        now,
		TxnId:            txID,
		Seq:              seq,
	}
	if err := appendLoanHistory(ctx, p.OriginalDocumentUid, entry); err != nil {
		return nil, fmt.Errorf("failed to append loan history: %v", err)
	}

	emitWriteEvent(ctx, eventLoanSatisfied, p.OriginalDocumentUid, "", txID)

	return &LoanWriteResponse{
		Status:              loanStatusNoLoan,
		OriginalDocumentUid: p.OriginalDocumentUid,
		TxnId:               txID,
	}, nil
}

// GetLoanState returns current loan encumbrance: ACTIVE_LOAN, NO_LOAN, or
// NOT_FOUND (absent key = never had a loan).
func (cc *IgrAnchorChaincode) GetLoanState(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (*LoanState, error) {
	if err := validateOriginalDocumentUid(originalDocumentUid); err != nil {
		return nil, err
	}
	state, exists, err := loadLoanState(ctx, originalDocumentUid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &LoanState{OriginalDocumentUid: originalDocumentUid, Status: loanStatusNotFound}, nil
	}
	return state, nil
}

// GetLoanHistory returns the ordered list of loan state transitions.
func (cc *IgrAnchorChaincode) GetLoanHistory(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (*LoanHistoryResponse, error) {
	if err := validateOriginalDocumentUid(originalDocumentUid); err != nil {
		return nil, err
	}
	entries, err := iterateLoanHistory(ctx, originalDocumentUid)
	if err != nil {
		return nil, err
	}
	return &LoanHistoryResponse{
		OriginalDocumentUid: originalDocumentUid,
		Transitions:         entries,
	}, nil
}
