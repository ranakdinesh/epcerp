package application

import (
	"context"
	"errors"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

func (s *AdvancesService) Queue(ctx context.Context, token string, offset int32) (out domain.AdvancesQueuePage, e error) {
	a, e := s.a.Resolve(ctx, token)
	if e != nil {
		return out, e
	}
	if !a.Active || a.ID == "" {
		return out, domain.AdvancesUnauthorized
	}
	if !a.Can("accounts.purchase_advances.view") || !a.Can("accounts.purchase_advances.read") {
		return out, domain.AdvancesForbidden
	}
	if offset < 0 || offset > 1000000 {
		return out, domain.AdvancesInvalid("offset", "Offset must be between 0 and 1000000")
	}
	e = s.u.Within(ctx, func(ctx context.Context, r ports.AdvancesRepository, c ports.AdvancesContext, _ ports.AdvancesFinance) error {
		q, ok := r.(ports.AdvancesQueueRepository)
		if !ok {
			return domain.AdvancesInvalid("queue", "Advance request listing is unavailable")
		}
		labels, ok := c.(ports.AdvancesQueueContext)
		if !ok {
			return domain.AdvancesInvalid("queue", "Project labels are unavailable")
		}
		candidates, err := q.Queue(ctx, offset)
		if err != nil {
			return err
		}
		out.Rows = []domain.AdvancesQueueRow{}
		for _, v := range candidates {
			caps, err := labels.FinanceRead(ctx, a, v.ProjectID)
			var d *domain.AdvancesError
			if errors.As(err, &d) && (d.Status == 403 || d.Status == 404) {
				continue
			}
			if err != nil {
				return err
			}
			if !caps {
				continue
			}
			name, number, err := labels.ProjectLabel(ctx, v.ProjectID)
			if err != nil {
				return err
			}
			out.Rows = append(out.Rows, domain.AdvancesQueueRow{ID: v.ID, ProjectID: v.ProjectID, PackageID: v.PackageID, ProjectName: name, ProjectNumber: number, Number: v.Source.Number, RequestNo: v.Details.RequestNo, State: v.State, Amount: v.Amount, Currency: v.Source.Currency, UpdatedAt: v.UpdatedAt})
		}
		if len(candidates) == 50 && offset <= 999950 {
			next := offset + 50
			out.Next = &next
		}
		return nil
	})
	if e != nil {
		out = domain.AdvancesQueuePage{}
	}
	return
}
