package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// AnchorCertifiedCopy anchors the hash of a final SRO-signed certified copy.
//
// It creates the immutable UID~<certifiedCopyUid> record and updates the
// DOC~<originalDocumentUid> pointer (incrementing the version counter). It is
// idempotent on certifiedCopyUid: a retry with the same finalSignedPdfHash
// returns the existing txnId; a retry with a different hash is rejected.
func (cc *IgrAnchorChaincode) AnchorCertifiedCopy(ctx contractapi.TransactionContextInterface, payloadJSON string) (*AnchorResponse, error) {
	var p CertifiedCopyAnchorPayload
	if err := json.Unmarshal([]byte(payloadJSON), &p); err != nil {
		return nil, fmt.Errorf("invalid anchor payload: %v", err)
	}

	if err := requireWriterMSP(ctx); err != nil {
		return nil, err
	}

	// Identifier validation.
	if err := validateOriginalDocumentUid(p.OriginalDocumentUid); err != nil {
		return nil, err
	}
	version, err := validateCertifiedCopyUid(p.CertifiedCopyUid, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	if p.CertifiedCopyVersion != 0 && p.CertifiedCopyVersion != version {
		return nil, fmt.Errorf("certifiedCopyVersion %d does not match certifiedCopyUid version %d", p.CertifiedCopyVersion, version)
	}

	// Hash validation (ADR 004) — both the document and signature hashes.
	finalHash, err := validateSha256Hash(p.FinalSignedPdfHash)
	if err != nil {
		return nil, fmt.Errorf("finalSignedPdfHash: %v", err)
	}
	signatureHash, err := validateSha256Hash(p.SignatureHash)
	if err != nil {
		return nil, fmt.Errorf("signatureHash: %v", err)
	}

	// Required non-identifier fields.
	if err := requireNonEmpty("requestId", p.RequestId); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("signatureVersion", p.SignatureVersion); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("signingTimestamp", p.SigningTimestamp); err != nil {
		return nil, err
	}

	// Length caps on opaque strings.
	for _, c := range []struct {
		field string
		val   string
		max   int
	}{
		{"requestId", p.RequestId, maxRequestIdLen},
		{"signatureVersion", p.SignatureVersion, maxSignatureVerLen},
		{"signingTimestamp", p.SigningTimestamp, maxTimestampLen},
		{"documentNo", p.DocumentNo, maxDocumentNoLen},
		{"district", p.District, maxDistrictLen},
		{"sroOffice", p.SroOffice, maxSroOfficeLen},
		{"documentType", p.DocumentType, maxDocumentTypeLen},
		{"sroId", p.SroId, maxSroIdLen},
		{"signingAuthorityRef", p.SigningAuthorityRef, maxRefLen},
	} {
		if err := checkLen(c.field, c.val, c.max); err != nil {
			return nil, err
		}
	}

	// Idempotency on certifiedCopyUid.
	existing, exists, err := loadAnchor(ctx, p.CertifiedCopyUid)
	if err != nil {
		return nil, err
	}
	if exists {
		if existing.FinalSignedPdfHash == finalHash {
			return &AnchorResponse{
				Status:           anchorStatusAnchored,
				CertifiedCopyUid: existing.CertifiedCopyUid,
				TxnId:            existing.TxnId,
				AnchorTimestamp:  existing.AnchorTimestamp,
			}, nil
		}
		return nil, fmt.Errorf("certifiedCopyUid %q already anchored with a different hash", p.CertifiedCopyUid)
	}

	// Version continuity: this must be exactly the next version, and
	// previousVersionRef must point at the current latest issuance.
	pointer, ptrExists, err := loadDocPointer(ctx, p.OriginalDocumentUid)
	if err != nil {
		return nil, err
	}
	expectedVersion := 1
	previousVersionRef := ""
	if ptrExists {
		expectedVersion = pointer.VersionCount + 1
		previousVersionRef = pointer.LatestCertifiedCopyUid
	}
	if version != expectedVersion {
		return nil, fmt.Errorf("certifiedCopyUid version %d is not the expected next version %d", version, expectedVersion)
	}
	// If the caller supplied previousVersionRef it must match the ledger.
	if p.PreviousVersionRef != nil {
		if previousVersionRef == "" || *p.PreviousVersionRef != previousVersionRef {
			return nil, fmt.Errorf("previousVersionRef %q does not match ledger state", *p.PreviousVersionRef)
		}
	}

	now, err := txTimestampRFC3339(ctx)
	if err != nil {
		return nil, err
	}
	txID := ctx.GetStub().GetTxID()

	record := &AnchorRecord{
		OriginalDocumentUid:  p.OriginalDocumentUid,
		CertifiedCopyUid:     p.CertifiedCopyUid,
		CertifiedCopyVersion: version,
		DocumentNo:           p.DocumentNo,
		RegistrationYear:     p.RegistrationYear,
		District:             p.District,
		SroOffice:            p.SroOffice,
		DocumentType:         p.DocumentType,
		FinalSignedPdfHash:   finalHash,
		SignatureHash:        signatureHash,
		SignatureVersion:     p.SignatureVersion,
		RequestId:            p.RequestId,
		SroId:                p.SroId,
		SigningAuthorityRef:  p.SigningAuthorityRef,
		SigningTimestamp:     p.SigningTimestamp,
		AnchorTimestamp:      now,
		PreviousVersionRef:   previousVersionRef,
		TxnId:                txID,
	}
	if err := saveAnchor(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to store anchor record: %v", err)
	}

	updated := &DocPointer{
		OriginalDocumentUid:    p.OriginalDocumentUid,
		LatestCertifiedCopyUid: p.CertifiedCopyUid,
		VersionCount:           version,
		UpdatedAt:              now,
	}
	if err := saveDocPointer(ctx, updated); err != nil {
		return nil, fmt.Errorf("failed to store doc pointer: %v", err)
	}

	emitWriteEvent(ctx, eventCertifiedCopyAnchored, p.OriginalDocumentUid, p.CertifiedCopyUid, txID)

	return &AnchorResponse{
		Status:           anchorStatusAnchored,
		CertifiedCopyUid: p.CertifiedCopyUid,
		TxnId:            txID,
		AnchorTimestamp:  now,
	}, nil
}

// GetCertifiedCopyAnchor returns the immutable anchor record for one issuance.
func (cc *IgrAnchorChaincode) GetCertifiedCopyAnchor(ctx contractapi.TransactionContextInterface, certifiedCopyUid string) (*AnchorRecord, error) {
	if certifiedCopyUid == "" {
		return nil, fmt.Errorf("certifiedCopyUid cannot be empty")
	}
	if !certifiedCopyUidPattern.MatchString(certifiedCopyUid) {
		return nil, fmt.Errorf("certifiedCopyUid %q has invalid format", certifiedCopyUid)
	}
	record, exists, err := loadAnchor(ctx, certifiedCopyUid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("certifiedCopyUid %q does not exist", certifiedCopyUid)
	}
	return record, nil
}

// GetDocumentHistory returns every certified-copy version anchored for a
// document, ordered by version.
func (cc *IgrAnchorChaincode) GetDocumentHistory(ctx contractapi.TransactionContextInterface, originalDocumentUid string) (*DocumentHistoryResponse, error) {
	if err := validateOriginalDocumentUid(originalDocumentUid); err != nil {
		return nil, err
	}
	pointer, exists, err := loadDocPointer(ctx, originalDocumentUid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("originalDocumentUid %q does not exist", originalDocumentUid)
	}

	versions := make([]DocumentVersion, 0, pointer.VersionCount)
	for v := 1; v <= pointer.VersionCount; v++ {
		uid := fmt.Sprintf("%s:CC%d", originalDocumentUid, v)
		record, ok, err := loadAnchor(ctx, uid)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		versions = append(versions, DocumentVersion{
			CertifiedCopyUid:     record.CertifiedCopyUid,
			CertifiedCopyVersion: record.CertifiedCopyVersion,
			FinalSignedPdfHash:   record.FinalSignedPdfHash,
			TxnId:                record.TxnId,
			AnchorTimestamp:      record.AnchorTimestamp,
		})
	}

	return &DocumentHistoryResponse{
		OriginalDocumentUid:    originalDocumentUid,
		LatestCertifiedCopyUid: pointer.LatestCertifiedCopyUid,
		Versions:               versions,
	}, nil
}

// VerifyDocumentHash compares a computed hash against anchored certified copies.
//
//   - With certifiedCopyUid: compare against exactly that version.
//   - Without it (originalDocumentUid required): version-aware — match against
//     any anchored version of the document.
//
// Returns MATCH / MISMATCH / NOT_FOUND. PENDING_ANCHOR is an API/DB concern and
// never originates here.
func (cc *IgrAnchorChaincode) VerifyDocumentHash(ctx contractapi.TransactionContextInterface, computedHash, certifiedCopyUid, originalDocumentUid string) (*VerifyResult, error) {
	normalized, err := validateSha256Hash(computedHash)
	if err != nil {
		return nil, fmt.Errorf("computedHash: %v", err)
	}

	// Specific-version verification.
	if certifiedCopyUid != "" {
		if !certifiedCopyUidPattern.MatchString(certifiedCopyUid) {
			return nil, fmt.Errorf("certifiedCopyUid %q has invalid format", certifiedCopyUid)
		}
		record, exists, err := loadAnchor(ctx, certifiedCopyUid)
		if err != nil {
			return nil, err
		}
		if !exists {
			return &VerifyResult{Status: verifyStatusNotFound}, nil
		}
		if record.FinalSignedPdfHash == normalized {
			return &VerifyResult{
				Status:                  verifyStatusMatch,
				MatchedCertifiedCopyUid: record.CertifiedCopyUid,
				MatchedVersion:          record.CertifiedCopyVersion,
				AnchoredHash:            record.FinalSignedPdfHash,
				TxnId:                   record.TxnId,
			}, nil
		}
		return &VerifyResult{
			Status:                  verifyStatusMismatch,
			MatchedCertifiedCopyUid: record.CertifiedCopyUid,
			MatchedVersion:          record.CertifiedCopyVersion,
			AnchoredHash:            record.FinalSignedPdfHash,
			TxnId:                   record.TxnId,
		}, nil
	}

	// Version-aware verification across all versions of a document.
	if err := validateOriginalDocumentUid(originalDocumentUid); err != nil {
		return nil, fmt.Errorf("either certifiedCopyUid or a valid originalDocumentUid is required: %v", err)
	}
	pointer, exists, err := loadDocPointer(ctx, originalDocumentUid)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &VerifyResult{Status: verifyStatusNotFound}, nil
	}
	for v := 1; v <= pointer.VersionCount; v++ {
		uid := fmt.Sprintf("%s:CC%d", originalDocumentUid, v)
		record, ok, err := loadAnchor(ctx, uid)
		if err != nil {
			return nil, err
		}
		if ok && record.FinalSignedPdfHash == normalized {
			return &VerifyResult{
				Status:                  verifyStatusMatch,
				MatchedCertifiedCopyUid: record.CertifiedCopyUid,
				MatchedVersion:          record.CertifiedCopyVersion,
				AnchoredHash:            record.FinalSignedPdfHash,
				TxnId:                   record.TxnId,
			}, nil
		}
	}
	// Document exists but no version matches — mismatch (not fraud by itself).
	return &VerifyResult{Status: verifyStatusMismatch}, nil
}
