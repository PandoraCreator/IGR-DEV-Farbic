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
	testOrigUid = "MH:PUNE:SR42:2026:991"
	testCC1     = "MH:PUNE:SR42:2026:991:CC1"
	testCC2     = "MH:PUNE:SR42:2026:991:CC2"
	testHash    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	testHash2   = "1111111111111111111111111111111111111111111111111111111111111111"
	testSigHash = "2222222222222222222222222222222222222222222222222222222222222222"
	zeroHash    = "0000000000000000000000000000000000000000000000000000000000000000"
	writerMSP   = "IGRPrimaryMSP"
)

// ---------------------------------------------------------------------------
// Mock Fabric infrastructure
// ---------------------------------------------------------------------------

type mockClientIdentity struct {
	mspID string
	id    string
}

func (m *mockClientIdentity) GetID() (string, error) {
	if m.id != "" {
		return m.id, nil
	}
	return "test-user", nil
}
func (m *mockClientIdentity) GetMSPID() (string, error) { return m.mspID, nil }
func (m *mockClientIdentity) GetAttributeValue(string) (string, bool, error) {
	return "", false, nil
}
func (m *mockClientIdentity) AssertAttributeValue(string, string) error      { return nil }
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

func (s *memoryStub) GetArgs() [][]byte                            { return nil }
func (s *memoryStub) GetStringArgs() []string                      { return nil }
func (s *memoryStub) GetFunctionAndParameters() (string, []string) { return "", nil }
func (s *memoryStub) GetArgsSlice() ([]byte, error)                { return nil, errors.New("not implemented") }
func (s *memoryStub) GetTxID() string                              { return s.txID }
func (s *memoryStub) GetChannelID() string                         { return "testchannel" }
func (s *memoryStub) InvokeChaincode(string, [][]byte, string) *peer.Response {
	return nil
}
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
func (s *memoryStub) SetStateValidationParameter(string, []byte) error {
	return errors.New("not implemented")
}
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
func (s *memoryStub) PutPrivateData(string, string, []byte) error {
	return errors.New("not implemented")
}
func (s *memoryStub) DelPrivateData(string, string) error   { return errors.New("not implemented") }
func (s *memoryStub) PurgePrivateData(string, string) error { return errors.New("not implemented") }
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
func (s *memoryStub) GetCreator() ([]byte, error)       { return []byte("creator"), nil }
func (s *memoryStub) GetDecorations() map[string][]byte { return nil }
func (s *memoryStub) GetSignedProposal() (*peer.SignedProposal, error) {
	return nil, errors.New("not implemented")
}
func (s *memoryStub) GetTransient() (map[string][]byte, error) { return nil, nil }
func (s *memoryStub) SetEvent(name string, payload []byte) error {
	s.events = append(s.events, name)
	return nil
}
func (s *memoryStub) SetTransient(map[string][]byte) error { return errors.New("not implemented") }
func (s *memoryStub) GetBinding() ([]byte, error)          { return nil, errors.New("not implemented") }
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

func (it *kvIterator) HasNext() bool { return it.index < len(it.records) }
func (it *kvIterator) Next() (*queryresult.KV, error) {
	if !it.HasNext() {
		return nil, errors.New("no more items in iterator")
	}
	kv := it.records[it.index]
	it.index++
	return kv, nil
}
func (it *kvIterator) Close() error { return nil }

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

// ---------------------------------------------------------------------------
// Test context + payload helpers
// ---------------------------------------------------------------------------

func newTestContext(mspID, txID string) (contractapi.TransactionContextInterface, *memoryStub) {
	return newTestContextOnStub(mspID, txID, newMemoryStub(txID))
}

func newTestContextOnStub(mspID, txID string, stub *memoryStub) (contractapi.TransactionContextInterface, *memoryStub) {
	stub.txID = txID
	ci := &mockClientIdentity{mspID: mspID, id: mspID + "-user"}
	ctx := &contractapi.TransactionContext{}
	ctx.SetStub(stub)
	ctx.SetClientIdentity(ci)
	return ctx, stub
}

