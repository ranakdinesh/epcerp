package domain

import (
	"testing"
	"time"
)

const advancesFirst = "11111111-1111-4111-8111-111111111111"
const advancesSecond = "22222222-2222-4222-8222-222222222222"
const advancesThird = "33333333-3333-4333-8333-333333333333"

func advancesFixture() (Principal, AdvancesAccess, AdvancesSource, AdvancesDetails, AdvancesTimestamp) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	return Principal{ID: advancesFirst, Active: true}, AdvancesAccess{Read: true, Write: true, Work: true, Review: true, Disburse: true, Scoped: true}, AdvancesSource{AdvancesSourceFacts: AdvancesSourceFacts{ProjectID: advancesFirst, PackageID: advancesSecond, OrderID: advancesThird, OrderVersion: 2, Number: "PO/VTP/2026-27/000001", Date: "2026-10-10", SupplierID: advancesThird, SupplierName: "Actual vendor", Currency: "INR", Total: "145.69", PaymentTerms: "Actual agreed advance terms", AcceptanceID: advancesThird, AcceptanceVersion: 1, SupplierVerification: "Actual current verification fingerprint", CalculationBasis: "po_total"}, Available: true}, AdvancesDetails{Kind: "payment", RequestNo: "ADV-actual", RequestDate: "2026-10-11", Percentage: "30", InvoiceReference: "Actual PI", InvoiceDate: "2026-10-09", BankReference: "Actual bank instruction reference"}, now
}
func TestAdvancesAdvanceExactPaymentLifecycleAndIndependentReview(t *testing.T) {
	a, c, s, d, n := advancesFixture()
	r, e := AdvancesApply(nil, a, c, AdvancesCommand{Action: "prepare", Reason: "Prepare actual advance", Details: &d}, s, n, advancesThird, "")
	if e != nil || r.Amount != "43.71" {
		t.Fatal("exact rounded amount", r.Amount, e)
	}
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit reviewed request"}, s, n, advancesThird, "")
	if e != nil || r.State != "requested" || r.SubmittedBy != a.ID || r.SubmittedVersion != 2 {
		t.Fatal(e)
	}
	a.Superuser = true
	if _, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 2, Action: "approve", Reason: "Self review"}, s, n, advancesThird, advancesThird); e != AdvancesForbidden {
		t.Fatal("self-review bypass", e)
	}
	a.ID = advancesSecond
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 2, Action: "approve", Reason: "Reviewed signed PO terms and PI"}, s, n, advancesThird, advancesThird)
	if e != nil || r.State != "approved" || r.Amount != "43.71" || r.ReviewedBy != a.ID {
		t.Fatal(e)
	}
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 3, Action: "disburse", Reason: "Record actual payment", Payment: &AdvancesPayment{Voucher: "Actual UTR", Date: "2026-10-11", Notes: "Actual payment evidence"}}, s, n, advancesThird, "")
	if e != nil || r.State != "disbursed" || r.Payment.Voucher != "Actual UTR" {
		t.Fatal(e)
	}
	if _, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 4, Action: "revise", Reason: "Overwrite payment"}, s, n, advancesThird, ""); e == nil {
		t.Fatal("disbursed payment mutable")
	}
	clone := AdvancesCopy(r)
	clone.Payment.Voucher = "forged"
	*clone.SubmittedAt = AdvancesTimestamp{}
	*clone.ReviewedAt = AdvancesTimestamp{}
	clone.Source.Blockers = append(clone.Source.Blockers, "forged")
	if r.Payment.Voucher != "Actual UTR" || r.SubmittedAt.IsZero() || r.ReviewedAt.IsZero() || len(r.Source.Blockers) > 0 {
		t.Fatal("snapshot aliased")
	}
}
func TestAdvancesNoAdvanceRequiresFinanceDecisionAndCannotBeDisbursed(t *testing.T) {
	a, c, s, d, n := advancesFixture()
	d = AdvancesDetails{Kind: "not_applicable", RequestNo: "NA-actual", RequestDate: "2026-10-11", Percentage: "0", Notes: "Signed PO does not require an advance"}
	r, e := AdvancesApply(nil, a, c, AdvancesCommand{Action: "prepare", Reason: "Prepare no-advance disposition", Details: &d}, s, n, advancesThird, "")
	if e != nil {
		t.Fatal(e)
	}
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 1, Action: "submit", Reason: "Request Finance confirmation"}, s, n, advancesThird, "")
	if e != nil || r.State != "requested" {
		t.Fatal(e)
	}
	a.ID = advancesSecond
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 2, Action: "approve", Reason: "Verified no advance in signed terms"}, s, n, advancesThird, advancesThird)
	if e != nil || r.State != "not_applicable" || r.Amount != "0.00" {
		t.Fatal(e)
	}
	if _, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 3, Action: "disburse", Reason: "Fake payment", Payment: &AdvancesPayment{Voucher: "forged", Date: "2026-10-11"}}, s, n, advancesThird, ""); e == nil {
		t.Fatal("waiver created payment")
	}
}
func TestAdvancesExactAdvanceMathRejectsAmbiguousOrExcessPercentage(t *testing.T) {
	for _, tc := range []struct{ total, pct, want string }{{"145.69", "30", "43.71"}, {"100", "12.35", "12.35"}, {"0.01", "50", "0.01"}, {"100", "0", "0.00"}, {"99999999999999.99", "100", "99999999999999.99"}} {
		v, e := AdvancesAmount(tc.total, tc.pct)
		if e != nil || v != tc.want {
			t.Fatal(tc, v, e)
		}
	}
	for _, bad := range []string{"101", "-1", "1e2", "10.001", "NaN", " 30 ", "01"} {
		if _, e := AdvancesAmount("145.69", bad); e == nil {
			t.Fatal("invalid percentage accepted", bad)
		}
	}
}
func TestAdvancesAdvanceSubmissionRequiresRealReferencesDatesAndCurrentSource(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*AdvancesDetails, *AdvancesSource, *AdvancesAccess)
	}{
		{"request reference", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.RequestNo = "" }},
		{"invoice reference", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.InvoiceReference = "" }},
		{"bank reference", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.BankReference = "" }},
		{"request before PO", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.RequestDate = "2026-10-09" }},
		{"future invoice", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.InvoiceDate = "2026-10-12" }},
		{"invoice after request", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) {
			d.InvoiceDate = "2026-10-11"
			d.RequestDate = "2026-10-10"
		}},
		{"zero payment", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { d.Percentage = "0" }},
		{"held project", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { s.Available = false }},
		{"conditional acceptance", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) {
			s.Blockers = []string{"Unresolved vendor exception"}
		}},
		{"no doer", func(d *AdvancesDetails, s *AdvancesSource, c *AdvancesAccess) { c.Work = false }},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, c, s, d, n := advancesFixture()
			test.edit(&d, &s, &c)
			draft := AdvancesRecord{ID: advancesThird, ProjectID: s.ProjectID, PackageID: s.PackageID, Version: 1, State: "draft", Source: s, Details: d, Active: true}
			if r, e := AdvancesApply(&draft, a, c, AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit"}, s, n, advancesThird, ""); e == nil || r.ID != "" {
				t.Fatal("invalid submission exposed")
			}
		})
	}
	a, c, s, d, n := advancesFixture()
	r, e := AdvancesApply(nil, a, c, AdvancesCommand{Action: "prepare", Reason: "Prepare", Details: &d}, s, n, advancesThird, "")
	if e != nil {
		t.Fatal(e)
	}
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 1, Action: "submit", Reason: "Submit"}, s, n, advancesThird, "")
	if e != nil {
		t.Fatal(e)
	}
	a.ID = advancesSecond
	s.AcceptanceVersion++
	if _, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 2, Action: "approve", Reason: "Obsolete acceptance"}, s, n, advancesThird, advancesThird); e != AdvancesStale {
		t.Fatal("changed source approved", e)
	}
	r, e = AdvancesApply(&r, a, c, AdvancesCommand{Version: 2, Action: "revise", Reason: "Refresh actual vendor acceptance"}, s, n, advancesThird, "")
	if e != nil || r.SubmittedBy != "" || r.DecisionID != "" || r.State != "draft" {
		t.Fatal("revision kept approval", e)
	}
}
