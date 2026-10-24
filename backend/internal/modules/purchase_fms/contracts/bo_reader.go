package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type BORecord = domain.BORecord
type BOReader interface {
	List(context.Context, string) ([]BORecord, error)
	Get(context.Context, string, string) (*BORecord, error)
}
