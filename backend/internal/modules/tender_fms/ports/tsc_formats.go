package ports

import (
	"context"
	"github.com/ranakdinesh/epcERP/internal/modules/tender_fms/domain"
)

type TSCFormatRepository interface {
	Authorize(context.Context, string, bool) error
	List(context.Context, bool) ([]domain.TSCFormat, error)
	Get(context.Context, string) (domain.TSCFormat, error)
	Save(context.Context, string, string, domain.TSCFormatInput) (domain.TSCFormat, error)
}
type TSCFormatUnit interface {
	Within(context.Context, func(context.Context, TSCFormatRepository) error) error
}

type TSCFormatPreviewer interface {
	Render(context.Context, domain.TSCFormat, bool) ([]byte, error)
}
