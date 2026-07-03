package main

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	testDocRef = "MH:PUNE:SR42:2026:991"
	testUidV1  = "MH:PUNE:SR42:2026:991:V1"
	testUidV2  = "MH:PUNE:SR42:2026:991:V2"
	testHash   = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	testSROId  = "42"
	testSROUser = "sro42-officer"
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

type memoryStub struct {
	state       map[string][]byte
	txID        string
	txTimestamp *timestamppb.Timestamp
	events      []string
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
func (s *memoryStub) SetEvent(name string, payload []byte) error {
	s.events = append(s.events, name)
	return nil
}
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

func createAnchor(t *testing.T, cc *IgrAnchorChaincode, stub *memoryStub, txID, uid, hash, ref string) string {
	t.Helper()
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", txID, stub, testSROId, testSROUser)
	txnId, err := cc.CreateAnchorVersion(ctx, uid, testDocRef, hash, ref, "signer-meta")
	if err != nil {
		t.Fatalf("CreateAnchorVersion(%s): %v", uid, err)
	}
	return txnId
}

func TestCreateAnchorVersion_V1(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-v1", stub, testSROId, testSROUser)

	txnId, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, testHash, "REF-001", "signer-meta")
	if err != nil {
		t.Fatalf("CreateAnchorVersion: %v", err)
	}
	if txnId != "tx-v1" {
		t.Fatalf("txnId = %q, want tx-v1", txnId)
	}

	docKey, err := docCompositeKey(stub, testDocRef)
	if err != nil {
		t.Fatalf("docCompositeKey: %v", err)
	}
	var pointer DocPointer
	if err := json.Unmarshal(stub.state[docKey], &pointer); err != nil {
		t.Fatalf("unmarshal DocPointer: %v", err)
	}
	if pointer.LatestUid != testUidV1 || pointer.Version != 1 {
		t.Fatalf("unexpected doc pointer: %+v", pointer)
	}

	uidKey, err := uidCompositeKey(stub, testUidV1)
	if err != nil {
		t.Fatalf("uidCompositeKey: %v", err)
	}
	var record AnchorRecord
	if err := json.Unmarshal(stub.state[uidKey], &record); err != nil {
		t.Fatalf("unmarshal AnchorRecord: %v", err)
	}
	if record.PdfHash != "sha256:"+testHash {
		t.Fatalf("stored hash = %q", record.PdfHash)
	}
	if record.ReferenceId != "REF-001" || record.SignerRef != "signer-meta" {
		t.Fatalf("unexpected anchor fields: %+v", record)
	}
	if len(stub.events) != 1 || stub.events[0] != "AnchorCreated" {
		t.Fatalf("events = %v, want [AnchorCreated]", stub.events)
	}
}

func TestGetLatestAnchor_AfterV1(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	createAnchor(t, cc, stub, "tx-v1", testUidV1, testHash, "REF-001")

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-read", stub, testSROId, testSROUser)
	latest, err := cc.GetLatestAnchor(ctx, testDocRef)
	if err != nil {
		t.Fatalf("GetLatestAnchor: %v", err)
	}
	if latest.Uid != testUidV1 {
		t.Fatalf("latest uid = %q, want %q", latest.Uid, testUidV1)
	}
}

func TestCreateAnchorVersion_V2Chain(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	createAnchor(t, cc, stub, "tx-v1", testUidV1, testHash, "REF-001")
	createAnchor(t, cc, stub, "tx-v2", testUidV2, testHash, "REF-002")

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-read", stub, testSROId, testSROUser)

	latest, err := cc.GetLatestAnchor(ctx, testDocRef)
	if err != nil {
		t.Fatalf("GetLatestAnchor: %v", err)
	}
	if latest.Uid != testUidV2 {
		t.Fatalf("latest uid = %q, want %q", latest.Uid, testUidV2)
	}

	v1, err := cc.GetAnchorByUid(ctx, testUidV1)
	if err != nil {
		t.Fatalf("GetAnchorByUid V1: %v", err)
	}
	if v1.ReferenceId != "REF-001" {
		t.Fatalf("V1 record changed: %+v", v1)
	}
}