func anchorPayload(orig, uid string, version int, finalHash, sigHash string, prev *string) string {
	p := CertifiedCopyAnchorPayload{
		OriginalDocumentUid:  orig,
		CertifiedCopyUid:     uid,
		CertifiedCopyVersion: version,
		DocumentNo:           "991",
		RegistrationYear:     2026,
		District:             "PUNE",
		SroOffice:            "SR42",
		DocumentType:         "SALE_DEED",
		FinalSignedPdfHash:   finalHash,
		SignatureHash:        sigHash,
		SignatureVersion:     "v1",
		RequestId:            "REQ-2026-000123",
		SroId:                "SR42",
		SigningAuthorityRef:  "sro-officer-ref",
		SigningTimestamp:     "2026-07-19T10:00:00Z",
		PreviousVersionRef:   prev,
	}
	b, _ := json.Marshal(p)
	return string(b)
}

func noiPayload(orig string) string {
	p := NOIApprovalPayload{
		OriginalDocumentUid:  orig,
		NoiApplicationId:     "NOI-2026-777",
		BankId:               "BANK-HDFC",
		LoanState:            "ACTIVE_LOAN",
		ApprovedBySroRef:     "sro-officer-ref",
		ApprovalTimestamp:    "2026-07-19T11:00:00Z",
		TransactionTimestamp: "2026-07-19T11:00:01Z",
	}
	b, _ := json.Marshal(p)
	return string(b)
}

func nocPayload(orig string) string {
	p := LoanFinishPayload{
		OriginalDocumentUid:  orig,
		NoiApplicationId:     "NOI-2026-777",
		BankId:               "BANK-HDFC",
		LoanState:            "NO_LOAN",
		NocReferenceId:       "NOC-2026-555",
		SubmittedTimestamp:   "2026-08-01T09:00:00Z",
		TransactionTimestamp: "2026-08-01T09:00:01Z",
	}
	b, _ := json.Marshal(p)
	return string(b)
}

func mustAnchor(t *testing.T, cc *IgrAnchorChaincode, stub *memoryStub, txID, orig, uid string, version int, finalHash string, prev *string) *AnchorResponse {
	t.Helper()
	ctx, _ := newTestContextOnStub(writerMSP, txID, stub)
	resp, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(orig, uid, version, finalHash, testSigHash, prev))
	if err != nil {
		t.Fatalf("AnchorCertifiedCopy(%s): %v", uid, err)
	}
	return resp
}

func strptr(s string) *string { return &s }

// ---------------------------------------------------------------------------
// Anchor tests
// ---------------------------------------------------------------------------

func TestAnchorCertifiedCopy_CC1Success(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	resp := mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)

	if resp.Status != anchorStatusAnchored || resp.TxnId != "tx-cc1" || resp.CertifiedCopyUid != testCC1 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	docKey, _ := docCompositeKey(stub, testOrigUid)
	var pointer DocPointer
	if err := json.Unmarshal(stub.state[docKey], &pointer); err != nil {
		t.Fatalf("unmarshal DocPointer: %v", err)
	}
	if pointer.LatestCertifiedCopyUid != testCC1 || pointer.VersionCount != 1 {
		t.Fatalf("unexpected doc pointer: %+v", pointer)
	}

	uidKey, _ := uidCompositeKey(stub, testCC1)
	var record AnchorRecord
	if err := json.Unmarshal(stub.state[uidKey], &record); err != nil {
		t.Fatalf("unmarshal AnchorRecord: %v", err)
	}
	if record.FinalSignedPdfHash != "sha256:"+testHash {
		t.Fatalf("stored final hash = %q", record.FinalSignedPdfHash)
	}
	if record.SignatureHash != "sha256:"+testSigHash {
		t.Fatalf("stored signature hash = %q", record.SignatureHash)
	}
	if record.CertifiedCopyVersion != 1 || record.RequestId != "REQ-2026-000123" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.PreviousVersionRef != "" {
		t.Fatalf("CC1 previousVersionRef = %q, want empty", record.PreviousVersionRef)
	}
	if len(stub.events) != 1 || stub.events[0] != eventCertifiedCopyAnchored {
		t.Fatalf("events = %v, want [%s]", stub.events, eventCertifiedCopyAnchored)
	}
}

