package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// originalDocumentUid: MH:<District>:<SRO>:<Year>:<DocNo> (ADR 002 v3).
	originalDocumentUidPattern = regexp.MustCompile(`^MH:[A-Za-z0-9_]+:SR[0-9]+:[0-9]{4}:[0-9]+$`)
	// certifiedCopyUid: <originalDocumentUid>:CC<version>.
	certifiedCopyUidPattern = regexp.MustCompile(`^MH:[A-Za-z0-9_]+:SR[0-9]+:[0-9]{4}:[0-9]+:CC[0-9]+$`)
	// certifiedCopy version suffix.
	ccVersionPattern = regexp.MustCompile(`:CC([0-9]+)$`)
)

// validateOriginalDocumentUid checks the registered-document identifier format.
func validateOriginalDocumentUid(originalDocumentUid string) error {
	if originalDocumentUid == "" {
		return fmt.Errorf("originalDocumentUid cannot be empty")
	}
	if !originalDocumentUidPattern.MatchString(originalDocumentUid) {
		return fmt.Errorf("originalDocumentUid %q has invalid format", originalDocumentUid)
	}
	return nil
}

// parseCertifiedCopyVersion extracts the positive integer version from a
// certifiedCopyUid's :CC<n> suffix.
func parseCertifiedCopyVersion(certifiedCopyUid string) (int, error) {
	m := ccVersionPattern.FindStringSubmatch(certifiedCopyUid)
	if m == nil {
		return 0, fmt.Errorf("certifiedCopyUid %q has invalid version suffix", certifiedCopyUid)
	}
	version, err := strconv.Atoi(m[1])
	if err != nil || version < 1 {
		return 0, fmt.Errorf("certifiedCopyUid %q has invalid version suffix", certifiedCopyUid)
	}
	return version, nil
}

// validateCertifiedCopyUid checks format and that the uid equals
// <originalDocumentUid>:CC<version>, returning the parsed version.
func validateCertifiedCopyUid(certifiedCopyUid, originalDocumentUid string) (int, error) {
	if certifiedCopyUid == "" {
		return 0, fmt.Errorf("certifiedCopyUid cannot be empty")
	}
	if !certifiedCopyUidPattern.MatchString(certifiedCopyUid) {
		return 0, fmt.Errorf("certifiedCopyUid %q has invalid format", certifiedCopyUid)
	}
	version, err := parseCertifiedCopyVersion(certifiedCopyUid)
	if err != nil {
		return 0, err
	}
	expected := fmt.Sprintf("%s:CC%d", originalDocumentUid, version)
	if certifiedCopyUid != expected {
		return 0, fmt.Errorf("certifiedCopyUid %q does not match originalDocumentUid %q", certifiedCopyUid, originalDocumentUid)
	}
	return version, nil
}

// originalFromCertifiedCopyUid strips the :CC<n> suffix. Returns "" if absent.
func originalFromCertifiedCopyUid(certifiedCopyUid string) string {
	idx := strings.LastIndex(certifiedCopyUid, ":CC")
	if idx < 0 {
		return ""
	}
	return certifiedCopyUid[:idx]
}

// validateSha256Hash enforces ADR 004: accept `sha256:<64hex>` or raw 64 hex,
// reject base64/paths/wrong-length, and normalize to `sha256:<lowercase hex>`.
func validateSha256Hash(hash string) (string, error) {
	if hash == "" {
		return "", fmt.Errorf("hash cannot be empty")
	}
	if strings.ContainsAny(hash, "+/=") {
		return "", fmt.Errorf("hash must be hex, not base64")
	}
	if strings.Contains(hash, "/") || strings.Contains(hash, "\\") {
		return "", fmt.Errorf("hash must not be a file path")
	}

	raw := hash
	if strings.HasPrefix(strings.ToLower(hash), "sha256:") {
		raw = hash[len("sha256:"):]
	}
	if len(raw) != 64 {
		return "", fmt.Errorf("hash must be 64 hex characters")
	}
	for _, c := range raw {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", fmt.Errorf("hash contains non-hex characters")
		}
	}
	return "sha256:" + strings.ToLower(raw), nil
}

// checkLen enforces a maximum length on an opaque string field.
func checkLen(field, val string, max int) error {
	if len(val) > max {
		return fmt.Errorf("%s exceeds maximum length %d", field, max)
	}
	return nil
}

// requireNonEmpty returns an error if val is empty.
func requireNonEmpty(field, val string) error {
	if val == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}
