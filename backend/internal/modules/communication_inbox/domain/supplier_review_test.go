package domain

import (
	"testing"
	"time"
)

func TestSupplierAssignmentNoticeSource(t *testing.T) {
	id := "11111111-1111-4111-8111-111111111111"
	recipient := "22222222-2222-4222-8222-222222222222"
	source := Source{EventID: id, EntityID: id, ActorID: id, Module: "supplier_review_assignments", EntityType: "supplier_review_assignment", Action: "assigned", ScopeKind: "platform", Visibility: "restricted", Version: 1, At: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)}
	link := "/dashboard?screen=suppliers&supplier_id=" + id
	notice, err := AddressedSupplierReview(id, source, recipient, "Review supplier", link)
	if err != nil || notice.RecipientID != recipient || notice.Kind != "supplier_review" || notice.ProjectID != "" {
		t.Fatal("valid notice rejected", err)
	}
	for _, mode := range []string{"module", "scope", "action", "recipient", "link"} {
		s, r, l := source, recipient, link
		switch mode {
		case "module":
			s.Module = "quotation_approvals"
		case "scope":
			s.ProjectID = id
		case "action":
			s.Action = "reviewed"
		case "recipient":
			r = "invalid"
		case "link":
			l = "https://outside.invalid"
		}
		if _, err := AddressedSupplierReview(id, s, r, "Review supplier", l); err == nil {
			t.Fatal("invalid notice accepted", mode)
		}
	}
}