func TestCreateAnchorVersion_DuplicateUidSameHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	createAnchor(t, cc, stub, "tx-v1", testUidV1, testHash, "REF-001")

	docKey, err := docCompositeKey(stub, testDocRef)
	if err != nil {
		t.Fatalf("docCompositeKey: %v", err)
	}
	uidKey, err := uidCompositeKey(stub, testUidV1)
	if err != nil {
		t.Fatalf("uidCompositeKey: %v", err)
	}
	docBefore := append([]byte(nil), stub.state[docKey]...)
	uidBefore := append([]byte(nil), stub.state[uidKey]...)
	eventsBefore := len(stub.events)

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-dup", stub, testSROId, testSROUser)
	txnId, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, testHash, "REF-002", "")
	if err != nil {
		t.Fatalf("expected idempotent success: %v", err)
	}
	if txnId != "tx-v1" {
		t.Fatalf("txnId = %q, want tx-v1 (existing)", txnId)
	}
	if string(stub.state[docKey]) != string(docBefore) {
		t.Fatal("doc pointer changed on idempotent retry")
	}
	if string(stub.state[uidKey]) != string(uidBefore) {
		t.Fatal("anchor record changed on idempotent retry")
	}
	if len(stub.events) != eventsBefore {
		t.Fatalf("events = %d, want %d (no new event on idempotent retry)", len(stub.events), eventsBefore)
	}
}

func TestCreateAnchorVersion_DuplicateUidDifferentHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	createAnchor(t, cc, stub, "tx-v1", testUidV1, testHash, "REF-001")

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-dup", stub, testSROId, testSROUser)
	otherHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, otherHash, "REF-002", "")
	if err == nil {
		t.Fatal("expected hash conflict error")
	}
	if !strings.Contains(err.Error(), "already anchored with different hash") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyDocHash_MatchAndMismatch(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-v1")
	createAnchor(t, cc, stub, "tx-v1", testUidV1, testHash, "REF-001")

	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-verify", stub, testSROId, testSROUser)

	result, err := cc.VerifyDocHash(ctx, testUidV1, testHash)
	if err != nil || result.Status != "MATCH" {
		t.Fatalf("VerifyDocHash match: result=%+v err=%v", result, err)
	}
	if result.AnchoredHash != "sha256:"+testHash || result.TxId != "tx-v1" {
		t.Fatalf("unexpected match result: %+v", result)
	}

	result, err = cc.VerifyDocHash(ctx, testUidV1, "sha256:"+strings.ToUpper(testHash))
	if err != nil || result.Status != "MATCH" {
		t.Fatalf("VerifyDocHash prefixed uppercase: result=%+v err=%v", result, err)
	}

	result, err = cc.VerifyDocHash(ctx, testUidV1, "0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("VerifyDocHash mismatch err: %v", err)
	}
	if result.Status != "MISMATCH" {
		t.Fatalf("expected MISMATCH, got %+v", result)
	}
	if result.AnchoredHash != "sha256:"+testHash || result.TxId != "tx-v1" {
		t.Fatalf("unexpected mismatch result: %+v", result)
	}
}

func TestVerifyDocHash_NotFound(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-verify")

	result, err := cc.VerifyDocHash(ctx, testUidV1, testHash)
	if err != nil {
		t.Fatalf("VerifyDocHash not found err: %v", err)
	}
	if result.Status != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %+v", result)
	}
	if result.AnchoredHash != "" || result.TxId != "" {
		t.Fatalf("NOT_FOUND should omit anchoredHash and txId: %+v", result)
	}
}

func TestValidatePdfHash_UppercaseNormalized(t *testing.T) {
	upper := strings.ToUpper(testHash)
	normalized, err := validatePdfHash(upper)
	if err != nil {
		t.Fatalf("validatePdfHash: %v", err)
	}
	if normalized != "sha256:"+testHash {
		t.Fatalf("normalized = %q", normalized)
	}
}

func TestCreateAnchorVersion_InvalidHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-bad-hash")

	cases := []string{
		"",
		"abc",
		"dGVzdA==",
		"/tmp/hash.txt",
	}
	for _, hash := range cases {
		_, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, hash, "REF-001", "")
		if err == nil {
			t.Fatalf("expected error for hash %q", hash)
		}
	}
}

func TestCreateAnchorVersion_WrongMSP(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-bank", newMemoryStub("tx-bank"), "", "bank-officer")

	_, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected MSP authorization error")
	}
	if !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAnchorVersion_MissingSROAttribute(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-no-sro", newMemoryStub("tx-no-sro"), "", testSROUser)

	_, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected missing sroId error")
	}
}

