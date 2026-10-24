package repository

import (
	"context"
	"errors"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	dtos "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/dtos"
	sqlc "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/adapters/repository/sqlc"
	contracts "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/contracts"
	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type bODemandReader struct{ c database.Connection }

func NewBODemandReader(c database.Connection) contracts.BODemandReader { return &bODemandReader{c} }
func (r *bODemandReader) Demand(ctx context.Context, p, id string) (*contracts.BODemandLink, error) {
	if !database.IsTransactionConnection(ctx, r.c) {
		return nil, errors.New("BO demand linkage requires the consuming transaction")
	}
	var e error
	p, e = domain.BOID(p)
	if e != nil {
		return nil, e
	}
	id, e = domain.BOID(id)
	if e != nil {
		return nil, e
	}
	rows, e := NewStore(r.c).BOExactDemandLink(ctx, sqlc.BOExactDemandLinkParams{ProjectID: dtos.UUID(p), DemandID: dtos.UUID(id)})
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, domain.BOInvalid("source", "Demand links to multiple active completed BO lines; reconcile source before sourcing")
	}
	v := rows[0]
	if !v.DemandID.Valid || !v.ItemID.Valid {
		return nil, domain.BOInvalid("source", "Completed BO line lacks published item/demand evidence")
	}
	return &domain.BODemandLink{BOID: v.BoID.String(), BOVersion: v.BoVersion, LineID: v.LineID.String(), DemandID: v.DemandID.String(), ItemID: v.ItemID.String(), Quantity: v.Quantity}, nil
}
