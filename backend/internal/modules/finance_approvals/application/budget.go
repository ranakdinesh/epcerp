package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/ports"
)

// ApproveBudget is called on the authorized consumer's transaction. Python budget
// approval has no submission actor; preserve scoped approval permission here.
func ApproveBudget(ctx context.Context, r ports.Repository, a ports.Authority, n ports.Notices, v ports.Values, actor domain.Actor, input domain.BudgetRequest) (string, error) {
	if !actor.Active || actor.ID == "" {
		return "", domain.Unauthorized
	}
	if !actor.Can("projects.budgets.read") || !actor.Can("projects.budgets.approve") {
		return "", domain.Forbidden
	}
	q, e := input.Validate()
	if e != nil {
		return "", e
	}
	if e = a.ApproveBudget(ctx, actor, q.ProjectID); e != nil {
		return "", e
	}
	d := domain.Decision{ID: v.ID(), ProjectID: q.ProjectID, ActorID: actor.ID, SourceVersion: q.Version, SourceRevision: q.Revision, Amount: q.Amount, Currency: q.Currency, Fingerprint: q.Fingerprint, Reason: q.Reason, CreatedAt: v.Now()}
	if e = r.Record(ctx, d); e != nil {
		return "", e
	}
	if e = n.Director(ctx, d.ID); e != nil {
		return "", e
	}
	return d.ID, nil
}
