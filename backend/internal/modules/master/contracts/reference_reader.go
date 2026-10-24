package contracts

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ReferenceReader interface {
	Get(context.Context, string, string) (*masterDomain.Reference, error)
}
