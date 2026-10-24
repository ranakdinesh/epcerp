package contracts

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentsRecord = domain.IndentsRecord
type IndentsReader interface {
	List(context.Context, string) ([]IndentsRecord, error)
	Get(context.Context, string, string) (*IndentsRecord, error)
	Committed(context.Context, string, string) (string, error)
}
