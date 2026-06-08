package main

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	testParentDocID = "2026SRO42DOC991"
	testNoi1        = "NoI1"
	testNoi2        = "NoI2"
	testDocHash     = "sha256:abc123def456"
	testSROId       = "42"
	testSROUser     = "sro42-officer"
)

type mockClientIdentity struct {
	mspID      string
	id         string
	attributes map[string]string
}

func (m *mockClientIdentity) GetID() (string, error) {
	if m.id != "" {
		return m.id, nil
	}
	return "test-user", nil
}
func (m *mockClientIdentity) GetMSPID() (string, error) { return m.mspID, nil }
func (m *mockClientIdentity) GetAttributeValue(attr string) (string, bool, error) {
	if m.attributes == nil {
		return "", false, nil
	}
	v, ok := m.attributes[attr]
	return v, ok, nil
}
func (m *mockClientIdentity) AssertAttributeValue(string, string) error { return nil }
func (m *mockClientIdentity) GetX509Certificate() (*x509.Certificate, error) { return nil, nil }

// memoryStub is a minimal in-memory stub for doc lifecycle unit tests.
type memoryStub struct {
	state       map[string][]byte
	txID        string
	txTimestamp *timestamppb.Timestamp
}

func newMemoryStub(txID string) *memoryStub {
	return &memoryStub{
		state: make(map[string][]byte),
		txID:  txID,
		txTimestamp: &timestamppb.Timestamp{
			Seconds: 1_700_000_000,
			Nanos:   0,
		},
	}
}

