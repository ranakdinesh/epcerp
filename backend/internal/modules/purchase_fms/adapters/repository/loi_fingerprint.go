package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type LOIFingerprint struct{}

func loiContentDigest(value any) (string, error) {
	b, e := json.Marshal(value)
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}
func (LOIFingerprint) Approval(a domain.QuotationApprovalsAttempt) (string, error) {
	return loiContentDigest(a)
}
func (LOIFingerprint) Content(c domain.LOIContent) (string, error) { return loiContentDigest(c) }
