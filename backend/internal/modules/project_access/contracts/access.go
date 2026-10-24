package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_access/domain"
)

type Actor = domain.Actor
type Access = domain.Access
type Owner = domain.Owner
type Error = domain.Error

var ErrForbidden = domain.Forbidden
var ErrUnauthorized = domain.Unauthorized
var ErrMissing = domain.Missing

// Gate and owner projection are internal and bound to the consumer transaction.
// Consumers authorize their own capability before exposing data to a browser.
type Gate interface {
	Check(context.Context, Actor, string, string, bool) (Access, error)
	Owners(context.Context, string) ([]Owner, error)
}

// ReadableProjects publishes only active specialist assignments with live dashboard
// read authority. Consumers still require their own master-read permission.
type ReadableProjects interface {
	Readable(context.Context, Actor) ([]string, error)
}
