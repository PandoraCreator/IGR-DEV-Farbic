package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

const (
	StateSign         = 1
	StateLoanApproval = 2
	StateNocApproval  = 3

	StatusSign         = "sign"
	StatusLoanApproval = "loan_approval"
	StatusNocApproval  = "noc_approval"

	ActionSign         = "SIGN"
	ActionLoanApproval = "LOAN_APPROVAL"
	ActionNocApproval  = "NOC_APPROVAL"

	docObjectType     = "doc"
	dochistObjectType = "dochist"

	sroIDAttribute = "sroId"
)

var (
	parentDocIDPattern = regexp.MustCompile(`^(\d{4})SRO(\d+)DOC(\d+)$`)
	noiIDPattern       = regexp.MustCompile(`^NoI(\d+)$`)
)

type ParentDocIDParts struct {
	Year   string `json:"year"`
	SRONum string `json:"sroNum"`
	DocNum string `json:"docNum"`
}

type DocMetadata struct {
	StateLabel string `json:"stateLabel"`
	UpdatedAt  int64  `json:"updatedAt"`
	Extra      string `json:"extra,omitempty"`
}

type DocTxRecord struct {
	DocID     string `json:"docId"`
	NoiID     string `json:"noiId"`
	DocHash   string `json:"docHash,omitempty"`
	TxID      string `json:"txId"`
	Action    string `json:"action"`
	State     int    `json:"state"`
	Status    string `json:"status"`
	Metadata  string `json:"metadata"`
	Timestamp int64  `json:"timestamp"`
	MSPID     string `json:"mspId"`
	SROId     string `json:"sroId,omitempty"`
	SignerID  string `json:"signerId,omitempty"`
}

type DocCurrent struct {
	DocID      string `json:"docId"`
	NoiID      string `json:"noiId"`
	DocHash    string `json:"docHash,omitempty"`
	State      int    `json:"state"`
	Status     string `json:"status"`
	OwnerSROId string `json:"ownerSroId,omitempty"`
	Metadata   string `json:"metadata"`
	LatestTxID string `json:"latestTxId"`
	UpdatedAt  int64  `json:"updatedAt"`
	TxCount    int    `json:"txCount"`
}

type DocLatestResponse struct {
	Tx     DocTxRecord `json:"tx"`
	State  int         `json:"state"`
	Status string      `json:"status"`
}

type NoiStatusEntry struct {
	NoiID   string `json:"noiId"`
	DocHash string `json:"docHash,omitempty"`
	State   int    `json:"state"`
	Status  string `json:"status"`
}

type DocNOIOverview struct {
	DocID string           `json:"docId"`
	Nois  []NoiStatusEntry `json:"nois"`
}

type DocRegistryChaincode struct {
	contractapi.Contract
}

func txTimestampMillis(ctx contractapi.TransactionContextInterface) (int64, error) {
	ts, err := ctx.GetStub().GetTxTimestamp()
	if err != nil {
		return 0, fmt.Errorf("failed to get tx timestamp: %v", err)
	}
	return ts.GetSeconds()*1000 + int64(ts.GetNanos()/1_000_000), nil
}

func parseParentDocID(docID string) (*ParentDocIDParts, error) {
	if docID == "" {
		return nil, fmt.Errorf("docID cannot be empty")
	}
	m := parentDocIDPattern.FindStringSubmatch(docID)
	if m == nil {
		return nil, fmt.Errorf("docID %q has invalid format, expected YYYYSRO<number>DOC<number>", docID)
	}
	return &ParentDocIDParts{Year: m[1], SRONum: m[2], DocNum: m[3]}, nil
}

func parseNoiID(noiID string) error {
	if noiID == "" {
		return fmt.Errorf("noiID cannot be empty")
	}
	if !noiIDPattern.MatchString(noiID) {
		return fmt.Errorf("noiID %q has invalid format, expected NoI<number> (e.g. NoI1, NoI2)", noiID)
	}
	return nil
}

func validateDocAndNoi(docID, noiID string) (*ParentDocIDParts, error) {
	parts, err := parseParentDocID(docID)
	if err != nil {
		return nil, err
	}
	if err := parseNoiID(noiID); err != nil {
		return nil, err
	}
	return parts, nil
}

func stateToStatus(state int) string {
	switch state {
	case StateSign:
		return StatusSign
	case StateLoanApproval:
		return StatusLoanApproval
	case StateNocApproval:
		return StatusNocApproval
	default:
		return ""
	}
}

