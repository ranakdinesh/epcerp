package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type CategoryRepository interface {
	Browse(context.Context, masterDomain.CategoryBrowse) (masterDomain.CategoryPage, error)
	Detail(context.Context, string, string) (masterDomain.CategorySummary, error)
	Lock(context.Context) error
	List(context.Context) (masterDomain.CategoryCatalogue, error)
	Insert(context.Context, string, masterDomain.CategoryImportRecord) error
	Update(context.Context, string, masterDomain.CategoryCorrection) error
}
type CategoryUnitOfWork interface {
	Within(context.Context, func(context.Context, CategoryRepository) error) error
}

type CategoryValues interface {
	ID() string
	Fingerprint(masterDomain.CategorySource) string
}
