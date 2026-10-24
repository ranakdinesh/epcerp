package contracts

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/work_calendar/domain"
)

type Count = domain.Count
type Reader interface {
	Count(context.Context, string, string) (Count, error)
}
