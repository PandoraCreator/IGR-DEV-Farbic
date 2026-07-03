package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

const (
	docObjectType  = "DOC"
	uidObjectType  = "UID"
	loanObjectType = "LOAN"

	sroIDAttribute = "sroId"

	loanStatusActive   = "ACTIVE"
	loanStatusReleased = "RELEASED"
	loanStatusNotFound = "NOT_FOUND"

	maxReferenceIDLen   = 256
	maxSignerRefLen     = 256
	maxLoanIDLen        = 128
	maxBankCodeLen      = 64
	maxReleaseReasonLen = 512
)

var (
	docRefPattern = regexp.MustCompile(`^MH:[A-Za-z0-9_]+:SR[0-9]+:[0-9]{4}:[0-9]+$`)
	uidPattern    = regexp.MustCompile(`^MH:[A-Za-z0-9_]+:SR[0-9]+:[0-9]{4}:[0-9]+:V[0-9]+$`)
	sroNumPattern = regexp.MustCompile(`:SR([0-9]+):`)
	uidVerPattern = regexp.MustCompile(`:V([0-9]+)$`)
)

type AnchorRecord struct {
	Uid         string `json:"uid"`
	DocRef      string `json:"docRef"`
	PdfHash     string `json:"pdfHash"`
	ReferenceId string `json:"referenceId"`
	SignerRef   string `json:"signerRef,omitempty"`
	AnchoredAt  int64  `json:"anchoredAt"`
	TxnId       string `json:"txnId"`
}

type DocPointer struct {
	DocRef     string `json:"docRef"`
	LatestUid  string `json:"latestUid"`
	Version    int    `json:"version"`
	UpdatedAt  int64  `json:"updatedAt"`
	LatestTxId string `json:"latestTxId"`
}

type VerifyResult struct {
	Status       string `json:"status"` // MATCH|MISMATCH|NOT_FOUND
	AnchoredHash string `json:"anchoredHash,omitempty"`
	TxId         string `json:"txId,omitempty"`
}

type LoanState struct {
	DocRef         string `json:"docRef"`
	Status         string `json:"status"` // ACTIVE|RELEASED|NOT_FOUND
	LoanId         string `json:"loanId,omitempty"`
	BankCode       string `json:"bankCode,omitempty"`
	ActivatedByUid string `json:"activatedByUid,omitempty"`
	UpdatedAt      int64  `json:"updatedAt,omitempty"`
	LatestTxId     string `json:"latestTxId,omitempty"`
}

type IgrAnchorChaincode struct {
	contractapi.Contract
}

func txTimestampMillis(ctx contractapi.TransactionContextInterface) (int64, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return 0, fmt.Errorf("failed to get tx timestamp: %v", err)
	}
	return ts.GetSeconds()*1000 + int64(ts.GetNanos()/1_000_000), nil
}

func getCallerSROId(ctx contractapi.TransactionContextInterface) (string, error) {
	val, found, err := ctx.GetClientIdentity().GetAttributeValue(sroIDAttribute)
	if err != nil {
		return "", fmt.Errorf("failed to read caller %q attribute: %v", sroIDAttribute, err)
	}
	if !found || val == "" {
		return "", fmt.Errorf("caller missing required certificate attribute %q", sroIDAttribute)
	}
	return val, nil
}

func getMSPID(ctx contractapi.TransactionContextInterface) (string, error) {
	mspID, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil {
		return "", fmt.Errorf("failed to get MSP ID: %v", err)
	}
	return mspID, nil
}

func requireMSP(mspID string, allowed ...string) error {
	for _, a := range allowed {
		if mspID == a {
			return nil
		}
	}
	return fmt.Errorf("MSP %s is not authorized for this operation", mspID)
}

func parseDocRef(docRef string) (string, error) {
	m := sroNumPattern.FindStringSubmatch(docRef)
	if m == nil {
		return "", fmt.Errorf("docRef %q has invalid format", docRef)
	}
	return m[1], nil
}

func parseUidVersion(uid string) (int, error) {
	m := uidVerPattern.FindStringSubmatch(uid)
	if m == nil {
		return 0, fmt.Errorf("uid %q has invalid version suffix", uid)
	}
	version, err := strconv.Atoi(m[1])
	if err != nil || version < 1 {
		return 0, fmt.Errorf("uid %q has invalid version suffix", uid)
	}
	return version, nil
}

func validateDocRef(docRef string) error {
	if docRef == "" {
		return fmt.Errorf("docRef cannot be empty")
	}
	if !docRefPattern.MatchString(docRef) {
		return fmt.Errorf("docRef %q has invalid format", docRef)
	}
	return nil
}

