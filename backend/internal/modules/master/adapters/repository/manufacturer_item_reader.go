package repository

import (
	"context"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ManufacturerItemReader struct{ q *db.Queries }

func NewManufacturerItemReader(c database.Connection) *ManufacturerItemReader {
	return &ManufacturerItemReader{NewStore(c).Queries}
}
func manufacturerMakeView(v masterDomain.Manufacturer) (masterContracts.ManufacturerMake, error) {
	raw, e := masterDTO.MarshalMasterJSON(v.Data)
	if e != nil {
		return masterContracts.ManufacturerMake{}, e
	}
	var data masterDomain.ManufacturerData
	e = masterDTO.UnmarshalMasterJSON(raw, &data)
	return masterContracts.ManufacturerMake{ID: v.ID, Code: v.Code, Name: v.Name, Active: v.Active, Eligible: data.Eligible}, e
}
func (r *ManufacturerItemReader) GetMake(ctx context.Context, id string) (masterContracts.ManufacturerMake, error) {
	v, e := (&manufacturerRepository{q: r.q}).Get(ctx, id, true)
	if e != nil {
		return masterContracts.ManufacturerMake{}, e
	}
	if v == nil {
		return masterContracts.ManufacturerMake{}, nil
	}
	return manufacturerMakeView(*v)
}
func (r *ManufacturerItemReader) ListMakes(ctx context.Context) ([]masterContracts.ManufacturerMake, error) {
	rows, e := (&manufacturerRepository{q: r.q}).List(ctx)
	out := []masterContracts.ManufacturerMake{}
	for _, v := range rows {
		m, err := manufacturerMakeView(v)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, e
}
