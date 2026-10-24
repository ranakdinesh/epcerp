package domain

import "testing"

func TestClientPOSignoffRequiresSubmissionPermissionAndExactRevision(t *testing.T) {
	a, c, s, d, now := ordersFixture()
	c.RecordClient = true
	o, e := OrdersApply(nil, a, c, OrdersCommand{Action: "prepare", Reason: "Prepare actual terms", Details: &d}, s, now, ordersUid(9), "")
	if e != nil {
		t.Fatal(e)
	}
	run := func(q OrdersCommand) {
		t.Helper()
		q.Version = o.Version
		q.Reason = "Actual external evidence"
		next, e := OrdersApply(&o, a, c, q, s, now, ordersUid(9), "")
		if e != nil {
			t.Fatal(e)
		}
		o = next
	}
	if _, e = OrdersApply(&o, a, c, OrdersCommand{Version: o.Version, Action: "release", Reason: "Missing client signoff"}, s, now, ordersUid(9), "PO-real"); e == nil {
		t.Fatal("released unsigned draft")
	}
	decision := OrdersClientDecision{Status: "approved", Reference: "CLIENT-PO", Date: OrdersToday(now), Signatory: "Actual signatory", Designation: "Client engineer", FileID: ordersUid(17)}
	if _, e = OrdersApply(&o, a, c, OrdersCommand{Version: o.Version, Action: "client_decision", Reason: "Unsubmitted", ClientDecision: &decision}, s, now, ordersUid(9), ""); e == nil {
		t.Fatal("approved before submission")
	}
	run(OrdersCommand{Action: "submit_client", Submission: &OrdersClientSubmission{Reference: "SUB-1", Date: OrdersToday(now)}})
	denied := c
	denied.RecordClient = false
	if _, e = OrdersApply(&o, a, denied, OrdersCommand{Version: o.Version, Action: "client_decision", Reason: "No permission", ClientDecision: &decision}, s, now, ordersUid(9), ""); e != OrdersForbidden {
		t.Fatal("missing explicit permission", e)
	}
	run(OrdersCommand{Action: "client_decision", ClientDecision: &decision})
	if !OrdersClientCleared(o) {
		t.Fatal("signed draft not cleared")
	}
	original := OrdersCopy(o)
	original.ClientApproval.Decision.Reference = "mutated"
	if o.ClientApproval.Decision.Reference == "mutated" {
		t.Fatal("copy aliases client evidence")
	}
	run(OrdersCommand{Action: "prepare", Details: &d})
	if OrdersClientCleared(o) || o.DraftRevision != 2 || o.ClientApproval.Decision != nil {
		t.Fatal("edited draft retained signoff")
	}
	run(OrdersCommand{Action: "submit_client", Submission: &OrdersClientSubmission{Reference: "SUB-2", Date: OrdersToday(now)}})
	decision.Status = "rejected"
	decision.FileID = ""
	run(OrdersCommand{Action: "client_decision", ClientDecision: &decision})
	if OrdersClientCleared(o) || o.ClientApproval.Status != "rejected" {
		t.Fatal("rejection did not return the draft")
	}
	run(OrdersCommand{Action: "prepare", Details: &d})
	run(OrdersCommand{Action: "submit_client", Submission: &OrdersClientSubmission{Reference: "SUB-3", Date: OrdersToday(now)}})
	decision.Status = "approved"
	decision.FileID = ordersUid(18)
	run(OrdersCommand{Action: "client_decision", ClientDecision: &decision})
	released, e := OrdersApply(&o, a, c, OrdersCommand{Version: o.Version, Action: "release", Reason: "Release current client-signed draft"}, s, now, ordersUid(9), "PO-actual")
	if e != nil || released.State != "released" {
		t.Fatal(e)
	}
}