func validateUid(uid, docRef string) error {
	if uid == "" {
		return fmt.Errorf("uid cannot be empty")
	}
	if !uidPattern.MatchString(uid) {
		return fmt.Errorf("uid %q has invalid format", uid)
	}
	expectedPrefix := docRef + ":V"
	if !strings.HasPrefix(uid, expectedPrefix) {
		return fmt.Errorf("uid %q does not match docRef %q", uid, docRef)
	}
	version, err := parseUidVersion(uid)
	if err != nil {
		return err
	}
	expectedUid := fmt.Sprintf("%s%d", expectedPrefix, version)
	if uid != expectedUid {
		return fmt.Errorf("uid %q does not match docRef %q version", uid, docRef)
	}
	return nil
}

func validatePdfHash(hash string) (string, error) {
	if hash == "" {
		return "", fmt.Errorf("pdfHash cannot be empty")
	}
	if strings.ContainsAny(hash, "+/=") {
		return "", fmt.Errorf("pdfHash must be hex, not base64")
	}
	if strings.Contains(hash, "/") || strings.Contains(hash, "\\") {
		return "", fmt.Errorf("pdfHash must not be a file path")
	}

	raw := hash
	if strings.HasPrefix(strings.ToLower(hash), "sha256:") {
		raw = hash[len("sha256:"):]
	}
	if len(raw) != 64 {
		return "", fmt.Errorf("pdfHash must be 64 hex characters")
	}
	for _, c := range raw {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", fmt.Errorf("pdfHash contains non-hex characters")
		}
	}
	return "sha256:" + strings.ToLower(raw), nil
}

func docCompositeKey(stub shim.ChaincodeStubInterface, docRef string) (string, error) {
	return stub.CreateCompositeKey(docObjectType, []string{docRef})
}

func uidCompositeKey(stub shim.ChaincodeStubInterface, uid string) (string, error) {
	return stub.CreateCompositeKey(uidObjectType, []string{uid})
}

func loanCompositeKey(stub shim.ChaincodeStubInterface, docRef string) (string, error) {
	return stub.CreateCompositeKey(loanObjectType, []string{docRef})
}

func validateLoanFieldLengths(loanId, bankCode, reason string) error {
	if len(loanId) > maxLoanIDLen {
		return fmt.Errorf("loanId exceeds maximum length %d", maxLoanIDLen)
	}
	if len(bankCode) > maxBankCodeLen {
		return fmt.Errorf("bankCode exceeds maximum length %d", maxBankCodeLen)
	}
	if len(reason) > maxReleaseReasonLen {
		return fmt.Errorf("reason exceeds maximum length %d", maxReleaseReasonLen)
	}
	return nil
}

func requireLoanWriteMSP(ctx contractapi.TransactionContextInterface) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	return requireMSP(mspID, "IGRBankMSP", "IGRPrimaryMSP")
}

