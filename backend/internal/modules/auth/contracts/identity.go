// Package contracts publishes Auth's safe identity API for bridge adapters.
package contracts

import (
	"context"
)

// Identity contains no passwords, tokens, persistence types or role-name authority.
// Inactive or invalid credentials return the zero value with no infrastructure error.
type Identity struct {
	AccountNotReady   bool // Valid active credentials require a password change; grants remain inactive.
	ID                string
	Active, Superuser bool
	Permissions       []string
}
type Resolver interface {
	Resolve(context.Context, string) (Identity, error)
}
