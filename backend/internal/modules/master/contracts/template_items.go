package contracts

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

// Template is a published immutable configuration revision.
type Template = masterDomain.Template
type TemplateItemReader interface {
	GetRevision(context.Context, string) (Template, error)
	ListLatest(context.Context) ([]Template, error)
}