func loadLoanState(ctx contractapi.TransactionContextInterface, docRef string) (*LoanState, bool, error) {
	key, err := loanCompositeKey(ctx.GetStub(), docRef)
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
	key, err := loanCompositeKey(ctx.GetStub(), state.DocRef)
	if err != nil {
		return err
	}
	b, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal loan state: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

func loadDocPointer(ctx contractapi.TransactionContextInterface, docRef string) (*DocPointer, bool, error) {
	key, err := docCompositeKey(ctx.GetStub(), docRef)
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
	key, err := docCompositeKey(ctx.GetStub(), pointer.DocRef)
	if err != nil {
		return err
	}
	b, err := json.Marshal(pointer)
	if err != nil {
		return fmt.Errorf("failed to marshal doc pointer: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

func loadAnchor(ctx contractapi.TransactionContextInterface, uid string) (*AnchorRecord, bool, error) {
	key, err := uidCompositeKey(ctx.GetStub(), uid)
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
	key, err := uidCompositeKey(ctx.GetStub(), record.Uid)
	if err != nil {
		return err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal anchor record: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

func requirePrimarySROMatch(ctx contractapi.TransactionContextInterface, docRef string) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if err := requireMSP(mspID, "IGRPrimaryMSP"); err != nil {
		return err
	}
	docSRO, err := parseDocRef(docRef)
	if err != nil {
		return err
	}
	callerSRO, err := getCallerSROId(ctx)
	if err != nil {
		return err
	}
	if callerSRO != docSRO {
		return fmt.Errorf("caller sroId %q does not match document SRO %q in docRef", callerSRO, docSRO)
	}
	return nil
}

func emitAnchorCreated(ctx contractapi.TransactionContextInterface, uid, docRef, txnId string) {
	event := map[string]string{
		"uid":    uid,
		"docRef": docRef,
		"txnId":  txnId,
	}
	eventJSON, _ := json.Marshal(event)
	_ = ctx.GetStub().SetEvent("AnchorCreated", eventJSON)
}

// CreateAnchorVersion anchors an immutable document version on the ledger.
func (cc *IgrAnchorChaincode) CreateAnchorVersion(
	ctx contractapi.TransactionContextInterface,
	uid, docRef, pdfHash, referenceId, signerMeta string,
) (string, error) {
	if err := validateDocRef(docRef); err != nil {
		return "", err
	}
	if err := validateUid(uid, docRef); err != nil {
		return "", err
	}
	normalizedHash, err := validatePdfHash(pdfHash)
	if err != nil {
		return "", err
	}
	if len(referenceId) > maxReferenceIDLen {
		return "", fmt.Errorf("referenceId exceeds maximum length %d", maxReferenceIDLen)
	}
	if len(signerMeta) > maxSignerRefLen {
		return "", fmt.Errorf("signerMeta exceeds maximum length %d", maxSignerRefLen)
	}
	if err := requirePrimarySROMatch(ctx, docRef); err != nil {
		return "", err
	}

	existing, exists, err := loadAnchor(ctx, uid)
	if err != nil {
		return "", err
	}
	if exists {
		if existing.PdfHash == normalizedHash {
			return existing.TxnId, nil
		}
		return "", fmt.Errorf("uid %q already anchored with different hash", uid)
	}

	version, err := parseUidVersion(uid)
	if err != nil {
		return "", err
	}

	now, err := txTimestampMillis(ctx)
	if err != nil {
		return "", err
	}
	txID := ctx.GetStub().GetTxID()

	record := &AnchorRecord{
		Uid:         uid,
		DocRef:      docRef,
		PdfHash:     normalizedHash,
		ReferenceId: referenceId,
		SignerRef:   signerMeta,
		AnchoredAt:  now,
		TxnId:       txID,
	}
	if err := saveAnchor(ctx, record); err != nil {
		return "", fmt.Errorf("failed to store anchor record: %v", err)
	}

	pointer := &DocPointer{
		DocRef:     docRef,
		LatestUid:  uid,
		Version:    version,
		UpdatedAt:  now,
		LatestTxId: txID,
	}
	if err := saveDocPointer(ctx, pointer); err != nil {
		return "", fmt.Errorf("failed to store doc pointer: %v", err)
	}

	emitAnchorCreated(ctx, uid, docRef, txID)
	return txID, nil
}

// GetLatestAnchor returns the latest anchored version for a document.
func (cc *IgrAnchorChaincode) GetLatestAnchor(ctx contractapi.TransactionContextInterface, docRef string) (*AnchorRecord, error) {
	if err := validateDocRef(docRef); err != nil {
		return nil, err
	}
	pointer, exists, err := loadDocPointer(ctx, docRef)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("docRef %q does not exist", docRef)
	}
	record, exists, err := loadAnchor(ctx, pointer.LatestUid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("latest anchor for docRef %q does not exist", docRef)
	}
	return record, nil
}

// GetAnchorByUid returns one immutable anchor record.
func (cc *IgrAnchorChaincode) GetAnchorByUid(ctx contractapi.TransactionContextInterface, uid string) (*AnchorRecord, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid cannot be empty")
	}
	if !uidPattern.MatchString(uid) {
		return nil, fmt.Errorf("uid %q has invalid format", uid)
	}
	record, exists, err := loadAnchor(ctx, uid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("uid %q does not exist", uid)
	}
	return record, nil
}

// VerifyDocHash compares the provided hash against the stored anchor for a uid.
func (cc *IgrAnchorChaincode) VerifyDocHash(ctx contractapi.TransactionContextInterface, uid, providedHash string) (*VerifyResult, error) {
	if uid == "" {
		return nil, fmt.Errorf("uid cannot be empty")
	}
	if !uidPattern.MatchString(uid) {
		return nil, fmt.Errorf("uid %q has invalid format", uid)
	}
	normalizedHash, err := validatePdfHash(providedHash)
	if err != nil {
		return nil, err
	}
	record, exists, err := loadAnchor(ctx, uid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &VerifyResult{Status: "NOT_FOUND"}, nil
	}
	if record.PdfHash == normalizedHash {
		return &VerifyResult{
			Status:       "MATCH",
			AnchoredHash: record.PdfHash,
			TxId:         record.TxnId,
		}, nil
	}
	return &VerifyResult{
		Status:       "MISMATCH",
		AnchoredHash: record.PdfHash,
		TxId:         record.TxnId,
	}, nil
}

// GetLoanState returns loan encumbrance for a docRef (ACTIVE, RELEASED, or NOT_FOUND).
func (cc *IgrAnchorChaincode) GetLoanState(ctx contractapi.TransactionContextInterface, docRef string) (*LoanState, error) {
	if err := validateDocRef(docRef); err != nil {
		return nil, err
	}
	state, exists, err := loadLoanState(ctx, docRef)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &LoanState{DocRef: docRef, Status: loanStatusNotFound}, nil
	}
	return state, nil
}

// MarkLoanActive sets LOAN~<docRef> to ACTIVE.
func (cc *IgrAnchorChaincode) MarkLoanActive(
	ctx contractapi.TransactionContextInterface,
	docRef, loanId, bankCode, activatedByUid string,
) error {
	if err := validateDocRef(docRef); err != nil {
		return err
	}
	if loanId == "" {
		return fmt.Errorf("loanId cannot be empty")
	}
	if bankCode == "" {
		return fmt.Errorf("bankCode cannot be empty")
	}
	if err := validateUid(activatedByUid, docRef); err != nil {
		return err
	}
	if err := validateLoanFieldLengths(loanId, bankCode, ""); err != nil {
		return err
	}
	if err := requireLoanWriteMSP(ctx); err != nil {
		return err
	}

	anchor, anchorExists, err := loadAnchor(ctx, activatedByUid)
	if err != nil {
		return err
	}
	if !anchorExists || anchor.DocRef != docRef {
		return fmt.Errorf("activatedByUid %q is not anchored for docRef %q", activatedByUid, docRef)
	}

	existing, exists, err := loadLoanState(ctx, docRef)
	if err != nil {
		return err
	}
	if exists && existing.Status == loanStatusActive {
		if existing.LoanId == loanId {
			return nil
		}
		return fmt.Errorf("loan for docRef %q is already ACTIVE with loanId %q", docRef, existing.LoanId)
	}

	now, err := txTimestampMillis(ctx)
	if err != nil {
		return err
	}
	txID := ctx.GetStub().GetTxID()

	state := &LoanState{
		DocRef:         docRef,
		Status:         loanStatusActive,
		LoanId:         loanId,
		BankCode:       bankCode,
		ActivatedByUid: activatedByUid,
		UpdatedAt:      now,
		LatestTxId:     txID,
	}
	if err := saveLoanState(ctx, state); err != nil {
		return fmt.Errorf("failed to store loan state: %v", err)
	}
	return nil
}

// ReleaseLoan sets LOAN~<docRef> to RELEASED.
func (cc *IgrAnchorChaincode) ReleaseLoan(
	ctx contractapi.TransactionContextInterface,
	docRef, loanId, bankCode, reason string,
) error {
	if err := validateDocRef(docRef); err != nil {
		return err
	}
	if loanId == "" {
		return fmt.Errorf("loanId cannot be empty")
	}
	if bankCode == "" {
		return fmt.Errorf("bankCode cannot be empty")
	}
	if err := validateLoanFieldLengths(loanId, bankCode, reason); err != nil {
		return err
	}
	if err := requireLoanWriteMSP(ctx); err != nil {
		return err
	}

	existing, exists, err := loadLoanState(ctx, docRef)
	if err != nil {
		return err
	}
	if !exists || existing.Status == loanStatusNotFound {
		return fmt.Errorf("loan for docRef %q does not exist", docRef)
	}
	if existing.Status == loanStatusReleased {
		if existing.LoanId == loanId {
			return nil
		}
		return fmt.Errorf("loan for docRef %q is already RELEASED", docRef)
	}
	if existing.LoanId != loanId {
		return fmt.Errorf("loanId %q does not match active loan %q for docRef %q", loanId, existing.LoanId, docRef)
	}
	if existing.BankCode != bankCode {
		return fmt.Errorf("bankCode %q does not match active loan bank %q for docRef %q", bankCode, existing.BankCode, docRef)
	}

	now, err := txTimestampMillis(ctx)
	if err != nil {
		return err
	}
	txID := ctx.GetStub().GetTxID()

	existing.Status = loanStatusReleased
	existing.UpdatedAt = now
	existing.LatestTxId = txID
	if err := saveLoanState(ctx, existing); err != nil {
		return fmt.Errorf("failed to store loan state: %v", err)
	}
	return nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&IgrAnchorChaincode{})
	if err != nil {
		log.Panicf("Error creating igr anchor chaincode: %v", err)
	}
	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting igr anchor chaincode: %v", err)
	}
}
