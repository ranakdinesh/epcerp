package repository

import (
	"context"
	"errors"
	masterDTO "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/dtos"

	pgx "github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

func categoryOptionalID(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return id.String()
}
func (r *categoryRepository) Browse(ctx context.Context, q masterDomain.CategoryBrowse) (masterDomain.CategoryPage, error) {
	out := masterDomain.CategoryPage{Rows: []masterDomain.CategorySummary{}, Page: q.Page, Size: q.Size}
	total, e := r.q.CategoriesCountBrowse(ctx, db.CategoriesCountBrowseParams{Kind: q.Kind, CategoryID: masterDTO.UUID(q.CategoryID), Search: q.Search, Status: q.Status})
	if e != nil {
		return out, e
	}
	out.Total = total
	rows, e := r.q.CategoriesBrowseRecords(ctx, db.CategoriesBrowseRecordsParams{Kind: q.Kind, CategoryID: masterDTO.UUID(q.CategoryID), Search: q.Search, Status: q.Status, Sort: q.Sort, Direction: q.Direction, PageSize: q.Size, PageOffset: (q.Page - 1) * q.Size})
	if e != nil {
		return out, e
	}
	for _, v := range rows {
		out.Rows = append(out.Rows, masterDomain.CategorySummary{ID: v.ID.String(), Code: v.Code, Name: v.Name, Active: v.IsActive, Version: v.Version, CategoryID: categoryOptionalID(v.CategoryID), UnitID: categoryOptionalID(v.UnitID), UnitCode: v.UnitCode, UnitName: v.UnitName, ChildCount: v.ChildCount})
	}
	return out, nil
}
func (r *categoryRepository) Detail(ctx context.Context, kind, id string) (masterDomain.CategorySummary, error) {
	v, e := r.q.CategoriesCatalogueDetail(ctx, db.CategoriesCatalogueDetailParams{Kind: kind, Identity: masterDTO.UUID(id)})
	if errors.Is(e, pgx.ErrNoRows) {
		return masterDomain.CategorySummary{}, &masterDomain.Error{Kind: masterDomain.NotFound, Detail: "Catalogue record not found"}
	}
	if e != nil {
		return masterDomain.CategorySummary{}, e
	}
	out := masterDomain.CategorySummary{ID: v.ID.String(), Code: v.Code, Name: v.Name, Active: v.IsActive, Version: v.Version, CategoryID: categoryOptionalID(v.CategoryID), UnitID: categoryOptionalID(v.UnitID), UnitCode: v.UnitCode, UnitName: v.UnitName, ChildCount: v.ChildCount}
	if len(v.Configuration) > 0 {
		var c masterDomain.CategoryConfiguration
		if e = masterDTO.UnmarshalMasterJSON(v.Configuration, &c); e != nil {
			return out, e
		}
		out.Configuration = &c
	}
	return out, nil
}
