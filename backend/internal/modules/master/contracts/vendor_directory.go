package contracts

import (
	"context"
)

type VendorDirectory interface {
	Exists(context.Context, string) (bool, error)
	Snapshot(context.Context, string) (*VendorSnapshot, error)
}

type VendorSnapshot struct {
	ID            string
	Version       int32
	Active        bool
	GSTRegistered bool
}