func (s *memoryStub) GetArgs() [][]byte                                      { return nil }
func (s *memoryStub) GetStringArgs() []string                                  { return nil }
func (s *memoryStub) GetFunctionAndParameters() (string, []string)           { return "", nil }
func (s *memoryStub) GetArgsSlice() ([]byte, error)                            { return nil, errors.New("not implemented") }
func (s *memoryStub) GetTxID() string                                          { return s.txID }
func (s *memoryStub) GetChannelID() string                                     { return "testchannel" }
func (s *memoryStub) InvokeChaincode(string, [][]byte, string) *peer.Response  { return nil }
func (s *memoryStub) GetState(key string) ([]byte, error) {
	v, ok := s.state[key]
	if !ok {
		return nil, nil
	}
	return v, nil
}
func (s *memoryStub) PutState(key string, value []byte) error {
	s.state[key] = value
	return nil
}
func (s *memoryStub) DelState(key string) error {
	delete(s.state, key)
	return nil
}
func (s *memoryStub) SetStateValidationParameter(string, []byte) error { return errors.New("not implemented") }
func (s *memoryStub) GetStateValidationParameter(string) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetStateByRange(string, string) (shim.StateQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetStateByRangeWithPagination(string, string, int32, string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *memoryStub) GetStateByPartialCompositeKey(objectType string, attributes []string) (shim.StateQueryIteratorInterface, error) {
	var kvs []*queryresult.KV
	for key, value := range s.state {
		if matchesPartialCompositeKey(key, objectType, attributes) {
			kvs = append(kvs, &queryresult.KV{Key: key, Value: value})
		}
	}
	sort.Slice(kvs, func(i, j int) bool { return kvs[i].Key < kvs[j].Key })
	return &kvIterator{records: kvs}, nil
}
func (s *memoryStub) GetStateByPartialCompositeKeyWithPagination(string, []string, int32, string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *memoryStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	return shim.CreateCompositeKey(objectType, attributes)
}
func (s *memoryStub) SplitCompositeKey(compositeKey string) (string, []string, error) {
	return splitCompositeKey(compositeKey)
}
func (s *memoryStub) GetQueryResult(string) (shim.StateQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetQueryResultWithPagination(string, int32, string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, errors.New("not implemented")
}
func (s *memoryStub) GetHistoryForKey(string) (shim.HistoryQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetPrivateData(string, string) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) PutPrivateData(string, string, []byte) error { return errors.New("not implemented") }
func (s *memoryStub) DelPrivateData(string, string) error           { return errors.New("not implemented") }
func (s *memoryStub) PurgePrivateData(string, string) error         { return errors.New("not implemented") }
func (s *memoryStub) GetPrivateDataByRange(string, string, string) (shim.StateQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetPrivateDataByPartialCompositeKey(string, string, []string) (shim.StateQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetPrivateDataQueryResult(string, string) (shim.StateQueryIteratorInterface, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetPrivateDataHash(string, string) ([]byte, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetCreator() ([]byte, error)              { return []byte("creator"), nil }
func (s *memoryStub) GetDecorations() map[string][]byte        { return nil }
func (s *memoryStub) GetSignedProposal() (*peer.SignedProposal, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetTransient() (map[string][]byte, error) { return nil, nil }
func (s *memoryStub) SetEvent(string, []byte) error            { return nil }
func (s *memoryStub) SetTransient(map[string][]byte) error     { return errors.New("not implemented") }
func (s *memoryStub) GetBinding() ([]byte, error)              { return nil, errors.New("not implemented") }
func (s *memoryStub) GetTxTimestamp() (*timestamppb.Timestamp, error) {
	return s.txTimestamp, nil
}
func (s *memoryStub) SetPrivateDataValidationParameter(string, string, []byte) error {
	return errors.New("not implemented")
}
func (s *memoryStub) GetPrivateDataValidationParameter(string, string) ([]byte, error) {
	return nil, errors.New("not implemented")
}

type kvIterator struct {
	records []*queryresult.KV
	index   int
}

func (it *kvIterator) HasNext() bool {
	return it.index < len(it.records)
}

func (it *kvIterator) Next() (*queryresult.KV, error) {
	if !it.HasNext() {
		return nil, errors.New("no more items in iterator")
	}
	kv := it.records[it.index]
	it.index++
	return kv, nil
}

func (it *kvIterator) Close() error {
	return nil
}

func splitCompositeKey(compositeKey string) (string, []string, error) {
	const delim = "\x00"
	componentIndex := 1
	components := []string{}
	for i := 1; i < len(compositeKey); i++ {
		if compositeKey[i] == delim[0] {
			components = append(components, compositeKey[componentIndex:i])
			componentIndex = i + 1
		}
	}
	if len(components) == 0 {
		return "", nil, fmt.Errorf("invalid composite key: %s", compositeKey)
	}
	return components[0], components[1:], nil
}

func matchesPartialCompositeKey(key, objectType string, attributes []string) bool {
	ot, keyAttrs, err := splitCompositeKey(key)
	if err != nil {
		return false
	}
	if ot != objectType {
		return false
	}
	if len(keyAttrs) < len(attributes) {
		return false
	}
	for i, attr := range attributes {
		if keyAttrs[i] != attr {
			return false
		}
	}
	return true
}

func newTestContext(mspID, txID string) (contractapi.TransactionContextInterface, *memoryStub) {
	sroId, signerID := "", ""
	if mspID == "IGRPrimaryMSP" {
		sroId = testSROId
		signerID = testSROUser
	}
	return newTestContextOnStub(mspID, txID, newMemoryStub(txID), sroId, signerID)
}

func newTestContextOnStub(mspID, txID string, stub *memoryStub, sroId, signerID string) (contractapi.TransactionContextInterface, *memoryStub) {
	stub.txID = txID
	ci := &mockClientIdentity{mspID: mspID, id: signerID}
	if sroId != "" {
		ci.attributes = map[string]string{sroIDAttribute: sroId}
	}
	ctx := &contractapi.TransactionContext{}
	ctx.SetStub(stub)
	ctx.SetClientIdentity(ci)
	return ctx, stub
}

func advanceNoiToState(t *testing.T, cc *DocRegistryChaincode, docID, noiID string, targetState int) *memoryStub {
	t.Helper()

	stub := newMemoryStub("tx-init")
	ctxSign, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign, docID, noiID, testDocHash, "signed"); err != nil {
		t.Fatalf("SignDoc: %v", err)
	}
	if targetState == StateSign {
		return stub
	}

	ctxLoan, _ := newTestContextOnStub("IGRBankMSP", "tx-loan", stub, "", "bank-officer")
	if err := cc.ApproveLoan(ctxLoan, docID, noiID, "loan"); err != nil {
		t.Fatalf("ApproveLoan: %v", err)
	}
	if targetState == StateLoanApproval {
		return stub
	}

	ctxNoc, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-noc", stub, testSROId, testSROUser)
	if err := cc.ApproveNoc(ctxNoc, docID, noiID, "noc"); err != nil {
		t.Fatalf("ApproveNoc: %v", err)
	}
	return stub
}

func logJSON(t *testing.T, label string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Logf("%s: %v", label, v)
		return
	}
	t.Logf("%s:\n%s", label, string(b))
}

func logDocStatus(t *testing.T, cc *DocRegistryChaincode, ctx contractapi.TransactionContextInterface, docID, noiID string) string {
	t.Helper()
	status, err := cc.GetDocStatus(ctx, docID, noiID)
	if err != nil {
		t.Logf("GetDocStatus(%q, %q) error: %v", docID, noiID, err)
		return ""
	}
	t.Logf("GetDocStatus(%q, %q) => %q", docID, noiID, status)
	return status
}

func logDocState(t *testing.T, cc *DocRegistryChaincode, ctx contractapi.TransactionContextInterface, docID, noiID string) int {
	t.Helper()
	state, err := cc.GetDocState(ctx, docID, noiID)
	if err != nil {
		t.Logf("GetDocState(%q, %q) error: %v", docID, noiID, err)
		return -1
	}
	t.Logf("GetDocState(%q, %q) => %d", docID, noiID, state)
	return state
}

func logFullHistory(t *testing.T, cc *DocRegistryChaincode, ctx contractapi.TransactionContextInterface, docID, noiID string) []DocTxRecord {
	t.Helper()
	history, err := cc.GetFullHistory(ctx, docID, noiID)
	if err != nil {
		t.Logf("GetFullHistory(%q, %q) error: %v", docID, noiID, err)
		return nil
	}
	logJSON(t, fmt.Sprintf("GetFullHistory(%q, %q)", docID, noiID), history)
	return history
}

func logDocLatest(t *testing.T, cc *DocRegistryChaincode, ctx contractapi.TransactionContextInterface, docID, noiID string) *DocLatestResponse {
	t.Helper()
	latest, err := cc.GetDocLatest(ctx, docID, noiID)
	if err != nil {
		t.Logf("GetDocLatest(%q, %q) error: %v", docID, noiID, err)
		return nil
	}
	logJSON(t, fmt.Sprintf("GetDocLatest(%q, %q)", docID, noiID), latest)
	return latest
}

func TestSignDoc_WritesState0(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, stub := newTestContext("IGRPrimaryMSP", "tx-sign-1")

	if err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, "note"); err != nil {
		t.Fatalf("SignDoc failed: %v", err)
	}
	t.Logf("SignDoc(%q, %q) => success", testParentDocID, testNoi1)

	state := logDocState(t, cc, ctx, testParentDocID, testNoi1)
	if state != StateSign {
		t.Fatalf("state = %d, want %d", state, StateSign)
	}
	if logDocStatus(t, cc, ctx, testParentDocID, testNoi1) != StatusSign {
		t.Fatalf("unexpected status")
	}

	docKey, err := docCompositeKey(stub, testParentDocID, testNoi1)
	if err != nil {
		t.Fatalf("docCompositeKey: %v", err)
	}
	var current DocCurrent
	if err := json.Unmarshal(stub.state[docKey], &current); err != nil {
		t.Fatalf("unmarshal DocCurrent: %v", err)
	}
	logJSON(t, "ledger DocCurrent", current)
	logFullHistory(t, cc, ctx, testParentDocID, testNoi1)
}

func TestSignDoc_RejectsWrongSROInParentDocID(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign-wrong-sro", newMemoryStub("tx-sign-wrong-sro"), "99", "sro99-officer")

	err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, "")
	if err == nil {
		t.Fatal("expected error when caller sroId does not match parent docID SRO")
	}
	t.Logf("SignDoc wrong parent SRO => error: %v", err)
}

func TestSignDoc_RejectsMissingSROAttribute(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign-no-attr", newMemoryStub("tx-sign-no-attr"), "", testSROUser)

	err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, "")
	if err == nil {
		t.Fatal("expected error when caller has no sroId certificate attribute")
	}
	t.Logf("SignDoc missing sroId => error: %v", err)
}

func TestSignDoc_RejectsEmptyDocHash(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-sign-no-hash")

	err := cc.SignDoc(ctx, testParentDocID, testNoi1, "", "note")
	if err == nil {
		t.Fatal("expected error when docHash is empty")
	}
	t.Logf("SignDoc empty docHash => error: %v", err)
}

func TestSignDoc_StoresDocHash(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-sign-hash")

	if err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc: %v", err)
	}

	history := logFullHistory(t, cc, ctx, testParentDocID, testNoi1)
	if history[0].DocHash != testDocHash {
		t.Fatalf("history docHash = %q, want %q", history[0].DocHash, testDocHash)
	}

	match, err := cc.VerifyDocHash(ctx, testParentDocID, testNoi1, testDocHash)
	if err != nil {
		t.Fatalf("VerifyDocHash: %v", err)
	}
	if !match {
		t.Fatal("VerifyDocHash expected true")
	}

	match, err = cc.VerifyDocHash(ctx, testParentDocID, testNoi1, "wrong-hash")
	if err != nil {
		t.Fatalf("VerifyDocHash wrong: %v", err)
	}
	if match {
		t.Fatal("VerifyDocHash expected false for wrong hash")
	}
}

func TestSignDoc_StoresSROAndSignerInHistory(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-sign-audit")

	if err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, "note"); err != nil {
		t.Fatalf("SignDoc: %v", err)
	}

	history := logFullHistory(t, cc, ctx, testParentDocID, testNoi1)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].NoiID != testNoi1 {
		t.Fatalf("history noiId = %q, want %q", history[0].NoiID, testNoi1)
	}
	if history[0].SROId != testSROId {
		t.Fatalf("history sroId = %q, want %q", history[0].SROId, testSROId)
	}
}

