package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type technicalClearanceReader struct {
	c database.Connection
	p TechnicalProviders
}

func NewTechnicalClearanceReader(c database.Connection, p TechnicalProviders) contracts.TechnicalClearanceReader {
	return &technicalClearanceReader{c, p}
}
func (r *technicalClearanceReader) Clearance(ctx context.Context, p, id string) (*contracts.TechnicalClearance, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Technical clearance requires the consuming transaction")
	}
	for _, v := range []string{p, id} {
		if e := domain.TechnicalID(v); e != nil {
			return nil, e
		}
	}
	repo := &technicalRepository{q: NewStore(r.c)}
	record, e := repo.Get(ctx, p, id)
	if e != nil || record == nil {
		return nil, e
	}
	source, e := NewTechnicalContext(r.p).Source(ctx, p, id)
	if e != nil {
		return nil, e
	}
	files, e := repo.Files(ctx, record.ID)
	if e != nil {
		return nil, e
	}
	return domain.TechnicalClearanceFact(*record, source, files)
}
