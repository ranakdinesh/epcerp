package domain

import "time"

type VendorReviewAssignment struct {
	ID, SupplierID, UserID, ReviewerName string
	Version                              int32
	Active                               bool
	CreatedAt, UpdatedAt                 time.Time
	CreatedBy, UpdatedBy                 *string
}
type VendorReviewer struct {
	ID, Name string
	Eligible bool
}
type VendorReviewAssignmentInput struct {
	UserID  string
	Version int32
	Active  bool
	Reason  string
}

func VendorReviewAssignmentValidate(in VendorReviewAssignmentInput) error {
	if !VendorDocumentValidID(in.UserID) {
		return VendorVerificationFieldError("user_id", "Choose an active Finance reviewer")
	}
	if in.Version < 0 {
		return VendorVerificationStale
	}
	if !VendorDocumentValidReason(in.Reason) {
		return VendorVerificationFieldError("reason", "Assignment reason is required (maximum 2000 bytes)")
	}
	return nil
}

type VendorReviewTask struct{ SupplierID, Code, Name, State, Message, Link string }