func docCompositeKey(stub shim.ChaincodeStubInterface, docID, noiID string) (string, error) {
	return stub.CreateCompositeKey(docObjectType, []string{docID, noiID})
}

func dochistCompositeKey(stub shim.ChaincodeStubInterface, docID, noiID string, seq int) (string, error) {
	return stub.CreateCompositeKey(dochistObjectType, []string{docID, noiID, fmt.Sprintf("%010d", seq)})
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

func requireCallerSROMatchesParentDoc(ctx contractapi.TransactionContextInterface, docID string) error {
	parts, err := parseParentDocID(docID)
	if err != nil {
		return err
	}
	callerSRO, err := getCallerSROId(ctx)
	if err != nil {
		return err
	}
	if callerSRO != parts.SRONum {
		return fmt.Errorf("caller sroId %q does not match document SRO %q in docID", callerSRO, parts.SRONum)
	}
	return nil
}

func requireCallerSROMatchesNoiOwner(ctx contractapi.TransactionContextInterface, docID, noiID string) error {
	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
	}
	callerSRO, err := getCallerSROId(ctx)
	if err != nil {
		return err
	}
	if callerSRO != current.OwnerSROId {
		return fmt.Errorf("caller sroId %q does not match noi owner SRO %q", callerSRO, current.OwnerSROId)
	}
	return nil
}

func getSignerID(ctx contractapi.TransactionContextInterface) string {
	id, err := ctx.GetClientIdentity().GetID()
	if err != nil {
		return ""
	}
	return id
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

func buildMetadata(stateLabel string, updatedAt int64, extra string) (string, error) {
	meta := DocMetadata{StateLabel: stateLabel, UpdatedAt: updatedAt, Extra: extra}
	b, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %v", err)
	}
	return string(b), nil
}

func loadNoiCurrent(ctx contractapi.TransactionContextInterface, docID, noiID string) (*DocCurrent, bool, error) {
	key, err := docCompositeKey(ctx.GetStub(), docID, noiID)
	if err != nil {
		return nil, false, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read noi state: %v", err)
	}
	if data == nil {
		return nil, false, nil
	}
	var current DocCurrent
	if err := json.Unmarshal(data, &current); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal noi state: %v", err)
	}
	return &current, true, nil
}

func saveNoiCurrent(ctx contractapi.TransactionContextInterface, current *DocCurrent) error {
	key, err := docCompositeKey(ctx.GetStub(), current.DocID, current.NoiID)
	if err != nil {
		return err
	}
	b, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("failed to marshal noi state: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

func appendNoiHistory(ctx contractapi.TransactionContextInterface, record DocTxRecord, seq int) error {
	key, err := dochistCompositeKey(ctx.GetStub(), record.DocID, record.NoiID, seq)
	if err != nil {
		return err
	}
	b, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal tx record: %v", err)
	}
	return ctx.GetStub().PutState(key, b)
}

func emitNoiStateChanged(ctx contractapi.TransactionContextInterface, docID, noiID, action, status string, state int, txID string, timestamp int64) {
	event := map[string]interface{}{
		"eventType": "NoiStateChanged",
		"docId":     docID,
		"noiId":     noiID,
		"action":    action,
		"state":     state,
		"status":    status,
		"txId":      txID,
		"timestamp": timestamp,
	}
	eventJSON, _ := json.Marshal(event)
	_ = ctx.GetStub().SetEvent("NoiStateChanged", eventJSON)
}

type transitionOpts struct {
	isSign                 bool
	expectedState          int
	newState               int
	status                 string
	action                 string
	allowedMSPs            []string
	requireParentSROMatch  bool
}

func (cc *DocRegistryChaincode) recordNoiTransition(
	ctx contractapi.TransactionContextInterface,
	docID, noiID, docHash, extraMetadata string,
	opts transitionOpts,
) error {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return err
	}

	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if err := requireMSP(mspID, opts.allowedMSPs...); err != nil {
		return err
	}
	if opts.requireParentSROMatch {
		if err := requireCallerSROMatchesParentDoc(ctx, docID); err != nil {
			return err
		}
	}

	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return err
	}

	if opts.isSign {
		if exists {
			return fmt.Errorf("noi %s under doc %s already exists", noiID, docID)
		}
		if docHash == "" {
			return fmt.Errorf("docHash cannot be empty when signing noi %s", noiID)
		}
		current = &DocCurrent{DocID: docID, NoiID: noiID, DocHash: docHash, TxCount: 0}
	} else {
		if !exists {
			return fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
		}
		if current.State == StateNocApproval {
			return fmt.Errorf("noi %s under doc %s is in terminal status %q", noiID, docID, StatusNocApproval)
		}
		if current.State != opts.expectedState {
			return fmt.Errorf("noi %s under doc %s is in state %d (%s), expected %d (%s)",
				noiID, docID, current.State, current.Status, opts.expectedState, stateToStatus(opts.expectedState))
		}
	}

	now, err := txTimestampMillis(ctx)
	if err != nil {
		return err
	}
	txID := ctx.GetStub().GetTxID()

	metaStr, err := buildMetadata(opts.status, now, extraMetadata)
	if err != nil {
		return err
	}

	callerSRO := ""
	if opts.requireParentSROMatch || mspID == "IGRPrimaryMSP" {
		callerSRO, _ = getCallerSROId(ctx)
	}

	record := DocTxRecord{
		DocID:     docID,
		NoiID:     noiID,
		DocHash:   current.DocHash,
		TxID:      txID,
		Action:    opts.action,
		State:     opts.newState,
		Status:    opts.status,
		Metadata:  metaStr,
		Timestamp: now,
		MSPID:     mspID,
		SROId:     callerSRO,
		SignerID:  getSignerID(ctx),
	}

	seq := current.TxCount
	if err := appendNoiHistory(ctx, record, seq); err != nil {
		return fmt.Errorf("failed to store history: %v", err)
	}

	current.State = opts.newState
	current.Status = opts.status
	current.Metadata = metaStr
	current.LatestTxID = txID
	current.UpdatedAt = now
	current.TxCount++
	if opts.isSign {
		current.OwnerSROId = callerSRO
	}

	if err := saveNoiCurrent(ctx, current); err != nil {
		return fmt.Errorf("failed to store noi state: %v", err)
	}

	emitNoiStateChanged(ctx, docID, noiID, opts.action, opts.status, opts.newState, txID, now)
	return nil
}