func TestAnchorCertifiedCopy_CC2ChainsPrevious(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)
	mustAnchor(t, cc, stub, "tx-cc2", testOrigUid, testCC2, 2, testHash2, strptr(testCC1))

	docKey, _ := docCompositeKey(stub, testOrigUid)
	var pointer DocPointer
	_ = json.Unmarshal(stub.state[docKey], &pointer)
	if pointer.LatestCertifiedCopyUid != testCC2 || pointer.VersionCount != 2 {
		t.Fatalf("unexpected doc pointer: %+v", pointer)
	}

	uidKey, _ := uidCompositeKey(stub, testCC2)
	var record AnchorRecord
	_ = json.Unmarshal(stub.state[uidKey], &record)
	if record.PreviousVersionRef != testCC1 {
		t.Fatalf("CC2 previousVersionRef = %q, want %s", record.PreviousVersionRef, testCC1)
	}
}

func TestAnchorCertifiedCopy_IdempotentSameHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)

	docKey, _ := docCompositeKey(stub, testOrigUid)
	uidKey, _ := uidCompositeKey(stub, testCC1)
	docBefore := append([]byte(nil), stub.state[docKey]...)
	uidBefore := append([]byte(nil), stub.state[uidKey]...)
	eventsBefore := len(stub.events)

	ctx, _ := newTestContextOnStub(writerMSP, "tx-dup", stub)
	resp, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC1, 1, testHash, testSigHash, nil))
	if err != nil {
		t.Fatalf("expected idempotent success: %v", err)
	}
	if resp.TxnId != "tx-cc1" {
		t.Fatalf("txnId = %q, want tx-cc1 (existing)", resp.TxnId)
	}
	if string(stub.state[docKey]) != string(docBefore) || string(stub.state[uidKey]) != string(uidBefore) {
		t.Fatal("ledger changed on idempotent retry")
	}
	if len(stub.events) != eventsBefore {
		t.Fatalf("events = %d, want %d (no new event)", len(stub.events), eventsBefore)
	}
}

func TestAnchorCertifiedCopy_ConflictDifferentHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)

	ctx, _ := newTestContextOnStub(writerMSP, "tx-dup", stub)
	_, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC1, 1, zeroHash, testSigHash, nil))
	if err == nil || !strings.Contains(err.Error(), "already anchored with a different hash") {
		t.Fatalf("expected hash conflict error, got %v", err)
	}
}

func TestAnchorCertifiedCopy_RejectsBadIdentifiers(t *testing.T) {
	cc := &IgrAnchorChaincode{}

	cases := []struct {
		name         string
		orig, uid    string
		version      int
		prev         *string
		preAnchorCC1 bool
	}{
		{name: "bad orig", orig: "2026SRO42DOC991", uid: testCC1, version: 1},
		{name: "v-suffix uid", orig: testOrigUid, uid: "MH:PUNE:SR42:2026:991:V1", version: 1},
		{name: "uid/orig mismatch", orig: testOrigUid, uid: "MH:MUM:SR42:2026:991:CC1", version: 1},
		{name: "version not next (CC2 first)", orig: testOrigUid, uid: testCC2, version: 2},
		{name: "version arg mismatch", orig: testOrigUid, uid: testCC1, version: 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := newMemoryStub("tx-seed")
			if tc.preAnchorCC1 {
				mustAnchor(t, cc, stub, "tx-seed", testOrigUid, testCC1, 1, testHash, nil)
			}
			ctx, _ := newTestContextOnStub(writerMSP, "tx-bad", stub)
			_, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(tc.orig, tc.uid, tc.version, testHash, testSigHash, tc.prev))
			if err == nil {
				t.Fatalf("expected error for case %q", tc.name)
			}
		})
	}
}