func TestSignDoc_RejectsInvalidDocIDFormat(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-sign-bad-id")

	err := cc.SignDoc(ctx, "invalid-doc-id", testNoi1, testDocHash, "")
	if err == nil {
		t.Fatal("expected error for invalid docID format")
	}
	err = cc.SignDoc(ctx, testParentDocID, "bad-noi", testDocHash, "")
	if err == nil {
		t.Fatal("expected error for invalid noiID format")
	}
}

func TestSignDoc_RejectsDuplicate(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateSign)

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign-dup", stub, testSROId, testSROUser)
	err := cc.SignDoc(ctx, testParentDocID, testNoi1, testDocHash, "")
	if err == nil {
		t.Fatal("expected error when signing existing noi")
	}
	t.Logf("SignDoc duplicate => error: %v", err)
}

func TestApproveLoan_WritesState1(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateSign)

	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-1", stub, "", "bank-officer")
	if err := cc.ApproveLoan(ctx, testParentDocID, testNoi1, "approved"); err != nil {
		t.Fatalf("ApproveLoan: %v", err)
	}

	if logDocState(t, cc, ctx, testParentDocID, testNoi1) != StateLoanApproval {
		t.Fatal("unexpected state after loan")
	}
	if logDocStatus(t, cc, ctx, testParentDocID, testNoi1) != StatusLoanApproval {
		t.Fatal("unexpected status after loan")
	}
	logFullHistory(t, cc, ctx, testParentDocID, testNoi1)
}

