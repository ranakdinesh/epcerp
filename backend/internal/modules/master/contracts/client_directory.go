package contracts

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
)

type ClientEntry = masterDomain.ClientDirectoryEntry
type ClientDirectory interface {
	Get(context.Context, string) (*ClientEntry, error)
	List(context.Context) ([]ClientEntry, error)
}
