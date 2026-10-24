package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"testing"
)

type advanceFixture struct {
	budgetFixture
	decision *domain.AdvanceDecision
}

func (f *advanceFixture) RecordAdvance(_ context.Context, d domain.AdvanceDecision) error {
	f.decision = &d
	return f.step("record")
}
func (f *advanceFixture) ReviewAdvance(context.Context, domain.Actor, string) error {
	return f.step("authority")
}
func advanceInput() (domain.Actor, domain.AdvanceRequest) {
	a := domain.Actor{ID: "11111111-1111-4111-8111-111111111111", Active: true, Permissions: []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read", "accounts.purchase_advances.write"}}
	q := domain.AdvanceRequest{ProjectID: a.ID, PackageID: a.ID, RequestID: a.ID, OrderID: a.ID, SubmittedBy: "22222222-2222-4222-8222-222222222222", Outcome: "approved", Version: 2, OrderVersion: 2, Kind: "payment", Amount: "100.00", Currency: "INR", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "Reviewed signed PO and actual advance request"}
	return a, q
}
func TestAdvanceDecisionIsIndependentAndDirectorNoticeMandatory(t *testing.T) {
	for _, outcome := range []string{"approved", "rejected"} {
		for _, fail := range []string{"", "authority", "record", "notice"} {
			a, q := advanceInput()
			q.Outcome = outcome
			f := &advanceFixture{budgetFixture: budgetFixture{fail: fail}}
			id, e := ReviewAdvance(t.Context(), f, f, f, f, a, q)
			if fail == "" {
				if e != nil || id == "" || len(f.calls) != 3 || f.decision.SubmittedBy != q.SubmittedBy || f.decision.Amount != q.Amount || f.decision.ActorID != a.ID {
					t.Fatal("lost exact Finance decision/notice", e)
				}
			} else if e == nil || id != "" {
				t.Fatal("failed action exposed decision", fail, e)
			}
		}
	}
	a, q := advanceInput()
	a.ID = q.SubmittedBy
	a.Superuser = true
	f := &advanceFixture{}
	if id, e := ReviewAdvance(t.Context(), f, f, f, f, a, q); e == nil || id != "" || len(f.calls) != 0 {
		t.Fatal("superadmin self-review bypass")
	}
}
func TestAdvanceReviewRequiresEveryFinancePermission(t *testing.T) {
	for i := range 3 {
		a, q := advanceInput()
		a.Permissions = append(a.Permissions[:i], a.Permissions[i+1:]...)
		f := &advanceFixture{}
		if id, e := ReviewAdvance(t.Context(), f, f, f, f, a, q); e == nil || id != "" || len(f.calls) != 0 {
			t.Fatal("missing Finance permission bypass")
		}
	}
	a, q := advanceInput()
	a.Active = false
	f := &advanceFixture{}
	if id, e := ReviewAdvance(t.Context(), f, f, f, f, a, q); e == nil || id != "" || len(f.calls) != 0 {
		t.Fatal("inactive reviewer bypass")
	}
}