func TestApproveLoan_RequiresBankMSP(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateSign)

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-loan-wrong-msp", stub, testSROId, testSROUser)
	if err := cc.ApproveLoan(ctx, testParentDocID, testNoi1, ""); err == nil {
		t.Fatal("expected MSP authorization error for ApproveLoan")
	}
}

func TestApproveNoc_WritesState2(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateLoanApproval)

	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-noc-1", stub, "", "bank-officer")
	if err := cc.ApproveNoc(ctx, testParentDocID, testNoi1, "finished"); err != nil {
		t.Fatalf("ApproveNoc: %v", err)
	}

	if logDocStatus(t, cc, ctx, testParentDocID, testNoi1) != StatusNocApproval {
		t.Fatal("unexpected status after NOC")
	}
}

func TestApproveNoc_PrimaryRequiresMatchingOwnerSRO(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateLoanApproval)

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-noc-wrong-sro", stub, "99", "sro99-officer")
	if err := cc.ApproveNoc(ctx, testParentDocID, testNoi1, ""); err == nil {
		t.Fatal("expected error when primary caller sroId does not match noi owner SRO")
	}
}

func TestApproveNoc_RejectsWhenNotInLoan(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateSign)

	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-noc-early", stub, "", "bank-officer")
	if err := cc.ApproveNoc(ctx, testParentDocID, testNoi1, ""); err == nil {
		t.Fatal("expected error when approving NOC before loan")
	}
}

