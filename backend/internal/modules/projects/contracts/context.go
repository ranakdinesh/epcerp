package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/projects/domain"
)

type Context = domain.Project
type Filter = domain.Filter
type Page = domain.Page

// Internal consumer must authorise its own scope; this directory is not an HTTP API.
// Bound to the same shared transaction by composition. Never exposes project writes.
type ContextReader interface {
	Get(context.Context, string, bool) (*Context, error)
	Page(context.Context, Filter, string, []string, bool) (Page, error)
}

type CalendarUpdater interface {
	Recalculate(context.Context, string) (int, error)
}