// SignDoc registers a new NOI under docID (status: sign). IGRPrimaryMSP only.
func (cc *DocRegistryChaincode) SignDoc(ctx contractapi.TransactionContextInterface, docID, noiID, docHash, metadata string) error {
	return cc.recordNoiTransition(ctx, docID, noiID, docHash, metadata, transitionOpts{
		isSign:                true,
		newState:              StateSign,
		status:                StatusSign,
		action:                ActionSign,
		allowedMSPs:           []string{"IGRPrimaryMSP"},
		requireParentSROMatch: true,
	})
}

// ApproveLoan moves NOI to loan_approval. IGRBankMSP only.
func (cc *DocRegistryChaincode) ApproveLoan(ctx contractapi.TransactionContextInterface, docID, noiID, metadata string) error {
	return cc.recordNoiTransition(ctx, docID, noiID, "", metadata, transitionOpts{
		expectedState: StateSign,
		newState:      StateLoanApproval,
		status:        StatusLoanApproval,
		action:        ActionLoanApproval,
		allowedMSPs:   []string{"IGRBankMSP"},
	})
}

// ApproveNoc finishes the loan (noc_approval). IGRPrimaryMSP or IGRBankMSP.
func (cc *DocRegistryChaincode) ApproveNoc(ctx contractapi.TransactionContextInterface, docID, noiID, metadata string) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if mspID == "IGRPrimaryMSP" {
		if err := requireCallerSROMatchesNoiOwner(ctx, docID, noiID); err != nil {
			return err
		}
	}
	return cc.recordNoiTransition(ctx, docID, noiID, "", metadata, transitionOpts{
		expectedState: StateLoanApproval,
		newState:      StateNocApproval,
		status:        StatusNocApproval,
		action:        ActionNocApproval,
		allowedMSPs:   []string{"IGRPrimaryMSP", "IGRBankMSP"},
	})
}

// VerifyDocHash checks whether the provided hash matches the stored hash for an NOI.
func (cc *DocRegistryChaincode) VerifyDocHash(ctx contractapi.TransactionContextInterface, docID, noiID, providedHash string) (bool, error) {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return false, err
	}
	if providedHash == "" {
		return false, fmt.Errorf("providedHash cannot be empty")
	}

	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
	}
	return current.DocHash == providedHash, nil
}

func loadNoiHistoryRecord(ctx contractapi.TransactionContextInterface, docID, noiID string, seq int) (*DocTxRecord, error) {
	key, err := dochistCompositeKey(ctx.GetStub(), docID, noiID, seq)
	if err != nil {
		return nil, err
	}
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %v", err)
	}
	if data == nil {
		return nil, fmt.Errorf("history record not found for doc %s noi %s seq %d", docID, noiID, seq)
	}
	var record DocTxRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history: %v", err)
	}
	return &record, nil
}