func TestTerminalState_RejectsFurtherWrites(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateNocApproval)

	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-after-terminal", stub, "", "bank-officer")
	if err := cc.ApproveNoc(ctx, testParentDocID, testNoi1, ""); err == nil {
		t.Fatal("expected error when noi is already finished")
	}
}

func TestGetFullHistory_ReturnsAllTransitionsInOrder(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateNocApproval)

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-read", stub, testSROId, testSROUser)
	history := logFullHistory(t, cc, ctx, testParentDocID, testNoi1)
	if len(history) != 3 {
		t.Fatalf("history length = %d, want 3", len(history))
	}

	wantStatuses := []string{StatusSign, StatusLoanApproval, StatusNocApproval}
	for i, rec := range history {
		if rec.Status != wantStatuses[i] {
			t.Fatalf("history[%d].status = %q, want %q", i, rec.Status, wantStatuses[i])
		}
		if rec.DocID != testParentDocID || rec.NoiID != testNoi1 {
			t.Fatalf("history[%d] doc/noi mismatch", i)
		}
	}
}

func TestGetFullHistory_EmptyForUnknownNoi(t *testing.T) {
	cc := &DocRegistryChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-read-missing")

	history, err := cc.GetFullHistory(ctx, testParentDocID, "NoI99")
	if err != nil {
		t.Fatalf("GetFullHistory: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("history length = %d, want 0", len(history))
	}
}

func TestGetDocLatest_ReturnsLatestTxAndState(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := advanceNoiToState(t, cc, testParentDocID, testNoi1, StateLoanApproval)

	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-read-latest", stub, "", "bank-officer")
	latest := logDocLatest(t, cc, ctx, testParentDocID, testNoi1)
	if latest == nil {
		t.Fatal("GetDocLatest returned nil")
	}
	if latest.Status != StatusLoanApproval || latest.Tx.TxID != "tx-loan" {
		t.Fatalf("unexpected latest: status=%q txId=%q", latest.Status, latest.Tx.TxID)
	}
}

func TestGetDocNOIs_ReturnsAllNoiStatuses(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := newMemoryStub("tx-init")

	ctxSign1, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-s1", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign1, testParentDocID, testNoi1, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc NoI1: %v", err)
	}
	ctxSign2, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-s2", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign2, testParentDocID, testNoi2, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc NoI2: %v", err)
	}
	ctxLoan, _ := newTestContextOnStub("IGRBankMSP", "tx-loan", stub, "", "bank-officer")
	if err := cc.ApproveLoan(ctxLoan, testParentDocID, testNoi1, ""); err != nil {
		t.Fatalf("ApproveLoan NoI1: %v", err)
	}

	ctxRead, _ := newTestContextOnStub("IGRBankMSP", "tx-read", stub, "", "bank-officer")
	overview, err := cc.GetDocNOIs(ctxRead, testParentDocID)
	if err != nil {
		t.Fatalf("GetDocNOIs: %v", err)
	}
	logJSON(t, "GetDocNOIs", overview)

	if len(overview.Nois) != 2 {
		t.Fatalf("nois count = %d, want 2", len(overview.Nois))
	}
	if overview.Nois[0].NoiID != testNoi1 || overview.Nois[0].Status != StatusLoanApproval {
		t.Fatalf("NoI1 overview unexpected: %+v", overview.Nois[0])
	}
	if overview.Nois[1].NoiID != testNoi2 || overview.Nois[1].Status != StatusSign {
		t.Fatalf("NoI2 overview unexpected: %+v", overview.Nois[1])
	}
}

