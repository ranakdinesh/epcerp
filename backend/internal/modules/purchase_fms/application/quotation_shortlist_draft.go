package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *QuotationApprovalsService) SaveDraft(ctx context.Context, token, p, b string, d domain.QuotationShortlistDraft) (out domain.QuotationApprovalsWorkspace, e error) {
	a, e := s.actor(ctx, token, p, b)
	if e != nil {
		return out, e
	}
	d.ProjectID = p
	d.BOID = b
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, f ports.QuotationApprovalsFinance) error {
		caps, e := c.Actions(ctx, a, p)
		if e != nil {
			return e
		}
		if !caps.Read || !caps.Submit {
			return domain.QuotationApprovalsForbidden
		}
		source, e := c.Source(ctx, p, b)
		if e != nil {
			return e
		}
		if !source.Available {
			return domain.QuotationApprovalsForbidden
		}
		record, e := r.Get(ctx, p, b)
		if e != nil {
			return e
		}
		attempts := []domain.QuotationApprovalsAttempt{}
		if record != nil {
			if !record.Active {
				return domain.QuotationApprovalsMissing
			}
			attempts = record.Attempts
		}
		d, e = domain.QuotationShortlistDraftValidate(d, source, attempts)
		if e != nil {
			return e
		}
		storage, ok := r.(ports.QuotationShortlistDraftRepository)
		if !ok {
			return domain.QuotationApprovalsInvalid("draft", "Draft storage unavailable")
		}
		if _, e = storage.SaveDraft(ctx, d, a.ID); e != nil {
			return e
		}
		out, e = quotationApprovalsWorkspace(ctx, r, c, a, p, b)
		return e
	})
	if e != nil {
		out = domain.QuotationApprovalsWorkspace{}
	}
	return
}
