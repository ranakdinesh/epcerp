package application

import (
	"context"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *QuotationApprovalsService) ProjectStatus(ctx context.Context, token, p string) (out []domain.QuotationProjectGroup, e error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return nil, e
	}
	if !a.Active || a.ID == "" {
		return nil, domain.QuotationApprovalsUnauthorized
	}
	if _, e = domain.QuotationApprovalsID(p); e != nil {
		return nil, e
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, _ ports.QuotationApprovalsFinance) error {
		gate, ok := c.(ports.QuotationApprovalsProjectContext)
		if !ok {
			return domain.QuotationApprovalsForbidden
		}
		allowed, e := gate.ProjectRead(ctx, a, p)
		if e != nil {
			return e
		}
		if !allowed {
			return domain.QuotationApprovalsForbidden
		}
		repo, ok := r.(ports.QuotationApprovalsProjectRepository)
		if !ok {
			return domain.QuotationApprovalsMissing
		}
		records, e := repo.ProjectRecords(ctx, p)
		if e != nil {
			return e
		}
		out = []domain.QuotationProjectGroup{}
		for _, record := range records {
			source, e := c.Source(ctx, p, record.BOID)
			if e != nil {
				return e
			}
			_, latest, _ := domain.QuotationApprovalsEvaluate(record.Attempts, source)
			for _, g := range source.Groups {
				row, exists := latest[g.ID]
				if exists {
					out = append(out, domain.QuotationProjectGroup{BOID: record.BOID, GroupID: g.ID, Name: g.Name, State: row.EffectiveState, Message: row.Message, ReturnAction: row.ReturnAction})
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
