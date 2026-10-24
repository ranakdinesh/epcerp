package contracts

import (
	"context"
)

type CategorySubcategory struct {
	ID     string
	Active bool
}
type CategoryDirectory interface {
	FindSubcategory(context.Context, string) (CategorySubcategory, bool, error)
}

type CategoryEligibilityChoice struct {
	Code, Name, Kind, ParentName, ParentCode string
	Active                                   bool
}
type CategoryEligibilityReader interface {
	FindEligibility(context.Context, []string) ([]CategoryEligibilityChoice, error)
	BrowseEligibility(context.Context, string, string, string, int32) ([]CategoryEligibilityChoice, int64, error)
}
