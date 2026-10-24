package outbound

import (
	"context"
	"errors"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/adapters/outbound/internal/db"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/finance_accounts_acceptance/domain"
)

type acceptedReader struct {
	c database.Connection
	p Providers
}

func NewAcceptedReader(c database.Connection, p Providers) contracts.AcceptedReader {
	return &acceptedReader{c, p}
}
func (r *acceptedReader) Accepted(ctx context.Context, project, pkg string) (*contracts.Accepted, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Accounts acceptance requires the consuming transaction")
	}
	for _, v := range []string{project, pkg} {
		if e := domain.ID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&repository{q: db.New(r.c)}).Get(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	source, e := NewContext(r.p).Source(ctx, project, pkg)
	if e != nil {
		return nil, e
	}
	return domain.AcceptedFact(record, source)
}
