package application

import (
	"context"

	masterDomain "github.com/ranakdinesh/epcERP/internal/modules/master/domain"
	masterPorts "github.com/ranakdinesh/epcERP/internal/modules/master/ports"
)

type ClientDirectory struct {
	reader masterPorts.ClientIdentityReader
}

func NewClientDirectory(r masterPorts.ClientIdentityReader) *ClientDirectory {
	return &ClientDirectory{r}
}
func (d *ClientDirectory) Get(ctx context.Context, id string) (*masterDomain.ClientDirectoryEntry, error) {
	return d.reader.Identity(ctx, id)
}
func (d *ClientDirectory) List(ctx context.Context) ([]masterDomain.ClientDirectoryEntry, error) {
	return d.reader.Identities(ctx)
}
