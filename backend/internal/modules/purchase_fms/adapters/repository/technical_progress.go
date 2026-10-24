package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
	ports "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/ports"
)

type technicalProgressReader struct {
	c      database.Connection
	source ports.TechnicalContext
}

func NewTechnicalProgressReader(c database.Connection, source ports.TechnicalContext) contracts.TechnicalProgressReader {
	return &technicalProgressReader{c, source}
}
func (r *technicalProgressReader) Progress(ctx context.Context, project, pkg string, parentCurrent bool) (domain.TechnicalProgress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.TechnicalProgress{}, errors.New("Technical progress requires the consumer transaction")
	}
	for _, id := range []string{project, pkg} {
		if e := domain.TechnicalID(id); e != nil {
			return domain.TechnicalProgress{}, e
		}
	}
	repo := &technicalRepository{q: NewStore(r.c)}
	if !parentCurrent {
		record, e := repo.Get(ctx, project, pkg)
		if e != nil {
			return domain.TechnicalProgress{}, e
		}
		return domain.TechnicalProgressFacts(record, domain.TechnicalSource{}, nil, false), nil
	}
	if r.source == nil {
		return domain.TechnicalProgress{}, errors.New("Technical progress source is not configured")
	}
	// Current source obtains the project lock before reading the owned case/files.
	current, e := r.source.Source(ctx, project, pkg)
	if e != nil {
		return domain.TechnicalProgress{}, e
	}
	record, e := repo.Get(ctx, project, pkg)
	if e != nil {
		return domain.TechnicalProgress{}, e
	}
	if record == nil || !record.Active {
		return domain.TechnicalProgressFacts(nil, current, nil, parentCurrent), nil
	}
	files, e := repo.Files(ctx, record.ID)
	if e != nil {
		return domain.TechnicalProgress{}, e
	}
	return domain.TechnicalProgressFacts(record, current, files, parentCurrent), nil
}