func TestFullLifecycle_StateAndHistory(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := newMemoryStub("tx-init")

	ctxSign, _ := newTestContextOnStub("IGRPrimaryMSP", "tx1", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign, testParentDocID, testNoi1, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc: %v", err)
	}
	ctxLoan, _ := newTestContextOnStub("IGRBankMSP", "tx2", stub, "", "bank-officer")
	if err := cc.ApproveLoan(ctxLoan, testParentDocID, testNoi1, ""); err != nil {
		t.Fatalf("ApproveLoan: %v", err)
	}
	ctxNoc, _ := newTestContextOnStub("IGRPrimaryMSP", "tx3", stub, testSROId, testSROUser)
	if err := cc.ApproveNoc(ctxNoc, testParentDocID, testNoi1, ""); err != nil {
		t.Fatalf("ApproveNoc: %v", err)
	}

	ctxRead, _ := newTestContextOnStub("IGRBankMSP", "tx-read", stub, "", "bank-officer")
	if logDocStatus(t, cc, ctxRead, testParentDocID, testNoi1) != StatusNocApproval {
		t.Fatal("final status unexpected")
	}
	if len(logFullHistory(t, cc, ctxRead, testParentDocID, testNoi1)) != 3 {
		t.Fatal("expected 3 history entries")
	}
}

func TestMultipleNois_IndependentLifecycleUnderSameDoc(t *testing.T) {
	cc := &DocRegistryChaincode{}
	stub := newMemoryStub("tx-init")

	ctxSign1, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign1", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign1, testParentDocID, testNoi1, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc NoI1: %v", err)
	}
	ctxSign2, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-sign2", stub, testSROId, testSROUser)
	if err := cc.SignDoc(ctxSign2, testParentDocID, testNoi2, testDocHash, ""); err != nil {
		t.Fatalf("SignDoc NoI2: %v", err)
	}

	ctxRead, _ := newTestContextOnStub("IGRBankMSP", "tx-read", stub, "", "bank-officer")
	ctxLoan1, _ := newTestContextOnStub("IGRBankMSP", "tx-loan1", stub, "", "bank-officer")
	if err := cc.ApproveLoan(ctxLoan1, testParentDocID, testNoi1, ""); err != nil {
		t.Fatalf("ApproveLoan NoI1: %v", err)
	}

	if logDocStatus(t, cc, ctxRead, testParentDocID, testNoi1) != StatusLoanApproval {
		t.Fatal("NoI1 should be loan_approval")
	}
	if logDocStatus(t, cc, ctxRead, testParentDocID, testNoi2) != StatusSign {
		t.Fatal("NoI2 should remain sign")
	}

	overview, _ := cc.GetDocNOIs(ctxRead, testParentDocID)
	logJSON(t, fmt.Sprintf("doc %s NOI overview", testParentDocID), overview)
}
