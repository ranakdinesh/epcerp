package domain

import (
	"testing"
)

func TestOrdersOrderProgressUsesActualIssuedEvidence(t *testing.T) {
	actor, access, source, _, now := ordersFixture()
	details := OrdersDetails{Date: "2026-10-11", IssuerName: "Issuer", IssuerAddress: "Issuer office", BillTo: "Client", DeliveryAddress: "Site"}
	order, e := OrdersApply(nil, actor, access, OrdersCommand{Action: "prepare", Reason: "Actual draft", Details: &details}, source, now, ordersUid(8), "")
	if e != nil {
		t.Fatal(e)
	}
	p := OrdersProgressFact(&order, source.ProjectID, source.PackageID)
	if p.State != "in_progress" || p.Current || p.Snapshot != nil {
		t.Fatal("draft counted as issued", p)
	}
	order.ClientApproval = ordersApprovedClient(order, now)
	released, e := OrdersApply(&order, actor, access, OrdersCommand{Version: order.Version, Action: "release", Reason: "Actual authorized release"}, source, now, ordersUid(8), "PO-1")
	if e != nil {
		t.Fatal(e)
	}
	p = OrdersProgressFact(&released, source.ProjectID, source.PackageID)
	if p.State != "completed" || !p.Current || p.Snapshot == nil {
		t.Fatal(p)
	}
	p.Snapshot.Details.IssuerName = "changed"
	if released.Details.IssuerName != "Issuer" {
		t.Fatal("aliased order snapshot")
	}
	released.ReleasedAt = nil
	if p = OrdersProgressFact(&released, source.ProjectID, source.PackageID); p.State != "blocked" || p.Current {
		t.Fatal("missing issued evidence credited", p)
	}
}
