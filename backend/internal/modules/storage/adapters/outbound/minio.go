package outbound

import (
	"github.com/ranakdinesh/epcERP/internal/infrastructure"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/domain"
)

type Objects = infrastructure.ObjectStore

func NewObjects(c Config) (*Objects, error) {
	return infrastructure.NewObjectStore(infrastructure.ObjectStoreConfig{Endpoint: c.Endpoint, AccessKey: c.AccessKey, SecretKey: c.SecretKey, Secure: c.Secure}, domain.MaximumDocumentBytes+28)
}
