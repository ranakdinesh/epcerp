package contracts

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type Item = masterDomain.Item
type Error = masterDomain.Error

// Internal, transaction-bound directory. Consumers authorize their own project.
// Approved rechecks the live configuration, unit and manufacturer eligibility.
type ItemReader interface {
	Get(context.Context, string) (Item, error)
	Approved(context.Context, string) (Item, error)
	List(context.Context) ([]Item, error)
}
