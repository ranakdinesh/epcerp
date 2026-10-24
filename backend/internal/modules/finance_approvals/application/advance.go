package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/ports"
)

func ReviewAdvance(ctx context.Context, r ports.AdvanceRepository, a ports.AdvanceAuthority, n ports.Notices, v ports.Values, actor domain.Actor, input domain.AdvanceRequest) (string, error) {
	if !actor.Active || actor.ID == "" {
		return "", domain.Unauthorized
	}
	for _, code := range []string{"accounts.purchase_advances.view", "accounts.purchase_advances.read", "accounts.purchase_advances.write"} {
		if !actor.Can(code) {
			return "", domain.Forbidden
		}
	}
	q, e := input.Validate()
	if e != nil {
		return "", e
	}
	if actor.ID == q.SubmittedBy {
		return "", domain.Forbidden
	}
	if e = a.ReviewAdvance(ctx, actor, q.ProjectID); e != nil {
		return "", e
	}
	decision := domain.AdvanceDecision{AdvanceRequest: q, ID: v.ID(), ActorID: actor.ID, CreatedAt: v.Now()}
	if e = r.RecordAdvance(ctx, decision); e != nil {
		return "", e
	}
	if e = n.Director(ctx, decision.ID); e != nil {
		return "", e
	}
	return decision.ID, nil
}
