package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type VendorRepository interface {
	List(context.Context) ([]masterDomain.Vendor, error)
	Get(context.Context, string) (*masterDomain.Vendor, error)
	Update(context.Context, masterDomain.Vendor) (*masterDomain.Vendor, error)
	Changed(context.Context, masterDomain.Vendor, string, string, string) error
	BySource(context.Context, string) (*masterDomain.Vendor, error)
	Insert(context.Context, masterDomain.Vendor) (*masterDomain.Vendor, error)
	Imported(context.Context, string, string, string) error
}
type VendorUnitOfWork interface {
	Within(context.Context, func(context.Context, VendorRepository) error) error
}

type VendorValues interface {
	ID() string
	Fingerprint(map[string]any) string
}
