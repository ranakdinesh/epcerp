package ports

import (
	"context"
	sharedIdentity "github.com/ranakdinesh/epcERP/internal/identity"
)

// Authorizer resolves live platform permissions for every Master entity service.
type Authorizer interface {
	Resolve(context.Context, string) (sharedIdentity.Principal, error)
}
