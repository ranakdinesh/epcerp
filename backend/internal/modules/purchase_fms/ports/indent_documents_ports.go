package ports

import (
	"context"

	domain "github.com/ranakdinesh/epcERP/internal/modules/purchase_fms/domain"
)

type IndentDocumentsRepository interface {
	List(context.Context, string, string) ([]domain.IndentDocumentsDocument, error)
	Save(context.Context, domain.IndentDocumentsDocument, string) (domain.IndentDocumentsDocument, error)
	Event(context.Context, domain.IndentDocumentsEvent) error
	History(context.Context, string, string) ([]domain.IndentDocumentsEvent, error)
	Notify(context.Context, domain.IndentDocumentsDocument, []string, string) error
	Resolve(context.Context, string, string) error
	Notices(context.Context, string) ([]domain.IndentDocumentsNotice, error)
}
type IndentDocumentsContext interface {
	Scope(context.Context, domain.Principal, string, string, bool) (bool, error)
	Revisions(context.Context, string, string) (map[string]int32, error)
	Generate(context.Context, domain.Principal, string, string, domain.IndentDocumentsCommand) (domain.IndentDocumentsData, error)
	Dispatch(context.Context, domain.Principal, domain.IndentDocumentsDocument, domain.IndentDocumentsCommand) error
	Recipients(context.Context, string) (domain.IndentDocumentsRecipients, error)
}
type IndentDocumentsUnitOfWork interface {
	Within(context.Context, func(context.Context, IndentDocumentsRepository, IndentDocumentsContext) error) error
}
type IndentDocumentsAuthorizer interface {
	Resolve(context.Context, string) (domain.Principal, error)
}
type IndentDocumentsValues interface {
	ID() string
	Now() domain.IndentDocumentsTimestamp
}
