package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"testing"
)

type quotationFixture struct{ budgetFixture }

func (f *quotationFixture) RecordQuotation(_ context.Context, d domain.QuotationDecision) error {
	return f.step("record")
}
func (f *quotationFixture) ReviewQuotation(context.Context, domain.Actor, string) error {
	return f.step("authority")
}
func TestQuotationDecisionRequiresAnotherReviewerAndMandatoryNotice(t *testing.T) {
	a := domain.Actor{ID: "11111111-1111-4111-8111-111111111111", Active: true, Permissions: []string{"accounts.quotation_approvals.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.write"}}
	q := domain.QuotationRequest{ProjectID: a.ID, BOID: a.ID, AttemptID: a.ID, SubmittedBy: "22222222-2222-4222-8222-222222222222", Outcome: "approved", Version: 1, Amount: "0.00", Currency: "INR", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "Reviewed every item"}
	for _, fail := range []string{"", "authority", "record", "notice"} {
		f := &quotationFixture{budgetFixture: budgetFixture{fail: fail}}
		id, e := ReviewQuotation(t.Context(), f, f, f, f, a, q)
		if fail == "" {
			if e != nil || id == "" || len(f.calls) != 3 {
				t.Fatal("financial decision", e)
			}
		} else if e == nil || id != "" {
			t.Fatal("failure exposed decision", fail, e)
		}
	}
	q.SubmittedBy = a.ID
	a.Superuser = true
	f := &quotationFixture{}
	id, e := ReviewQuotation(t.Context(), f, f, f, f, a, q)
	if e == nil || id != "" || len(f.calls) != 0 {
		t.Fatal("recovery self-review bypass")
	}
}
