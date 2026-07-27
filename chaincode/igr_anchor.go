// Package main implements the IGR igr_anchor Hyperledger Fabric chaincode.
//
// It anchors certified-copy document hashes and tracks loan encumbrance state
// using the two-tier identity model from the IGR blockchain knowledge base
// (originalDocumentUid / certifiedCopyUid). See igr-blockchain-docs:
//   - igr-blockchain/05-smart-contract-design.md (authoritative design)
//   - igr-blockchain/04-api-contracts.md + openapi.yaml (route/payload shapes)
//   - adr/002 (identifiers), adr/003 (loan state), adr/004 (hash format)
//
// Migration note (pre-production): this is the v3 model. Deploying it over a
// ledger that used the previous v2 model (UID~...:V*, ACTIVE/RELEASED loan
// states) requires a dev/UAT ledger wipe + CCAAS redeploy — the key layouts
// and record schemas are not backward compatible.
package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

const (
	// Ledger composite-key object types.
	docObjectType      = "DOC"      // DOC~<originalDocumentUid>          -> latest pointer + version counter
	uidObjectType      = "UID"      // UID~<certifiedCopyUid>             -> immutable anchor record
	loanObjectType     = "LOAN"     // LOAN~<originalDocumentUid>         -> current loan state
	loanHistObjectType = "LOANHIST" // LOANHIST~<originalDocumentUid>~seq -> append-only loan transitions

	// writerMSPID is the single authorized writer identity (the blockchain
	// service / government MSP). The SRO-vs-bank business distinction is
	// expressed via separate functions + state guards + payload fields, not
	// via different Fabric MSP roles (KB 05 §RBAC).
	writerMSPID = "IGRPrimaryMSP"

	// Loan states (ADR 003 v3).
	loanStatusActive   = "ACTIVE_LOAN"
	loanStatusNoLoan   = "NO_LOAN"
	loanStatusNotFound = "NOT_FOUND"

	// Verify result states (PENDING_ANCHOR is an API/DB concern, not on-chain).
	verifyStatusMatch    = "MATCH"
	verifyStatusMismatch = "MISMATCH"
	verifyStatusNotFound = "NOT_FOUND"

	// Anchor write status.
	anchorStatusAnchored = "ANCHORED"

	// Write event types (KB 05 §Events).
	eventCertifiedCopyAnchored = "CERTIFIED_COPY_ANCHORED"
	eventNOIApproved           = "NOI_APPROVED"
	eventLoanSatisfied         = "LOAN_SATISFIED"

	// Field length caps (defensive limits on opaque strings written to ledger).
	maxRequestIdLen    = 256
	maxRefLen          = 256
	maxBankIdLen       = 64
	maxDocumentTypeLen = 64
	maxDistrictLen     = 64
	maxSroOfficeLen    = 32
	maxSroIdLen        = 32
	maxSignatureVerLen = 32
	maxDocumentNoLen   = 64
	maxTimestampLen    = 64
	maxNoiAppIdLen     = 128
	maxNocRefLen       = 128
)

// IgrAnchorChaincode is the smart contract. Every exported method is a
// transaction: writes (AnchorCertifiedCopy, RecordNOIApproval,
// RecordLoanSatisfaction) require the writer MSP; reads are evaluate-only.
type IgrAnchorChaincode struct {
	contractapi.Contract
}

func main() {
	chaincode, err := contractapi.NewChaincode(&IgrAnchorChaincode{})
	if err != nil {
		log.Panicf("Error creating igr_anchor chaincode: %v", err)
	}
	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting igr_anchor chaincode: %v", err)
	}
}
