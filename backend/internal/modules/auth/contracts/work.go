package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/auth/domain"
)

type Responsibility = domain.Responsibility

// WorkSource is a read-only, source-owned inventory bound to the consumer's shared
// transaction. Consumers must authorize the employee/reporting scope before reads.
// Current returns live assignments for one immutable record reference; empty means
// the responsibility has been closed or revoked, never an automatic reassignment.
type WorkSource interface {
	Inventory(context.Context, string) ([]Responsibility, error)
	Current(context.Context, string, string) ([]Responsibility, error)
}
