package application

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"testing"
)

type budgetFixture struct {
	fail     string
	calls    []string
	decision domain.Decision
}

func (f *budgetFixture) step(s string) error {
	f.calls = append(f.calls, s)
	if f.fail == s {
		return errors.New("fixture failure")
	}
	return nil
}
func (f *budgetFixture) Record(_ context.Context, d domain.Decision) error {
	f.decision = d
	return f.step("record")
}
func (f *budgetFixture) ApproveBudget(context.Context, domain.Actor, string) error {
	return f.step("authority")
}
func (f *budgetFixture) Director(context.Context, string) error { return f.step("notice") }
func (*budgetFixture) ID() string                               { return "11111111-1111-4111-8111-111111111111" }
func (*budgetFixture) Now() domain.Timestamp                    { return domain.Timestamp{} }
func request() domain.BudgetRequest {
	return domain.BudgetRequest{ProjectID: "22222222-2222-4222-8222-222222222222", Version: 3, Revision: 2, Amount: "123.45", Currency: "INR", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: " Saved approval "}
}
func TestBudgetDecisionRequiresAuthorityAndMandatoryNotice(t *testing.T) {
	actor := domain.Actor{ID: "33333333-3333-4333-8333-333333333333", Active: true, Permissions: []string{"projects.budgets.read", "projects.budgets.approve"}}
	for _, fail := range []string{"", "authority", "record", "notice"} {
		t.Run(fail, func(t *testing.T) {
			f := &budgetFixture{fail: fail}
			id, e := ApproveBudget(t.Context(), f, f, f, f, actor, request())
			if fail != "" {
				if e == nil || id != "" {
					t.Fatal("failed approval exposed a decision", id, e)
				}
				return
			}
			if e != nil || id != f.ID() || f.decision.Reason != "Saved approval" || f.decision.ActorID != actor.ID || f.decision.SourceVersion != 3 || f.decision.SourceRevision != 2 || len(f.calls) != 3 || f.calls[0] != "authority" || f.calls[1] != "record" || f.calls[2] != "notice" {
				t.Fatal("approval evidence/order", e)
			}
		})
	}
	for _, a := range []domain.Actor{{}, {ID: actor.ID, Active: true, Permissions: []string{"projects.budgets.read"}}} {
		f := &budgetFixture{}
		id, e := ApproveBudget(t.Context(), f, f, f, f, a, request())
		if e == nil || id != "" || len(f.calls) != 0 {
			t.Fatal("unauthorized decision reached persistence")
		}
	}
	q := request()
	q.Amount = "1e3"
	f := &budgetFixture{}
	if _, e := ApproveBudget(t.Context(), f, f, f, f, actor, q); e == nil || len(f.calls) != 0 {
		t.Fatal("invalid source reached persistence")
	}
}
