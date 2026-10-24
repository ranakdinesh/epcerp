package dtos

import (
	domain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	"time"
)

type VendorReviewAssignmentRequest struct {
	UserID  string `json:"user_id"`
	Version int32  `json:"version"`
	Active  *bool  `json:"is_active"`
	Reason  string `json:"reason"`
}

func (v VendorReviewAssignmentRequest) Domain() (domain.VendorReviewAssignmentInput, error) {
	if v.Active == nil {
		return domain.VendorReviewAssignmentInput{}, domain.VendorVerificationFieldError("is_active", "Assignment status is required")
	}
	return domain.VendorReviewAssignmentInput{UserID: v.UserID, Version: v.Version, Active: *v.Active, Reason: v.Reason}, nil
}

type VendorReviewAssignmentResponse struct {
	ID           string    `json:"id"`
	SupplierID   string    `json:"supplier_id"`
	UserID       string    `json:"user_id"`
	ReviewerName string    `json:"reviewer_name"`
	Version      int32     `json:"version"`
	Active       bool      `json:"is_active"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func VendorReviewAssignment(v *domain.VendorReviewAssignment) *VendorReviewAssignmentResponse {
	if v == nil {
		return nil
	}
	return &VendorReviewAssignmentResponse{v.ID, v.SupplierID, v.UserID, v.ReviewerName, v.Version, v.Active, v.UpdatedAt}
}

type VendorReviewerResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
