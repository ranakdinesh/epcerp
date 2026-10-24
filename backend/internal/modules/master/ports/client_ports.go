package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ClientRepository interface {
	Get(context.Context, string, bool) (*masterDomain.Client, error)
	List(context.Context, masterDomain.ClientFilter) (masterDomain.ClientPage, error)
	Save(context.Context, masterDomain.Client, bool) error
	Contacts(context.Context, string, []masterDomain.ClientContact, string, bool) error
}
type ClientDirectory interface {
	Get(context.Context, string, string) (*masterDomain.ClientReference, error)
}
type ClientUnitOfWork interface {
	Within(context.Context, func(context.Context, ClientRepository, ClientDirectory) error) error
}

type ClientValues interface{ ID() string }

type ClientIdentityReader interface {
	Identity(context.Context, string) (*masterDomain.ClientDirectoryEntry, error)
	Identities(context.Context) ([]masterDomain.ClientDirectoryEntry, error)
}
