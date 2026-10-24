package contracts

import (
	"context"
)

type ManufacturerMake struct {
	ID, Code, Name string
	Active         bool
	Eligible       []string
}
type ManufacturerItemReader interface {
	GetMake(context.Context, string) (ManufacturerMake, error)
	ListMakes(context.Context) ([]ManufacturerMake, error)
}
