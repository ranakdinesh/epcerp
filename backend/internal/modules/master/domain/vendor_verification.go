package domain

import (
	"strings"
	"time"
)

type VendorVerificationEvidence struct {
	ID, DocType, SHA256       string
	Version                   int32
	Previewable               bool
	ApprovalKnown             bool
	Approval, ApprovalComment string
}
type VendorVerificationSnapshot struct {
	ID                    string
	Version               int32
	Active, GSTRegistered bool
	Documents             []VendorVerificationEvidence
}
type VendorVerificationDecision struct {
	DocumentID string
	Decision   string
	Comment    string
}
type VendorVerificationReview struct {
	SupplierID  string
	Version     int32
	Fingerprint string
	Decisions   []VendorVerificationDecision
	Reason      string
	UpdatedAt   time.Time
	UpdatedBy   *string
}
type VendorVerificationCommand struct {
	Version     int32
	Fingerprint string
	Decisions   []VendorVerificationDecision
	Reason      string
}
type VendorVerificationStatus struct {
	State        string
	Message      string
	MissingTypes []string
	CanInvite    bool
	CanOrder     bool
	Fingerprint  string
	Review       *VendorVerificationReview
}

var VendorVerificationMissing = &Error{Kind: NotFound, Detail: "Supplier not found"}
var VendorVerificationForbidden = &Error{Kind: AccessDenied, Detail: "Insufficient permission"}
var VendorVerificationUnauthorized = &Error{Kind: Unauthenticated, Detail: "Invalid or expired credentials"}
var VendorVerificationStale = &Error{Kind: Conflict, Detail: "Supplier evidence or review changed; refresh and review again"}

func VendorVerificationRequired(s VendorVerificationSnapshot) []string {
	found := map[string]bool{}
	for _, d := range s.Documents {
		found[d.DocType] = true
	}
	missing := []string{}
	for _, kind := range []string{"pan", "cheque"} {
		if !found[kind] {
			missing = append(missing, kind)
		}
	}
	if s.GSTRegistered && !found["gst"] {
		missing = append(missing, "gst")
	}
	return missing
}
func VendorVerificationValidate(s VendorVerificationSnapshot, c VendorVerificationCommand) error {
	if !s.Active {
		return &Error{Kind: InvalidInput, Detail: "Inactive or suspended supplier cannot be verified"}
	}
	if strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 2000 {
		return VendorVerificationFieldError("reason", "Review reason is required (maximum 2000 bytes)")
	}
	return vendorVerificationValidateDecisions(s, c.Decisions)
}

// Readiness validates persisted document decisions, not metadata required for a new manual command.
func vendorVerificationValidateDecisions(s VendorVerificationSnapshot, decisions []VendorVerificationDecision) error {
	expected := map[string]bool{}
	for _, d := range s.Documents {
		expected[d.ID] = d.Previewable
	}
	seen := map[string]bool{}
	for _, d := range decisions {
		preview, ok := expected[d.DocumentID]
		if !ok || seen[d.DocumentID] {
			return &Error{Kind: InvalidInput, Detail: "Review only active documents without duplicates"}
		}
		seen[d.DocumentID] = true
		if d.Decision != "accept" && d.Decision != "reject" {
			return VendorVerificationFieldError("decision_"+d.DocumentID, "Choose accept or reject for every document")
		}
		if strings.TrimSpace(d.Comment) == "" || len(d.Comment) > 2000 {
			return VendorVerificationFieldError("comment_"+d.DocumentID, "A review comment is required for every document")
		}
		if d.Decision == "accept" && !preview {
			return VendorVerificationFieldError("decision_"+d.DocumentID, "This file cannot be previewed. Choose Replace on this document and upload a readable PDF or image.")
		}
	}
	if len(seen) == 0 {
		return &Error{Kind: InvalidInput, Detail: "Choose a document to review"}
	}
	return nil
}
func VendorVerificationEvaluate(s VendorVerificationSnapshot, r *VendorVerificationReview, fingerprint string) VendorVerificationStatus {
	// Project current per-document decisions independently of unrelated uploads.
	effective := VendorVerificationReview{Fingerprint: fingerprint}
	if r != nil {
		effective = *r
		effective.Fingerprint = fingerprint
	}
	effective.Decisions = nil
	for _, d := range s.Documents {
		if d.ApprovalKnown {
			if d.Approval == "accept" || d.Approval == "reject" {
				effective.Decisions = append(effective.Decisions, VendorVerificationDecision{DocumentID: d.ID, Decision: d.Approval, Comment: d.ApprovalComment})
			}
		} else if r != nil && r.Fingerprint == fingerprint {
			for _, decision := range r.Decisions {
				if decision.DocumentID == d.ID {
					effective.Decisions = append(effective.Decisions, decision)
				}
			}
		}
	}
	if r != nil || len(effective.Decisions) > 0 {
		r = &effective
	}
	result := VendorVerificationStatus{State: "unverified", Message: "Finance must verify all active documents before a purchase order", MissingTypes: VendorVerificationRequired(s), CanInvite: s.Active, Fingerprint: fingerprint, Review: r}
	if !s.Active {
		result.State = "blocked"
		result.Message = "Supplier is inactive or suspended"
		return result
	}
	if len(result.MissingTypes) > 0 {
		result.State = "blocked"
		result.Message = "Required evidence missing: " + strings.Join(result.MissingTypes, ", ")
		return result
	}
	for _, d := range s.Documents {
		if !d.Previewable {
			result.State = "blocked"
			result.Message = "Supplier document content is missing or unreadable; restore or replace the file before verification"
			return result
		}
	}
	if r == nil || r.Fingerprint != fingerprint {
		return result
	}
	if vendorVerificationValidateDecisions(s, r.Decisions) != nil {
		return result
	}
	for _, d := range r.Decisions {
		if d.Decision == "reject" {
			result.State = "rejected"
			result.Message = "Finance rejected supplier evidence; resolve the document review"
			return result
		}
	}
	// Saved document decisions are progress, never approval of the whole vendor.
	if len(r.Decisions) != len(s.Documents) {
		result.Message = "Finance document review in progress"
		return result
	}
	result.State = "verified"
	result.Message = "All active supplier documents verified by Finance"
	result.CanOrder = true
	return result
}
func VendorVerificationRequirePurchaseOrder(status VendorVerificationStatus) error {
	if !status.CanOrder {
		return &Error{Kind: Conflict, Detail: "Cannot generate purchase order: " + status.Message}
	}
	return nil
}

func VendorVerificationFieldError(field, message string) error {
	return &Error{Kind: InvalidInput, Detail: message, Field: field}
}

var VendorVerificationErrTransactionRequired = &Error{Kind: InternalFailure, Detail: "Purchase-order gate requires the caller transaction"}
