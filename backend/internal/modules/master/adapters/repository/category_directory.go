package repository

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v5"
	"github.com/ranakdinesh/epcERP/internal/infrastructure/database"
	db "github.com/ranakdinesh/epcERP/internal/modules/master/adapters/repository/sqlc"
	masterContracts "github.com/ranakdinesh/epcERP/internal/modules/master/contracts"
)

type CategoryDirectory struct{ q *db.Queries }

func NewCategoryDirectory(c database.Connection) *CategoryDirectory {
	return &CategoryDirectory{NewStore(c).Queries}
}
func (d *CategoryDirectory) FindSubcategory(ctx context.Context, code string) (masterContracts.CategorySubcategory, bool, error) {
	id, e := d.q.CategoriesFindSubcategoryByCode(ctx, code)
	if errors.Is(e, pgx.ErrNoRows) {
		return masterContracts.CategorySubcategory{}, false, nil
	}
	if e != nil {
		return masterContracts.CategorySubcategory{}, false, e
	}
	return masterContracts.CategorySubcategory{ID: id.ID.String(), Active: id.Active}, true, nil
}

func (d *CategoryDirectory) FindEligibility(ctx context.Context, codes []string) ([]masterContracts.CategoryEligibilityChoice, error) {
	rows, e := d.q.CategoriesFindEligibility(ctx, codes)
	out := []masterContracts.CategoryEligibilityChoice{}
	for _, v := range rows {
		out = append(out, masterContracts.CategoryEligibilityChoice{Code: v.Code, Name: v.Name, Kind: v.Kind, ParentName: v.ParentName, ParentCode: v.ParentCode, Active: v.Active})
	}
	return out, e
}
func (d *CategoryDirectory) BrowseEligibility(ctx context.Context, search, kind, category string, skip int32) ([]masterContracts.CategoryEligibilityChoice, int64, error) {
	rows, e := d.q.CategoriesBrowseEligibility(ctx, db.CategoriesBrowseEligibilityParams{Search: search, Kind: kind, CategoryCode: category, Skip: skip})
	if e != nil {
		return nil, 0, e
	}
	total, e := d.q.CategoriesCountEligibility(ctx, db.CategoriesCountEligibilityParams{Search: search, Kind: kind, CategoryCode: category})
	out := []masterContracts.CategoryEligibilityChoice{}
	for _, v := range rows {
		out = append(out, masterContracts.CategoryEligibilityChoice{Code: v.Code, Name: v.Name, Kind: v.Kind, ParentName: v.ParentName, ParentCode: v.ParentCode, Active: v.Active})
	}
	return out, total, e
}
