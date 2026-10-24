package domain

import (
	"fmt"
	"testing"
	"time"
)

func ordersUid(i int) string { return fmt.Sprintf("%08x-1111-4111-8111-111111111111", i) }
func ordersFixture() (Principal, OrdersAccess, OrdersSource, OrdersDetails, OrdersTimestamp) {
	s := OrdersSource{OrdersSourceFacts: OrdersSourceFacts{ProjectID: ordersUid(1), PackageID: ordersUid(2), ProjectName: "Actual project", SourcingID: ordersUid(3), SourcingVersion: 7, LOIReference: "LOI-real", LOIDate: "2026-10-10", FinancialReference: "finance:fingerprint", SupplierVerification: "verified-current", Supplier: OrdersSupplier{ID: ordersUid(4), Name: "Actual vendor", Address: "Actual vendor location"}, Technical: OrdersTechnical{CaseID: ordersUid(5), Version: 10, Revision: 1, Reference: "Client-letter", ApprovalDate: "2026-10-11", ApprovalFileID: ordersUid(6), QAPApproved: true, ConditionsClosed: true}, ItemID: ordersUid(7), SKU: "SKU-real", Description: "Reviewed cable", UOM: "metre", Quantity: "10", Currency: "INR", PricingMode: "unit_price", UnitPrice: "12.3456", Base: "123.46", Discount: "0.00", Tax: "22.23", Freight: "0.00", Other: "0.00", Total: "145.69", DeliveryDays: 21, PaymentTerms: "Reviewed payment terms", Warranty: "Reviewed warranty", LOITerms: "Actual agreed operational terms"}, Available: true, Blockers: []string{}}
	return Principal{ID: ordersUid(8), Active: true}, OrdersAccess{Read: true, Write: true, Release: true, Scoped: true, Work: true}, s, OrdersDetails{Date: "2026-10-11", IssuerName: "Actual issuer", IssuerAddress: "Registered office", IssuerEmail: "procurement@example.test", BillTo: "Actual invoice address", DeliveryAddress: "Actual project site"}, time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
}
func TestOrdersOrderPreservesExactCommercialSourcesAndFreezesReleasedDocument(t *testing.T) {
	a, c, s, d, now := ordersFixture()
	if e := s.Guard(); e != nil {
		t.Fatal(e)
	}
	draft, e := OrdersApply(nil, a, c, OrdersCommand{Action: "prepare", Reason: "Reviewed actual terms", Details: &d}, s, now, ordersUid(9), "")
	if e != nil {
		t.Fatal(e)
	}
	number, e := OrdersNumber(d.Date, 1)
	if e != nil || number != "PO/VTP/2026-27/000001" {
		t.Fatal(number, e)
	}
	draft.ClientApproval = ordersApprovedClient(draft, now)
	order, e := OrdersApply(&draft, a, c, OrdersCommand{Version: 1, Action: "release", Reason: "Commit approved full kit"}, s, now, ordersUid(10), number)
	if e != nil || order.State != "released" || order.Source.UnitPrice != "12.3456" || order.Source.Total != "145.69" || order.ReleasedBy != a.ID {
		t.Fatal(order, e)
	}
	if _, e = OrdersApply(&order, a, c, OrdersCommand{Version: 2, Action: "prepare", Reason: "Overwrite released order", Details: &d}, s, now, ordersUid(11), ""); e == nil {
		t.Fatal("released order changed")
	}
	clone := OrdersCopy(order)
	clone.Source.Blockers = append(clone.Source.Blockers, "changed")
	*clone.ReleasedAt = time.Time{}
	if len(order.Source.Blockers) != 0 || order.ReleasedAt.IsZero() {
		t.Fatal("snapshot aliases mutated order")
	}
}
func TestOrdersReleaseRejectsStaleAuthorityAndEachMandatoryGate(t *testing.T) {
	a, c, s, d, now := ordersFixture()
	draft, e := OrdersApply(nil, a, c, OrdersCommand{Action: "prepare", Reason: "Prepare", Details: &d}, s, now, ordersUid(9), "")
	if e != nil {
		t.Fatal(e)
	}
	changes := []func(*OrdersSource){func(v *OrdersSource) { v.FinancialReference = "new-finance" }, func(v *OrdersSource) { v.Technical.Version++ }, func(v *OrdersSource) { v.SupplierVerification = "new-vendor-documents" }, func(v *OrdersSource) { v.SourcingVersion++ }, func(v *OrdersSource) { v.Supplier.Address = "changed address" }, func(v *OrdersSource) { v.Available = false }, func(v *OrdersSource) { v.Blockers = []string{"held"} }, func(v *OrdersSource) { v.Technical.QAPApproved = false }, func(v *OrdersSource) { v.Technical.ConditionsClosed = false }, func(v *OrdersSource) { v.Total = "145.68" }, func(v *OrdersSource) { v.UnitPrice = "12.3455" }, func(v *OrdersSource) { v.Quantity = "11" }}
	for i, change := range changes {
		v := s
		change(&v)
		if _, e := OrdersApply(&draft, a, c, OrdersCommand{Version: 1, Action: "release", Reason: "Recheck gates"}, v, now, ordersUid(10), "PO-real"); e == nil {
			t.Fatal("accepted changed authority", i)
		}
	}
	for _, caps := range []OrdersAccess{{Read: true, Write: true, Release: true}, {Read: true, Write: true, Work: true}, {Read: true, Release: true, Work: true}, {Write: true, Release: true, Work: true}} {
		if _, e := OrdersApply(&draft, a, caps, OrdersCommand{Version: 1, Action: "release", Reason: "Try missing permission"}, s, now, ordersUid(10), "PO-real"); e != OrdersForbidden {
			t.Fatal("missing live authority", e)
		}
	}
	if _, e := OrdersApply(&draft, a, c, OrdersCommand{Version: 0, Action: "release", Reason: "Stale draft"}, s, now, ordersUid(10), "PO-real"); e != OrdersStale {
		t.Fatal(e)
	}
}
func TestOrdersDatesDirectTotalsAndFinancialYearBoundaries(t *testing.T) {
	for _, row := range []struct{ date, want string }{{"2026-03-31", "PO/VTP/2025-26/000001"}, {"2026-04-01", "PO/VTP/2026-27/000001"}} {
		n, e := OrdersNumber(row.date, 1)
		if e != nil || n != row.want {
			t.Fatal(n, e)
		}
	}
	if _, e := OrdersNumber("2026-02-30", 1); e == nil {
		t.Fatal("invalid date accepted")
	}
	a, c, s, d, now := ordersFixture()
	s.PricingMode = "line_total"
	s.UnitPrice = ""
	s.Base = "123.46"
	if e := s.Guard(); e != nil {
		t.Fatal(e)
	}
	if base, e := OrdersBasicAmount("unit_price", "12.3456", "", "10"); e != nil || base != "123.46" {
		t.Fatal(base, e)
	}
	for _, date := range []string{"2026-10-09", "2026-10-12", "2026-02-30"} {
		d.Date = date
		if _, e := OrdersApply(nil, a, c, OrdersCommand{Action: "prepare", Reason: "Try invalid date", Details: &d}, s, now, ordersUid(9), ""); e == nil {
			t.Fatal("invalid release chronology", date)
		}
	}
	s.UnitPrice = "12.346"
	if e := s.Guard(); e == nil {
		t.Fatal("invented direct-total rate accepted")
	}
}

func ordersApprovedClient(o OrdersOrder, now OrdersTimestamp) OrdersClientApproval {
	return OrdersClientApproval{Status: "approved", DraftRevision: o.DraftRevision, Submission: &OrdersClientSubmission{Reference: "SUB", Date: OrdersToday(now)}, Decision: &OrdersClientDecision{Status: "approved", Reference: "CLIENT-PO", Date: OrdersToday(now), Signatory: "Client reviewer", Designation: "Engineer", FileID: ordersUid(17)}, RecordedBy: ordersUid(8), RecordedAt: &now}
}
