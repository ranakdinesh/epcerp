package contracts

import (
	"context"
)

type CategoryItemContext struct {
	ID, Code, Name, CategoryID, CategoryCode, CategoryName, UnitCode, FamilyCode string
	Active                                                                       bool
}
type CategoryItemReader interface {
	ItemContexts(context.Context) ([]CategoryItemContext, error)
	GetItemContext(context.Context, string) (CategoryItemContext, error)
}
