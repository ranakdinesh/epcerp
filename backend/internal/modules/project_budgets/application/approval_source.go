package application

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/ports"
)

func ApprovedSource(ctx context.Context, r ports.Repository, project string) (*domain.ApprovedSource, error) {
	record, e := r.Get(ctx, project)
	if e != nil {
		return nil, e
	}
	return domain.EffectiveApprovedSource(record), nil
}
