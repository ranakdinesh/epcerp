package domain

import (
	"testing"
)

func TestOrdersReleasedFactIsHistoricalImmutableEvidence(t *testing.T) {
	a, c, s, d, n := ordersFixture()
	draft, e := OrdersApply(nil, a, c, OrdersCommand{Action: "prepare", Reason: "Prepare", Details: &d}, s, n, ordersUid(9), "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = OrdersReleasedFact(&draft, s.ProjectID, s.PackageID); e == nil {
		t.Fatal("draft leaked")
	}
	if _, e = OrdersReleasedFact(nil, s.ProjectID, s.PackageID); e != OrdersMissing {
		t.Fatal("missing PO", e)
	}
	draft.ClientApproval = ordersApprovedClient(draft, n)
	released, e := OrdersApply(&draft, a, c, OrdersCommand{Version: 1, Action: "release", Reason: "Release"}, s, n, ordersUid(10), "PO/VTP/2026-27/000001")
	if e != nil {
		t.Fatal(e)
	}
	fact, e := OrdersReleasedFact(&released, s.ProjectID, s.PackageID)
	if e != nil {
		t.Fatal(e)
	}
	fact.Source.Total = "0"
	*fact.ReleasedAt = OrdersTimestamp{}
	if released.Source.Total != "145.69" || released.ReleasedAt.IsZero() {
		t.Fatal("released source aliased")
	}
	if _, e = OrdersReleasedFact(&released, ordersUid(20), s.PackageID); e != OrdersMissing {
		t.Fatal("foreign PO", e)
	}
	released.Active = false
	if _, e = OrdersReleasedFact(&released, s.ProjectID, s.PackageID); e != OrdersMissing {
		t.Fatal("inactive PO", e)
	}
}
