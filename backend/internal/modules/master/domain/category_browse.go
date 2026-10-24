package domain

import (
	"regexp"
)

type CategoryBrowse struct {
	Kind, CategoryID, Search, Status, Sort, Direction string
	Page, Size                                        int32
}
type CategorySummary struct {
	ID            string
	Code          string
	Name          string
	Active        bool
	Version       int32
	CategoryID    string
	UnitID        string
	Configuration *CategoryConfiguration
	UnitCode      string
	UnitName      string
	ChildCount    int64
}
type CategoryPage struct {
	Rows  []CategorySummary
	Total int64
	Page  int32
	Size  int32
}

var categoryIdentityPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func CategoryValidateBrowse(q CategoryBrowse) error {
	if q.Kind != "category" && q.Kind != "subcategory" && q.Kind != "unit" {
		return CategoryInvalid("Invalid collection")
	}
	if q.Kind == "subcategory" && !categoryIdentityPattern.MatchString(q.CategoryID) {
		return CategoryInvalid("A category is required")
	}
	if q.Kind != "subcategory" && q.CategoryID != "" {
		return CategoryInvalid("Unexpected category filter")
	}
	if q.Page < 1 || q.Page > 1000000 || (q.Size != 20 && q.Size != 50 && q.Size != 100) {
		return CategoryInvalid("Invalid page or size")
	}
	if len(q.Search) > 128 || (q.Status != "" && q.Status != "Active" && q.Status != "Inactive") {
		return CategoryInvalid("Invalid search or status")
	}
	if q.Sort != "code" && q.Sort != "name" && q.Sort != "unit" && q.Sort != "status" && q.Sort != "types" {
		return CategoryInvalid("Invalid sort")
	}
	if q.Direction != "asc" && q.Direction != "desc" {
		return CategoryInvalid("Invalid direction")
	}
	return nil
}
func CategoryValidateDetail(kind, id string) error {
	if (kind != "category" && kind != "subcategory" && kind != "unit") || !categoryIdentityPattern.MatchString(id) {
		return CategoryInvalid("Invalid record identity")
	}
	return nil
}
