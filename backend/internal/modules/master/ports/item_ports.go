package ports

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ItemRepository interface {
	Lock(context.Context) error
	List(context.Context) ([]masterDomain.Item, error)
	Get(context.Context, string) (masterDomain.Item, error)
	Find(context.Context, string) (*masterDomain.Item, error)
	Candidates(context.Context, masterDomain.ItemIdentity) ([]masterDomain.Item, error)
	Next(context.Context, string) (int32, error)
	Create(context.Context, masterDomain.Item, string) (masterDomain.Item, error)
	Change(context.Context, masterDomain.Item, string) (masterDomain.Item, error)
	Event(context.Context, masterDomain.ItemEvent) error
	History(context.Context, string) ([]masterDomain.ItemEvent, error)
}
type ItemContext interface {
	References(context.Context) (masterDomain.ItemReferences, error)
	Configuration(context.Context, string) (masterDomain.ItemConfiguration, error)
	Manufacturer(context.Context, string) (masterDomain.ItemMake, error)
}
type ItemUnitOfWork interface {
	Within(context.Context, func(context.Context, ItemRepository, ItemContext) error) error
}

type ItemValues interface {
	ID() string
	NormalizeModel(string) string
	Keys(masterDomain.ItemIdentity) (string, string)
}
