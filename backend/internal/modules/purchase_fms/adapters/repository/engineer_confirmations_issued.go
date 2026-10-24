package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type engineerConfirmationsConfirmedReader struct {
	c database.Connection
	p EngineerConfirmationsProviders
}

func NewEngineerConfirmationsConfirmedReader(c database.Connection, p EngineerConfirmationsProviders) contracts.EngineerConfirmationsConfirmedReader {
	return &engineerConfirmationsConfirmedReader{c, p}
}
func (r *engineerConfirmationsConfirmedReader) Confirmed(ctx context.Context, project, pkg string) (*contracts.EngineerConfirmationsConfirmed, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Engineer confirmation requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.EngineerConfirmationsID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&engineerConfirmationsRepository{q: NewStore(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewEngineerConfirmationsContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.EngineerConfirmationsConfirmedFact(record, source)
}
