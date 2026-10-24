package application

import (
	"context"
	"errors"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *QuotationApprovalsService) Queue(ctx context.Context, token string, offset int32) (out domain.QuotationApprovalsQueuePage, e error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return out, e
	}
	if !a.Active || a.ID == "" {
		return out, domain.QuotationApprovalsUnauthorized
	}
	if !a.Can("accounts.quotation_approvals.view") || !a.Can("accounts.quotation_approvals.read") {
		return out, domain.QuotationApprovalsForbidden
	}
	if offset < 0 || offset > 1000000 {
		return out, domain.QuotationApprovalsInvalid("offset", "Offset must be between 0 and 1000000")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.QuotationApprovalsRepository, c ports.QuotationApprovalsContext, _ ports.QuotationApprovalsFinance) error {
		q, ok := r.(ports.QuotationApprovalsQueueRepository)
		if !ok {
			return domain.QuotationApprovalsInvalid("queue", "Quotation request listing is unavailable")
		}
		labels, ok := c.(ports.QuotationApprovalsQueueContext)
		if !ok {
			return domain.QuotationApprovalsInvalid("queue", "Project labels are unavailable")
		}
		candidates, err := q.Queue(ctx, offset)
		if err != nil {
			return err
		}
		out.Rows = []domain.QuotationApprovalsQueueRow{}
		for _, v := range candidates {
			caps, err := c.Actions(ctx, a, v.ProjectID)
			var d *domain.QuotationApprovalsError
			if errors.As(err, &d) && (d.Status == 403 || d.Status == 404) {
				continue
			}
			if err != nil {
				return err
			}
			if !caps.ReviewRead {
				continue
			}
			name, number, err := labels.ProjectLabel(ctx, v.ProjectID)
			if err != nil {
				return err
			}
			record, err := r.Get(ctx, v.ProjectID, v.BOID)
			if err != nil {
				return err
			}
			if record == nil || !record.Active {
				continue
			}
			latest := map[string]string{}
			for _, attempt := range record.Attempts {
				latest[attempt.GroupID] = attempt.ID
			}
			for _, attempt := range record.Attempts {
				if latest[attempt.GroupID] != attempt.ID {
					continue
				}
				out.Rows = append(out.Rows, domain.QuotationApprovalsQueueRow{ProjectID: v.ProjectID, ProjectName: name, ProjectNumber: number, BOID: v.BOID, Version: v.Version, UpdatedAt: v.UpdatedAt, AttemptID: attempt.ID, GroupID: attempt.GroupID, GroupName: attempt.GroupName, State: attempt.State, Amount: attempt.Total, Currency: attempt.Currency, SubmittedAt: attempt.SubmittedAt})
			}
		}
		if len(candidates) == 50 && offset <= 999950 {
			next := offset + 50
			out.Next = &next
		}
		return nil
	})
	if e != nil {
		out = domain.QuotationApprovalsQueuePage{}
	}
	return
}
