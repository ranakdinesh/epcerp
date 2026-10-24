package repository

import (
	"context"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
)

func (d *CategoryDirectory) ItemContexts(ctx context.Context) ([]masterContracts.CategoryItemContext, error) {
	rows, e := d.q.CategoriesItemContexts(ctx)
	out := []masterContracts.CategoryItemContext{}
	for _, v := range rows {
		out = append(out, masterContracts.CategoryItemContext{ID: v.ID.String(), Code: v.Code, Name: v.Name, CategoryID: v.CategoryID.String(), CategoryCode: v.CategoryCode, CategoryName: v.CategoryName, UnitCode: v.UnitCode, FamilyCode: v.FamilyCode, Active: v.Active})
	}
	return out, e
}

func (d *CategoryDirectory) GetItemContext(ctx context.Context, id string) (masterContracts.CategoryItemContext, error) {
	v, e := d.q.CategoriesGetItemContext(ctx, masterDTO.UUID(id))
	return masterContracts.CategoryItemContext{ID: v.ID.String(), Code: v.Code, Name: v.Name, CategoryID: v.CategoryID.String(), CategoryCode: v.CategoryCode, CategoryName: v.CategoryName, UnitCode: v.UnitCode, FamilyCode: v.FamilyCode, Active: v.Active}, e
}
