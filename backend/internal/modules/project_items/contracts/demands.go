package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/project_items/domain"
)

type Record = domain.Record
type Save = domain.Save

// BO completion consumes this on its current transaction after authorization.
type DemandPort interface {
	List(context.Context, string) ([]Record, error)
	Get(context.Context, string, string) (*Record, error)
	Save(context.Context, string, string, Save, string) (Record, error)
}

type Error = domain.Error
