package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/application"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/project_budgets/domain"
	projects "github.com/ranakdinesh/epcERP/internal/modules/projects/contracts"
)

type approvedReader struct {
	c        database.Connection
	projects projects.ContextReader
}

func NewApprovedReader(c database.Connection, p projects.ContextReader) contracts.ApprovedReader {
	return &approvedReader{c, p}
}
func (r *approvedReader) Approved(ctx context.Context, project string) (*domain.ApprovedSource, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Approved budget source requires the consuming transaction")
	}
	id, e := domain.ID(project)
	if e != nil {
		return nil, e
	}
	// Budget and purchasing decisions use this shared project lock; source evidence
	// cannot change between the consumer's checks and its decision/notice commit.
	p, e := r.projects.Get(ctx, id, true)
	if e != nil {
		return nil, e
	}
	if p == nil {
		return nil, domain.Missing
	}
	return application.ApprovedSource(ctx, &repository{q: db.New(r.c)}, id)
}
