package main

// ---------------------------------------------------------------------------
// Ledger records
// ---------------------------------------------------------------------------

// AnchorRecord is the immutable per-issuance certified-copy anchor stored at
// UID~<certifiedCopyUid>. Field set matches igr-blockchain/05-smart-contract-design.md.
type AnchorRecord struct {
	OriginalDocumentUid  string `json:"originalDocumentUid"`
	CertifiedCopyUid     string `json:"certifiedCopyUid"`
	CertifiedCopyVersion int    `json:"certifiedCopyVersion"`
	DocumentNo           string `json:"documentNo,omitempty" metadata:",optional"`
	RegistrationYear     int    `json:"registrationYear,omitempty" metadata:",optional"`
	District             string `json:"district,omitempty" metadata:",optional"`
	SroOffice            string `json:"sroOffice,omitempty" metadata:",optional"`
	DocumentType         string `json:"documentType,omitempty" metadata:",optional"`
	FinalSignedPdfHash   string `json:"finalSignedPdfHash"`
	SignatureHash        string `json:"signatureHash"`
	SignatureVersion     string `json:"signatureVersion"`
	RequestId            string `json:"requestId"`
	SroId                string `json:"sroId,omitempty" metadata:",optional"`
	SigningAuthorityRef  string `json:"signingAuthorityRef,omitempty" metadata:",optional"`
	SigningTimestamp     string `json:"signingTimestamp"`
	AnchorTimestamp      string `json:"anchorTimestamp"`
	PreviousVersionRef   string `json:"previousVersionRef,omitempty" metadata:",optional"`
	TxnId                string `json:"txnId"`
}

// DocPointer is stored at DOC~<originalDocumentUid> and tracks the latest
// certified-copy issuance and the authoritative version counter.
type DocPointer struct {
	OriginalDocumentUid    string `json:"originalDocumentUid"`
	LatestCertifiedCopyUid string `json:"latestCertifiedCopyUid"`
	VersionCount           int    `json:"versionCount"`
	UpdatedAt              string `json:"updatedAt"`
}

// LoanState is stored at LOAN~<originalDocumentUid>. Field set matches KB 05.
type LoanState struct {
	OriginalDocumentUid string `json:"originalDocumentUid"`
	Status              string `json:"status"`
	NoiApplicationId    string `json:"noiApplicationId,omitempty" metadata:",optional"`
	BankId              string `json:"bankId,omitempty" metadata:",optional"`
	ApprovedBySroRef    string `json:"approvedBySroRef,omitempty" metadata:",optional"`
	NocReferenceId      string `json:"nocReferenceId,omitempty" metadata:",optional"`
	UpdatedAt           string `json:"updatedAt,omitempty" metadata:",optional"`
	LatestTxId          string `json:"latestTxId,omitempty" metadata:",optional"`
}

// LoanHistoryEntry is an append-only transition stored at
// LOANHIST~<originalDocumentUid>~<seq>.
type LoanHistoryEntry struct {
	FromState        string `json:"fromState"`
	ToState          string `json:"toState"`
	NoiApplicationId string `json:"noiApplicationId,omitempty" metadata:",optional"`
	BankId           string `json:"bankId,omitempty" metadata:",optional"`
	ApprovedBySroRef string `json:"approvedBySroRef,omitempty" metadata:",optional"`
	NocReferenceId   string `json:"nocReferenceId,omitempty" metadata:",optional"`
	Timestamp        string `json:"timestamp"`
	TxnId            string `json:"txnId"`
	Seq              int    `json:"seq"`
}

// ---------------------------------------------------------------------------
// Write payloads (unmarshalled from the single JSON-string transaction arg)
// ---------------------------------------------------------------------------

