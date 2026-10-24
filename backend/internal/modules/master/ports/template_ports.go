package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type TemplateRepository interface {
	Lock(context.Context) error
	Get(context.Context, string) (masterDomain.Template, error)
	Insert(context.Context, masterDomain.Template, masterDomain.TemplateDefinition, string) error
	Publish(context.Context, masterDomain.Template, masterDomain.Template, masterDomain.TemplateDefinition, string, string) error
}
type TemplateDirectory interface {
	Resolve(context.Context, string) (masterDomain.TemplateSubcategory, error)
}
type TemplateUnitOfWork interface {
	Within(context.Context, func(context.Context, TemplateRepository, TemplateDirectory) error) error
}

type TemplateValues interface {
	ID() string
	Fingerprint(masterDomain.TemplateDefinition) string
	EngineeringKey(masterDomain.TemplateIdentity) string
}
