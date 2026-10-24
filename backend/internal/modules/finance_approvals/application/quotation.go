package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_approvals/ports"
)

func ReviewQuotation(ctx context.Context, r ports.QuotationRepository, a ports.QuotationAuthority, n ports.Notices, v ports.Values, actor domain.Actor, input domain.QuotationRequest) (string, error) {
	if !actor.Active || actor.ID == "" {
		return "", domain.Unauthorized
	}
	for _, code := range []string{"accounts.quotation_approvals.view", "accounts.quotation_approvals.read", "accounts.quotation_approvals.write"} {
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
	if e = a.ReviewQuotation(ctx, actor, q.ProjectID); e != nil {
		return "", e
	}
	d := domain.QuotationDecision{QuotationRequest: q, ID: v.ID(), ActorID: actor.ID, CreatedAt: v.Now()}
	if e = r.RecordQuotation(ctx, d); e != nil {
		return "", e
	}
	if e = n.Director(ctx, d.ID); e != nil {
		return "", e
	}
	return d.ID, nil
}