func TestAnchorCertifiedCopy_WrongPreviousVersionRef(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)

	ctx, _ := newTestContextOnStub(writerMSP, "tx-cc2", stub)
	_, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC2, 2, testHash2, testSigHash, strptr("MH:PUNE:SR42:2026:991:CC9")))
	if err == nil || !strings.Contains(err.Error(), "previousVersionRef") {
		t.Fatalf("expected previousVersionRef mismatch error, got %v", err)
	}
}

func TestAnchorCertifiedCopy_RejectsWrongMSP(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext("IGRBankMSP", "tx-bank")
	_, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC1, 1, testHash, testSigHash, nil))
	if err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected MSP authorization error, got %v", err)
	}
}

func TestAnchorCertifiedCopy_RejectsBadHash(t *testing.T) {
	cc := &IgrAnchorChaincode{}

	badFinal := []string{"", "abc", "dGVzdA==", "/tmp/hash.txt"}
	for _, h := range badFinal {
		ctx, _ := newTestContext(writerMSP, "tx-bad-final")
		if _, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC1, 1, h, testSigHash, nil)); err == nil {
			t.Fatalf("expected error for final hash %q", h)
		}
	}
	// invalid signature hash
	ctx, _ := newTestContext(writerMSP, "tx-bad-sig")
	if _, err := cc.AnchorCertifiedCopy(ctx, anchorPayload(testOrigUid, testCC1, 1, testHash, "abc", nil)); err == nil {
		t.Fatal("expected error for invalid signatureHash")
	}
}

func TestAnchorCertifiedCopy_RejectsMissingRequiredFields(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext(writerMSP, "tx-missing")

	p := CertifiedCopyAnchorPayload{
		OriginalDocumentUid: testOrigUid,
		CertifiedCopyUid:    testCC1,
		FinalSignedPdfHash:  testHash,
		SignatureHash:       testSigHash,
		SignatureVersion:    "v1",
		SigningTimestamp:    "2026-07-19T10:00:00Z",
		// RequestId intentionally empty
	}
	b, _ := json.Marshal(p)
	if _, err := cc.AnchorCertifiedCopy(ctx, string(b)); err == nil {
		t.Fatal("expected error for missing requestId")
	}
}

func TestGetCertifiedCopyAnchor(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)

	ctx, _ := newTestContextOnStub(writerMSP, "tx-read", stub)
	rec, err := cc.GetCertifiedCopyAnchor(ctx, testCC1)
	if err != nil {
		t.Fatalf("GetCertifiedCopyAnchor: %v", err)
	}
	if rec.CertifiedCopyUid != testCC1 {
		t.Fatalf("unexpected record: %+v", rec)
	}

	if _, err := cc.GetCertifiedCopyAnchor(ctx, testCC2); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Verify tests
// ---------------------------------------------------------------------------

func TestVerifyDocumentHash_SpecificUid(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)
	ctx, _ := newTestContextOnStub(writerMSP, "tx-verify", stub)

	// MATCH (raw hex)
	res, err := cc.VerifyDocumentHash(ctx, testHash, testCC1, "")
	if err != nil || res.Status != verifyStatusMatch {
		t.Fatalf("match: res=%+v err=%v", res, err)
	}
	if res.MatchedCertifiedCopyUid != testCC1 || res.MatchedVersion != 1 || res.TxnId != "tx-cc1" {
		t.Fatalf("unexpected match result: %+v", res)
	}

	// MATCH (prefixed uppercase normalization)
	res, err = cc.VerifyDocumentHash(ctx, "sha256:"+strings.ToUpper(testHash), testCC1, "")
	if err != nil || res.Status != verifyStatusMatch {
		t.Fatalf("normalized match: res=%+v err=%v", res, err)
	}

	// MISMATCH
	res, err = cc.VerifyDocumentHash(ctx, zeroHash, testCC1, "")
	if err != nil || res.Status != verifyStatusMismatch {
		t.Fatalf("mismatch: res=%+v err=%v", res, err)
	}
	if res.AnchoredHash != "sha256:"+testHash {
		t.Fatalf("mismatch should carry anchoredHash: %+v", res)
	}

	// NOT_FOUND (unanchored version)
	res, err = cc.VerifyDocumentHash(ctx, testHash, testCC2, "")
	if err != nil || res.Status != verifyStatusNotFound {
		t.Fatalf("not found: res=%+v err=%v", res, err)
	}
}

