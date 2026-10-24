package ports

import (
	"context"
)

// DatabaseProbe verifies the configured database can execute a query.
type DatabaseProbe interface {
	Check(context.Context) error
}
