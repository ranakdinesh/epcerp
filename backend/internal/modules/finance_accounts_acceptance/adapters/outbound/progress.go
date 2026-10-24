package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
)

type progressReader struct{ c database.Connection }

func NewProgressReader(c database.Connection) contracts.ProgressReader { return &progressReader{c} }
func (r *progressReader) Progress(ctx context.Context, p, pkg string, expected contracts.ProgressSource, upstreamCurrent bool) (domain.Progress, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return domain.Progress{}, errors.New("Evidence progress requires the consumer transaction")
	}
	for _, id := range []string{p, pkg} {
		if e := domain.ID(id); e != nil {
			return domain.Progress{}, e
		}
	}
	if expected.ProjectID != p || expected.PackageID != pkg {
		return domain.Progress{}, domain.Invalid("source", "Evidence reporting source must match project and package")
	}
	record, err := (&repository{q: db.New(r.c)}).Get(ctx, p, pkg)
	if err != nil {
		return domain.Progress{}, err
	}
	return domain.ProgressFact(record, expected, upstreamCurrent), nil
}
