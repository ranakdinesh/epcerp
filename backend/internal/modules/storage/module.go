// Package storage exposes object-document storage as a composed module.
package storage

import (
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	auth "github.com/ranakdinesh/epcERP/internal/modules/auth/contracts"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/inbound"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/adapters/outbound"
	"github.com/ranakdinesh/epcERP/internal/modules/storage/application"
)

type Module struct{}

func New(router chi.Router, database *pgxpool.Pool, identity auth.Resolver) (*Module, error) {
	configuration, err := outbound.LoadConfig(os.LookupEnv)
	if err != nil {
		return nil, err
	}
	objects, err := outbound.NewObjects(configuration)
	if err != nil {
		return nil, err
	}
	cipher, err := outbound.CipherFromConfig(configuration)
	if err != nil {
		return nil, err
	}
	service := application.New(outbound.NewUnitOfWork(database), outbound.NewAuthBridge(identity), objects, cipher, outbound.SystemValues{}, configuration.Bucket)
	inbound.Register(router, service)
	return &Module{}, nil
}