// CertifiedCopyAnchorPayload is the body of anchorCertifiedCopy.
type CertifiedCopyAnchorPayload struct {
	OriginalDocumentUid  string  `json:"originalDocumentUid"`
	CertifiedCopyUid     string  `json:"certifiedCopyUid"`
	CertifiedCopyVersion int     `json:"certifiedCopyVersion"`
	DocumentNo           string  `json:"documentNo"`
	RegistrationYear     int     `json:"registrationYear"`
	District             string  `json:"district"`
	SroOffice            string  `json:"sroOffice"`
	DocumentType         string  `json:"documentType"`
	FinalSignedPdfHash   string  `json:"finalSignedPdfHash"`
	SignatureHash        string  `json:"signatureHash"`
	SignatureVersion     string  `json:"signatureVersion"`
	RequestId            string  `json:"requestId"`
	SroId                string  `json:"sroId"`
	SigningAuthorityRef  string  `json:"signingAuthorityRef"`
	SigningTimestamp     string  `json:"signingTimestamp"`
	PreviousVersionRef   *string `json:"previousVersionRef"`
}

// NOIApprovalPayload is the body of recordNOIApproval (SRO-approved semantics).
type NOIApprovalPayload struct {
	OriginalDocumentUid  string `json:"originalDocumentUid"`
	NoiApplicationId     string `json:"noiApplicationId"`
	BankId               string `json:"bankId"`
	LoanState            string `json:"loanState"`
	ApprovedBySroRef     string `json:"approvedBySroRef"`
	ApprovalTimestamp    string `json:"approvalTimestamp"`
	NoiDocumentHash      string `json:"noiDocumentHash"`
	TransactionTimestamp string `json:"transactionTimestamp"`
}

// LoanFinishPayload is the body of recordLoanSatisfaction (bank NOC, no SRO).
type LoanFinishPayload struct {
	OriginalDocumentUid  string `json:"originalDocumentUid"`
	NoiApplicationId     string `json:"noiApplicationId"`
	BankId               string `json:"bankId"`
	LoanState            string `json:"loanState"`
	NocReferenceId       string `json:"nocReferenceId"`
	SubmittedTimestamp   string `json:"submittedTimestamp"`
	TransactionTimestamp string `json:"transactionTimestamp"`
}

// ---------------------------------------------------------------------------
// Read/write responses
// ---------------------------------------------------------------------------

// AnchorResponse is returned by anchorCertifiedCopy.
type AnchorResponse struct {
	Status           string `json:"status"`
	CertifiedCopyUid string `json:"certifiedCopyUid"`
	TxnId            string `json:"txnId"`
	AnchorTimestamp  string `json:"anchorTimestamp"`
}

// VerifyResult is returned by verifyDocumentHash.
type VerifyResult struct {
	Status                  string `json:"status"`
	MatchedCertifiedCopyUid string `json:"matchedCertifiedCopyUid,omitempty" metadata:",optional"`
	MatchedVersion          int    `json:"matchedVersion,omitempty" metadata:",optional"`
	AnchoredHash            string `json:"anchoredHash,omitempty" metadata:",optional"`
	TxnId                   string `json:"txnId,omitempty" metadata:",optional"`
}

// DocumentVersion is one entry in a document history response.
type DocumentVersion struct {
	CertifiedCopyUid     string `json:"certifiedCopyUid"`
	CertifiedCopyVersion int    `json:"certifiedCopyVersion"`
	FinalSignedPdfHash   string `json:"finalSignedPdfHash"`
	TxnId                string `json:"txnId"`
	AnchorTimestamp      string `json:"anchorTimestamp"`
}

// DocumentHistoryResponse is returned by getDocumentHistory.
type DocumentHistoryResponse struct {
	OriginalDocumentUid    string            `json:"originalDocumentUid"`
	LatestCertifiedCopyUid string            `json:"latestCertifiedCopyUid"`
	Versions               []DocumentVersion `json:"versions"`
}

// LoanHistoryResponse is returned by getLoanHistory.
type LoanHistoryResponse struct {
	OriginalDocumentUid string             `json:"originalDocumentUid"`
	Transitions         []LoanHistoryEntry `json:"transitions"`
}

// LoanWriteResponse is returned by recordNOIApproval / recordLoanSatisfaction.
type LoanWriteResponse struct {
	Status              string `json:"status"`
	OriginalDocumentUid string `json:"originalDocumentUid"`
	TxnId               string `json:"txnId"`
}
