package dtos

import (
	"time"

	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorVerificationDecisionDTO struct {
	DocumentID string `json:"document_id"`
	Decision   string `json:"decision"`
	Comment    string `json:"comment"`
}

func VendorVerificationDecisionDTOFromDomain(v domain.VendorVerificationDecision) VendorVerificationDecisionDTO {
	return VendorVerificationDecisionDTO{
		DocumentID: v.DocumentID,
		Decision:   v.Decision,
		Comment:    v.Comment,
	}
}
func (v VendorVerificationDecisionDTO) Domain() domain.VendorVerificationDecision {
	return domain.VendorVerificationDecision{
		DocumentID: v.DocumentID,
		Decision:   v.Decision,
		Comment:    v.Comment,
	}
}

type VendorVerificationReviewDTO struct {
	SupplierID  string                          `json:"supplier_id"`
	Version     int32                           `json:"version"`
	Fingerprint string                          `json:"fingerprint"`
	Decisions   []VendorVerificationDecisionDTO `json:"decisions"`
	Reason      string                          `json:"reason"`
	UpdatedAt   time.Time                       `json:"reviewed_at"`
	UpdatedBy   *string                         `json:"reviewed_by_id"`
}

func VendorVerificationReviewDTOFromDomain(v domain.VendorVerificationReview) VendorVerificationReviewDTO {
	return VendorVerificationReviewDTO{
		SupplierID:  v.SupplierID,
		Version:     v.Version,
		Fingerprint: v.Fingerprint,
		Decisions:   dtoSlice(v.Decisions, VendorVerificationDecisionDTOFromDomain),
		Reason:      v.Reason,
		UpdatedAt:   v.UpdatedAt,
		UpdatedBy:   v.UpdatedBy,
	}
}
func (v VendorVerificationReviewDTO) Domain() domain.VendorVerificationReview {
	return domain.VendorVerificationReview{
		SupplierID:  v.SupplierID,
		Version:     v.Version,
		Fingerprint: v.Fingerprint,
		Decisions:   dtoSlice(v.Decisions, VendorVerificationDecisionDTO.Domain),
		Reason:      v.Reason,
		UpdatedAt:   v.UpdatedAt,
		UpdatedBy:   v.UpdatedBy,
	}
}
