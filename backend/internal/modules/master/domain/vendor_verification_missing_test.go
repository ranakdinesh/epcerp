package domain

import (
	"strings"
	"testing"
)

func TestMissingSupplierContentBlocksPreviouslyAcceptedEvidence(t *testing.T) {
	s := VendorVerificationSnapshot{Active: true, Documents: []VendorVerificationEvidence{{ID: "pan", DocType: "pan", Previewable: false, ApprovalKnown: true, Approval: "accept", ApprovalComment: "Reviewed"}, {ID: "cheque", DocType: "cheque", Previewable: true, ApprovalKnown: true, Approval: "accept", ApprovalComment: "Reviewed"}}}
	status := VendorVerificationEvaluate(s, &VendorVerificationReview{Fingerprint: "same"}, "same")
	if status.State != "blocked" || status.CanOrder || !strings.Contains(status.Message, "missing or unreadable") {
		t.Fatalf("missing evidence not blocked: %+v", status)
	}
	if VendorVerificationRequirePurchaseOrder(status) == nil {
		t.Fatal("PO gate bypassed")
	}
	s.Documents[0].Previewable = true
	status = VendorVerificationEvaluate(s, &VendorVerificationReview{Fingerprint: "same"}, "same")
	if !status.CanOrder {
		t.Fatal("restored approved evidence remains blocked")
	}
}
