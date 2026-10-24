package contracts

import (
	"context"
)

// BundleDirectory is an internal, transaction-bound live assignment directory.
// It publishes no account secrets or role-name authorization rules.
type Bundle struct {
	ID          string
	Name        string
	Active      bool
	Permissions []string
}
type BundleDirectory interface {
	Get(context.Context, string) (*Bundle, error)
	List(context.Context) ([]Bundle, error)
	Member(context.Context, string, string) (bool, error)
	Assigned(context.Context, string) ([]Bundle, error)
}
