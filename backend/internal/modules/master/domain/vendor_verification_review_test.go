package domain

import (
	"testing"
)

func vendorVerificationFixture() (VendorVerificationSnapshot, VendorVerificationCommand) {
	s := VendorVerificationSnapshot{ID: "supplier", Version: 1, Active: true, Documents: []VendorVerificationEvidence{{ID: "pan", DocType: "pan", Previewable: true}, {ID: "cheque", DocType: "cheque", Previewable: true}}}
	c := VendorVerificationCommand{Fingerprint: "current", Reason: "Checked evidence", Decisions: []VendorVerificationDecision{{DocumentID: "pan", Decision: "accept", Comment: "Readable"}, {DocumentID: "cheque", Decision: "accept", Comment: "Matches supplier"}}}
	return s, c
}
func TestVendorVerificationReviewAndIndependentProcurementEligibility(t *testing.T) {
	s, c := vendorVerificationFixture()
	if e := VendorVerificationValidate(s, c); e != nil {
		t.Fatal(e)
	}
	unverified := VendorVerificationEvaluate(s, nil, "current")
	if !unverified.CanInvite || unverified.CanOrder || VendorVerificationRequirePurchaseOrder(unverified) == nil {
		t.Fatal("quotation and order gates conflated")
	}
	r := &VendorVerificationReview{Fingerprint: "current", Decisions: c.Decisions, Reason: c.Reason}
	v := VendorVerificationEvaluate(s, r, "current")
	if !v.CanOrder || VendorVerificationRequirePurchaseOrder(v) != nil {
		t.Fatal("valid approval blocked")
	}
	if VendorVerificationEvaluate(s, r, "changed").CanOrder {
		t.Fatal("stale evidence approved")
	}
	r.Decisions[0].Decision = "reject"
	if VendorVerificationEvaluate(s, r, "current").State != "rejected" {
		t.Fatal("rejection ignored")
	}
	s.Active = false
	if v = VendorVerificationEvaluate(s, r, "current"); v.CanInvite || v.CanOrder {
		t.Fatal("inactive supplier eligible")
	}
}
func TestVendorVerificationExactReviewAndRequiredEvidence(t *testing.T) {
	for _, mode := range []string{"duplicate", "unknown", "empty_comment", "empty_reason", "bad_decision", "invalid_file"} {
		t.Run(mode, func(t *testing.T) {
			s, c := vendorVerificationFixture()
			switch mode {
			case "duplicate":
				c.Decisions[1] = c.Decisions[0]
			case "unknown":
				c.Decisions[1].DocumentID = "another_supplier"
			case "empty_comment":
				c.Decisions[0].Comment = " "
			case "empty_reason":
				c.Reason = ""
			case "bad_decision":
				c.Decisions[0].Decision = "verified"
			case "invalid_file":
				s.Documents[0].Previewable = false
			}
			if VendorVerificationValidate(s, c) == nil {
				t.Fatal("invalid review accepted")
			}
		})
	}
}

func TestDocumentApprovalsPersistWithoutApprovingIncompleteVendor(t *testing.T) {
	s, c := vendorVerificationFixture()
	c.Decisions = c.Decisions[:1]
	if e := VendorVerificationValidate(s, c); e != nil {
		t.Fatal("document approval rejected", e)
	}
	review := &VendorVerificationReview{Fingerprint: "current", Decisions: c.Decisions, Reason: c.Reason}
	if state := VendorVerificationEvaluate(s, review, "current"); state.CanOrder || state.State == "verified" {
		t.Fatal("partial review approved vendor")
	}
	s.GSTRegistered = true
	if e := VendorVerificationValidate(s, c); e != nil {
		t.Fatal("missing GST prevented reviewing PAN", e)
	}
	if state := VendorVerificationEvaluate(s, review, "current"); state.CanOrder || len(state.MissingTypes) != 1 {
		t.Fatal("missing required evidence approved vendor")
	}
	c.Decisions = nil
	if VendorVerificationValidate(s, c) == nil {
		t.Fatal("empty review accepted")
	}
}

func TestCurrentDocumentApprovalsDetermineReadinessWithoutManualReview(t *testing.T) {
	for _, mode := range []string{"all_accepted", "pending", "rejected", "unreadable", "missing_gst", "inactive"} {
		t.Run(mode, func(t *testing.T) {
			s, command := vendorVerificationFixture()
			for i := range s.Documents {
				s.Documents[i].ApprovalKnown = true
				s.Documents[i].Approval = "accept"
				s.Documents[i].ApprovalComment = "Automatically approved by upload permission"
			}
			switch mode {
			case "pending":
				s.Documents[0].Approval = "pending"
			case "rejected":
				s.Documents[0].Approval = "reject"
			case "unreadable":
				s.Documents[0].Previewable = false
			case "missing_gst":
				s.GSTRegistered = true
			case "inactive":
				s.Active = false
			}
			for _, review := range []*VendorVerificationReview{nil, {Fingerprint: "outdated", Reason: ""}} {
				status := VendorVerificationEvaluate(s, review, "current")
				if status.CanOrder != (mode == "all_accepted") {
					t.Fatalf("readiness disagrees with current document decisions: %+v", status)
				}
			}
			command.Reason = ""
			if VendorVerificationValidate(s, command) == nil {
				t.Fatal("manual review command lost required reason")
			}
		})
	}
}