func TestCreateAnchorVersion_SROMismatch(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-wrong-sro", newMemoryStub("tx-wrong-sro"), "99", "sro99-officer")

	_, err := cc.CreateAnchorVersion(ctx, testUidV1, testDocRef, testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected sroId mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match document SRO") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateAnchorVersion_BadDocRef(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-bad-docref")

	_, err := cc.CreateAnchorVersion(ctx, testUidV1, "2026SRO42DOC991", testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected bad docRef error")
	}
}

func TestCreateAnchorVersion_BadUid(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-bad-uid")

	_, err := cc.CreateAnchorVersion(ctx, "NoI1", testDocRef, testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected bad uid error")
	}
}

func TestCreateAnchorVersion_UidDocRefMismatch(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-mismatch")

	_, err := cc.CreateAnchorVersion(ctx, "MH:MUM:SR42:2026:991:V1", testDocRef, testHash, "REF-001", "")
	if err == nil {
		t.Fatal("expected uid/docRef mismatch error")
	}
}

func TestGetAnchorByUid_NotFound(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRPrimaryMSP", "tx-read")

	_, err := cc.GetAnchorByUid(ctx, testUidV1)
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetLoanState_NotFound(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRBankMSP", "tx-loan-read")

	state, err := cc.GetLoanState(ctx, testDocRef)
	if err != nil {
		t.Fatalf("GetLoanState: %v", err)
	}
	if state.Status != loanStatusNotFound {
		t.Fatalf("status = %q, want NOT_FOUND", state.Status)
	}
}

func TestMarkLoanActive_SetsActive(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-anchor")
	createAnchor(t, cc, stub, "tx-anchor", testUidV1, testHash, "REF-001")

	ctxWrite, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-active", stub, "", "bank-officer")
	if err := cc.MarkLoanActive(ctxWrite, testDocRef, "LN-2026-001", "IGRBANK", testUidV1); err != nil {
		t.Fatalf("MarkLoanActive: %v", err)
	}

	ctxRead, _ := newTestContextOnStub("IGRPrimaryMSP", "tx-loan-read", stub, testSROId, testSROUser)
	state, err := cc.GetLoanState(ctxRead, testDocRef)
	if err != nil {
		t.Fatalf("GetLoanState: %v", err)
	}
	if state.Status != loanStatusActive {
		t.Fatalf("status = %q, want ACTIVE", state.Status)
	}
	if state.LoanId != "LN-2026-001" || state.ActivatedByUid != testUidV1 {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestReleaseLoan_SetsReleased(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-anchor")
	createAnchor(t, cc, stub, "tx-anchor", testUidV1, testHash, "REF-001")

	ctxActive, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-active", stub, "", "bank-officer")
	if err := cc.MarkLoanActive(ctxActive, testDocRef, "LN-2026-001", "IGRBANK", testUidV1); err != nil {
		t.Fatalf("MarkLoanActive: %v", err)
	}

	ctxRelease, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-release", stub, "", "bank-officer")
	if err := cc.ReleaseLoan(ctxRelease, testDocRef, "LN-2026-001", "IGRBANK", "loan closed"); err != nil {
		t.Fatalf("ReleaseLoan: %v", err)
	}

	ctxRead, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-read", stub, "", "bank-officer")
	state, err := cc.GetLoanState(ctxRead, testDocRef)
	if err != nil {
		t.Fatalf("GetLoanState: %v", err)
	}
	if state.Status != loanStatusReleased {
		t.Fatalf("status = %q, want RELEASED", state.Status)
	}
}

func TestMarkLoanActive_RequiresAnchoredUid(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-loan-no-anchor")
	ctx, _ := newTestContextOnStub("IGRBankMSP", "tx-loan-active", stub, "", "bank-officer")

	err := cc.MarkLoanActive(ctx, testDocRef, "LN-2026-001", "IGRBANK", testUidV1)
	if err == nil {
		t.Fatal("expected error when activatedByUid is not anchored")
	}
}

func TestMarkLoanActive_RejectsWrongMSP(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-anchor")
	createAnchor(t, cc, stub, "tx-anchor", testUidV1, testHash, "REF-001")

	ctx, _ := newTestContextOnStub("Org1MSP", "tx-loan-active", stub, "", "other-user")
	if err := cc.MarkLoanActive(ctx, testDocRef, "LN-2026-001", "IGRBANK", testUidV1); err == nil {
		t.Fatal("expected MSP authorization error")
	}
}

func TestGetLoanState_RejectsBadDocRef(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRBankMSP", "tx-bad-docref")

	_, err := cc.GetLoanState(ctx, "2026SRO42DOC991")
	if err == nil {
		t.Fatal("expected invalid docRef error")
	}
}
