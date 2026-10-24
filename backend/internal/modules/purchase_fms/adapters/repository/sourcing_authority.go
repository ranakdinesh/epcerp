package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type sourcingAuthorityReader struct {
	c database.Connection
	p SourcingProviders
}

func NewSourcingAuthorityReader(c database.Connection, p SourcingProviders) contracts.SourcingAuthorityReader {
	return &sourcingAuthorityReader{c, p}
}
func (r *sourcingAuthorityReader) CurrentReleased(ctx context.Context, p, id string) (*contracts.SourcingReleased, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("Current LOI authority requires the consuming transaction")
	}
	for _, v := range []string{p, id} {
		if e := domain.SourcingID(v); e != nil {
			return nil, e
		}
	}
	record, e := (&sourcingRepository{q: NewStore(r.c)}).Get(ctx, p, id)
	if e != nil || record == nil {
		return nil, e
	}
	if !record.Active || record.State != "released" {
		return nil, nil
	}
	c := NewSourcingContext(r.p)
	source, e := c.Source(ctx, p, id)
	if e != nil {
		return nil, e
	}
	suppliers, e := c.Suppliers(ctx, p, id)
	if e != nil {
		return nil, e
	}
	var supplier *domain.SourcingSupplier
	for _, row := range suppliers {
		if record.Decision != nil && row.ID == record.Decision.Offer.SupplierID {
			v := row
			supplier = &v
		}
	}
	if record.Decision == nil {
		return nil, domain.SourcingInvalid("loi", "Released LOI has no approved commercial evidence")
	}
	proof, e := c.VerifySelection(ctx, p, id, record.Decision.Offer)
	if e != nil {
		return nil, e
	}
	return domain.SourcingCurrentReleasedFact(*record, source, supplier, proof, domain.SourcingToday((Values{}).Now()))
}