func TestVerifyDocumentHash_VersionAware(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)
	mustAnchor(t, cc, stub, "tx-cc2", testOrigUid, testCC2, 2, testHash2, strptr(testCC1))
	ctx, _ := newTestContextOnStub(writerMSP, "tx-verify", stub)

	// Matches the OLDER version (CC1) even though CC2 is latest.
	res, err := cc.VerifyDocumentHash(ctx, testHash, "", testOrigUid)
	if err != nil || res.Status != verifyStatusMatch {
		t.Fatalf("version-aware match: res=%+v err=%v", res, err)
	}
	if res.MatchedCertifiedCopyUid != testCC1 || res.MatchedVersion != 1 {
		t.Fatalf("expected match on CC1, got %+v", res)
	}

	// Unknown hash across all versions -> MISMATCH.
	res, err = cc.VerifyDocumentHash(ctx, zeroHash, "", testOrigUid)
	if err != nil || res.Status != verifyStatusMismatch {
		t.Fatalf("version-aware mismatch: res=%+v err=%v", res, err)
	}

	// Unknown document -> NOT_FOUND.
	res, err = cc.VerifyDocumentHash(ctx, testHash, "", "MH:PUNE:SR42:2026:777")
	if err != nil || res.Status != verifyStatusNotFound {
		t.Fatalf("version-aware not-found: res=%+v err=%v", res, err)
	}
}

func TestVerifyDocumentHash_RequiresIdentifier(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext(writerMSP, "tx-verify")
	if _, err := cc.VerifyDocumentHash(ctx, testHash, "", ""); err == nil {
		t.Fatal("expected error when neither identifier is supplied")
	}
}

// ---------------------------------------------------------------------------
// Document history tests
// ---------------------------------------------------------------------------

func TestGetDocumentHistory(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-cc1")
	mustAnchor(t, cc, stub, "tx-cc1", testOrigUid, testCC1, 1, testHash, nil)
	mustAnchor(t, cc, stub, "tx-cc2", testOrigUid, testCC2, 2, testHash2, strptr(testCC1))
	ctx, _ := newTestContextOnStub(writerMSP, "tx-hist", stub)

	hist, err := cc.GetDocumentHistory(ctx, testOrigUid)
	if err != nil {
		t.Fatalf("GetDocumentHistory: %v", err)
	}
	if hist.LatestCertifiedCopyUid != testCC2 || len(hist.Versions) != 2 {
		t.Fatalf("unexpected history: %+v", hist)
	}
	if hist.Versions[0].CertifiedCopyVersion != 1 || hist.Versions[1].CertifiedCopyVersion != 2 {
		t.Fatalf("versions not ordered: %+v", hist.Versions)
	}

	if _, err := cc.GetDocumentHistory(ctx, "MH:PUNE:SR42:2026:777"); err == nil {
		t.Fatal("expected not-found error for unknown document")
	}
}

// ---------------------------------------------------------------------------
// Loan tests
// ---------------------------------------------------------------------------

func TestGetLoanState_NotFound(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext(writerMSP, "tx-loan-read")
	state, err := cc.GetLoanState(ctx, testOrigUid)
	if err != nil {
		t.Fatalf("GetLoanState: %v", err)
	}
	if state.Status != loanStatusNotFound {
		t.Fatalf("status = %q, want NOT_FOUND", state.Status)
	}
}

