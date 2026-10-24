package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ManufacturerRepository interface {
	Lock(context.Context) error
	List(context.Context) ([]masterDomain.Manufacturer, error)
	Get(context.Context, string, bool) (*masterDomain.Manufacturer, error)
	Create(context.Context, masterDomain.Manufacturer) (masterDomain.Manufacturer, error)
	Update(context.Context, masterDomain.Manufacturer, bool) (masterDomain.Manufacturer, error)
	Event(context.Context, masterDomain.ManufacturerEvent) error
	History(context.Context, string) ([]masterDomain.ManufacturerEvent, error)
	FindImport(context.Context, string) (*masterDomain.Manufacturer, string, error)
	RecordImport(context.Context, string, masterDomain.ManufacturerLegacy, string, string) error
}
type ManufacturerEligibility interface {
	Validate(context.Context, masterDomain.ManufacturerData) error
	Options(context.Context, string, []string, int32, string, string) (masterDomain.ManufacturerSelection, error)
}
type ManufacturerUnitOfWork interface {
	Within(context.Context, func(context.Context, ManufacturerRepository, ManufacturerEligibility) error) error
}

type ManufacturerValues interface {
	ID() string
	Fingerprint(masterDomain.ManufacturerLegacy) string
}