// GetFullHistory returns all transactions for one NOI under a docID.
func (cc *DocRegistryChaincode) GetFullHistory(ctx contractapi.TransactionContextInterface, docID, noiID string) ([]DocTxRecord, error) {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return nil, err
	}

	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(dochistObjectType, []string{docID, noiID})
	if err != nil {
		return nil, fmt.Errorf("failed to query history: %v", err)
	}
	defer iter.Close()

	type keyedRecord struct {
		seq    int
		record DocTxRecord
	}
	var records []keyedRecord

	for iter.HasNext() {
		resp, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate history: %v", err)
		}
		_, parts, err := ctx.GetStub().SplitCompositeKey(resp.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to parse history key: %v", err)
		}
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid history key attributes for %s", resp.Key)
		}
		seq, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid history sequence in key %s: %v", resp.Key, err)
		}
		var record DocTxRecord
		if err := json.Unmarshal(resp.Value, &record); err != nil {
			return nil, fmt.Errorf("failed to unmarshal history record: %v", err)
		}
		records = append(records, keyedRecord{seq: seq, record: record})
	}

	sort.Slice(records, func(i, j int) bool { return records[i].seq < records[j].seq })

	history := make([]DocTxRecord, len(records))
	for i, r := range records {
		history[i] = r.record
	}
	return history, nil
}

// GetDocNOIs returns all NOI entries and their statuses under a parent docID.
func (cc *DocRegistryChaincode) GetDocNOIs(ctx contractapi.TransactionContextInterface, docID string) (*DocNOIOverview, error) {
	if _, err := parseParentDocID(docID); err != nil {
		return nil, err
	}

	iter, err := ctx.GetStub().GetStateByPartialCompositeKey(docObjectType, []string{docID})
	if err != nil {
		return nil, fmt.Errorf("failed to query nois: %v", err)
	}
	defer iter.Close()

	overview := &DocNOIOverview{DocID: docID, Nois: []NoiStatusEntry{}}
	for iter.HasNext() {
		resp, err := iter.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to iterate nois: %v", err)
		}
		var current DocCurrent
		if err := json.Unmarshal(resp.Value, &current); err != nil {
			return nil, fmt.Errorf("failed to unmarshal noi: %v", err)
		}
		status := current.Status
		if status == "" {
			status = stateToStatus(current.State)
		}
		overview.Nois = append(overview.Nois, NoiStatusEntry{
			NoiID:   current.NoiID,
			DocHash: current.DocHash,
			State:   current.State,
			Status:  status,
		})
	}

	sort.Slice(overview.Nois, func(i, j int) bool { return overview.Nois[i].NoiID < overview.Nois[j].NoiID })
	return overview, nil
}

// GetDocLatest returns the latest transaction for one NOI.
func (cc *DocRegistryChaincode) GetDocLatest(ctx contractapi.TransactionContextInterface, docID, noiID string) (*DocLatestResponse, error) {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return nil, err
	}

	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return nil, err
	}
	if !exists || current.TxCount == 0 {
		return nil, fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
	}

	latest, err := loadNoiHistoryRecord(ctx, docID, noiID, current.TxCount-1)
	if err != nil {
		return nil, err
	}

	return &DocLatestResponse{
		Tx:     *latest,
		State:  current.State,
		Status: current.Status,
	}, nil
}

// GetDocStatus returns the current status for one NOI (sign, loan_approval, noc_approval).
func (cc *DocRegistryChaincode) GetDocStatus(ctx contractapi.TransactionContextInterface, docID, noiID string) (string, error) {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return "", err
	}

	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
	}
	if current.Status != "" {
		return current.Status, nil
	}
	return stateToStatus(current.State), nil
}

// GetDocState returns the current numeric state (1, 2, or 3) for one NOI.
func (cc *DocRegistryChaincode) GetDocState(ctx contractapi.TransactionContextInterface, docID, noiID string) (int, error) {
	if _, err := validateDocAndNoi(docID, noiID); err != nil {
		return 0, err
	}

	current, exists, err := loadNoiCurrent(ctx, docID, noiID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, fmt.Errorf("noi %s under doc %s does not exist", noiID, docID)
	}
	return current.State, nil
}

func main() {
	chaincode, err := contractapi.NewChaincode(&DocRegistryChaincode{})
	if err != nil {
		log.Panicf("Error creating doc registry chaincode: %v", err)
	}
	if err := chaincode.Start(); err != nil {
		log.Panicf("Error starting doc registry chaincode: %v", err)
	}
}