func TestRecordNOIApproval_SetsActive(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-noi")
	ctx, _ := newTestContextOnStub(writerMSP, "tx-noi", stub)

	resp, err := cc.RecordNOIApproval(ctx, noiPayload(testOrigUid))
	if err != nil {
		t.Fatalf("RecordNOIApproval: %v", err)
	}
	if resp.Status != loanStatusActive || resp.TxnId != "tx-noi" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	ctxRead, _ := newTestContextOnStub(writerMSP, "tx-read", stub)
	state, _ := cc.GetLoanState(ctxRead, testOrigUid)
	if state.Status != loanStatusActive || state.NoiApplicationId != "NOI-2026-777" || state.ApprovedBySroRef != "sro-officer-ref" {
		t.Fatalf("unexpected loan state: %+v", state)
	}

	hist, _ := cc.GetLoanHistory(ctxRead, testOrigUid)
	if len(hist.Transitions) != 1 {
		t.Fatalf("history len = %d, want 1", len(hist.Transitions))
	}
	e := hist.Transitions[0]
	if e.FromState != loanStatusNotFound || e.ToState != loanStatusActive || e.Seq != 1 {
		t.Fatalf("unexpected history entry: %+v", e)
	}
	if len(stub.events) != 1 || stub.events[0] != eventNOIApproved {
		t.Fatalf("events = %v, want [%s]", stub.events, eventNOIApproved)
	}
}

func TestRecordNOIApproval_DoubleReject(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-noi")
	ctx, _ := newTestContextOnStub(writerMSP, "tx-noi", stub)
	if _, err := cc.RecordNOIApproval(ctx, noiPayload(testOrigUid)); err != nil {
		t.Fatalf("first approval: %v", err)
	}

	ctx2, _ := newTestContextOnStub(writerMSP, "tx-noi-2", stub)
	if _, err := cc.RecordNOIApproval(ctx2, noiPayload(testOrigUid)); err == nil || !strings.Contains(err.Error(), "already ACTIVE_LOAN") {
		t.Fatalf("expected double-activation rejection, got %v", err)
	}
}

func TestRecordLoanSatisfaction_SetsNoLoan(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-noi")
	ctxNoi, _ := newTestContextOnStub(writerMSP, "tx-noi", stub)
	if _, err := cc.RecordNOIApproval(ctxNoi, noiPayload(testOrigUid)); err != nil {
		t.Fatalf("approval: %v", err)
	}

	ctxNoc, _ := newTestContextOnStub(writerMSP, "tx-noc", stub)
	resp, err := cc.RecordLoanSatisfaction(ctxNoc, nocPayload(testOrigUid))
	if err != nil {
		t.Fatalf("RecordLoanSatisfaction: %v", err)
	}
	if resp.Status != loanStatusNoLoan {
		t.Fatalf("unexpected response: %+v", resp)
	}

	ctxRead, _ := newTestContextOnStub(writerMSP, "tx-read", stub)
	state, _ := cc.GetLoanState(ctxRead, testOrigUid)
	if state.Status != loanStatusNoLoan || state.NocReferenceId != "NOC-2026-555" {
		t.Fatalf("unexpected loan state: %+v", state)
	}

	hist, _ := cc.GetLoanHistory(ctxRead, testOrigUid)
	if len(hist.Transitions) != 2 {
		t.Fatalf("history len = %d, want 2", len(hist.Transitions))
	}
	if hist.Transitions[1].FromState != loanStatusActive || hist.Transitions[1].ToState != loanStatusNoLoan || hist.Transitions[1].Seq != 2 {
		t.Fatalf("unexpected transition: %+v", hist.Transitions[1])
	}
}

func TestRecordLoanSatisfaction_RejectWhenNotActive(t *testing.T) {
	cc := &IgrAnchorChaincode{}

	// Fresh document (NOT_FOUND) -> reject.
	ctx, stub := newTestContext(writerMSP, "tx-noc")
	if _, err := cc.RecordLoanSatisfaction(ctx, nocPayload(testOrigUid)); err == nil {
		t.Fatal("expected rejection on NOT_FOUND loan")
	}

	// After satisfaction (NO_LOAN) -> reject again.
	ctxNoi, _ := newTestContextOnStub(writerMSP, "tx-noi", stub)
	_, _ = cc.RecordNOIApproval(ctxNoi, noiPayload(testOrigUid))
	ctxNoc, _ := newTestContextOnStub(writerMSP, "tx-noc-1", stub)
	_, _ = cc.RecordLoanSatisfaction(ctxNoc, nocPayload(testOrigUid))
	ctxNoc2, _ := newTestContextOnStub(writerMSP, "tx-noc-2", stub)
	if _, err := cc.RecordLoanSatisfaction(ctxNoc2, nocPayload(testOrigUid)); err == nil {
		t.Fatal("expected rejection when already NO_LOAN")
	}
}

func TestLoanReLoanAfterSatisfaction(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	stub := newMemoryStub("tx-noi")
	ctxNoi, _ := newTestContextOnStub(writerMSP, "tx-noi", stub)
	_, _ = cc.RecordNOIApproval(ctxNoi, noiPayload(testOrigUid))
	ctxNoc, _ := newTestContextOnStub(writerMSP, "tx-noc", stub)
	_, _ = cc.RecordLoanSatisfaction(ctxNoc, nocPayload(testOrigUid))

	// NO_LOAN -> ACTIVE_LOAN allowed again.
	ctxNoi2, _ := newTestContextOnStub(writerMSP, "tx-noi-2", stub)
	if _, err := cc.RecordNOIApproval(ctxNoi2, noiPayload(testOrigUid)); err != nil {
		t.Fatalf("re-loan after NO_LOAN should be allowed: %v", err)
	}
	ctxRead, _ := newTestContextOnStub(writerMSP, "tx-read", stub)
	state, _ := cc.GetLoanState(ctxRead, testOrigUid)
	if state.Status != loanStatusActive {
		t.Fatalf("status = %q, want ACTIVE_LOAN", state.Status)
	}
	hist, _ := cc.GetLoanHistory(ctxRead, testOrigUid)
	if len(hist.Transitions) != 3 {
		t.Fatalf("history len = %d, want 3", len(hist.Transitions))
	}
}

func TestLoanWrites_RejectWrongMSP(t *testing.T) {
	cc := &IgrAnchorChaincode{}

	ctx, _ := newTestContext("IGRBankMSP", "tx-noi")
	if _, err := cc.RecordNOIApproval(ctx, noiPayload(testOrigUid)); err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected MSP rejection for NOI approval, got %v", err)
	}
	ctx2, _ := newTestContext("IGRBankMSP", "tx-noc")
	if _, err := cc.RecordLoanSatisfaction(ctx2, nocPayload(testOrigUid)); err == nil || !strings.Contains(err.Error(), "not authorized") {
		t.Fatalf("expected MSP rejection for loan finish, got %v", err)
	}
}

func TestGetLoanHistory_EmptyInitially(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext(writerMSP, "tx-hist")
	hist, err := cc.GetLoanHistory(ctx, testOrigUid)
	if err != nil {
		t.Fatalf("GetLoanHistory: %v", err)
	}
	if len(hist.Transitions) != 0 {
		t.Fatalf("expected empty history, got %+v", hist.Transitions)
	}
}

func TestGetLoanState_RejectsBadOriginalUid(t *testing.T) {
	cc := &IgrAnchorChaincode{}
	ctx, _ := newTestContext(writerMSP, "tx-bad")
	if _, err := cc.GetLoanState(ctx, "2026SRO42DOC991"); err == nil {
		t.Fatal("expected invalid originalDocumentUid error")
	}
}

// ---------------------------------------------------------------------------
// Unit: hash validation
// ---------------------------------------------------------------------------

func TestValidateSha256Hash_Normalization(t *testing.T) {
	normalized, err := validateSha256Hash(strings.ToUpper(testHash))
	if err != nil {
		t.Fatalf("validateSha256Hash: %v", err)
	}
	if normalized != "sha256:"+testHash {
		t.Fatalf("normalized = %q", normalized)
	}
}
